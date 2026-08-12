package e2e

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

var sshImage string
var shipBin string

func TestMain(m *testing.M) {
	if _, err := exec.LookPath("docker"); err != nil {
		os.Exit(0)
	}
	if err := exec.Command("docker", "info").Run(); err != nil {
		os.Exit(0)
	}

	root := absRoot()
	shipBin = root + "/ship-e2e"
	build := exec.Command("go", "build", "-o", shipBin, "github.com/leviyehonatan/ship/cmd/ship")
	build.Dir = root
	build.Stderr = os.Stderr
	if err := build.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "build ship: %v\n", err)
		os.Exit(1)
	}

	sshImage = "ship-e2e-ssh:latest"
	buildSSH := exec.Command("docker", "build", "-q", "-t", sshImage,
		"-f", "testdata/Dockerfile.ssh", "testdata")
	buildSSH.Stderr = os.Stderr
	buildSSH.Stdout = os.Stderr
	if err := buildSSH.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "build ssh image: %v\n", err)
		os.Exit(0) // skip gracefully, not fatal
	}

	os.Exit(m.Run())
}

func absRoot() string {
	cwd, _ := os.Getwd()
	for cwd != "/" {
		if _, err := os.Stat(cwd + "/go.mod"); err == nil {
			return cwd
		}
		cwd = cwd + "/.."
	}
	return "."
}

// ---- per-test SSH harness ----

type testEnv struct {
	t        *testing.T
	port     string // e.g. "12222"
	sshName  string // e.g. "ship-e2e-ssh-test1"
	appName  string // e.g. "ship-e2e-fullcycle"
	appPort  string // e.g. "9090"
	dir      string // temp dir
	sshCmd   func(args ...string) (string, error)
}

func newTestEnv(t *testing.T, suffix string) *testEnv {
	t.Helper()

	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("docker not installed")
	}
	if err := exec.Command("docker", "info").Run(); err != nil {
		t.Skip("docker daemon not running")
	}
	if _, err := exec.LookPath("sshpass"); err != nil {
		t.Skip("sshpass not installed")
	}
	if testing.Short() {
		t.Skip("short mode")
	}

	env := &testEnv{
		t:       t,
		port:    "12222",
		sshName: "ship-e2e-ssh-" + suffix,
		appName: "ship-e2e-" + suffix,
		appPort: "9090",
		dir:     t.TempDir(),
	}

	// Containers run inside the Docker VM (colima/Docker Desktop), so the
	// mount source is the VM's own socket, not the host-side CLI socket.
	dockerSock := "/var/run/docker.sock"

	// Remove any leftover container from previous crashed run
	exec.Command("docker", "rm", "-f", env.sshName, env.appName).Run()

	// Start SSH container
	run := exec.Command("docker", "run", "-d", "--rm",
		"--name", env.sshName,
		"-p", fmt.Sprintf("%s:22", env.port),
		"-v", fmt.Sprintf("%s:/var/run/docker.sock", dockerSock),
		sshImage,
	)
	run.Stderr = os.Stderr
	out, err := run.Output()
	if err != nil {
		t.Fatalf("start SSH container: %v", err)
	}
	containerID := strings.TrimSpace(string(out))

	t.Cleanup(func() {
		exec.Command("docker", "rm", "-f", containerID).Run()
		exec.Command("docker", "rm", "-f", env.appName).Run()
	})

	// SSH helper
	env.sshCmd = func(args ...string) (string, error) {
		all := []string{"-p", "root", "ssh",
			"-o", "StrictHostKeyChecking=no",
			"-o", "UserKnownHostsFile=/dev/null",
			"-p", env.port, "root@localhost",
		}
		all = append(all, args...)
		var stdout, stderr bytes.Buffer
		cmd := exec.Command("sshpass", all...)
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		err := cmd.Run()
		if err != nil {
			return "", fmt.Errorf("ssh: %w\nstderr: %s", err, stderr.String())
		}
		return strings.TrimSpace(stdout.String()), nil
	}

	// Wait for SSH
	time.Sleep(2 * time.Second)
	for i := 0; i < 15; i++ {
		if _, err := env.sshCmd("echo ready"); err == nil {
			break
		}
		if i == 14 {
			t.Fatalf("SSH container didn't become ready")
		}
		time.Sleep(2 * time.Second)
	}

	t.Logf("SSH ready on port %s, app=%s", env.port, env.appName)
	return env
}

func (env *testEnv) runShip(args ...string) (string, error) {
	all := append([]string{}, args...)
	cmd := exec.Command(shipBin, all...)
	cmd.Dir = env.dir
	cmd.Env = append(os.Environ(), "SSHPASS=root")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.String() + stderr.String(), err
}

func (env *testEnv) runShipOut(args ...string) {
	cmd := exec.Command(shipBin, args...)
	cmd.Dir = env.dir
	cmd.Env = append(os.Environ(), "SSHPASS=root")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Run()
}

func (env *testEnv) writeFile(name, content string) {
	os.WriteFile(env.dir+"/"+name, []byte(content), 0644)
}

func (env *testEnv) writeShipTOML() {
	toml := fmt.Sprintf(`app = "%s"
server = "localhost:%s"

[build]
dockerfile = "Dockerfile"

[deploy]
port = %s
health_check = "/"
`, env.appName, env.port, env.appPort)
	env.writeFile("ship.toml", toml)
}

func (env *testEnv) writeDockerfile() {
	env.writeFile("Dockerfile", `FROM alpine:3.21
COPY start.sh /start.sh
RUN chmod +x /start.sh
EXPOSE `+env.appPort+`
CMD ["/start.sh"]
`)
	env.writeFile("start.sh", `#!/bin/sh
echo "=== ENV START ==="
env | sort
echo "=== ENV END ==="
while true; do sleep 60; done
`)
}
