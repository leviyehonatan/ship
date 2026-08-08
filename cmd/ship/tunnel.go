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
	Short: "Open an SSH tunnel to a database on the server",
	Long: `Sets up SSH port forwarding so you can connect local tools
(pgAdmin, DBeaver, redis-cli) to services running on the server.

Services:
  db       Postgres (localhost:5432 → server:5432)
  couch    CouchDB  (localhost:5984 → server:5984)
  redis    Redis    (localhost:6379 → server:6379)

Example:
  ship tunnel db          # then connect to localhost:5432 with pgAdmin
  ship tunnel couch       # then open http://localhost:5984/_utils`,
	Args:      cobra.ExactArgs(1),
	ValidArgs: []string{"db", "couch", "redis"},
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load(config.DefaultPath())
		if err != nil {
			return fmt.Errorf("loading ship.toml: %w", err)
		}
		if cfg.Server == "" {
			return fmt.Errorf("server not set in ship.toml")
		}

		ip, err := resolveServer("hetzner", cfg.Server)
		if err != nil {
			return err
		}

		// Strip port from IP if present (SSH -L needs host without port)
		sshHost := ip
		sshPort := "22"
		if strings.Contains(ip, ":") {
			host, port, err := net.SplitHostPort(ip)
			if err == nil {
				sshHost = host
				sshPort = port
			}
		}

		service := args[0]
		var remotePort int
		switch service {
		case "db":
			remotePort = 5432
		case "couch":
			remotePort = 5984
		case "redis":
			remotePort = 6379
		default:
			return fmt.Errorf("unknown service %q — use: db, couch, redis", service)
		}

		sshArgs := []string{
			"-N", "-q",
			"-L", fmt.Sprintf("%d:127.0.0.1:%d", remotePort, remotePort),
			fmt.Sprintf("root@%s", sshHost),
			"-p", sshPort,
			"-o", "StrictHostKeyChecking=no",
			"-o", "UserKnownHostsFile=/dev/null",
			"-o", "ExitOnForwardFailure=yes",
		}

		// Use password auth if SSHPASS is set
		sshBin := "ssh"
		if os.Getenv("SSHPASS") != "" {
			sshBin = "sshpass"
			sshArgs = append([]string{"-p", os.Getenv("SSHPASS"), "ssh"}, sshArgs...)
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Tunneling %s:%d to localhost:%d\n", sshHost, remotePort, remotePort)
		fmt.Fprintf(cmd.OutOrStdout(), "  Press Ctrl+C to disconnect\n\n")

		sshCmd := exec.Command(sshBin, sshArgs...)
		sshCmd.Stdin = os.Stdin
		sshCmd.Stdout = os.Stdout
		sshCmd.Stderr = os.Stderr

		if err := sshCmd.Start(); err != nil {
			return fmt.Errorf("starting tunnel: %w", err)
		}

		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, os.Interrupt)

		done := make(chan error, 1)
		go func() {
			done <- sshCmd.Wait()
		}()

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
