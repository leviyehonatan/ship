package deploy

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sort"
	"strings"

	"github.com/leviyehonatan/ship/internal/ssh"
)

type Deployer struct {
	tag    string
	env    EnvFile
	client *ssh.Client
}

func NewDeployer(tag string, envFile string, sshClient *ssh.Client) (*Deployer, error) {
	env, err := ParseEnvFile(envFile)
	if err != nil {
		env = make(EnvFile)
	}
	return &Deployer{
		tag:    tag,
		env:    env,
		client: sshClient,
	}, nil
}

func NewDeployerWithEnv(tag string, env map[string]string, sshClient *ssh.Client) *Deployer {
	ef := make(EnvFile)
	for k, v := range env {
		ef[k] = v
	}
	return &Deployer{
		tag:    tag,
		env:    ef,
		client: sshClient,
	}
}

func (d *Deployer) Build(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "docker", "build", "-t", d.tag+":latest", ".")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (d *Deployer) BuildWithArgs(ctx context.Context, args []string) error {
	buildArgs := []string{"build", "-t", d.tag + ":latest"}
	for _, a := range args {
		buildArgs = append(buildArgs, "--build-arg", a)
	}
	buildArgs = append(buildArgs, ".")
	cmd := exec.CommandContext(ctx, "docker", buildArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (d *Deployer) PushOverSSH() error {
	if d.client == nil {
		return fmt.Errorf("ssh client required")
	}

	// Start docker save — its stdout becomes stdin for remote docker load
	saveCmd := exec.Command("docker", "save", d.tag+":latest")
	saveCmd.Stderr = os.Stderr

	pr, pw := io.Pipe()
	saveCmd.Stdout = pw

	// Run docker save in background
	saveErr := make(chan error, 1)
	go func() {
		saveErr <- saveCmd.Run()
		pw.Close()
	}()

	// Stream over SSH to remote docker load
	if err := d.client.Stream("docker load", pr, os.Stdout); err != nil {
		return fmt.Errorf("push: %w", err)
	}

	// Check docker save status
	if err := <-saveErr; err != nil {
		return fmt.Errorf("docker save: %w", err)
	}

	return nil
}

func (d *Deployer) RunRemote(opts RunOpts) error {
	if d.client == nil {
		return fmt.Errorf("ssh client required")
	}

	envArgs := make([]string, 0, len(d.env)*2)
	for k, v := range d.env {
		envArgs = append(envArgs, "-e", fmt.Sprintf("%s=%s", k, v))
	}

	volumeArgs := make([]string, 0, len(opts.Volumes)*2)
	for _, v := range opts.Volumes {
		volumeArgs = append(volumeArgs, "-v", v)
	}

	portArgs := make([]string, 0, len(opts.Ports)*2)
	for _, p := range opts.Ports {
		portArgs = append(portArgs, "-p", p)
	}

	runCmd := fmt.Sprintf(
		"docker stop %s 2>/dev/null; docker rm %s 2>/dev/null; docker run -d --name %s --restart unless-stopped %s %s %s %s:latest",
		d.tag, d.tag, d.tag,
		strings.Join(envArgs, " "),
		strings.Join(volumeArgs, " "),
		strings.Join(portArgs, " "),
		d.tag,
	)

	_, err := d.client.Run(runCmd)
	return err
}

func (d *Deployer) Logs(w io.Writer, tail string) error {
	if d.client == nil {
		return fmt.Errorf("ssh client required")
	}
	out, err := d.client.Run(fmt.Sprintf("docker logs %s --tail %s", d.tag, tail))
	if err != nil {
		return err
	}
	fmt.Fprint(w, out)
	return nil
}

func (d *Deployer) Status() (string, error) {
	if d.client == nil {
		return "", fmt.Errorf("ssh client required")
	}
	out, err := d.client.Run(fmt.Sprintf("docker ps --filter name=%s --format '{{.Status}}'", d.tag))
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

type RunOpts struct {
	Ports   []string
	Volumes []string
}

type EnvFile map[string]string

func ParseEnvFile(path string) (EnvFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	env := make(EnvFile)
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if ok {
			env[k] = v
		}
	}
	return env, nil
}

func BuildArgsChecksum(args []string) string {
	sorted := make([]string, len(args))
	copy(sorted, args)
	sort.Strings(sorted)
	h := sha256.New()
	h.Write([]byte(strings.Join(sorted, "\n")))
	return hex.EncodeToString(h.Sum(nil))[:16]
}
