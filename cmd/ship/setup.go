package main

import (
	"fmt"

	"github.com/leviyehonatan/ship/internal/config"
	shipssh "github.com/leviyehonatan/ship/internal/ssh"
	"github.com/spf13/cobra"
)

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Install Docker and Caddy on a server",
	Long: `Runs the one-time setup on a VPS:
- Installs Docker
- Installs Caddy with auto-SSL
- Configures Docker log rotation
- Sets up a ship app directory

Use --server to specify a server name or IP. Names are resolved via
the provider (e.g. --server my-server looks up "my-server" in hcloud).`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load(config.DefaultPath())
		appName := "app"
		if err == nil {
			appName = cfg.App
		}

		serverFlag, _ := cmd.Flags().GetString("server")
		if serverFlag == "" && err == nil {
			serverFlag = cfg.Server
		}
		serverAddr := serverFlag

		if serverAddr == "" {
			return fmt.Errorf("server not set — use --server <name-or-ip> or set 'server' in ship.toml")
		}

		// Resolve name to IP
		ip, err := resolveServer("hetzner", serverAddr)
		if err != nil {
			return err
		}
		if ip != serverAddr {
			fmt.Fprintf(cmd.OutOrStdout(), "Resolved %s → %s\n", serverAddr, ip)
		}

		sshClient, err := shipssh.NewClientInsecure(ip, "root", "")
		if err != nil {
			return fmt.Errorf("ssh to %s: %w", serverAddr, err)
		}

		fmt.Fprintln(cmd.OutOrStdout(), "Setting up server...")

		// Install Docker (best effort — may already be available via mounted socket)
		fmt.Fprintln(cmd.OutOrStdout(), "  Installing Docker...")
		dockerInstalled, _ := sshClient.Run("which docker 2>/dev/null")
		if dockerInstalled == "" {
			sshClient.Run("apt-get update -qq && apt-get install -y -qq docker.io 2>/dev/null || apk add --no-cache docker 2>/dev/null || true")
		}
		if err != nil {
			return fmt.Errorf("installing docker: %w", err)
		}

		// Configure log rotation (best effort — doesn't block setup)
		fmt.Fprintln(cmd.OutOrStdout(), "  Configuring log rotation...")
		daemonJSON := `mkdir -p /etc/docker && cat > /etc/docker/daemon.json << 'EOF'
{
  "log-driver": "json-file",
  "log-opts": {
    "max-size": "10m",
    "max-file": "3"
  }
}
EOF
(systemctl restart docker 2>/dev/null || service docker restart 2>/dev/null) || true`
		sshClient.Run(daemonJSON) // best effort

		// Install Caddy (best effort — Alpine/Ubuntu compatible)
		fmt.Fprintln(cmd.OutOrStdout(), "  Installing Caddy...")
		caddyInstalled, _ := sshClient.Run("which caddy 2>/dev/null")
		if caddyInstalled == "" {
			sshClient.Run("(apt-get install -y -qq debian-keyring debian-archive-keyring apt-transport-https && curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/gpg.key' | gpg --dearmor -o /usr/share/keyrings/caddy-stable-archive-keyring.gpg && curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/debian.deb.txt' | tee /etc/apt/sources.list.d/caddy-stable.list && apt-get update -qq && apt-get install -y -qq caddy) 2>/dev/null || true")
		}

		// Create app directories
		mkdirCmd := fmt.Sprintf("mkdir -p /opt/ship/%s /opt/ship/data && chown -R 1000:1000 /opt/ship", appName)
		_, err = sshClient.Run(mkdirCmd)
		if err != nil {
			return fmt.Errorf("creating directories: %w", err)
		}

		fmt.Fprintln(cmd.OutOrStdout(), "✓ Server ready")
		sshClient.Run("touch /opt/ship/.setup-complete")
		fmt.Fprintln(cmd.OutOrStdout(), "  Next: ship deploy")
		return nil
	},
}
