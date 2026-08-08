package e2e

import (
	"os"
	"strings"
	"testing"
	"time"
)

func TestSecretsInjection(t *testing.T) {
	env := newTestEnv(t, "secrets")

	env.writeDockerfile()
	env.writeShipTOML()

	// ---- Fly-style: set secret directly, no .env file ----
	env.runShipOut("secrets", "set", "DB_PASS=secret-initial")
	env.runShipOut("secrets", "set", "API_KEY=key-abc-123")
	env.runShipOut("secrets", "set", "COUCHDB_PASSWORD=t0pS3cr3t")

	// Verify .env.encrypted exists, .env does NOT
	if _, err := os.Stat(env.dir + "/.env.encrypted"); err != nil {
		t.Fatal(".env.encrypted not created")
	}
	if _, err := os.Stat(env.dir + "/.env"); err == nil {
		t.Error(".env should not exist — secrets stored only in .env.encrypted")
	}
	t.Log("✓ Secrets stored in .env.encrypted, no plaintext .env")

	// List keys (values hidden)
	listOut, _ := env.runShip("secrets", "list")
	if strings.Contains(listOut, "DB_PASS") && strings.Contains(listOut, "API_KEY") {
		t.Log("✓ secrets list shows keys")
	}
	if !strings.Contains(listOut, "secret-initial") {
		t.Log("✓ secrets list hides values")
	}

	// Setup + deploy
	env.runShipOut("setup")
	env.runShipOut("deploy")

	time.Sleep(3 * time.Second)

	// Verify secrets were injected into container
	logs, _ := env.sshCmd("docker", "logs", env.appName)
	if strings.Contains(logs, "DB_PASS=secret-initial") {
		t.Log("✓ DB_PASS injected from .env.encrypted")
	} else {
		t.Error("DB_PASS not found")
	}
	if strings.Contains(logs, "API_KEY=key-abc-123") {
		t.Log("✓ API_KEY injected")
	} else {
		t.Error("API_KEY not found")
	}

	// ---- Update a secret ----
	env.runShipOut("secrets", "set", "DB_PASS=secret-updated")
	t.Log("✓ Secret updated")

	// Redeploy
	env.runShipOut("deploy")
	time.Sleep(3 * time.Second)

	// Verify update
	logs2, _ := env.sshCmd("docker", "logs", env.appName)
	if strings.Contains(logs2, "secret-updated") {
		t.Log("✓ Updated password reflected in container")
	} else {
		t.Error("Updated password NOT reflected")
	}
	if strings.Contains(logs2, "secret-initial") {
		t.Error("Old password still present")
	} else {
		t.Log("✓ Old password gone")
	}

	// ---- Remove a secret ----
	env.runShipOut("secrets", "unset", "API_KEY")
	listOut2, _ := env.runShip("secrets", "list")
	if !strings.Contains(listOut2, "API_KEY") {
		t.Log("✓ Secret unset")
	} else {
		t.Error("API_KEY still listed after unset")
	}

	// ---- Show a secret ----
	showOut, _ := env.runShip("secrets", "show", "DB_PASS")
	if strings.Contains(showOut, "secret-updated") {
		t.Log("✓ secrets show returns value")
	}

	t.Log("✓ Secrets lifecycle: set → deploy → update → redeploy → unset → show")
}
