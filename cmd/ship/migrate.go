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
	Long: `Full migration from Fly.io:
  1. Dumps Postgres + CouchDB from the Fly machine
  2. Copies all env vars
  3. Generates ship.toml from fly.toml
  4. Encrypts secrets with age (safe to commit)
  5. Sets up Docker + Caddy on target
  6. Restores databases on target

Requires flyctl to be installed and authenticated.
Run this from your Fly project directory (where fly.toml lives).`,
	RunE: func(cmd *cobra.Command, args []string) error {
		targetIP, _ := cmd.Flags().GetString("to")
		if targetIP == "" {
			return fmt.Errorf("--to <server-ip> is required")
		}

		return migrate.FromFly(targetIP)
	},
}

func initMigrate() {
	migrateFromFlyCmd.Flags().String("to", "", "Target server IP address")
	migrateCmd.AddCommand(migrateFromFlyCmd)
}
