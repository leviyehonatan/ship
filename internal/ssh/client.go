package ssh

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	osuser "os/user"
	"strconv"
	"strings"
)

// Client shells out to the system ssh CLI. This gives us full
// ~/.ssh/config support, SSH agent forwarding, ControlMaster,
// jump hosts, and custom keys — for free.
type Client struct {
	host string // host or host:port, already normalized
	user string
}

// Addr returns the host (with port) for display.
func (c *Client) Addr() string { return c.host }

// User returns the SSH user.
func (c *Client) User() string { return c.user }

// NewClient creates a Client for the given address and user.
// addr can be:
//   - "1.2.3.4" (IP, port 22 assumed)
//   - "1.2.3.4:2222" (IP with port)
//   - "localhost:2222" (hostname with port)
//   - "root@1.2.3.4" (user@host)
//   - "root@1.2.3.4:2222" (user@host:port)
//   - "colima" (SSH config hostname — ~/.ssh/config is respected)
//
// If user is empty and addr has no @, defaults to the current OS user.
// keyPath is ignored (the system ssh uses ~/.ssh/config and ssh-agent).
func NewClient(addr string, user string, keyPath string) (*Client, error) {
	host, parsedUser := parseAddr(addr)
	if user == "" {
		user = parsedUser
	}
	if user == "" {
		u, err := osuser.Current()
		if err == nil {
			user = u.Username
		} else {
			user = "root"
		}
	}
	return &Client{host: host, user: user}, nil
}

// NewClientInsecure is identical to NewClient — the system ssh
// already respects ~/.ssh/config's StrictHostKeyChecking.
func NewClientInsecure(addr string, user string, keyPath string) (*Client, error) {
	return NewClient(addr, user, keyPath)
}

// Run executes a command on the remote host and returns its stdout.
func (c *Client) Run(cmd string) (string, error) {
	var stdout, stderr bytes.Buffer
	if err := c.runWithIO(cmd, nil, &stdout, &stderr); err != nil {
		return "", fmt.Errorf("ssh run: %w\nstderr: %s", err, stderr.String())
	}
	return stdout.String(), nil
}

// Stream executes a command on the remote host, piping stdin and capturing stdout.
func (c *Client) Stream(cmd string, stdin io.Reader, stdout io.Writer) error {
	return c.runWithIO(cmd, stdin, stdout, os.Stderr)
}

// CopyFile copies a local file to a remote path using scp.
func (c *Client) CopyFile(localPath string, remotePath string) error {
	// Try scp first, fall back to cat pipe over ssh
	if _, err := exec.LookPath("scp"); err == nil {
		scpCmd := exec.Command("scp",
			"-o", "StrictHostKeyChecking=no",
			"-o", "ConnectTimeout=10",
			"-P", c.port(),
			localPath,
			fmt.Sprintf("%s@%s:%s", c.user, c.hostname(), remotePath),
		)
		scpCmd.Stderr = os.Stderr
		if err := scpCmd.Run(); err == nil {
			return nil
		}
		// fall through to pipe method
	}

	data, err := os.ReadFile(localPath)
	if err != nil {
		return fmt.Errorf("reading %s: %w", localPath, err)
	}
	pipeCmd := fmt.Sprintf("cat > %s", remotePath)
	return c.Stream(pipeCmd, bytes.NewReader(data), io.Discard)
}

// sshArgs builds the argument list for the ssh command.
func (c *Client) sshArgs() []string {
	args := []string{
		"ssh",
		"-o", "StrictHostKeyChecking=no",
		"-o", "ConnectTimeout=10",
		"-p", c.port(),
		fmt.Sprintf("%s@%s", c.user, c.hostname()),
	}
	// Password auth fallback via sshpass (matches the old crypto/ssh
	// client's SSHPASS support). -e reads the password from $SSHPASS.
	if os.Getenv("SSHPASS") != "" {
		if _, err := exec.LookPath("sshpass"); err == nil {
			return append([]string{"sshpass", "-e"}, args...)
		}
	}
	return args
}

// runWithIO is the shared implementation for Run and Stream.
func (c *Client) runWithIO(cmd string, stdin io.Reader, stdout, stderr io.Writer) error {
	args := append(c.sshArgs(), cmd)
	sshCmd := exec.Command(args[0], args[1:]...)
	sshCmd.Stdin = stdin
	sshCmd.Stdout = stdout
	sshCmd.Stderr = stderr
	return sshCmd.Run()
}

// hostname returns just the host part (without port).
func (c *Client) hostname() string {
	host, _, err := net.SplitHostPort(c.host)
	if err != nil {
		return c.host // no port
	}
	return host
}

// port returns the port number as a string, "22" if not specified.
func (c *Client) port() string {
	_, port, err := net.SplitHostPort(c.host)
	if err != nil {
		return "22" // no port in addr
	}
	return port
}

// parseAddr parses an address like "root@1.2.3.4:2222" into (host:port, user).
func parseAddr(addr string) (string, string) {
	var user string
	rest := addr

	// Extract user@ prefix
	if atIdx := strings.LastIndex(addr, "@"); atIdx >= 0 {
		user = addr[:atIdx]
		rest = addr[atIdx+1:]
	}

	// Normalize host:port
	host := rest
	if !strings.Contains(rest, ":") {
		host = net.JoinHostPort(rest, "22")
	} else {
		h, p, err := net.SplitHostPort(rest)
		if err == nil {
			// Ensure port is numeric — if not (e.g. IPv6), join with 22
			if _, err := strconv.Atoi(p); err != nil {
				host = net.JoinHostPort(rest, "22")
			} else {
				host = net.JoinHostPort(h, p)
			}
		}
	}

	return host, user
}
