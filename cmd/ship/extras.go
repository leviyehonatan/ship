package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/leviyehonatan/ship/internal/config"
	deploy "github.com/leviyehonatan/ship/internal/docker"
	shipssh "github.com/leviyehonatan/ship/internal/ssh"
	"github.com/spf13/cobra"
)

var imageCmd = &cobra.Command{
	Use:   "image",
	Short: "Show deployed Docker image",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := newShipCtx(cmd)
		if err != nil {
			return err
		}
		out, err := ctx.Run("docker ps --filter name=" + deploy.AppContainer(ctx.Config.App) + " --format '{{.Image}}\t{{.Status}}\t{{.CreatedAt}}'")
		if err != nil {
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), strings.TrimSpace(out))
		return nil
	},
}

var servicesCmd = &cobra.Command{
	Use:   "services",
	Short: "Show exposed ports and services",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := newShipCtx(cmd)
		if err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "App:  %s\n", cfg.Config.App)
		fmt.Fprintf(cmd.OutOrStdout(), "Host: %s\n", cfg.IP)
		fmt.Fprintf(cmd.OutOrStdout(), "Port: %d\n", cfg.Config.Deploy.Port)
		if len(cfg.Config.Deploy.Domains) > 0 {
			fmt.Fprintf(cmd.OutOrStdout(), "SSL:  https://%s\n", cfg.Config.Deploy.Domains[0])
		}
		out, _ := cfg.Run("docker ps --filter name=" + deploy.AppContainer(cfg.Config.App) + " --format '{{.Ports}}'")
		if out != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "Ports: %s\n", strings.TrimSpace(out))
		}
		return nil
	},
}

var consoleCmd = &cobra.Command{
	Use:   "console",
	Short: "Open interactive SSH session",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := newShipCtx(cmd)
		if err != nil {
			return err
		}
		sshHost := cfg.IP
		sshPort := "22"
		if parts := strings.SplitN(cfg.IP, ":", 2); len(parts) == 2 {
			sshHost, sshPort = parts[0], parts[1]
		}
		sshArgs := []string{"-t", "-p", sshPort, "root@" + sshHost,
			"-o", "StrictHostKeyChecking=no", "-o", "UserKnownHostsFile=/dev/null"}

		if os.Getenv("SSHPASS") != "" {
			sshArgs = append([]string{"-p", os.Getenv("SSHPASS"), "ssh"}, sshArgs...)
			execCmd := exec.Command("sshpass", sshArgs...)
			execCmd.Stdin, execCmd.Stdout, execCmd.Stderr = os.Stdin, os.Stdout, os.Stderr
			return execCmd.Run()
		}
		execCmd := exec.Command("ssh", sshArgs...)
		execCmd.Stdin, execCmd.Stdout, execCmd.Stderr = os.Stdin, os.Stdout, os.Stderr
		return execCmd.Run()
	},
}

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Diagnose local tools and server health",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Fprintln(cmd.OutOrStdout(), "Local tools:")
		localChecks := map[string]string{
			"docker":  "docker --version",
			"hcloud":  "hcloud version",
			"ssh":     "ssh -V",
			"sshpass": "sshpass -V",
			"flyctl":  "fly version",
		}
		// Check age key
		home, _ := os.UserHomeDir()
		envOK := os.Getenv("SHIP_AGE_KEY") != ""
		_, fileOK := os.Stat(home + "/.config/ship/age-key.txt")
		_, keychainOK := exec.Command("security", "find-generic-password", "-s", "ship-age-key", "-w").Output()

		if envOK {
			fmt.Fprintf(cmd.OutOrStdout(), "  ✓ age key: SHIP_AGE_KEY\n")
		} else if keychainOK == nil {
			fmt.Fprintf(cmd.OutOrStdout(), "  ✓ age key: iCloud Keychain (syncs across Apple devices)\n")
		} else if fileOK == nil {
			fmt.Fprintf(cmd.OutOrStdout(), "  ✓ age key: file only (~/.config/ship)\n")
		} else {
			fmt.Fprintf(cmd.OutOrStdout(), "  ✗ age key: not generated (run 'ship secrets set KEY=val')\n")
		}
		for name, check := range localChecks {
			c := exec.Command("sh", "-c", check+" 2>&1")
			out, err := c.Output()
			if err != nil || len(out) < 5 {
				fmt.Fprintf(cmd.OutOrStdout(), "  ✗ %s: not installed\n", name)
			} else {
				firstLine := strings.SplitN(strings.TrimSpace(string(out)), "\n", 2)[0]
				if len(firstLine) > 60 {
					firstLine = firstLine[:60]
				}
				fmt.Fprintf(cmd.OutOrStdout(), "  ✓ %s\n", name)
			}
		}

		// Remote server checks (if config available)
		cfg, err := config.Load(config.DefaultPath())
		if err != nil {
			fmt.Fprintln(cmd.OutOrStdout(), "\nRemote: (no ship.toml — run 'ship init' first)")
			return nil
		}
		serverAddr := cfg.Server
		if serverFlag, _ := cmd.Flags().GetString("server"); serverFlag != "" {
			serverAddr = serverFlag
		}
		if serverAddr == "" {
			fmt.Fprintln(cmd.OutOrStdout(), "\nRemote: (no server configured — use 'ship use <name>')")
			return nil
		}

		ip, err := resolveServer("", serverAddr)
		if err != nil {
			fmt.Fprintf(cmd.OutOrStdout(), "\nRemote %s: unreachable (%v)\n", serverAddr, err)
			return nil
		}

		sshClient, err := shipssh.NewClientInsecure(ip, "root", "")
		if err != nil {
			fmt.Fprintf(cmd.OutOrStdout(), "\nRemote %s: SSH failed (%v)\n", ip, err)
			return nil
		}

		fmt.Fprintf(cmd.OutOrStdout(), "\nRemote (%s):\n", ip)
		remoteChecks := map[string]string{
			"docker":  "docker --version 2>&1 | head -1",
			"caddy":   "caddy version 2>&1 | head -1",
			"disk":    "df -h / | tail -1 | awk '{print $4\" free of \"$2}'",
			"memory":  "free -h | grep Mem | awk '{print $4\" free of \"$2}'",
			"uptime":  "uptime | awk -F',' '{print $1}'",
		}
		if cfg.App != "" {
			remoteChecks["app"] = fmt.Sprintf("docker ps --filter name=%s --format '{{.Status}}' 2>&1", deploy.AppContainer(cfg.App))
		}

		for name, check := range remoteChecks {
			out, err := sshClient.Run(check)
			if err != nil || strings.TrimSpace(out) == "" {
				fmt.Fprintf(cmd.OutOrStdout(), "  ✗ %s: not found\n", name)
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "  ✓ %s: %s\n", name, strings.TrimSpace(out))
			}
		}
		return nil
	},
}

var dashboardCmd = &cobra.Command{
	Use:   "dashboard",
	Short: "Open provider web console",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Fprintln(cmd.OutOrStdout(), "Opening https://console.hetzner.cloud ...")
		return exec.Command("open", "https://console.hetzner.cloud").Run()
	},
}

var sftpCmd = &cobra.Command{
	Use:   "sftp [source] [dest]",
	Short: "Transfer files to/from server",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := newShipCtx(cmd)
		if err != nil {
			return err
		}
		scpArgs := []string{"-r", "-o", "StrictHostKeyChecking=no", "-o", "UserKnownHostsFile=/dev/null"}
		if strings.Contains(ctx.IP, ":") {
			parts := strings.SplitN(ctx.IP, ":", 2)
			scpArgs = append(scpArgs, "-P", parts[1])
		}
		scpArgs = append(scpArgs, args[0], args[1])
		c := exec.Command("scp", scpArgs...)
		c.Stdin, c.Stdout, c.Stderr = os.Stdin, os.Stdout, os.Stderr
		return c.Run()
	},
}
