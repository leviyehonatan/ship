package main

import (
	"fmt"
	"strings"

	"github.com/leviyehonatan/ship/internal/config"
	"github.com/leviyehonatan/ship/internal/releases"
	shipssh "github.com/leviyehonatan/ship/internal/ssh"
	"github.com/spf13/cobra"
)

var releasesCmd = &cobra.Command{
	Use:   "releases",
	Short: "Show deployment history",
	Long:  `Lists all deployments with version, timestamp, and image reference.`,
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

		rel, err := releases.List(sshClient, cfg.App)
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
