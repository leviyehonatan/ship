package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

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
		out, err := ctx.Run("docker ps --filter name=" + ctx.Config.App + " --format '{{.Image}}\t{{.Status}}\t{{.CreatedAt}}'")
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
		out, _ := cfg.Run("docker ps --filter name=" + cfg.Config.App + " --format '{{.Ports}}'")
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
	Short: "Diagnose server health",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := newShipCtx(cmd)
		if err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Checking %s...\n\n", ctx.IP)
		checks := map[string]string{
			"Docker":        "docker --version",
			"Caddy":         "caddy version",
			"Disk":          "df -h / | tail -1",
			"Memory":        "free -h | grep Mem",
			"Uptime":        "uptime",
			"App container": fmt.Sprintf("docker ps --filter name=%s --format '{{.Status}}'", ctx.Config.App),
		}
		for name, check := range checks {
			out, err := ctx.Run(check)
			if err != nil || strings.TrimSpace(out) == "" {
				fmt.Fprintf(cmd.OutOrStdout(), "  ✗ %s: not found\n", name)
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "  ✓ %s: %s\n", name, strings.TrimSpace(strings.SplitN(out, "\n", 2)[0]))
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
