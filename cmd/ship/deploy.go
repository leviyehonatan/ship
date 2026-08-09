package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/leviyehonatan/ship/internal/config"
	deploy "github.com/leviyehonatan/ship/internal/docker"
	"github.com/leviyehonatan/ship/internal/releases"
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
		printWarnings(cmd, cfg.Warnings)

		serverFlag, _ := cmd.Flags().GetString("server")
		if serverFlag == "" {
			serverFlag = cfg.Server
		}
		if serverFlag == "" {
			return fmt.Errorf("no server configured — use 'ship use <name>' or set 'server' in ship.toml")
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

		// Auto-setup if server hasn't been initialized
		setupDone, _ := sshClient.Run("test -f /opt/ship/.setup-complete && echo yes || echo no")
		if !strings.Contains(setupDone, "yes") {
			fmt.Fprintln(cmd.OutOrStdout(), "First deploy — setting up server...")
			// Self-call setup via the same binary
			shipBin, _ := os.Executable()
			setup := exec.Command(shipBin, "setup")
			setup.Stdout = cmd.OutOrStdout()
			setup.Stderr = cmd.ErrOrStderr()
			setup.Env = os.Environ()
			if err := setup.Run(); err != nil {
				fmt.Fprintf(cmd.OutOrStdout(), "  Warning: setup incomplete — continuing anyway\n")
			}
		}

		// Start sidecar services (Postgres, Redis, etc.) if configured
		for name, svc := range cfg.Services {
			containerName := cfg.App + "-" + name
			running, _ := sshClient.Run(fmt.Sprintf("docker ps --filter name=%s --format '{{.Status}}'", containerName))
			if strings.TrimSpace(running) == "" {
				fmt.Fprintf(cmd.OutOrStdout(), "Starting %s...\n", name)
				envArgs := ""
				for k, v := range svc.Env {
					envArgs += fmt.Sprintf(" -e %s='%s'", k, v)
				}
				startCmd := fmt.Sprintf(
					"docker rm -f %s 2>/dev/null; docker run -d --name %s --restart unless-stopped -p %d:%d%s %s",
					containerName, containerName, svc.Port, svc.Port, envArgs, svc.Image,
				)
				if _, err := sshClient.Run(startCmd); err != nil {
					fmt.Fprintf(cmd.OutOrStdout(), "  Warning: failed to start %s: %v\n", name, err)
				}
			}
		}
		mgr := snapshot.NewManager(sshClient, cfg.App)
		fmt.Fprintln(cmd.OutOrStdout(), "Snapshotting...")
		if err := mgr.Create(); err != nil {
			fmt.Fprintf(cmd.OutOrStdout(), "  Warning: snapshot failed: %v\n", err)
		}

		// Build (with build args from ship.toml if present)
		deployer := deploy.NewDeployerWithEnv(cfg.App, envVars, sshClient)
		skipBuild, _ := cmd.Flags().GetBool("skip-build")
		if !skipBuild {
			fmt.Fprintf(cmd.OutOrStdout(), "Building %s...\n", cfg.App)
			if len(cfg.Build.Args) > 0 {
				if err := deployer.BuildWithArgs(cmd.Context(), cfg.Build.Args); err != nil {
					return fmt.Errorf("build: %w", err)
				}
			} else {
				if err := deployer.Build(cmd.Context()); err != nil {
					return fmt.Errorf("build: %w", err)
				}
			}
		} else {
			fmt.Fprintln(cmd.OutOrStdout(), "Skipping build (--skip-build)")
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
		// On macOS Docker Desktop, arbitrary paths like /opt/ship/data are blocked.
		// Use a project-relative path instead — docker build context includes it,
		// and it's automatically excluded via .gitignore.
		isLocal := strings.HasPrefix(ip, "localhost") || strings.HasPrefix(ip, "127.")
		for _, v := range cfg.Volumes {
			if isLocal {
				localPath := filepath.Join(".ship-data", cfg.App, v.Path)
				os.MkdirAll(localPath, 0755)
				absPath, _ := filepath.Abs(localPath)
				volumes = append(volumes, fmt.Sprintf("%s:%s", absPath, v.Path))
			} else {
				volumes = append(volumes, fmt.Sprintf("/opt/ship/data/%s:%s", cfg.App, v.Path))
			}
		}
		if len(cfg.Volumes) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "  ⚠ No volumes configured — container state will be lost on redeploy")
		}
		if isLocal {
			// Ensure .ship-data is gitignored
			addToGitignore(".ship-data/")
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

		// Run release command (migrations, etc.) if configured
		if cfg.Deploy.ReleaseCommand != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "Running release command...\n")
			releaseCmd := fmt.Sprintf("docker exec %s %s", cfg.App, cfg.Deploy.ReleaseCommand)
			if out, err := sshClient.Run(releaseCmd); err != nil {
				fmt.Fprintf(cmd.OutOrStdout(), "  Warning: release command failed: %v\n%s\n", err, out)
			} else {
				if strings.TrimSpace(out) != "" {
					fmt.Fprint(cmd.OutOrStdout(), out)
				}
			}
		}

		// Record release
		releases.Record(sshClient, cfg.App, cfg.App+":latest")

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

func addToGitignore(pattern string) {
	data, err := os.ReadFile(".gitignore")
	if err == nil && !strings.Contains(string(data), pattern) {
		f, _ := os.OpenFile(".gitignore", os.O_APPEND|os.O_WRONLY, 0644)
		if f != nil {
			f.WriteString("\n" + pattern + "\n")
			f.Close()
		}
	}
}

func init() {
	deployCmd.Flags().String("server", "", "Server name or IP to deploy to")
	logsCmd.Flags().String("tail", "50", "Number of lines to tail")
}

func printWarnings(cmd *cobra.Command, warnings []string) {
	for _, w := range warnings {
		fmt.Fprintf(cmd.ErrOrStderr(), "⚠ %s\n", w)
	}
}
