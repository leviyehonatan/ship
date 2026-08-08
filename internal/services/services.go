package services

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/leviyehonatan/ship/internal/config"
	shipssh "github.com/leviyehonatan/ship/internal/ssh"
)

// Ensure provisions all services defined in ship.toml.
// Returns a map of service name → connection string for env injection.
func Ensure(client *shipssh.Client, cfg *config.ShipConfig, appName string, isLocal bool) (map[string]string, error) {
	env := make(map[string]string)
	networkName := fmt.Sprintf("ship-net-%s", appName)

	// Create shared bridge network for this app
	client.Run(fmt.Sprintf("docker network create %s 2>/dev/null || true", networkName))

	for name, svc := range cfg.Services {
		containerName := fmt.Sprintf("ship-svc-%s-%s", appName, name)

		// Check if already running
		status, _ := client.Run(fmt.Sprintf("docker ps --filter name=%s --format '{{.Status}}'", containerName))
		if strings.Contains(status, "Up") {
			continue
		}

		// Stop and remove old container
		client.Run(fmt.Sprintf("docker stop %s 2>/dev/null; docker rm %s 2>/dev/null", containerName, containerName))

		// Build run command with bridge network
		runArgs := fmt.Sprintf("-d --name %s --restart unless-stopped --network %s --network-alias %s",
			containerName, networkName, name)

		// Volume
		if svc.Volume != "" {
			var volPath string
			if isLocal {
				cwd, _ := os.Getwd()
				volPath = filepath.Join(cwd, ".ship-data", appName, svc.Volume)
			} else {
				volPath = fmt.Sprintf("/opt/ship/data/%s/%s", appName, svc.Volume)
			}
			os.MkdirAll(volPath, 0755)
			runArgs += fmt.Sprintf(" -v %s:%s", volPath, svc.Volume)
		}

		// Env vars for the service
		for k, v := range svc.Env {
			runArgs += fmt.Sprintf(" -e %s=%s", k, v)
		}

		client.Run(fmt.Sprintf("docker run %s %s", runArgs, svc.Image))

		// Connection strings use service name (bridge network DNS), not 127.0.0.1
		switch {
		case strings.Contains(svc.Image, "postgres"):
			pass := svc.Env["POSTGRES_PASSWORD"]
			if pass == "" {
				pass = "postgres"
			}
			db := svc.Env["POSTGRES_DB"]
			if db == "" {
				db = appName
			}
			env["DATABASE_URL"] = fmt.Sprintf("postgresql://postgres:%s@%s:%d/%s", pass, name, svc.Port, db)
		case strings.Contains(svc.Image, "redis"):
			env["REDIS_URL"] = fmt.Sprintf("redis://%s:%d", name, svc.Port)
		}
	}

	return env, nil
}

func Status(client *shipssh.Client, appName, serviceName string) (string, error) {
	containerName := fmt.Sprintf("ship-svc-%s-%s", appName, serviceName)
	out, err := client.Run(fmt.Sprintf("docker ps --filter name=%s --format '{{.Status}}'", containerName))
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}
