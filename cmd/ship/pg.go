package main

import (
	"fmt"

	"github.com/leviyehonatan/ship/internal/pg"
	"github.com/leviyehonatan/ship/internal/secrets"
	"github.com/spf13/cobra"
)

var pgCmd = &cobra.Command{
	Use:   "pg",
	Short: "Manage Postgres databases on your server",
}

var pgCreateCmd = &cobra.Command{
	Use:   "create [name]",
	Short: "Create a Postgres database",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := newShipCtx(cmd)
		if err != nil {
			return err
		}

		name := "app"
		if len(args) > 0 {
			name = args[0]
		}
		password, _ := cmd.Flags().GetString("password")

		fmt.Fprintf(cmd.OutOrStdout(), "Creating Postgres on %s...\n", ctx.IP)
		containerID, password, err := pg.Create(ctx.SSH, name, password)
		if err != nil {
			return err
		}

		connStr := pg.ConnectionString(ctx.IP, name, password)
		fmt.Fprintf(cmd.OutOrStdout(), "✓ Postgres running (%s)\n", containerID[:12])
		fmt.Fprintf(cmd.OutOrStdout(), "  Database: %s\n", name)
		fmt.Fprintf(cmd.OutOrStdout(), "  Password: %s\n", password)
		fmt.Fprintf(cmd.OutOrStdout(), "  URL:      %s\n", connStr)

		linkApp, _ := cmd.Flags().GetString("link")
		if linkApp == "" {
			linkApp = ctx.Config.App
		}
		if linkApp != "" && linkApp != "app" {
			secrets.Set(".env.encrypted", "DATABASE_URL", connStr)
			fmt.Fprintf(cmd.OutOrStdout(), "  Linked:   %s (DATABASE_URL set)\n", linkApp)
		} else {
			fmt.Fprintf(cmd.OutOrStdout(), "  Set manually: ship secrets set DATABASE_URL=%s\n", connStr)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "  Connect: ship tunnel db\n")
		return nil
	},
}

var pgListCmd = &cobra.Command{
	Use:   "list",
	Short: "List Postgres databases",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := newShipCtx(cmd)
		if err != nil {
			return err
		}
		dbs, err := pg.List(ctx.SSH)
		if err != nil {
			return err
		}
		if len(dbs) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "No Postgres databases.")
			fmt.Fprintln(cmd.OutOrStdout(), "  Create one: ship pg create mydb")
			return nil
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Postgres on %s:\n", ctx.IP)
		for _, db := range dbs {
			status, _ := pg.Status(ctx.SSH, db)
			fmt.Fprintf(cmd.OutOrStdout(), "  %-15s  %s\n", db, status)
		}
		return nil
	},
}

var pgConnectCmd = &cobra.Command{
	Use:   "connect [name]",
	Short: "Show connection string",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := newShipCtx(cmd)
		if err != nil {
			return err
		}
		name := "app"
		if len(args) > 0 {
			name = args[0]
		}
		fmt.Fprintf(cmd.OutOrStdout(), "%s\n", pg.ConnectionString(ctx.IP, name, ""))
		return nil
	},
}

func initPG() {
	pgCreateCmd.Flags().String("server", "", "Server name or IP")
	pgCreateCmd.Flags().String("password", "", "Postgres password (auto-generated)")
	pgCreateCmd.Flags().String("link", "", "App to auto-link")
	pgCmd.AddCommand(pgCreateCmd)
	pgCmd.AddCommand(pgListCmd)
	pgCmd.AddCommand(pgConnectCmd)
}
