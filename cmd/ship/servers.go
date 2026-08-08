package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/leviyehonatan/ship/internal/detect"
	"github.com/leviyehonatan/ship/internal/provider"
	"github.com/leviyehonatan/ship/internal/state"
	"github.com/leviyehonatan/ship/providers/aws"
	"github.com/leviyehonatan/ship/providers/digitalocean"
	"github.com/leviyehonatan/ship/providers/gcp"
	"github.com/leviyehonatan/ship/providers/hetzner"
	"github.com/leviyehonatan/ship/providers/linode"
	"github.com/leviyehonatan/ship/providers/vultr"
	"github.com/spf13/cobra"
)

var serversCmd = &cobra.Command{
	Use:   "servers",
	Short: "Manage your VPS servers",
}

var serverUseCmd = &cobra.Command{
	Use:   "use [name]",
	Short: "Set the default server",
	Long: `Without arguments, shows available servers from all configured providers.
With a name, sets it as the default target for all commands.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			// Show available servers
			fmt.Fprintln(cmd.OutOrStdout(), "Available servers:")
			for _, sp := range detect.SystemProviders {
				p, _ := mustProvider(sp.Name)
				if p == nil {
					continue
				}
				servers, err := p.ListServers(context.Background())
				if err != nil || len(servers) == 0 {
					continue
				}
				fmt.Fprintf(cmd.OutOrStdout(), "  %s:\n", sp.Name)
				for _, s := range servers {
					marker := ""
					if state.Current() == s.Name {
						marker = " ← current"
					}
					fmt.Fprintf(cmd.OutOrStdout(), "    %-20s  %-8s  %-15s  %s%s\n",
						s.Name, s.Size, s.PublicIPv4, s.Status, marker)
				}
			}
			fmt.Fprintf(cmd.OutOrStdout(), "\n  Set default: ship use <name>\n")
			return nil
		}

		if err := state.SetCurrent(args[0]); err != nil {
			return err
		}
		s, err := state.LoadServer(args[0])
		if err != nil {
			fmt.Fprintf(cmd.OutOrStdout(), "✓ Default server: %s\n", args[0])
			return nil
		}
		fmt.Fprintf(cmd.OutOrStdout(), "✓ Default server: %s (%s, %s)\n", s.Name, s.IP, s.Size)
		return nil
	},
}

var serverCreateCmd = &cobra.Command{
	Use:   "create [name]",
	Short: "Provision a new server",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := "ship-server"
		if len(args) > 0 {
			name = args[0]
		}

		region, _ := cmd.Flags().GetString("region")
		size, _ := cmd.Flags().GetString("size")
		image, _ := cmd.Flags().GetString("image")
		keyPath, _ := cmd.Flags().GetString("key")
		providerName, _ := cmd.Flags().GetString("provider")
		if providerName == "" || providerName == "hetzner" {
			// Auto-detect first available provider
			for _, sp := range detect.SystemProviders {
				if detect.DetectSystem(sp).Status == detect.StatusReady {
					providerName = sp.Name
					break
				}
			}
			if providerName == "" {
				return fmt.Errorf("no provider configured — install one: brew install hcloud")
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Using provider: %s\n", providerName)
		}

		if keyPath == "" {
			home, _ := os.UserHomeDir()
			keyPath = filepath.Join(home, ".ssh", "id_rsa.pub")
		}

		p, err := mustProvider(providerName)
		if err != nil {
			return err
		}

		ctx := context.Background()

		pubKey, err := os.ReadFile(keyPath)
		if err != nil {
			return fmt.Errorf("reading SSH key %s: %w", keyPath, err)
		}

		sshKey, err := p.CreateSSHKey(ctx, "ship-"+name, pubKey)
		if err != nil {
			return fmt.Errorf("uploading SSH key: %w", err)
		}

		server, err := p.CreateServer(ctx, provider.CreateServerOpts{
			Name:      name,
			Region:    region,
			Size:      size,
			Image:     image,
			SSHKeyIDs: []string{sshKey.ID},
		})
		if err != nil {
			return fmt.Errorf("creating server: %w", err)
		}

		fmt.Fprintf(cmd.OutOrStdout(), "✓ Server %s created\n", server.Name)
		fmt.Fprintf(cmd.OutOrStdout(), "  ID:       %s\n", server.ID)
		fmt.Fprintf(cmd.OutOrStdout(), "  IPv4:     %s\n", server.PublicIPv4)
		fmt.Fprintf(cmd.OutOrStdout(), "  IPv6:     %s\n", server.PublicIPv6)
		fmt.Fprintf(cmd.OutOrStdout(), "  Region:   %s\n", server.Region)
		fmt.Fprintf(cmd.OutOrStdout(), "  Size:     %s\n", server.Size)
		fmt.Fprintf(cmd.OutOrStdout(), "  SSH key:  %s\n", sshKey.Name)

		// Save to state
		state.SaveServer(state.Server{
			Name:     server.Name,
			ID:       server.ID,
			IP:       server.PublicIPv4,
			Provider: providerName,
			Size:     server.Size,
			Region:   server.Region,
		})
		state.SetCurrent(server.Name)
		fmt.Fprintf(cmd.OutOrStdout(), "\n  Set as current server. Use 'ship server use <name>' to switch.\n")
		return nil
	},
}

var serverListCmd = &cobra.Command{
	Use:   "list",
	Short: "List your servers",
	RunE: func(cmd *cobra.Command, args []string) error {
		p, err := mustProvider("hetzner")
		if err != nil {
			return err
		}

		servers, err := p.ListServers(context.Background())
		if err != nil {
			return fmt.Errorf("listing servers: %w", err)
		}

		if len(servers) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "No servers found.")
			return nil
		}

		fmt.Fprintf(cmd.OutOrStdout(), "%-20s  %-12s  %-10s  %-15s  %s\n", "NAME", "STATUS", "SIZE", "IPV4", "REGION")
		for _, s := range servers {
			fmt.Fprintf(cmd.OutOrStdout(), "%-20s  %-12s  %-10s  %-15s  %s\n",
				s.Name, s.Status, s.Size, s.PublicIPv4, s.Region)
		}
		return nil
	},
}

var serverDeleteCmd = &cobra.Command{
	Use:   "delete [id]",
	Short: "Delete a server",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		p, err := mustProvider("hetzner")
		if err != nil {
			return err
		}

		if err := p.DeleteServer(context.Background(), args[0]); err != nil {
			return fmt.Errorf("deleting server: %w", err)
		}

		fmt.Fprintf(cmd.OutOrStdout(), "✓ Server %s deleted\n", args[0])
		return nil
	},
}

var regionsCmd = &cobra.Command{
	Use:   "regions",
	Short: "List available regions",
	RunE: func(cmd *cobra.Command, args []string) error {
		p, err := mustProvider("hetzner")
		if err != nil {
			return err
		}

		regions, err := p.ListRegions(context.Background())
		if err != nil {
			return err
		}

		for _, r := range regions {
			fmt.Fprintf(cmd.OutOrStdout(), "  %-6s  %s\n", r.ID, r.City)
		}
		return nil
	},
}

var sizesCmd = &cobra.Command{
	Use:   "sizes",
	Short: "List available server sizes with pricing",
	RunE: func(cmd *cobra.Command, args []string) error {
		p, err := mustProvider("hetzner")
		if err != nil {
			return err
		}

		sizes, err := p.ListSizes(context.Background())
		if err != nil {
			return err
		}

		fmt.Fprintf(cmd.OutOrStdout(), "%-8s  %6s  %10s  %8s  %8s\n", "SIZE", "VCPU", "RAM", "DISK", "PRICE")
		for _, s := range sizes {
			fmt.Fprintf(cmd.OutOrStdout(), "%-8s  %2d vCPU  %5.1fGB RAM  %4dGB disk  $%7.2f/mo\n",
				s.ID, s.VCPUs, s.MemoryGB, s.DiskGB, s.MonthlyPrice)
		}
		return nil
	},
}

func mustProvider(name string) (provider.Provider, error) {
	switch name {
	case "hetzner":
		if err := detect.MustAuth(detect.SystemProviders[0]); err != nil {
			return nil, err
		}
		return hetzner.New(), nil
	case "linode":
		if err := detect.MustAuth(detect.SystemProviders[1]); err != nil {
			return nil, err
		}
		return linode.New(), nil
	case "digitalocean":
		if err := detect.MustAuth(detect.SystemProviders[2]); err != nil {
			return nil, err
		}
		return digitalocean.New(), nil
	case "vultr":
		if err := detect.MustAuth(detect.SystemProviders[3]); err != nil {
			return nil, err
		}
		return vultr.New(), nil
	case "aws":
		if err := detect.MustAuth(detect.SystemProviders[4]); err != nil {
			return nil, err
		}
		return aws.New(), nil
	case "gcp":
		if err := detect.MustAuth(detect.SystemProviders[5]); err != nil {
			return nil, err
		}
		return gcp.New(), nil
	default:
		return nil, fmt.Errorf("unknown provider %q — supported: hetzner, linode, digitalocean, vultr", name)
	}
}
