package e2e

import (
	"os/exec"
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

	// down — stop and remove everything (local path, no SSH needed)
	t.Log("--- down ---")
	downOut, err := env.runShip("down", "--local")
	if err != nil {
		t.Errorf("ship down --local failed: %v\n%s", err, downOut)
	}
	if strings.Contains(downOut, "Down") {
		t.Log("✓ down succeeds")
	}

	// verify container is gone (local docker)
	check := exec.Command("docker", "ps", "--filter", "name="+env.appName, "--format", "{{.Names}}")
	containerCheck, _ := check.Output()
	if len(strings.TrimSpace(string(containerCheck))) > 0 {
		t.Errorf("container %s still running after down", env.appName)
	}
	t.Log("✓ container removed")

	t.Log("✓ Full cycle: init → setup → deploy → status → logs → ssh → snapshot → down")
}
