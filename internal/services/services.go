package services

import (
	"crypto/rand"
	"encoding/hex"
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

		// Volume — auto-create if not specified
		volPath := svc.Volume
		if volPath == "" {
			volPath = fmt.Sprintf("/data/%s", name) // default: /data/postgres, /data/redis, etc.
		}
		var hostPath string
		if isLocal {
			cwd, _ := os.Getwd()
			hostPath = filepath.Join(cwd, ".ship-data", appName, volPath)
		} else {
			hostPath = fmt.Sprintf("/opt/ship/data/%s%s", appName, volPath)
		}
		os.MkdirAll(hostPath, 0755)
		runArgs += fmt.Sprintf(" -v %s:%s", hostPath, volPath)

		// Env vars for the service
		for k, v := range svc.Env {
			runArgs += fmt.Sprintf(" -e %s=%s", k, v)
		}

		// Auto-generate password for Postgres if not set
		if strings.Contains(svc.Image, "postgres") {
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
		}

		// Auto-generate REDIS_URL
		if strings.Contains(svc.Image, "redis") {
			redisURL := fmt.Sprintf("redis://%s:%d", name, svc.Port)
			env["REDIS_URL"] = redisURL
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

func randomPassword() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)[:16]
}
