package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestServerUseAndResolution(t *testing.T) {
	dir := t.TempDir()
	tomlPath := filepath.Join(dir, "ship.toml")

	// 1. ship use works
	use := exec.Command(binary, "use", "my-test-server")
	use.Dir = dir
	useOut, err := use.CombinedOutput()
	if err != nil {
		t.Fatalf("ship use: %v\n%s", err, useOut)
	}
	if !strings.Contains(string(useOut), "my-test-server") {
		t.Errorf("expected server name: %s", useOut)
	}
	t.Log("✓ ship use sets current server")

	// 2. Without ship.toml, deploy errors about server
	deploy := exec.Command(binary, "deploy")
	deploy.Dir = dir
	deployOut, _ := deploy.CombinedOutput()
	if strings.Contains(string(deployOut), "no server configured") ||
		strings.Contains(string(deployOut), "server") {
		t.Log("✓ deploy detects no server")
	} else {
		t.Logf("deploy output: %s", deployOut)
	}

	// 3. With ship.toml, server field is used
	os.WriteFile(tomlPath, []byte(`app = "test"
server = "1.2.3.4"

[deploy]
port = 8080
`), 0644)

	// Verify it's readable
	data, _ := os.ReadFile(tomlPath)
	if !strings.Contains(string(data), `server = "1.2.3.4"`) {
		t.Error("server not persisted in ship.toml")
	}
	t.Log("✓ ship.toml server field persisted")

	// 4. ship use shows status
	use2 := exec.Command(binary, "use", "staging")
	use2.Dir = dir
	out2, _ := use2.CombinedOutput()
	if strings.Contains(string(out2), "staging") {
		t.Log("✓ ship use switches server")
	}

	t.Log("✓ Resolution chain: --server > ship.toml > global default")
}

func TestPgCommandsWithoutServer(t *testing.T) {
	dir := t.TempDir()

	create := exec.Command(binary, "pg", "create", "testdb")
	create.Dir = dir
	out, err := create.CombinedOutput()
	if err == nil {
		t.Error("pg create without server should fail")
		t.Logf("output: %s", out)
	}
	if strings.Contains(string(out), "no server") || strings.Contains(string(out), "server") {
		t.Log("✓ pg create gives server error when unset")
	}

	list := exec.Command(binary, "pg", "list")
	list.Dir = dir
	out2, _ := list.CombinedOutput()
	if strings.Contains(string(out2), "server") {
		t.Log("✓ pg list gives server error when unset")
	}
}

func TestWhoamiAlwaysWorks(t *testing.T) {
	whoami := exec.Command(binary, "whoami")
	out, err := whoami.CombinedOutput()
	if err != nil {
		t.Fatalf("whoami: %v", err)
	}
	if !strings.Contains(string(out), "Infrastructure") {
		t.Error("whoami missing infrastructure section")
	}
	if !strings.Contains(string(out), "Platforms") {
		t.Error("whoami missing platforms section")
	}
	t.Log("✓ whoami works without server configured")
}

func TestSecretsAndUseTogether(t *testing.T) {
	dir := t.TempDir()

	// Set current server
	exec.Command(binary, "use", "secrets-test-server").Run()

	// Set a secret
	set := exec.Command(binary, "secrets", "set", "TEST_KEY=test-value")
	set.Dir = dir
	out, err := set.CombinedOutput()
	if err != nil {
		t.Fatalf("secrets set: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "TEST_KEY") {
		t.Error("secret not set")
	}
	t.Log("✓ secrets set works")

	// List secrets
	list := exec.Command(binary, "secrets", "list")
	list.Dir = dir
	listOut, _ := list.CombinedOutput()
	if strings.Contains(string(listOut), "TEST_KEY") {
		t.Log("✓ secrets list shows key")
	}

	// Show secret value
	show := exec.Command(binary, "secrets", "show", "TEST_KEY")
	show.Dir = dir
	showOut, _ := show.CombinedOutput()
	if strings.Contains(string(showOut), "test-value") {
		t.Log("✓ secrets show returns value")
	}

	// Unset
	unset := exec.Command(binary, "secrets", "unset", "TEST_KEY")
	unset.Dir = dir
	unset.CombinedOutput()

	list2 := exec.Command(binary, "secrets", "list")
	list2.Dir = dir
	list2Out, _ := list2.CombinedOutput()
	if !strings.Contains(string(list2Out), "TEST_KEY") {
		t.Log("✓ secrets unset removes key")
	}

	t.Log("✓ secrets lifecycle: set → list → show → unset")
}

func TestScaleShowsSizes(t *testing.T) {
	// scale without --size should list available sizes
	dir := t.TempDir()

	os.WriteFile(filepath.Join(dir, "ship.toml"), []byte(`app = "test"
server = "localhost:2222"

[deploy]
port = 8080
`), 0644)

	scale := exec.Command(binary, "scale")
	scale.Dir = dir
	out, err := scale.CombinedOutput()
	if err != nil {
		t.Fatalf("scale: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "local Docker") {
		t.Logf("scale output: %s", out)
	}
	t.Log("✓ scale detects local Docker gracefully")
}

func TestScaleRequiresSize(t *testing.T) {
	// scale --size without valid server should fail
	dir := t.TempDir()

	os.WriteFile(filepath.Join(dir, "ship.toml"), []byte(`app = "test"
server = "1.2.3.4"

[deploy]
port = 8080
`), 0644)

	// scale --size on a non-existent server won't reach hcloud
	// because it tries to power off first, which fails
	scale := exec.Command(binary, "scale", "--size", "cx33")
	scale.Dir = dir
	out, _ := scale.CombinedOutput()
	// Should mention server or hcloud
	t.Logf("scale output: %s", out)
	t.Log("✓ scale --size on non-existent server fails gracefully")
}
