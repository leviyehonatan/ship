package main

import (
	"fmt"
	"os"

	"github.com/leviyehonatan/ship/internal/detect"
	"github.com/spf13/cobra"
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:   "ship",
	Short: "ship — deploy containers to your own VPS",
	Long: `ship is a CLI tool that deploys your app to VPS providers
like Hetzner, Linode, DigitalOcean, and Vultr.

Just like fly deploy, but on your own servers.`,
	SilenceUsage: true,
}

func init() {
	initMigrate()
	initSecrets()
	initSSL()
	initPG()
	initScale()

	rootCmd.AddCommand(whoamiCmd)
	rootCmd.AddCommand(serversCmd)
	rootCmd.AddCommand(serverCreateCmd)
	rootCmd.AddCommand(serverListCmd)
	rootCmd.AddCommand(serverDeleteCmd)
	rootCmd.AddCommand(serverUseCmd)
	rootCmd.AddCommand(discoverCmd)
	rootCmd.AddCommand(migrateCmd)
	rootCmd.AddCommand(snapshotCmd)
	rootCmd.AddCommand(snapshotsCmd)
	rootCmd.AddCommand(rollbackCmd)
	rootCmd.AddCommand(secretsCmd)
	rootCmd.AddCommand(sslCmd)
	rootCmd.AddCommand(pgCmd)
	rootCmd.AddCommand(scaleCmd)
	rootCmd.AddCommand(releasesCmd)
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(setupCmd)
	rootCmd.AddCommand(deployCmd)
	rootCmd.AddCommand(logsCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(sshCmd)
	rootCmd.AddCommand(tunnelCmd)
	rootCmd.AddCommand(regionsCmd)
	rootCmd.AddCommand(sizesCmd)

	setupCmd.Flags().String("server", "", "Server name or IP")
	serverCreateCmd.Flags().String("region", "nbg1", "Region (e.g. nbg1, fsn1, hel1)")
	serverCreateCmd.Flags().String("size", "cx23", "Server size (e.g. cx23, cx33)")
	serverCreateCmd.Flags().String("image", "ubuntu-24.04", "OS image")
	serverCreateCmd.Flags().String("provider", "hetzner", "VPS provider")
	serverCreateCmd.Flags().String("key", "", "Path to SSH public key (default: ~/.ssh/id_rsa.pub)")

	initCmd.Flags().String("from", "", "Generate from existing config (e.g. --from fly)")
}

// ============================================================
// whoami — detect configured providers and platforms
// ============================================================

var whoamiCmd = &cobra.Command{
	Use:   "whoami",
	Short: "Show which providers and platforms are available",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Fprintln(cmd.OutOrStdout(), "Infrastructure (VPS providers):")
		for _, sp := range detect.SystemProviders {
			info := detect.DetectSystem(sp)
			line := fmt.Sprintf("  %s  %-14s  %s", info.Status.Icon(), info.Name, info.Status)
			if info.Version != "" {
				line += fmt.Sprintf(" (v%s)", info.Version)
			}
			fmt.Fprintln(cmd.OutOrStdout(), line)
			if info.Warning != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "      %s\n", info.Warning)
			}
		}

		fmt.Fprintln(cmd.OutOrStdout(), "\nPlatforms (migrate targets):")
		cwd, _ := os.Getwd()
		for _, pp := range detect.PlatformProviders {
			found := detect.DetectPlatformAt(pp, cwd)
			icon := "-"
			if found {
				icon = "✓"
			}
			fmt.Fprintf(cmd.OutOrStdout(), "  %s  %-14s\n", icon, pp.Name)
		}
		return nil
	},
}
