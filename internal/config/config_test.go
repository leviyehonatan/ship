package config

import (
	"os"
	"strings"
	"testing"
)

func TestLoad(t *testing.T) {
	toml := `
app = "test-app"

[build]
dockerfile = "Dockerfile"

[deploy]
port = 3000
health_check = "/api/health"
domains = ["example.com"]

[[volumes]]
path = "/data"
size = "10GB"
`
	os.WriteFile("test_ship.toml", []byte(toml), 0644)
	defer os.Remove("test_ship.toml")

	cfg, err := Load("test_ship.toml")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.App != "test-app" {
		t.Errorf("app = %q, want test-app", cfg.App)
	}
	if cfg.Build.Dockerfile != "Dockerfile" {
		t.Errorf("dockerfile = %q", cfg.Build.Dockerfile)
	}
	if cfg.Deploy.Port != 3000 {
		t.Errorf("port = %d", cfg.Deploy.Port)
	}
	if cfg.Deploy.HealthCheck != "/api/health" {
		t.Errorf("health = %q", cfg.Deploy.HealthCheck)
	}
	if len(cfg.Deploy.Domains) != 1 || cfg.Deploy.Domains[0] != "example.com" {
		t.Errorf("domains = %v", cfg.Deploy.Domains)
	}
	if len(cfg.Volumes) != 1 || cfg.Volumes[0].Path != "/data" {
		t.Errorf("volumes = %v", cfg.Volumes)
	}
}

func TestDefaults(t *testing.T) {
	toml := `app = "minimal"`
	os.WriteFile("test_minimal.toml", []byte(toml), 0644)
	defer os.Remove("test_minimal.toml")

	cfg, err := Load("test_minimal.toml")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.Deploy.Port != 8080 {
		t.Errorf("default port = %d, want 8080", cfg.Deploy.Port)
	}
	if cfg.Deploy.HealthCheck != "/health" {
		t.Errorf("default health = %q, want /health", cfg.Deploy.HealthCheck)
	}
	if cfg.EnvFile != ".env" {
		t.Errorf("default env = %q, want .env", cfg.EnvFile)
	}
}

func TestSetServer(t *testing.T) {
	path := t.TempDir() + "/ship.toml"
	content := `app = "test"

[build]
dockerfile = "Dockerfile"

[deploy]
port = 3000
`
	os.WriteFile(path, []byte(content), 0644)

	if err := SetServer(path, "my-server"); err != nil {
		t.Fatalf("SetServer: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !strings.Contains(string(data), `server = "my-server"`) {
		t.Errorf("expected server field, got:\n%s", string(data))
	}
	if !strings.Contains(string(data), `app = "test"`) {
		t.Error("lost app field")
	}
}
