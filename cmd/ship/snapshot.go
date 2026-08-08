package main

import (
	"fmt"

	"github.com/leviyehonatan/ship/internal/snapshot"
	"github.com/spf13/cobra"
)

var snapshotCmd = &cobra.Command{
	Use:   "snapshot",
	Short: "Create a database snapshot for rollback",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := newShipCtx(cmd)
		if err != nil {
			return err
		}
		mgr := snapshot.NewManager(ctx.SSH, ctx.Config.App)
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
		ctx, err := newShipCtx(cmd)
		if err != nil {
			return err
		}
		mgr := snapshot.NewManager(ctx.SSH, ctx.Config.App)
		snapshots, err := mgr.List()
		if err != nil {
			return err
		}
		if len(snapshots) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "No snapshots.")
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
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := newShipCtx(cmd)
		if err != nil {
			return err
		}
		mgr := snapshot.NewManager(ctx.SSH, ctx.Config.App)
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
		return nil
	},
}
