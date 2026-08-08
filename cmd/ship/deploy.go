package main

import (
	"fmt"
	"os"

	"github.com/leviyehonatan/ship/internal/config"
	deploy "github.com/leviyehonatan/ship/internal/docker"
	"github.com/leviyehonatan/ship/internal/secrets"
	"github.com/leviyehonatan/ship/internal/snapshot"
	"github.com/leviyehonatan/ship/internal/ssl"
	shipssh "github.com/leviyehonatan/ship/internal/ssh"
	"github.com/spf13/cobra"
)

var deployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Build and deploy your app to a server",
	Long: `Full deployment pipeline:
  1. Snapshot database (safety net)
  2. Build Docker image
  3. Push image over SSH (no registry needed)
  4. Restart container
  5. Configure SSL (if domains set in ship.toml)

Reads ship.toml for configuration and .env for secrets.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load(config.DefaultPath())
		if err != nil {
			return fmt.Errorf("loading ship.toml: %w\n  run 'ship init' first", err)
		}

		serverFlag, _ := cmd.Flags().GetString("server")
		if serverFlag == "" {
			serverFlag = cfg.Server
		}
		if serverFlag == "" {
			return fmt.Errorf("--server is required (or set 'server' in ship.toml)")
		}

		ip, err := resolveServer("hetzner", serverFlag)
		if err != nil {
			return err
		}
		if ip != serverFlag {
			fmt.Fprintf(cmd.OutOrStdout(), "Resolved %s → %s\n", serverFlag, ip)
		}

		sshClient, err := shipssh.NewClientInsecure(ip, "root", "")
		if err != nil {
			return fmt.Errorf("ssh: %w", err)
		}

		// Merge ship.toml [env] with encrypted secrets
		envVars := make(map[string]string)
		for k, v := range cfg.Env {
			envVars[k] = v
		}
		// Read secrets directly from .env.encrypted (no plaintext .env needed)
		secretsPath := ".env.encrypted"
		if _, err := os.Stat(secretsPath); err == nil {
			secretsMap, err := secrets.ReadAll(secretsPath)
			if err == nil {
				for k, v := range secretsMap {
					envVars[k] = v
				}
			}
		}

		// Check server is set up (Docker must exist)
		installed, _ := sshClient.Run("which docker 2>/dev/null")
		if installed == "" {
			fmt.Fprintln(cmd.OutOrStdout(), "Docker not found on server.")
			fmt.Fprintf(cmd.OutOrStdout(), "run 'ship setup --server %s' to install it.\n", ip)
			return nil
		}

		// Snapshot before deploy
		mgr := snapshot.NewManager(sshClient, cfg.App)
		fmt.Fprintln(cmd.OutOrStdout(), "Snapshotting...")
		if err := mgr.Create(); err != nil {
			fmt.Fprintf(cmd.OutOrStdout(), "  Warning: snapshot failed: %v\n", err)
		}

		// Build (with build args from ship.toml if present)
		fmt.Fprintf(cmd.OutOrStdout(), "Building %s...\n", cfg.App)
		deployer := deploy.NewDeployerWithEnv(cfg.App, envVars, sshClient)
		if len(cfg.Build.Args) > 0 {
			if err := deployer.BuildWithArgs(cmd.Context(), cfg.Build.Args); err != nil {
				return fmt.Errorf("build: %w", err)
			}
		} else {
			if err := deployer.Build(cmd.Context()); err != nil {
				return fmt.Errorf("build: %w", err)
			}
		}

		// Push
		fmt.Fprintln(cmd.OutOrStdout(), "Pushing to server...")
		if err := deployer.PushOverSSH(); err != nil {
			return fmt.Errorf("push: %w", err)
		}

		// Run
		fmt.Fprintln(cmd.OutOrStdout(), "Starting container...")
		ports := []string{fmt.Sprintf("%d:%d", cfg.Deploy.Port, cfg.Deploy.Port)}
		var volumes []string
		for _, v := range cfg.Volumes {
			volumes = append(volumes, fmt.Sprintf("/opt/ship/data/%s:%s", cfg.App, v.Path))
		}

		if err := deployer.RunRemote(deploy.RunOpts{
			Ports:   ports,
			Volumes: volumes,
		}); err != nil {
			return fmt.Errorf("run: %w", err)
		}

		// SSL — additive, never overwrites other domains
		for _, domain := range cfg.Deploy.Domains {
			if domain == "" {
				continue
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Configuring SSL for %s...\n", domain)
			if err := ssl.Configure(sshClient, domain, cfg.Deploy.Port); err != nil {
				fmt.Fprintf(cmd.OutOrStdout(), "  Warning: SSL setup failed: %v\n", err)
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "  https://%s (Let's Encrypt — may take a moment)\n", domain)
			}
		}

		status, _ := deployer.Status()
		fmt.Fprintf(cmd.OutOrStdout(), "\n  App:  %s\n", cfg.App)
		fmt.Fprintf(cmd.OutOrStdout(), "  Host: %s\n", ip)
		fmt.Fprintf(cmd.OutOrStdout(), "  Port: %d\n", cfg.Deploy.Port)
		fmt.Fprintf(cmd.OutOrStdout(), "  Status: %s\n", status)
		if len(cfg.Deploy.Domains) > 0 {
			fmt.Fprintf(cmd.OutOrStdout(), "  HTTPS: https://%s\n", cfg.Deploy.Domains[0])
		}
		fmt.Fprintf(cmd.OutOrStdout(), "  Health: http://%s:%d%s\n", ip, cfg.Deploy.Port, cfg.Deploy.HealthCheck)
		return nil
	},
}

var logsCmd = &cobra.Command{
	Use:   "logs",
	Short: "Tail container logs",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load(config.DefaultPath())
		if err != nil {
			return fmt.Errorf("loading ship.toml: %w", err)
		}
		if cfg.Server == "" {
			return fmt.Errorf("server not set in ship.toml")
		}

		ip, err := resolveServer("hetzner", cfg.Server)
		if err != nil {
			return err
		}

		tail, _ := cmd.Flags().GetString("tail")
		sshClient, err := shipssh.NewClientInsecure(ip, "root", "")
		if err != nil {
			return err
		}

		deployer, err := deploy.NewDeployer(cfg.App, cfg.EnvFile, sshClient)
		if err != nil {
			return err
		}
		return deployer.Logs(cmd.OutOrStdout(), tail)
	},
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show deployment status",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load(config.DefaultPath())
		if err != nil {
			return fmt.Errorf("loading ship.toml: %w", err)
		}
		if cfg.Server == "" {
			return fmt.Errorf("server not set in ship.toml")
		}

		ip, err := resolveServer("hetzner", cfg.Server)
		if err != nil {
			return err
		}

		sshClient, err := shipssh.NewClientInsecure(ip, "root", "")
		if err != nil {
			return err
		}

		deployer, err := deploy.NewDeployer(cfg.App, cfg.EnvFile, sshClient)
		if err != nil {
			return err
		}

		status, err := deployer.Status()
		if err != nil {
			return err
		}

		fmt.Fprintf(cmd.OutOrStdout(), "%s: %s\n", cfg.App, status)
		return nil
	},
}

var sshCmd = &cobra.Command{
	Use:   "ssh",
	Short: "SSH into the server",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load(config.DefaultPath())
		if err != nil {
			return fmt.Errorf("loading ship.toml: %w", err)
		}
		if cfg.Server == "" {
			return fmt.Errorf("server not set in ship.toml")
		}

		ip, err := resolveServer("hetzner", cfg.Server)
		if err != nil {
			return err
		}

		sshClient, err := shipssh.NewClientInsecure(ip, "root", "")
		if err != nil {
			return err
		}

		out, err := sshClient.Run("hostname && docker ps --format 'table {{.Names}}\t{{.Status}}\t{{.Ports}}'")
		if err != nil {
			return err
		}

		fmt.Fprintln(cmd.OutOrStdout(), out)
		return nil
	},
}

func init() {
	deployCmd.Flags().String("server", "", "Server name or IP to deploy to")
	logsCmd.Flags().String("tail", "50", "Number of lines to tail")
}
