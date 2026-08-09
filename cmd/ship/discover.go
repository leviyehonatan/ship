package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/leviyehonatan/ship/internal/detect"
	"github.com/leviyehonatan/ship/internal/provider"
	"github.com/leviyehonatan/ship/internal/state"
	"github.com/spf13/cobra"
)

var discoverCmd = &cobra.Command{
	Use:   "discover [provider]",
	Short: "Show existing infrastructure",
	Long: `Lists servers from all configured providers.
Specify a provider name to filter to just one.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// Filter to a specific provider if requested
		if len(args) > 0 {
			sp := detect.FindProvider(args[0])
			if sp == nil {
				return fmt.Errorf("unknown provider %q", args[0])
			}
			if err := detect.MustAuth(*sp); err != nil {
				return err
			}
			p, err := mustProvider(sp.Name)
			if err != nil {
				return err
			}
			return printServers(cmd, p, sp.Name)
		}

		// Show all configured providers
		anyFound := false
		for _, sp := range detect.SystemProviders {
			if detect.DetectSystem(sp).Status != detect.StatusReady &&
				detect.DetectSystem(sp).Status != detect.StatusWarning {
				continue
			}
			p, err := mustProvider(sp.Name)
			if err != nil {
				continue
			}
			anyFound = true
			printServers(cmd, p, sp.Name)
		}
		if !anyFound {
			fmt.Fprintln(cmd.OutOrStdout(), "No servers found on any provider.")
			fmt.Fprintln(cmd.OutOrStdout(), "  No providers configured. Install a provider CLI to get started.")
		}
		return nil
	},
}

func printServers(cmd *cobra.Command, p provider.Provider, providerName string) error {
	servers, err := p.ListServers(context.Background())
	if err != nil {
		return fmt.Errorf("listing %s servers: %w", providerName, err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "\n%s:\n", providerName)
	if len(servers) == 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "  (no servers)\n")
		fmt.Fprintf(cmd.OutOrStdout(), "  Create: ship servers create my-server --provider %s\n", providerName)
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
	return nil
}

// resolveServer takes a name or IP and returns the IP address.
// If the input is already an IP, returns it unchanged.
// Otherwise looks up the server by name in the provider.
func resolveServer(providerName, nameOrIP string) (string, error) {
	if nameOrIP == "" {
		current := state.Current()
		if current != "" {
			s, err := state.LoadServer(current)
			if err == nil && s.IP != "" {
				return s.IP, nil
			}
		}
		return "", fmt.Errorf("no server set — use 'ship server use <name>' or set 'server' in ship.toml")
	}

	// provider/name syntax
	if before, after, ok := strings.Cut(nameOrIP, "/"); ok {
		providerName = before
		nameOrIP = after
	}

	// Direct address: IP, IP:port, hostname:port
	if strings.HasPrefix(nameOrIP, "localhost") || strings.Count(nameOrIP, ".") == 3 || strings.Contains(nameOrIP, ":") {
		return nameOrIP, nil
	}

	// Local cache
	if s, err := state.LoadServer(nameOrIP); err == nil && s.IP != "" {
		return s.IP, nil
	}

	// Live lookup — search specified provider or all providers
	search := func(p provider.Provider) (provider.Server, bool) {
		servers, err := p.ListServers(context.Background())
		if err != nil {
			return provider.Server{}, false
		}
		for _, s := range servers {
			if s.Name == nameOrIP || s.ID == nameOrIP {
				return s, true
			}
		}
		return provider.Server{}, false
	}

	providers := []string{}
	if providerName != "" {
		providers = append(providers, providerName)
	} else {
		for _, sp := range detect.SystemProviders {
			providers = append(providers, sp.Name)
		}
	}

	for _, pn := range providers {
		p, err := mustProvider(pn)
		if err != nil {
			continue
		}
		if s, ok := search(p); ok {
			// Cache result locally
			state.SaveServer(state.Server{
				Name:     s.Name,
				ID:       s.ID,
				IP:       s.PublicIPv4,
				Provider: pn,
				Size:     s.Size,
				Region:   s.Region,
			})
			return s.PublicIPv4, nil
		}
	}

	return "", fmt.Errorf("server %q not found — check with 'ship discover'", nameOrIP)
}
