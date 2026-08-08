package main

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"strings"

	"github.com/spf13/cobra"
)

var tunnelCmd = &cobra.Command{
	Use:       "tunnel [service]",
	Short:     "Open an SSH tunnel to a database",
	Args:      cobra.ExactArgs(1),
	ValidArgs: []string{"db", "couch", "redis"},
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := newShipCtx(cmd)
		if err != nil {
			return err
		}

		sshHost := ctx.IP
		sshPort := "22"
		if strings.Contains(ctx.IP, ":") {
			if host, port, err := net.SplitHostPort(ctx.IP); err == nil {
				sshHost, sshPort = host, port
			}
		}

		var remotePort int
		switch args[0] {
		case "db":
			remotePort = 5432
		case "couch":
			remotePort = 5984
		case "redis":
			remotePort = 6379
		default:
			return fmt.Errorf("unknown service %q — use: db, couch, redis", args[0])
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
