package main

import (
	"fmt"

	"github.com/leviyehonatan/ship/internal/ssl"
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
		ctx, err := newShipCtx(cmd)
		if err != nil {
			return err
		}
		port := ctx.Config.Deploy.Port
		if port == 0 {
			port = 8080
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Enabling HTTPS for %s...\n", args[0])
		if err := ssl.Configure(ctx.SSH, args[0], port); err != nil {
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
		ctx, err := newShipCtx(cmd)
		if err != nil {
			return err
		}
		return ssl.Remove(ctx.SSH, args[0])
	},
}

var sslStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show SSL certificate status",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := newShipCtx(cmd)
		if err != nil {
			return err
		}
		out, err := ssl.Status(ctx.SSH)
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
		ctx, err := newShipCtx(cmd)
		if err != nil {
			return err
		}
		ctx.Run("caddy reload --force --config /etc/caddy/Caddyfile 2>/dev/null || true")
		fmt.Fprintln(cmd.OutOrStdout(), "✓ Renewal requested")
		return nil
	},
}

func initSSL() {
	sslCmd.AddCommand(sslOnCmd)
	sslCmd.AddCommand(sslOffCmd)
	sslCmd.AddCommand(sslStatusCmd)
	sslCmd.AddCommand(sslRenewCmd)
}
