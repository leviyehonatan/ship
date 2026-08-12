package main

import (
	"fmt"

	"github.com/leviyehonatan/ship/internal/config"
	shipssh "github.com/leviyehonatan/ship/internal/ssh"
	"github.com/spf13/cobra"
)

type shipCtx struct {
	Config *config.ShipConfig
	SSH    *shipssh.Client
	IP     string
	Dir    string
}

func newShipCtx(cmd *cobra.Command) (*shipCtx, error) {
	cfg, err := config.Load(config.DefaultPath())
	if err != nil {
		return nil, fmt.Errorf("loading ship.toml: %w\n  run 'ship init' first", err)
	}

	serverFlag, _ := cmd.Flags().GetString("server")
	if serverFlag == "" {
		serverFlag = cfg.Server
	}
	if serverFlag == "" {
		return nil, fmt.Errorf("no server — set 'server' in ship.toml or use 'ship use <name>'")
	}

	ip, err := resolveServer("", serverFlag)
	if err != nil {
		return nil, err
	}

	sshClient, err := shipssh.NewClientInsecure(ip, "root", "")
	if err != nil {
		return nil, err
	}

	return &shipCtx{
		Config: cfg,
		SSH:    sshClient,
		IP:     ip,
	}, nil
}

func (c *shipCtx) Run(cmd string) (string, error) {
	return c.SSH.Run(cmd)
}

func (c *shipCtx) Runf(format string, args ...interface{}) (string, error) {
	return c.SSH.Run(fmt.Sprintf(format, args...))
}
