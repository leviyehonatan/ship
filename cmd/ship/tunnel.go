package main

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"strings"

	"github.com/leviyehonatan/ship/internal/config"
	"github.com/spf13/cobra"
)

var tunnelCmd = &cobra.Command{
	Use:   "tunnel [service]",
	Short: "Open an SSH tunnel to a service",
	Long:  `Forwards a port from a service on the server to your local machine.`,
	Args:  cobra.MaximumNArgs(1),
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

		sshHost, sshPort := ip, "22"
		if strings.Contains(ip, ":") {
			if h, p, e := net.SplitHostPort(ip); e == nil {
				sshHost, sshPort = h, p
			}
		}

		// Default: tunnel to the app
		remotePort := cfg.Deploy.Port
		if remotePort == 0 {
			remotePort = 8080
		}
		serviceName := cfg.App

		// If a service name is given, look it up in ship.toml [services]
		if len(args) > 0 {
			serviceName = args[0]
			if svc, ok := cfg.Services[serviceName]; ok {
				remotePort = svc.Port
			} else {
				// Try legacy aliases
				switch serviceName {
				case "db":
					remotePort = 5432
				case "couch":
					remotePort = 5984
				case "redis":
					remotePort = 6379
				}
			}
		}

		sshArgs := []string{"-N", "-q", "-L", fmt.Sprintf("%d:127.0.0.1:%d", remotePort, remotePort),
			fmt.Sprintf("root@%s", sshHost), "-p", sshPort,
			"-o", "StrictHostKeyChecking=no", "-o", "UserKnownHostsFile=/dev/null",
			"-o", "ExitOnForwardFailure=yes"}

		sshBin := "ssh"
		if os.Getenv("SSHPASS") != "" {
			sshBin = "sshpass"
			sshArgs = append([]string{"-p", os.Getenv("SSHPASS"), "ssh"}, sshArgs...)
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Tunneling %s:%d to localhost:%d\n", sshHost, remotePort, remotePort)
		fmt.Fprintf(cmd.OutOrStdout(), "  Press Ctrl+C to disconnect\n")

		sshCmd := exec.Command(sshBin, sshArgs...)
		sshCmd.Stdin = os.Stdin
		sshCmd.Stdout = os.Stdout
		sshCmd.Stderr = os.Stderr
		if err := sshCmd.Start(); err != nil {
			return fmt.Errorf("tunnel: %w", err)
		}

		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, os.Interrupt)
		done := make(chan error, 1)
		go func() { done <- sshCmd.Wait() }()

		select {
		case <-sigCh:
			sshCmd.Process.Signal(os.Interrupt)
			fmt.Fprintln(cmd.OutOrStdout(), "\n  Tunnel closed.")
		case err := <-done:
			if err != nil {
				return fmt.Errorf("tunnel: %w", err)
			}
		}
		return nil
	},
}
