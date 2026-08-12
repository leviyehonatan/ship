package pg

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"

	deploy "github.com/leviyehonatan/ship/internal/docker"
	shipssh "github.com/leviyehonatan/ship/internal/ssh"
)

// Create provisions a Postgres container and returns containerID + password.
func Create(client *shipssh.Client, name, password string) (containerID string, generatedPassword string, err error) {
	containerName := deploy.PgContainer(name)

	// Replace a previous ship-managed container; never touch a foreign one.
	if err := deploy.RemoveIfManaged(client, containerName); err != nil {
		return "", "", err
	}

	if password == "" {
		password = randomPassword()
	}

	runCmd := fmt.Sprintf(
		`docker run -d --name %s --restart unless-stopped -p 5432:5432 --label %s=true --label %s=pg-%s -e POSTGRES_PASSWORD='%s' -e POSTGRES_DB=%s postgres:16-alpine`,
		containerName, deploy.ManagedLabel, deploy.AppLabel, name, password, name,
	)
	out, err := deploy.RunDocker(client, runCmd)
	if err != nil {
		return "", "", fmt.Errorf("starting postgres: %w", err)
	}
	return strings.TrimSpace(out), password, nil
}

// Status returns the uptime of a ship-managed Postgres container.
func Status(client *shipssh.Client, name string) (string, error) {
	containerName := deploy.PgContainer(name)
	out, err := deploy.RunDocker(client, fmt.Sprintf("docker ps --filter name=%s --format '{{.Status}}'", containerName))
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// ConnectionString returns a DATABASE_URL for the Postgres container.
func ConnectionString(serverIP, name, password string) string {
	return fmt.Sprintf("postgresql://postgres:%s@%s:5432/%s", password, serverIP, name)
}

// List returns the names of all ship-managed Postgres containers.
func List(client *shipssh.Client) ([]string, error) {
	out, err := deploy.RunDocker(client, "docker ps --filter name=ship-pg- --format '{{.Names}}'")
	if err != nil {
		return nil, err
	}
	var names []string
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if line != "" {
			names = append(names, strings.TrimPrefix(line, "ship-pg-"))
		}
	}
	return names, nil
}

func randomPassword() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)[:16]
}
