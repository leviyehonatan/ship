package main

import (
	"fmt"

	"github.com/leviyehonatan/ship/internal/migrate"
	"github.com/spf13/cobra"
)

var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Migrate an app from another platform",
}

var migrateFromFlyCmd = &cobra.Command{
	Use:   "fly [--to ip]",
	Short: "Migrate a Fly.io app to your server",
	Long: `Dumps databases and volumes from a running Fly machine,
copies them to your server, and generates a ship.toml.

Requires flyctl to be installed and authenticated.

Run this from your project directory (where fly.toml lives).`,
	RunE: func(cmd *cobra.Command, args []string) error {
		targetIP, _ := cmd.Flags().GetString("to")
		if targetIP == "" {
			return fmt.Errorf("--to <server-ip> is required — where should the data go?")
		}

		return migrate.FromFly(targetIP)
	},
}

func initMigrate() {
	migrateFromFlyCmd.Flags().String("to", "", "Target server IP address")
	migrateCmd.AddCommand(migrateFromFlyCmd)
}
