package main

import (
	"fmt"

	"github.com/leviyehonatan/ship/internal/config"
	"github.com/leviyehonatan/ship/internal/pg"
	shipssh "github.com/leviyehonatan/ship/internal/ssh"
	"github.com/spf13/cobra"
)

var pgCmd = &cobra.Command{
	Use:   "pg",
	Short: "Manage Postgres databases on your server",
	Long:  `Provisions and manages Postgres as a Docker container on your server.`,
}

var pgCreateCmd = &cobra.Command{
	Use:   "create [name]",
	Short: "Create a Postgres database",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load(config.DefaultPath())
		serverAddr := ""
		if err == nil {
			serverAddr = cfg.Server
		}
		serverFlag, _ := cmd.Flags().GetString("server")
		if serverFlag != "" {
			serverAddr = serverFlag
		}

		ip, err := resolveServer("hetzner", serverAddr)
		if err != nil {
			return err
		}

		sshClient, err := shipssh.NewClientInsecure(ip, "root", "")
		if err != nil {
			return err
		}

		name := "app"
		if len(args) > 0 {
			name = args[0]
		}

		password, _ := cmd.Flags().GetString("password")

		fmt.Fprintf(cmd.OutOrStdout(), "Creating Postgres on %s...\n", ip)
		containerID, err := pg.Create(sshClient, name, password)
		if err != nil {
			return err
		}

		fmt.Fprintf(cmd.OutOrStdout(), "✓ Postgres running (%s)\n", containerID[:12])
		fmt.Fprintf(cmd.OutOrStdout(), "  Container: ship-pg-%s\n", name)
		fmt.Fprintf(cmd.OutOrStdout(), "  Port: 5432\n")
		fmt.Fprintf(cmd.OutOrStdout(), "  Add to ship.toml:\n")
		fmt.Fprintf(cmd.OutOrStdout(), "    ship secrets set DATABASE_URL=%s\n", pg.ConnectionString(ip, name, password))
		fmt.Fprintf(cmd.OutOrStdout(), "  Connect locally:\n")
		fmt.Fprintf(cmd.OutOrStdout(), "    ship tunnel db\n")
		return nil
	},
}

var pgListCmd = &cobra.Command{
	Use:   "list",
	Short: "List Postgres databases on the current server",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load(config.DefaultPath())
		serverAddr := ""
		if err == nil {
			serverAddr = cfg.Server
		}

		ip, err := resolveServer("hetzner", serverAddr)
		if err != nil {
			return err
		}

		sshClient, err := shipssh.NewClientInsecure(ip, "root", "")
		if err != nil {
			return err
		}

		dbs, err := pg.List(sshClient)
		if err != nil {
			return err
		}

		if len(dbs) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "No Postgres databases found.")
			fmt.Fprintln(cmd.OutOrStdout(), "  Create one: ship pg create mydb")
			return nil
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Postgres databases on %s:\n", ip)
		for _, db := range dbs {
			status, _ := pg.Status(sshClient, db)
			fmt.Fprintf(cmd.OutOrStdout(), "  %-15s  %s\n", db, status)
		}
		return nil
	},
}

var pgConnectCmd = &cobra.Command{
	Use:   "connect [name]",
	Short: "Show connection string for a Postgres database",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load(config.DefaultPath())
		serverAddr := ""
		if err == nil {
			serverAddr = cfg.Server
		}

		ip, err := resolveServer("hetzner", serverAddr)
		if err != nil {
			return err
		}

		name := "app"
		if len(args) > 0 {
			name = args[0]
		}

		fmt.Fprintf(cmd.OutOrStdout(), "%s\n", pg.ConnectionString(ip, name, ""))
		return nil
	},
}

func initPG() {
	pgCreateCmd.Flags().String("server", "", "Server name or IP")
	pgCreateCmd.Flags().String("password", "", "Postgres password (auto-generated if empty)")

	pgCmd.AddCommand(pgCreateCmd)
	pgCmd.AddCommand(pgListCmd)
	pgCmd.AddCommand(pgConnectCmd)
}
