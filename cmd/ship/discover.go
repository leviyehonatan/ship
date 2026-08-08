package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/leviyehonatan/ship/internal/detect"
	"github.com/spf13/cobra"
)

var discoverCmd = &cobra.Command{
	Use:   "discover [provider]",
	Short: "Show existing infrastructure on a provider",
	Long: `Discovers your existing servers, volumes, and SSH keys.
If provider is omitted, auto-detects the first configured one.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		providerName := "hetzner"
		if len(args) > 0 {
			providerName = args[0]
		}

		// Find the provider definition for auth check
		var sp *detect.SystemProvider
		for i := range detect.SystemProviders {
			if detect.SystemProviders[i].Name == providerName {
				sp = &detect.SystemProviders[i]
				break
			}
		}
		if sp == nil {
			return fmt.Errorf("unknown provider %q", providerName)
		}

		if err := detect.MustAuth(*sp); err != nil {
			return err
		}

		p, err := mustProvider(providerName)
		if err != nil {
			return err
		}

		ctx := context.Background()

		// Servers
		servers, err := p.ListServers(ctx)
		if err != nil {
			return fmt.Errorf("listing servers: %w", err)
		}

		fmt.Fprintln(cmd.OutOrStdout(), "SERVERS:")
		if len(servers) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "  (none)")
			fmt.Fprintf(cmd.OutOrStdout(), "\n  Create one: ship servers create my-server\n")
			return nil
		}

		fmt.Fprintf(cmd.OutOrStdout(), "  %-12s  %-20s  %-10s  %-8s  %-15s  %s\n",
			"ID", "NAME", "STATUS", "SIZE", "IP", "REGION")
		for _, s := range servers {
			shortID := s.ID
			if len(shortID) > 10 {
				shortID = shortID[:10]
			}
			fmt.Fprintf(cmd.OutOrStdout(), "  %-12s  %-20s  %-10s  %-8s  %-15s  %s\n",
				shortID, s.Name, s.Status, s.Size, s.PublicIPv4, s.Region)
		}

		// Suggest setup
		if len(servers) == 1 {
			s := servers[0]
			fmt.Fprintf(cmd.OutOrStdout(), "\n  Use this server?\n")
			fmt.Fprintf(cmd.OutOrStdout(), "    ship setup --server %s    # install Docker + Caddy\n", s.PublicIPv4)
			fmt.Fprintf(cmd.OutOrStdout(), "    ship deploy               # build and deploy\n")
		} else {
			fmt.Fprintf(cmd.OutOrStdout(), "\n  Pick a server by name or IP:\n")
			fmt.Fprintf(cmd.OutOrStdout(), "    ship setup --server <name-or-ip>\n")
			fmt.Fprintf(cmd.OutOrStdout(), "    ship deploy\n")
		}

		return nil
	},
}

// resolveServer takes a name or IP and returns the IP address.
// If the input is already an IP, returns it unchanged.
// Otherwise looks up the server by name in the provider.
func resolveServer(providerName, nameOrIP string) (string, error) {
	// Direct address: IP, IP:port, hostname:port
	if strings.HasPrefix(nameOrIP, "localhost") {
		return nameOrIP, nil
	}
	if strings.Count(nameOrIP, ".") == 3 {
		return nameOrIP, nil
	}
	// Contains port suffix — use directly
	if strings.Contains(nameOrIP, ":") {
		return nameOrIP, nil
	}

	p, err := mustProvider(providerName)
	if err != nil {
		return "", err
	}

	servers, err := p.ListServers(context.Background())
	if err != nil {
		return "", fmt.Errorf("looking up %s: %w", nameOrIP, err)
	}

	for _, s := range servers {
		if s.Name == nameOrIP || s.ID == nameOrIP {
			return s.PublicIPv4, nil
		}
	}

	return "", fmt.Errorf("server %q not found on %s — check the name with 'ship discover'", nameOrIP, providerName)
}
