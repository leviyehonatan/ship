package e2e

import (
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestDatabaseMigration(t *testing.T) {
	if testing.Short() {
		t.Skip("short mode")
	}
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("docker not installed")
	}
	if err := exec.Command("docker", "info").Run(); err != nil {
		t.Skip("docker daemon not running")
	}

	image := "ship-e2e-ssh:latest"

	// Cleanup containers from previous runs
	for _, c := range []string{"ship-migrate-src", "ship-migrate-dst"} {
		exec.Command("docker", "rm", "-f", c).Run()
	}
	t.Cleanup(func() {
		for _, c := range []string{"ship-migrate-src", "ship-migrate-dst"} {
			exec.Command("docker", "rm", "-f", c).Run()
		}
	})

	// Source: start container, seed data
	exec.Command("docker", "run", "-d", "--rm", "--name", "ship-migrate-src", image).Run()
	time.Sleep(5 * time.Second)

	execSrc := func(cmd string) string {
		out, _ := exec.Command("docker", "exec", "ship-migrate-src",
			"/bin/bash", "-c", cmd).Output()
		return string(out)
	}

	execSrc("sudo -u postgres pg_ctlcluster 16 main start 2>/dev/null || true")
	time.Sleep(2 * time.Second)
	execSrc("sudo -u postgres createdb ship_test 2>/dev/null || true")
	execSrc("sudo -u postgres psql ship_test -c \"CREATE TABLE IF NOT EXISTS items (id serial, name text);\"")
	execSrc("sudo -u postgres psql ship_test -c \"INSERT INTO items (name) VALUES ('migration-test-data');\"")

	// Verify source
	srcResult := execSrc("sudo -u postgres psql ship_test -t -c 'SELECT name FROM items;'")
	if !strings.Contains(srcResult, "migration-test-data") {
		t.Fatalf("source seeding failed: %s", srcResult)
	}
	t.Log("✓ Source seeded")

	// Detect
	detect := exec.Command("docker", "exec", "ship-migrate-src", "pg_isready", "-h", "127.0.0.1").Run() == nil
	if !detect {
		t.Fatal("Postgres not detected in source")
	}
	t.Log("✓ Detection works")

	// Dump
	dump, _ := exec.Command("docker", "exec", "ship-migrate-src",
		"/bin/bash", "-c",
		"sudo -u postgres pg_dump ship_test").Output()
	if len(dump) < 100 || !strings.Contains(string(dump), "migration-test-data") {
		t.Fatal("dump failed or lacks data")
	}
	t.Logf("✓ Dump: %d bytes", len(dump))
	os.WriteFile("/tmp/ship-migrate-test.sql", dump, 0644)
	defer os.Remove("/tmp/ship-migrate-test.sql")

	// Target: fresh container, restore
	exec.Command("docker", "run", "-d", "--rm", "--name", "ship-migrate-dst",
		"-v", "/tmp/ship-migrate-test.sql:/dump.sql:ro", image).Run()
	time.Sleep(5 * time.Second)

	execDst := func(cmd string) string {
		out, _ := exec.Command("docker", "exec", "ship-migrate-dst",
			"/bin/bash", "-c", cmd).Output()
		return string(out)
	}

	execDst("sudo -u postgres pg_ctlcluster 16 main start 2>/dev/null || true")
	time.Sleep(2 * time.Second)
	execDst("sudo -u postgres createdb ship_test 2>/dev/null || true")
	execDst("sudo -u postgres psql ship_test -f /dump.sql")

	// Verify target
	dstResult := execDst("sudo -u postgres psql ship_test -t -c 'SELECT name FROM items;'")
	if strings.Contains(dstResult, "migration-test-data") {
		t.Log("✓ Data migrated to target")
	} else {
		t.Errorf("data missing in target: %s", dstResult)
	}

	t.Log("✓ Full migration: seed → detect → dump → restore → verify")
}
