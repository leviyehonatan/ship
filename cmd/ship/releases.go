package main

import (
	"fmt"
	"strings"

	"github.com/leviyehonatan/ship/internal/releases"
	"github.com/spf13/cobra"
)

var releasesCmd = &cobra.Command{
	Use:   "releases",
	Short: "Show deployment history",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := newShipCtx(cmd)
		if err != nil {
			return err
		}

		rel, err := releases.List(ctx.SSH, ctx.Config.App)
		if err != nil {
			return err
		}
		if len(rel) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "No releases yet. Run 'ship deploy' first.")
			return nil
		}

		fmt.Fprintf(cmd.OutOrStdout(), "%-8s  %-20s  %-30s  %s\n", "VERSION", "TIMESTAMP", "IMAGE", "COMMIT")
		for _, r := range rel {
			marker := ""
			if r.Version == rel[len(rel)-1].Version {
				marker = " ← current"
			}
			ts := r.Timestamp
			if len(ts) > 19 {
				ts = ts[:19]
			}
			ts = strings.Replace(ts, "T", " ", 1)
			image := r.Image
			if len(image) > 28 {
				image = image[:28]
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%-8d  %-20s  %-30s  %s%s\n",
				r.Version, ts, image, r.Commit, marker)
		}
		return nil
	},
}
