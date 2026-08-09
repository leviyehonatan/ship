package deploy

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"syscall"

	"github.com/leviyehonatan/ship/internal/ssh"
)

type Deployer struct {
	tag    string
	env    EnvFile
	client *ssh.Client
	Stdout io.Writer
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

func (d *Deployer) stdout() io.Writer {
	if d.Stdout != nil {
		return d.Stdout
	}
	return os.Stdout
}

func (d *Deployer) Build(ctx context.Context) error {
	if err := checkDisk(); err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, "docker", "build", "-t", d.tag+":latest", ".")
	cmd.Stdout = d.stdout()
	cmd.Stderr = d.stdout()
	return cmd.Run()
}

func (d *Deployer) BuildWithArgs(ctx context.Context, args []string) error {
	if err := checkDisk(); err != nil {
		return err
	}
	buildArgs := []string{"build", "-t", d.tag + ":latest"}
	for _, a := range args {
		buildArgs = append(buildArgs, "--build-arg", a)
	}
	buildArgs = append(buildArgs, ".")
	cmd := exec.CommandContext(ctx, "docker", buildArgs...)
	cmd.Stdout = d.stdout()
	cmd.Stderr = d.stdout()
	return cmd.Run()
}

func (d *Deployer) PushOverSSH() error {
	if d.client == nil {
		return nil
	}

	// Start docker save — its stdout becomes stdin for remote docker load
	saveCmd := exec.Command("docker", "save", d.tag+":latest")
	saveCmd.Stderr = d.stdout()

	pr, pw := io.Pipe()
	saveCmd.Stdout = pw

	// Run docker save in background
	saveErr := make(chan error, 1)
	go func() {
		saveErr <- saveCmd.Run()
		pw.Close()
	}()

	// Stream over SSH to remote docker load
	if err := d.client.Stream("docker load", pr, d.stdout()); err != nil {
		return fmt.Errorf("push: %w", err)
	}

	// Check docker save status
	if err := <-saveErr; err != nil {
		return fmt.Errorf("docker save: %w", err)
	}

	return nil
}

func (d *Deployer) RunRemote(opts RunOpts) error {
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
		"docker stop %s 2>/dev/null; docker rm %s 2>/dev/null; docker run -d --name %s --restart unless-stopped %s %s %s %s %s:latest",
		d.tag, d.tag, d.tag,
		opts.NetworkArgs,
		strings.Join(envArgs, " "),
		strings.Join(volumeArgs, " "),
		strings.Join(portArgs, " "),
		d.tag,
	)

	if d.client == nil {
		if _, err := runLocal(runCmd); err != nil {
			return fmt.Errorf("run: %w", err)
		}
		return nil
	}

	_, err := d.client.Run(runCmd)
	return err
}

func runLocal(cmd string) (string, error) {
	run := exec.Command("sh", "-c", cmd)
	run.Stderr = os.Stderr
	out, err := run.Output()
	return string(out), err
}

func checkDisk() error {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(".", &stat); err != nil {
		return nil
	}
	free := stat.Bavail * uint64(stat.Bsize)
	minFree := uint64(2 * 1e9) // 2GB minimum for Docker build
	if free < minFree {
		return fmt.Errorf("low disk space: %dGB free (need %dGB for Docker build)",
			free/1e9, minFree/1e9)
	}
	return nil
}

func (d *Deployer) Logs(w io.Writer, tail string) error {
	cmd := fmt.Sprintf("docker logs %s --tail %s", d.tag, tail)
	if d.client == nil {
		out, err := runLocal(cmd)
		if err != nil {
			return err
		}
		fmt.Fprint(w, out)
		return nil
	}
	out, err := d.client.Run(cmd)
	if err != nil {
		return err
	}
	fmt.Fprint(w, out)
	return nil
}

func (d *Deployer) Status() (string, error) {
	cmd := fmt.Sprintf("docker ps --filter name=%s --format '{{.Status}}'", d.tag)
	if d.client == nil {
		out, err := runLocal(cmd)
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(out), nil
	}
	out, err := d.client.Run(cmd)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

func (d *Deployer) StopRemove() error {
	cmd := fmt.Sprintf("docker stop %s 2>/dev/null; docker rm %s 2>/dev/null", d.tag, d.tag)
	if d.client == nil {
		runLocal(cmd)
		return nil
	}
	d.client.Run(cmd)
	return nil
}

func (d *Deployer) StopRemoveSvc(svcName string) error {
	containerName := fmt.Sprintf("ship-svc-%s-%s", d.tag, svcName)
	cmd := fmt.Sprintf("docker stop %s 2>/dev/null; docker rm %s 2>/dev/null", containerName, containerName)
	if d.client == nil {
		runLocal(cmd)
		return nil
	}
	d.client.Run(cmd)
	return nil
}

type RunOpts struct {
	Ports       []string
	Volumes     []string
	NetworkArgs string
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
