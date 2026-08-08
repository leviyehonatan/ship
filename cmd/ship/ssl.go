package main

import (
	"fmt"
	"strconv"

	"github.com/leviyehonatan/ship/internal/config"
	"github.com/leviyehonatan/ship/internal/ssl"
	shipssh "github.com/leviyehonatan/ship/internal/ssh"
	"github.com/spf13/cobra"
)

var sslCmd = &cobra.Command{
	Use:   "ssl",
	Short: "Manage SSL certificates via Caddy",
}

var sslOnCmd = &cobra.Command{
	Use:   "on [domain]",
	Short: "Enable HTTPS for a domain",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load(config.DefaultPath())
		if err != nil {
			return err
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

		port := cfg.Deploy.Port
		if port == 0 {
			port = 8080
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Enabling HTTPS for %s...\n", args[0])
		if err := ssl.Configure(sshClient, args[0], port); err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "✓ https://%s → localhost:%d\n", args[0], port)
		return nil
	},
}

var sslOffCmd = &cobra.Command{
	Use:   "off [domain]",
	Short: "Disable HTTPS for a domain",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load(config.DefaultPath())
		if err != nil {
			return err
		}
		if cfg.Server == "" {
			return fmt.Errorf("server not set")
		}

		ip, err := resolveServer("hetzner", cfg.Server)
		if err != nil {
			return err
		}

		sshClient, err := shipssh.NewClientInsecure(ip, "root", "")
		if err != nil {
			return err
		}

		if err := ssl.Remove(sshClient, args[0]); err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "✓ HTTPS disabled for %s\n", args[0])
		return nil
	},
}

var sslStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show SSL certificate status for all domains",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load(config.DefaultPath())
		if err != nil {
			return err
		}
		if cfg.Server == "" {
			return fmt.Errorf("server not set")
		}

		ip, err := resolveServer("hetzner", cfg.Server)
		if err != nil {
			return err
		}

		sshClient, err := shipssh.NewClientInsecure(ip, "root", "")
		if err != nil {
			return err
		}

		out, err := ssl.Status(sshClient)
		if err != nil {
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), out)
		return nil
	},
}

var sslRenewCmd = &cobra.Command{
	Use:   "renew",
	Short: "Force certificate renewal",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load(config.DefaultPath())
		if err != nil {
			return err
		}
		if cfg.Server == "" {
			return fmt.Errorf("server not set")
		}

		ip, err := resolveServer("hetzner", cfg.Server)
		if err != nil {
			return err
		}

		sshClient, err := shipssh.NewClientInsecure(ip, "root", "")
		if err != nil {
			return err
		}

		sshClient.Run("caddy reload --force --config /etc/caddy/Caddyfile 2>/dev/null || true")
		fmt.Fprintln(cmd.OutOrStdout(), "✓ Certificate renewal requested")
		fmt.Fprintln(cmd.OutOrStdout(), "  Caddy will auto-renew 30 days before expiry")
		return nil
	},
}

func init() {
	_ = strconv.Itoa(0) // keep import
}

func initSSL() {
	sslCmd.AddCommand(sslOnCmd)
	sslCmd.AddCommand(sslOffCmd)
	sslCmd.AddCommand(sslStatusCmd)
	sslCmd.AddCommand(sslRenewCmd)
}
