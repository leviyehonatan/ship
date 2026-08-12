package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/leviyehonatan/ship/internal/config"
	deploy "github.com/leviyehonatan/ship/internal/docker"
	shiplog "github.com/leviyehonatan/ship/internal/log"
	"github.com/leviyehonatan/ship/internal/releases"
	"github.com/leviyehonatan/ship/internal/secrets"
	"github.com/leviyehonatan/ship/internal/services"
	"github.com/leviyehonatan/ship/internal/snapshot"
	"github.com/leviyehonatan/ship/internal/ssl"
	shipssh "github.com/leviyehonatan/ship/internal/ssh"
	"github.com/spf13/cobra"
)

var deployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Build and deploy your app to a server",
	Long: `Full deployment pipeline:
  1. Provision services (Postgres, Redis, etc. from ship.toml)
  2. Snapshot database (safety net, remote only)
  3. Build Docker image
  4. Push image over SSH (no registry needed, remote only)
  5. Restart container
  6. Configure SSL (if domains set, remote only)

With --local, skips SSH, snapshot, push, and SSL — runs directly on local Docker.
Reads ship.toml for configuration. Secrets from .env.encrypted are auto-decrypted.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		localMode, _ := cmd.Flags().GetBool("local")
		shiplog.Verbose("local mode: %v\n", localMode)

		cfg, err := config.Load(config.DefaultPath())
		if err != nil {
			return fmt.Errorf("loading ship.toml: %w\n  run 'ship init' first", err)
		}
		printWarnings(cmd, cfg.Warnings)

		shiplog.Verbose("app: %s\n", cfg.App)

		var ip, serverFlag string
		if localMode {
			ip = "localhost"
			shiplog.Verbose("target: localhost (local mode)\n")
		} else {
			serverFlag, _ = cmd.Flags().GetString("server")
			if serverFlag == "" {
				serverFlag = cfg.Server
			}
			shiplog.Verbose("server: %s (flag=%q, config=%q)\n", serverFlag, cmd.Flag("server").Value, cfg.Server)
			if serverFlag == "" {
				return fmt.Errorf("no server — set 'server' in ship.toml or use 'ship use <name>'")
			}
			ip, err = resolveServer("", serverFlag)
			if err != nil {
				shiplog.Verbose("resolve error: %v\n", err)
				return err
			}
			if ip != serverFlag {
				fmt.Fprintf(cmd.OutOrStdout(), "Resolved %s → %s\n", serverFlag, ip)
				shiplog.Verbose("resolved: %s → %s\n", serverFlag, ip)
			}
		}

		var sshClient *shipssh.Client
		if !localMode {
			shiplog.Verbose("connecting ssh: root@%s\n", ip)
			sshClient, err = shipssh.NewClientInsecure(ip, "root", "")
			if err != nil {
				return fmt.Errorf("ssh: %w", err)
			}
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
		if !localMode {
			setupDone, _ := sshClient.Run("test -f /opt/ship/.setup-complete && echo yes || echo no")
			if !strings.Contains(setupDone, "yes") {
				fmt.Fprintln(cmd.OutOrStdout(), "First deploy — setting up server...")
				shipBin, _ := os.Executable()
				setup := exec.Command(shipBin, "setup")
				setup.Stdout = cmd.OutOrStdout()
				setup.Stderr = cmd.ErrOrStderr()
				setup.Env = os.Environ()
				if err := setup.Run(); err != nil {
					fmt.Fprintf(cmd.OutOrStdout(), "  Warning: setup incomplete — continuing anyway\n")
				}
			}

			// Snapshot before deploy
			mgr := snapshot.NewManager(sshClient, cfg.App)
			fmt.Fprintln(cmd.OutOrStdout(), "Snapshotting...")
			if err := mgr.Create(); err != nil {
				fmt.Fprintf(cmd.OutOrStdout(), "  Warning: snapshot failed: %v\n", err)
			}
		}

		// Provision services (Postgres, Redis, etc. from ship.toml [services])
		if len(cfg.Services) > 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "Provisioning services...")
			svcEnv, err := services.Ensure(sshClient, cfg, cfg.App, localMode)
			if err != nil {
				fmt.Fprintf(cmd.OutOrStdout(), "  Warning: %v\n", err)
			}
			for k, v := range svcEnv {
				if _, exists := envVars[k]; !exists {
					envVars[k] = v
				}
				fmt.Fprintf(cmd.OutOrStdout(), "  ✓ %s\n", k)
			}
		}

		// Build (with build args from ship.toml if present)
		deployer := deploy.NewDeployerWithEnv(cfg.App, envVars, sshClient)
		deployer.Stdout = cmd.OutOrStdout()
		skipBuild, _ := cmd.Flags().GetBool("skip-build")
		shiplog.Verbose("build: tag=%s, args=%v, skip=%v\n", cfg.App, cfg.Build.Args, skipBuild)
		if skipBuild {
			fmt.Fprintln(cmd.OutOrStdout(), "Skipping build (--skip-build)")
		} else if len(cfg.Build.Args) > 0 {
			fmt.Fprintf(cmd.OutOrStdout(), "Building %s...\n", cfg.App)
			if err := deployer.BuildWithArgs(cmd.Context(), cfg.Build.Args); err != nil {
				shiplog.Verbose("build failed: %v\n", err)
				return fmt.Errorf("build: %w", err)
			}
		} else {
			fmt.Fprintf(cmd.OutOrStdout(), "Building %s...\n", cfg.App)
			if err := deployer.Build(cmd.Context()); err != nil {
				shiplog.Verbose("build failed: %v\n", err)
				return fmt.Errorf("build: %w", err)
			}
		}

		// Push
		if !localMode {
			fmt.Fprintln(cmd.OutOrStdout(), "Pushing to server...")
			shiplog.Verbose("push: %s → %s\n", cfg.App, ip)
			if err := deployer.PushOverSSH(); err != nil {
				return fmt.Errorf("push: %w", err)
			}
		} else {
			shiplog.Verbose("push: skipped (local mode)\n")
		}

		// Run
		fmt.Fprintln(cmd.OutOrStdout(), "Starting container...")
		ports := []string{fmt.Sprintf("%d:%d", cfg.Deploy.Port, cfg.Deploy.Port)}
		var volumes []string
		for _, v := range cfg.Volumes {
			if localMode {
				localPath := filepath.Join(".ship-data", cfg.App, v.Path)
				os.MkdirAll(localPath, 0755)
				absPath, _ := filepath.Abs(localPath)
				volumes = append(volumes, fmt.Sprintf("%s:%s", absPath, v.Path))
			} else {
				volumes = append(volumes, fmt.Sprintf("/opt/ship/data/%s:%s", cfg.App, v.Path))
			}
		}
		if len(cfg.Volumes) == 0 && len(cfg.Services) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "  ⚠ No volumes configured — container state will be lost on redeploy")
		}
		if localMode {
			addToGitignore(".ship-data/")
		}

		// Attach app to bridge network if services are defined
		networkArgs := ""
		if len(cfg.Services) > 0 {
			networkArgs = fmt.Sprintf("--network %s", deploy.Network(cfg.App))
		}

		if err := deployer.RunRemote(deploy.RunOpts{
			Ports:       ports,
			Volumes:     volumes,
			NetworkArgs: networkArgs,
		}); err != nil {
			return fmt.Errorf("run: %w", err)
		}

		// SSL — additive, never overwrites other domains
		if !localMode {
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

			// Record release
			releases.Record(sshClient, cfg.App, deploy.ImageRef(cfg.App))
		}

		// Run release command (migrations, etc.) if configured
		if cfg.Deploy.ReleaseCommand != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "Running release command...\n")
			var out string
			var relErr error
			if localMode {
				rel := exec.Command("docker", append([]string{"exec", deploy.AppContainer(cfg.App)}, strings.Fields(cfg.Deploy.ReleaseCommand)...)...)
				b, err := rel.CombinedOutput()
				out = string(b)
				relErr = err
			} else {
				out, relErr = sshClient.Run(fmt.Sprintf("docker exec %s %s", deploy.AppContainer(cfg.App), cfg.Deploy.ReleaseCommand))
			}
			if relErr != nil {
				fmt.Fprintf(cmd.OutOrStdout(), "  Warning: release command failed: %v\n%s\n", relErr, out)
			} else if strings.TrimSpace(out) != "" {
				fmt.Fprint(cmd.OutOrStdout(), out)
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

		ip, err := resolveServer("", cfg.Server)
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

		ip, err := resolveServer("", cfg.Server)
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

		ip, err := resolveServer("", cfg.Server)
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

var downCmd = &cobra.Command{
	Use:   "down",
	Short: "Stop app and services, remove network (keep data)",
	Long: `Stops and removes the app container, all service containers,
and the bridge network. Data under .ship-data/ (local) or
/opt/ship/data/ (remote) is preserved unless --volumes is set.

Use --local to target local Docker instead of a remote server.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		localMode, _ := cmd.Flags().GetBool("local")
		rmVolumes, _ := cmd.Flags().GetBool("volumes")

		cfg, err := config.Load(config.DefaultPath())
		if err != nil {
			return fmt.Errorf("loading ship.toml: %w", err)
		}

		var ip string
		var sshClient *shipssh.Client

		if localMode {
			ip = "localhost"
		} else {
			serverFlag, _ := cmd.Flags().GetString("server")
			if serverFlag == "" {
				serverFlag = cfg.Server
			}
			if serverFlag == "" {
				return fmt.Errorf("no server — set 'server' in ship.toml or use 'ship use <name>'")
			}
			ip, err = resolveServer("", serverFlag)
			if err != nil {
				return err
			}
			sshClient, err = shipssh.NewClientInsecure(ip, "root", "")
			if err != nil {
				return fmt.Errorf("ssh: %w", err)
			}
		}

		d := deploy.NewDeployerWithEnv(cfg.App, nil, sshClient)

		// Stop and remove app container
		fmt.Fprintf(cmd.OutOrStdout(), "  %s\n", deploy.AppContainer(cfg.App))
		_ = d.StopRemove()

		// Stop and remove service containers
		for name := range cfg.Services {
			fmt.Fprintf(cmd.OutOrStdout(), "  %s\n", deploy.ServiceContainer(cfg.App, name))
			_ = d.StopRemoveSvc(name)
		}

		// Remove bridge network
		network := deploy.Network(cfg.App)
		if localMode {
			exec.Command("docker", "network", "rm", network).Run()
		} else {
			sshClient.Run(fmt.Sprintf("docker network rm %s 2>/dev/null || true", network))
		}
		fmt.Fprintf(cmd.OutOrStdout(), "  %s\n", network)

		// Optional: remove data
		if rmVolumes {
			if localMode {
				dataPath := filepath.Join(".ship-data", cfg.App)
				if err := os.RemoveAll(dataPath); err != nil {
					fmt.Fprintf(cmd.OutOrStdout(), "  Warning: %v\n", err)
				} else {
					fmt.Fprintf(cmd.OutOrStdout(), "  %s (data)\n", dataPath)
				}
			} else {
				dataPath := fmt.Sprintf("/opt/ship/data/%s", cfg.App)
				sshClient.Run(fmt.Sprintf("rm -rf %s", dataPath))
				fmt.Fprintf(cmd.OutOrStdout(), "  %s (data)\n", dataPath)
			}
		}

		fmt.Fprintln(cmd.OutOrStdout(), "✓ Down")
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
	deployCmd.Flags().Bool("local", false, "Deploy to local Docker (skip SSH, no SSL)")
	deployCmd.Flags().String("server", "", "Server name or IP to deploy to")
	downCmd.Flags().Bool("local", false, "Target local Docker instead of remote server")
	downCmd.Flags().Bool("volumes", false, "Also remove persistent data (.ship-data/ or /opt/ship/data/)")
	downCmd.Flags().String("server", "", "Server name or IP")
	logsCmd.Flags().String("tail", "50", "Number of lines to tail")
}

func printWarnings(cmd *cobra.Command, warnings []string) {
	for _, w := range warnings {
		fmt.Fprintf(cmd.ErrOrStderr(), "⚠ %s\n", w)
	}
}
