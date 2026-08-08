package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

var binary string

func TestMain(m *testing.M) {
	// Build the binary once for all tests
	tmpDir, err := os.MkdirTemp("", "ship-test-*")
	if err != nil {
		os.Exit(1)
	}
	binary = filepath.Join(tmpDir, "ship")
	build := exec.Command("go", "build", "-o", binary, "github.com/leviyehonatan/ship/cmd/ship")
	build.Stderr = os.Stderr
	build.Stdout = os.Stdout
	if err := build.Run(); err != nil {
		os.Exit(1)
	}

	code := m.Run()
	os.RemoveAll(tmpDir)
	os.Exit(code)
}

func run(args ...string) (string, error) {
	cmd := exec.Command(binary, args...)
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()
	return string(out), err
}

func TestHelp(t *testing.T) {
	out, err := run("--help")
	if err != nil {
		t.Fatalf("ship --help: %v", err)
	}
	expected := []string{
		"deploy", "setup", "snapshot", "rollback",
		"tunnel", "migrate", "whoami", "discover",
		"sizes", "regions", "init", "status", "logs", "ssh",
	}
	for _, cmd := range expected {
		if !strings.Contains(out, cmd) {
			t.Errorf("help missing %q", cmd)
		}
	}
}

func TestInit(t *testing.T) {
	dir := t.TempDir()
	tomlPath := filepath.Join(dir, "ship.toml")

	input := "testy\nDockerfile\n\n3000\n/api/ping\ntesty.example.com\n\n.env\n\n\n"
	cmd := exec.Command(binary, "init")
	cmd.Dir = dir
	cmd.Stdin = strings.NewReader(input)
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("ship init: %v", err)
	}
	if !strings.Contains(string(out), "Created ship.toml") {
		t.Errorf("unexpected output: %s", out)
	}

	data, err := os.ReadFile(tomlPath)
	if err != nil {
		t.Fatalf("reading ship.toml: %v", err)
	}
	content := string(data)

	checks := []string{
		`app = "testy"`,
		`dockerfile = "Dockerfile"`,
		`port = 3000`,
		`health_check = "/api/ping"`,
		`domains = ["testy.example.com"]`,
	}
	for _, c := range checks {
		if !strings.Contains(content, c) {
			t.Errorf("ship.toml missing %q\n\nGot:\n%s", c, content)
		}
	}
}

func TestInitMinimal(t *testing.T) {
	dir := t.TempDir()
	tomlPath := filepath.Join(dir, "ship.toml")

	// All defaults
	input := "\n\n\n\n\n\n\n\n\n\n"
	cmd := exec.Command(binary, "init")
	cmd.Dir = dir
	cmd.Stdin = strings.NewReader(input)
	cmd.Stderr = os.Stderr
	_, err := cmd.Output()
	if err != nil {
		t.Fatalf("ship init: %v", err)
	}

	data, err := os.ReadFile(tomlPath)
	if err != nil {
		t.Fatalf("reading ship.toml: %v", err)
	}
	content := string(data)

	if !strings.Contains(content, "port = 8080") {
		t.Error("default port should be 8080")
	}
	if !strings.Contains(content, `health_check = "/health"`) {
		t.Error("default health check should be /health")
	}
}

func TestWhoami(t *testing.T) {
	out, err := run("whoami")
	if err != nil {
		t.Fatalf("ship whoami: %v", err)
	}
	// Should at least show the providers section
	if !strings.Contains(out, "Infrastructure") {
		t.Errorf("whoami missing providers section: %s", out)
	}
	if !strings.Contains(out, "Platforms") {
		t.Errorf("whoami missing platforms section: %s", out)
	}
}

func TestDeployRequiresConfig(t *testing.T) {
	dir := t.TempDir()
	cmd := exec.Command(binary, "deploy")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Error("deploy without ship.toml should fail")
	}
	if !strings.Contains(string(out), "ship.toml") {
		t.Errorf("expected ship.toml error, got: %s", out)
	}
}

func TestSetupRequiresServer(t *testing.T) {
	dir := t.TempDir()
	cmd := exec.Command(binary, "setup")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Error("setup without server should fail")
	}
	if !strings.Contains(string(out), "server") {
		t.Errorf("expected server error, got: %s", out)
	}
}
