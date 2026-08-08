package main

import (
	"fmt"

	"github.com/leviyehonatan/ship/internal/config"
	"github.com/leviyehonatan/ship/internal/snapshot"
	shipssh "github.com/leviyehonatan/ship/internal/ssh"
	"github.com/spf13/cobra"
)

var snapshotCmd = &cobra.Command{
	Use:   "snapshot",
	Short: "Create a database snapshot for rollback",
	Long: `Dumps Postgres and CouchDB data from the running container.
Snapshots are stored on the server at /opt/ship/snapshots/<app>/.

Keeps the last 5 snapshots — older ones are auto-cleaned.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load(config.DefaultPath())
		if err != nil {
			return fmt.Errorf("loading ship.toml: %w\n  Run 'ship init' first", err)
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

		mgr := snapshot.NewManager(sshClient, cfg.App)
		fmt.Fprintln(cmd.OutOrStdout(), "Creating snapshot...")
		if err := mgr.Create(); err != nil {
			return fmt.Errorf("snapshot: %w", err)
		}

		snapshots, _ := mgr.List()
		fmt.Fprintf(cmd.OutOrStdout(), "✓ Snapshot created (%d total)\n", len(snapshots))
		if len(snapshots) > 0 {
			fmt.Fprintf(cmd.OutOrStdout(), "  Latest: %s\n", snapshots[0])
		}
		return nil
	},
}

var snapshotsCmd = &cobra.Command{
	Use:   "snapshots",
	Short: "List available snapshots",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load(config.DefaultPath())
		if err != nil {
			return fmt.Errorf("loading ship.toml: %w", err)
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

		mgr := snapshot.NewManager(sshClient, cfg.App)
		snapshots, err := mgr.List()
		if err != nil {
			return err
		}

		if len(snapshots) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "No snapshots found.")
			fmt.Fprintln(cmd.OutOrStdout(), "  Create one: ship snapshot")
			return nil
		}

		fmt.Fprintln(cmd.OutOrStdout(), "Snapshots:")
		for _, s := range snapshots {
			fmt.Fprintf(cmd.OutOrStdout(), "  %s\n", s)
		}
		return nil
	},
}

var rollbackCmd = &cobra.Command{
	Use:   "rollback [snapshot-id]",
	Short: "Restore a database snapshot",
	Long: `Stops the app, restores Postgres and CouchDB from a snapshot,
and restarts the container. If no snapshot ID is given, uses the latest.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load(config.DefaultPath())
		if err != nil {
			return fmt.Errorf("loading ship.toml: %w", err)
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

		mgr := snapshot.NewManager(sshClient, cfg.App)

		snapshotID := ""
		if len(args) > 0 {
			snapshotID = args[0]
		} else {
			snapshots, err := mgr.List()
			if err != nil {
				return err
			}
			if len(snapshots) == 0 {
				return fmt.Errorf("no snapshots available")
			}
			snapshotID = snapshots[0]
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Rolling back to %s...\n", snapshotID)
		if err := mgr.Restore(snapshotID); err != nil {
			return fmt.Errorf("rollback: %w", err)
		}

		fmt.Fprintln(cmd.OutOrStdout(), "✓ Rollback complete")
		fmt.Fprintln(cmd.OutOrStdout(), "  Check status: ship status")
		return nil
	},
}
