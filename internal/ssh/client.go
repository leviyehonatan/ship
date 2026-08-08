package ssh

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/crypto/ssh"
)

type Client struct {
	addr string // host or host:port, already normalized
	user string
	cfg  *ssh.ClientConfig
}

func (c *Client) Addr() string { return c.addr }
func (c *Client) User() string { return c.user }

func NewClient(addr string, user string, keyPath string) (*Client, error) {
	addr = splitHostPort(addr)
	if user == "" {
		user = "root"
	}

	var authMethods []ssh.AuthMethod

	// Prefer SSH key if available
	if keyPath == "" {
		home, _ := os.UserHomeDir()
		keyPath = filepath.Join(home, ".ssh", "id_rsa")
	}
	key, err := os.ReadFile(keyPath)
	if err == nil {
		signer, err := ssh.ParsePrivateKey(key)
		if err == nil {
			authMethods = append(authMethods, ssh.PublicKeys(signer))
		}
	}

	// Fall back to password auth if SSHPASS is set
	if password := os.Getenv("SSHPASS"); password != "" {
		authMethods = append(authMethods, ssh.Password(password))
	}

	if len(authMethods) == 0 {
		return nil, fmt.Errorf("no SSH auth methods available — set SSHPASS or ensure ~/.ssh/id_rsa exists")
	}

	cfg := &ssh.ClientConfig{
		User:            user,
		Auth:            authMethods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	return &Client{addr: addr, user: user, cfg: cfg}, nil
}

func NewClientInsecure(addr string, user string, keyPath string) (*Client, error) {
	c, err := NewClient(addr, user, keyPath)
	if err != nil {
		return nil, err
	}
	c.cfg.HostKeyCallback = ssh.InsecureIgnoreHostKey()
	return c, nil
}

func (c *Client) Run(cmd string) (string, error) {
	client, err := ssh.Dial("tcp", c.addr, c.cfg)
	if err != nil {
		return "", fmt.Errorf("ssh dial %s: %w", c.addr, err)
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return "", fmt.Errorf("ssh session: %w", err)
	}
	defer session.Close()

	var stdout, stderr bytes.Buffer
	session.Stdout = &stdout
	session.Stderr = &stderr

	if err := session.Run(cmd); err != nil {
		return "", fmt.Errorf("ssh run: %w\nstderr: %s", err, stderr.String())
	}

	return stdout.String(), nil
}

func (c *Client) Stream(cmd string, stdin io.Reader, stdout io.Writer) error {
	client, err := ssh.Dial("tcp", c.addr, c.cfg)
	if err != nil {
		return fmt.Errorf("ssh dial %s: %w", c.addr, err)
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("ssh session: %w", err)
	}
	defer session.Close()

	session.Stdin = stdin
	session.Stdout = stdout
	session.Stderr = os.Stderr

	return session.Run(cmd)
}

func (c *Client) CopyFile(localPath string, remotePath string) error {
	data, err := os.ReadFile(localPath)
	if err != nil {
		return fmt.Errorf("reading %s: %w", localPath, err)
	}
	cmd := fmt.Sprintf("cat > %s", remotePath)
	return c.Stream(cmd, bytes.NewReader(data), io.Discard)
}

// splitHostPort normalizes addr to host:port format.
// "1.2.3.4" → "1.2.3.4:22"
// "1.2.3.4:2222" → "1.2.3.4:2222"
// "localhost:2222" → "localhost:2222"
// "my-server" → "my-server:22"
func splitHostPort(addr string) string {
	if strings.Contains(addr, ":") {
		host, port, err := net.SplitHostPort(addr)
		if err == nil {
			return net.JoinHostPort(host, port)
		}
	}
	return net.JoinHostPort(addr, "22")
}
