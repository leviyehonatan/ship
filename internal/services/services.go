package services

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/leviyehonatan/ship/internal/config"
	deploy "github.com/leviyehonatan/ship/internal/docker"
	shipssh "github.com/leviyehonatan/ship/internal/ssh"
)

// Ensure provisions all services defined in ship.toml.
// Returns a map of service name → connection string for env injection.
func Ensure(client *shipssh.Client, cfg *config.ShipConfig, appName string, isLocal bool) (map[string]string, error) {
	env := make(map[string]string)
	networkName := deploy.Network(appName)

	// Create shared bridge network for this app
	deploy.RunDocker(client, fmt.Sprintf("docker network create %s 2>/dev/null || true", networkName))

	for name, svc := range cfg.Services {
		containerName := deploy.ServiceContainer(appName, name)

		// Check if already running
		status, _ := deploy.RunDocker(client, fmt.Sprintf("docker ps --filter name=%s --format '{{.Status}}'", containerName))
		if strings.Contains(status, "Up") {
			continue
		}

		// Replace a previous ship-managed container; never touch a foreign one.
		if err := deploy.RemoveIfManaged(client, containerName); err != nil {
			return env, err
		}

		// Build run command with bridge network + DNS alias + labels
		runArgs := fmt.Sprintf("-d --name %s --restart unless-stopped --network %s --network-alias %s --label %s=true --label %s=%s",
			containerName, networkName, name, deploy.ManagedLabel, deploy.AppLabel, appName)

		// Volume — auto-create if not specified
		volPath := svc.Volume
		if volPath == "" {
			volPath = fmt.Sprintf("/data/%s", name) // default: /data/postgres, /data/redis, etc.
		}
		var hostPath string
		if isLocal {
			cwd, _ := os.Getwd()
			hostPath = filepath.Join(cwd, ".ship-data", appName, volPath)
			os.MkdirAll(hostPath, 0755)
		} else {
			hostPath = fmt.Sprintf("/opt/ship/data/%s%s", appName, volPath)
			deploy.RunDocker(client, fmt.Sprintf("mkdir -p %s", hostPath))
		}
		runArgs += fmt.Sprintf(" -v %s:%s", hostPath, volPath)

		// Env vars for the service
		for k, v := range svc.Env {
			runArgs += fmt.Sprintf(" -e %s=%s", k, v)
		}

		// Connection strings use the service's DNS alias (bridge network),
		// not 127.0.0.1. Auto-generate credentials where sensible.
		switch {
		case strings.Contains(svc.Image, "postgres"):
			pass := svc.Env["POSTGRES_PASSWORD"]
			if pass == "" {
				pass = randomPassword()
				runArgs += fmt.Sprintf(" -e POSTGRES_PASSWORD=%s", pass)
			}
			db := svc.Env["POSTGRES_DB"]
			if db == "" {
				db = appName
			}
			env["DATABASE_URL"] = fmt.Sprintf("postgresql://postgres:%s@%s:%d/%s", pass, name, svc.Port, db)
		case strings.Contains(svc.Image, "redis"):
			env["REDIS_URL"] = fmt.Sprintf("redis://%s:%d", name, svc.Port)
		}

		deploy.RunDocker(client, fmt.Sprintf("docker run %s %s", runArgs, svc.Image))
	}

	return env, nil
}

func Status(client *shipssh.Client, appName, serviceName string) (string, error) {
	containerName := deploy.ServiceContainer(appName, serviceName)
	out, err := deploy.RunDocker(client, fmt.Sprintf("docker ps --filter name=%s --format '{{.Status}}'", containerName))
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

func randomPassword() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)[:16]
}
