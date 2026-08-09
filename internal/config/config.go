package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

type ShipConfig struct {
	App      string             `toml:"app"`
	Server   string             `toml:"server"`
	Build    Build              `toml:"build"`
	Deploy   Deploy             `toml:"deploy"`
	Env      map[string]string  `toml:"env,omitempty"`
	Services map[string]Service `toml:"services,omitempty"`
	Volumes  []Volume           `toml:"volumes"`
	EnvFile  string             `toml:"env_file,omitempty"`
}

type Build struct {
	Dockerfile string   `toml:"dockerfile"`
	Ignore     []string `toml:"ignore"`
	Args       []string `toml:"args,omitempty"`
}

type Deploy struct {
	Port        int      `toml:"port"`
	Domains     []string `toml:"domains"`
	HealthCheck string   `toml:"health_check"`
}

type Volume struct {
	Path string `toml:"path"`
	Size string `toml:"size"`
}

type Service struct {
	Image  string `toml:"image"`
	Port   int    `toml:"port"`
	Volume string `toml:"volume,omitempty"`
	Env    map[string]string `toml:"env,omitempty"`
}

func Load(path string) (*ShipConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	var cfg ShipConfig
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	if cfg.Build.Dockerfile == "" {
		cfg.Build.Dockerfile = "Dockerfile"
	}
	if cfg.Deploy.Port == 0 {
		cfg.Deploy.Port = 8080
	}
	if cfg.Deploy.HealthCheck == "" {
		cfg.Deploy.HealthCheck = "/health"
	}
	if cfg.EnvFile == "" {
		cfg.EnvFile = ".env"
	}
	return &cfg, nil
}

func DefaultPath() string {
	return "ship.toml"
}

func SetServer(path string, server string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading %s: %w", path, err)
	}
	var cfg ShipConfig
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("parsing %s: %w", path, err)
	}
	cfg.Server = server
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return toml.NewEncoder(f).Encode(cfg)
}

func HomeDir() string {
	home, _ := os.UserHomeDir()
	return home
}

func StateDir() string {
	return filepath.Join(HomeDir(), ".config", "ship")
}

func ServersDir() string {
	return filepath.Join(StateDir(), "servers")
}
