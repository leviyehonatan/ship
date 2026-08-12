package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/leviyehonatan/ship/internal/config"
	"github.com/leviyehonatan/ship/internal/state"
	"github.com/spf13/cobra"
)

var scaleCmd = &cobra.Command{
	Use:   "scale",
	Short: "Show or change server size",
	Long: `Without flags, shows the current server size and available options.
With --size, resizes the server to the specified type.`,
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

		ip, err := resolveServer("", serverAddr)
		if err != nil {
			return err
		}

		// Detect local Docker — no resize possible
		if strings.HasPrefix(ip, "localhost") || strings.HasPrefix(ip, "127.") {
			fmt.Fprintf(cmd.OutOrStdout(), "Server: %s (local Docker)\n", ip)
			fmt.Fprintf(cmd.OutOrStdout(), "Size:   not applicable (using host resources)\n")
			fmt.Fprintf(cmd.OutOrStdout(), "\nScale is only available for cloud VPS servers.\n")
			fmt.Fprintf(cmd.OutOrStdout(), "To resize a Docker container: docker update --memory 4g <container>\n")
			return nil
		}

		sizeFlag, _ := cmd.Flags().GetString("size")

		// Find which provider owns this server
		srv, err := findServerByIP(ip)
		if err != nil {
			return err
		}
		providerName := srv.Provider
		if providerName == "" {
			providerName = "hetzner"
		}

		p, err := mustProvider(providerName)
		if err != nil {
			return err
		}
		ctx := context.Background()

		if sizeFlag == "" {
			fmt.Fprintf(cmd.OutOrStdout(), "Current: %s (%s, %s)\n\n", srv.Name, srv.Size, ip)
			fmt.Fprintf(cmd.OutOrStdout(), "Available sizes:\n")

			sizes, err := p.ListSizes(ctx)
			if err == nil {
				for _, s := range sizes {
					fmt.Fprintf(cmd.OutOrStdout(), "  %-8s  %2d vCPU  %5.1fGB RAM  %4dGB disk  $%7.2f/mo\n",
						s.ID, s.VCPUs, s.MemoryGB, s.DiskGB, s.MonthlyPrice)
				}
			} else {
				// Fallback to raw hcloud output
				c := exec.Command("hcloud", "server-type", "list", "--output", "json")
				out, _ := c.Output()
				os.Stdout.Write(out)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "\n  Resize: ship scale --size cx33\n")
			return nil
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Resizing %s to %s...\n", srv.Name, sizeFlag)
		yes, _ := cmd.Flags().GetBool("yes")
		if !yes {
			fmt.Fprintf(cmd.OutOrStdout(), "  Server will be powered off, resized, and restarted.\n")
			fmt.Fprintf(cmd.OutOrStdout(), "  Continue? [y/N] ")
			var confirm string
			fmt.Scanln(&confirm)
			if strings.ToLower(confirm) != "y" {
				return fmt.Errorf("cancelled")
			}
		}

		if err := p.ShutdownServer(ctx, srv.ID); err != nil {
			return fmt.Errorf("power off: %w", err)
		}
		fmt.Fprintln(cmd.OutOrStdout(), "  Server powered off")

		if err := p.ResizeServer(ctx, srv.ID, sizeFlag); err != nil {
			// Try to power back on even if resize fails
			p.PowerOnServer(ctx, srv.ID)
			return fmt.Errorf("resize: %w", err)
		}

		if err := p.PowerOnServer(ctx, srv.ID); err != nil {
			fmt.Fprintf(cmd.OutOrStdout(), "  Warning: power on failed: %v\n", err)
		}

		srv.Size = sizeFlag
		state.SaveServer(*srv)

		fmt.Fprintf(cmd.OutOrStdout(), "✓ Server scaled to %s\n", sizeFlag)
		fmt.Fprintf(cmd.OutOrStdout(), "  SSH available in ~30s\n")
		return nil
	},
}

func findServerByIP(ip string) (*state.Server, error) {
	servers, err := state.ListServers()
	if err != nil {
		return nil, err
	}
	for _, s := range servers {
		if s.IP == ip || s.Name == ip {
			return &s, nil
		}
	}
	return &state.Server{Name: ip, IP: ip}, nil
}

func initScale() {
	scaleCmd.Flags().String("server", "", "Server name or IP")
	scaleCmd.Flags().String("size", "", "Target server size (e.g. cx33)")
	scaleCmd.Flags().Bool("yes", false, "Skip confirmation prompts")
}
