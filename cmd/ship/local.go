package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
)

var localCmd = &cobra.Command{
	Use:   "local",
	Short: "Deploy to a local Docker SSH container for testing",
	Long: `Runs a local SSH container that acts as a fake VPS.
Useful for testing deployments before pushing to real infrastructure.`,
}

var localStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start a local test server",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Check if already running
		if out, _ := exec.Command("docker", "ps", "--filter", "name=ship-local", "--format", "{{.Status}}").Output(); len(out) > 0 {
			fmt.Fprintf(cmd.OutOrStdout(), "Local server already running: %s\n", strings.TrimSpace(string(out)))
			fmt.Fprintf(cmd.OutOrStdout(), "  ship use local\n")
			return nil
		}

		// Build the SSH image if needed
		if err := exec.Command("docker", "inspect", "ship-local-ssh").Run(); err != nil {
			fmt.Fprintln(cmd.OutOrStdout(), "Building local SSH image...")
			// Use a minimal Dockerfile inline
			df := `FROM alpine:3.21
RUN apk add --no-cache openssh docker-cli && ssh-keygen -A && \
    echo "PermitRootLogin yes" >> /etc/ssh/sshd_config && \
    echo "PasswordAuthentication yes" >> /etc/ssh/sshd_config && \
    echo "root:root" | chpasswd && mkdir /var/run/sshd
EXPOSE 22
CMD ["/usr/sbin/sshd", "-D", "-e"]
`
			build := exec.Command("docker", "build", "-t", "ship-local-ssh", "-")
			build.Stdin = strings.NewReader(df)
			build.Stderr = os.Stderr
			if err := build.Run(); err != nil {
				return fmt.Errorf("building local image: %w", err)
			}
		}

		exec.Command("docker", "rm", "-f", "ship-local").Run()

		dockerSock := "/var/run/docker.sock"
		if _, err := os.Stat(dockerSock); err != nil {
			dockerSock = os.ExpandEnv("$HOME/.docker/run/docker.sock")
		}

		fmt.Fprintln(cmd.OutOrStdout(), "Starting local server...")
		run := exec.Command("docker", "run", "-d", "--rm",
			"--name", "ship-local",
			"-p", "2222:22",
			"-v", fmt.Sprintf("%s:/var/run/docker.sock", dockerSock),
			"ship-local-ssh",
		)
		run.Stderr = os.Stderr
		out, err := run.Output()
		if err != nil {
			return fmt.Errorf("starting local server: %w", err)
		}
		containerID := strings.TrimSpace(string(out))

		fmt.Fprintf(cmd.OutOrStdout(), "✓ Local server running (%s)\n", containerID[:12])
		// Inject host SSH key for passwordless access
		home, _ := os.UserHomeDir()
		keyPath := home + "/.ssh/id_rsa"
		pubKeyPath := keyPath + ".pub"

		// Generate SSH key if none exists
		if _, err := os.Stat(keyPath); os.IsNotExist(err) {
			fmt.Fprintln(cmd.OutOrStdout(), "  No SSH key found — generating ship-specific key...")
			os.MkdirAll(home+"/.ssh", 0700)
			exec.Command("ssh-keygen", "-t", "ed25519", "-f", keyPath,
				"-N", "", "-C", "ship-local").Run()
		}

		if pubKey, err := os.ReadFile(pubKeyPath); err == nil {
			fmt.Fprintln(cmd.OutOrStdout(), "  Injecting SSH key...")
			exec.Command("docker", "exec", "ship-local", "mkdir", "-p", "/root/.ssh").Run()
			exec.Command("docker", "exec", "ship-local", "sh", "-c",
				"echo '"+strings.TrimSpace(string(pubKey))+"' >> /root/.ssh/authorized_keys").Run()
		}
		fmt.Fprintf(cmd.OutOrStdout(), "  ship use local\n")
		fmt.Fprintf(cmd.OutOrStdout(), "  ship setup\n")
		fmt.Fprintf(cmd.OutOrStdout(), "  ship deploy\n")
		return nil
	},
}

var localStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the local test server",
	RunE: func(cmd *cobra.Command, args []string) error {
		exec.Command("docker", "rm", "-f", "ship-local").Run()
		fmt.Fprintln(cmd.OutOrStdout(), "✓ Local server stopped")
		return nil
	},
}

var localSetupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Full local setup: start, init, deploy",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Start local
		exec.Command("docker", "rm", "-f", "ship-local").Run()
		dockerSock := "/var/run/docker.sock"
		if _, err := os.Stat(dockerSock); err != nil {
			dockerSock = os.ExpandEnv("$HOME/.docker/run/docker.sock")
		}
		run := exec.Command("docker", "run", "-d", "--rm",
			"--name", "ship-local",
			"-p", "2222:22",
			"-v", fmt.Sprintf("%s:/var/run/docker.sock", dockerSock),
			"ship-local-ssh",
		)
		run.Stderr = os.Stderr
		if out, err := run.Output(); err != nil {
			return fmt.Errorf("starting local: %w\n%s", err, out)
		}
		fmt.Fprintln(cmd.OutOrStdout(), "✓ Local server started")

		// Set as current
		exec.Command(cmd.CommandPath(), "use", "local").Run()

		fmt.Fprintln(cmd.OutOrStdout(), "  Next: ship setup && ship deploy")
		return nil
	},
}

func initLocal() {
	localCmd.AddCommand(localStartCmd)
	localCmd.AddCommand(localStopCmd)
	localCmd.AddCommand(localSetupCmd)
}
