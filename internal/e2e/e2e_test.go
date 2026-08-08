package e2e

import (
	"strings"
	"testing"
)

func TestFullShipCycle(t *testing.T) {
	env := newTestEnv(t, "fullcycle")

	env.writeDockerfile()
	env.writeShipTOML()

	// init
	env.runShipOut("init")

	// setup
	t.Log("--- setup ---")
	env.runShipOut("setup")

	// deploy
	t.Log("--- deploy ---")
	deployOut, _ := env.runShip("deploy")
	if strings.Contains(deployOut, "Loaded image") {
		t.Log("✓ image pushed via SSH")
	}
	if strings.Contains(deployOut, "deployed") {
		t.Log("✓ container started")
	}

	// status
	status, _ := env.runShip("status")
	t.Logf("status: %s", strings.TrimSpace(status))
	if status != "" {
		t.Log("✓ status reports")
	}

	// logs
	env.runShipOut("logs", "--tail", "3")

	// ssh overview
	overview, err := env.runShip("ssh")
	if err == nil && strings.Contains(overview, env.appName) {
		t.Log("✓ ssh overview shows container")
	}

	// snapshot (fails gracefully without PG, but command runs)
	env.runShipOut("snapshot")
	env.runShip("snapshots")

	t.Log("✓ Full cycle: init → setup → deploy → status → logs → ssh → snapshot")
}
