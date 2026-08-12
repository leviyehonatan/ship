package deploy

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// DetectSocket finds the Docker socket path using multiple strategies:
//  1. DOCKER_HOST env var
//  2. `docker context inspect` (works with colima, Orbstack, etc.)
//  3. Common well-known paths (Docker Desktop, colima, Orbstack)
//
// Returns "/var/run/docker.sock" as a fallback.
func DetectSocket() string {
	// 1. DOCKER_HOST env var
	if host := os.Getenv("DOCKER_HOST"); host != "" {
		sock := strings.TrimPrefix(host, "unix://")
		if _, err := os.Stat(sock); err == nil {
			return sock
		}
	}

	// 2. docker context inspect (works with colima, Orbstack, etc.)
	if out, err := exec.Command("docker", "context", "inspect", "--format", "{{.Endpoints.docker.Host}}").Output(); err == nil {
		sock := strings.TrimPrefix(strings.TrimSpace(string(out)), "unix://")
		if _, err := os.Stat(sock); err == nil {
			return sock
		}
	}

	// 3. Common paths
	home, _ := os.UserHomeDir()
	candidates := []string{
		"/var/run/docker.sock",
		filepath.Join(home, ".docker", "run", "docker.sock"),
		filepath.Join(home, ".colima", "default", "docker.sock"),
		filepath.Join(home, ".colima", "docker.sock"),
		filepath.Join(home, ".orbstack", "run", "docker.sock"),
	}
	for _, sock := range candidates {
		if _, err := os.Stat(sock); err == nil {
			return sock
		}
	}

	return "/var/run/docker.sock" // fallback
}
