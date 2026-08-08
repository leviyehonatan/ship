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

func TestCouchDBMigration(t *testing.T) {
	if testing.Short() { t.Skip("short mode") }
	if _, err := exec.LookPath("docker"); err != nil { t.Skip("docker not installed") }
	if err := exec.Command("docker", "info").Run(); err != nil { t.Skip("docker daemon not running") }

	image := "ship-e2e-ssh:latest"
	for _, c := range []string{"ship-couch-src", "ship-couch-dst"} {
		exec.Command("docker", "rm", "-f", c).Run()
	}
	t.Cleanup(func() {
		for _, c := range []string{"ship-couch-src", "ship-couch-dst"} {
			exec.Command("docker", "rm", "-f", c).Run()
		}
	})

	// Source: seed CouchDB
	exec.Command("docker", "run", "-d", "--rm", "--name", "ship-couch-src", image).Run()
	time.Sleep(5 * time.Second)

	execSrc := func(cmd string) string {
		out, _ := exec.Command("docker", "exec", "ship-couch-src",
			"/bin/bash", "-c", cmd).Output()
		return string(out)
	}

	// Install and start CouchDB in container
	execSrc("apt-get update -qq && apt-get install -y -qq curl 2>/dev/null")
	execSrc("curl -fsSL https://couchdb.apache.org/repo/bintray-pubkey.asc 2>/dev/null | apt-key add - 2>/dev/null || true")
	execSrc("echo 'deb https://apache.jfrog.io/artifactory/couchdb-deb noble main' > /etc/apt/sources.list.d/couchdb.list 2>/dev/null || true")
	execSrc("apt-get update -qq && apt-get install -y -qq couchdb 2>/dev/null || true")

	// Configure single-node + start
	execSrc("mkdir -p /opt/couchdb/etc/local.d")
	execSrc(`cat > /opt/couchdb/etc/local.d/test.ini << 'INI'
[couchdb]
single_node = true
database_dir = /tmp/couch-data
view_index_dir = /tmp/couch-data
[chttpd]
bind_address = 0.0.0.0
port = 5984
[admins]
admin = testpass
INI`)
	execSrc("/opt/couchdb/bin/couchdb -b 2>/dev/null || /opt/couchdb/bin/couchdb &")
	time.Sleep(5 * time.Second)

	// Seed test document
	execSrc(`curl -s -u admin:testpass -X PUT http://127.0.0.1:5984/testdb`)
	execSrc(`curl -s -u admin:testpass -X POST http://127.0.0.1:5984/testdb -H 'Content-Type: application/json' -d '{"_id":"doc1","name":"couch-migration-test"}'`)

	// Verify source
	srcResult := execSrc(`curl -s -u admin:testpass http://127.0.0.1:5984/testdb/doc1`)
	if !strings.Contains(srcResult, "couch-migration-test") {
		t.Fatalf("CouchDB source seeding failed: %s", srcResult)
	}
	t.Log("✓ CouchDB source seeded")

	// Detect (check data directory exists)
	detect := execSrc("test -d /tmp/couch-data")
	if !strings.Contains(detect, "") { // test returns empty on success
		t.Log("✓ CouchDB detected")
	} else {
		t.Fatal("CouchDB data dir not detected")
	}

	// Dump
	dump, _ := exec.Command("docker", "exec", "ship-couch-src",
		"/bin/bash", "-c", "tar czf /tmp/couch.tar.gz -C /tmp couch-data 2>/dev/null; cat /tmp/couch.tar.gz").Output()
	if len(dump) < 100 {
		t.Fatal("CouchDB dump too small or failed")
	}
	t.Logf("✓ CouchDB dump: %d bytes", len(dump))
	os.WriteFile("/tmp/ship-couch-test.tar.gz", dump, 0644)
	defer os.Remove("/tmp/ship-couch-test.tar.gz")

	// Target: restore
	exec.Command("docker", "run", "-d", "--rm", "--name", "ship-couch-dst",
		"-v", "/tmp/ship-couch-test.tar.gz:/dump.tar.gz:ro", image).Run()
	time.Sleep(5 * time.Second)

	execDst := func(cmd string) string {
		out, _ := exec.Command("docker", "exec", "ship-couch-dst",
			"/bin/bash", "-c", cmd).Output()
		return string(out)
	}

	execDst("apt-get update -qq && apt-get install -y -qq couchdb curl 2>/dev/null || true")
	execDst("mkdir -p /opt/couchdb/etc/local.d")
	execDst(`cat > /opt/couchdb/etc/local.d/test.ini << 'INI'
[couchdb]
single_node = true
database_dir = /tmp/couch-data
view_index_dir = /tmp/couch-data
[chttpd]
bind_address = 0.0.0.0
port = 5984
[admins]
admin = testpass
INI`)
	execDst("tar xzf /dump.tar.gz -C /tmp")
	execDst("/opt/couchdb/bin/couchdb -b 2>/dev/null || /opt/couchdb/bin/couchdb &")
	time.Sleep(5 * time.Second)

	// Verify target
	dstResult := execDst(`curl -s -u admin:testpass http://127.0.0.1:5984/testdb/doc1`)
	if strings.Contains(dstResult, "couch-migration-test") {
		t.Log("✓ CouchDB data migrated to target")
	} else {
		t.Errorf("CouchDB data missing in target: %s", dstResult)
	}

	t.Log("✓ CouchDB migration: seed → detect → dump → restore → verify")
}

func TestRedisMigration(t *testing.T) {
	if testing.Short() { t.Skip("short mode") }
	if _, err := exec.LookPath("docker"); err != nil { t.Skip("docker not installed") }
	if err := exec.Command("docker", "info").Run(); err != nil { t.Skip("docker daemon not running") }

	image := "ship-e2e-ssh:latest"
	for _, c := range []string{"ship-redis-src", "ship-redis-dst"} {
		exec.Command("docker", "rm", "-f", c).Run()
	}
	t.Cleanup(func() {
		for _, c := range []string{"ship-redis-src", "ship-redis-dst"} {
			exec.Command("docker", "rm", "-f", c).Run()
		}
	})

	// Source: seed Redis
	exec.Command("docker", "run", "-d", "--rm", "--name", "ship-redis-src", image).Run()
	time.Sleep(5 * time.Second)

	execSrc := func(cmd string) string {
		out, _ := exec.Command("docker", "exec", "ship-redis-src",
			"/bin/bash", "-c", cmd).Output()
		return string(out)
	}

	execSrc("redis-server --daemonize yes 2>/dev/null || true")
	time.Sleep(2 * time.Second)
	execSrc("redis-cli SET migration_key 'redis-test-value'")

	srcResult := execSrc("redis-cli GET migration_key")
	if !strings.Contains(srcResult, "redis-test-value") {
		t.Fatalf("Redis source seeding failed: %s", srcResult)
	}
	t.Log("✓ Redis source seeded")

	// Detect
	detect := execSrc("redis-cli -h 127.0.0.1 ping")
	if strings.Contains(detect, "PONG") {
		t.Log("✓ Redis detected")
	} else {
		t.Fatal("Redis not detected")
	}

	// Dump
	execSrc("redis-cli --rdb /tmp/dump.rdb SAVE")
	dump, _ := exec.Command("docker", "exec", "ship-redis-src",
		"/bin/bash", "-c", "cat /tmp/dump.rdb").Output()
	if len(dump) < 50 {
		t.Fatal("Redis dump too small")
	}
	t.Logf("✓ Redis dump: %d bytes", len(dump))
	os.WriteFile("/tmp/ship-redis-test.rdb", dump, 0644)
	defer os.Remove("/tmp/ship-redis-test.rdb")

	// Target: restore
	exec.Command("docker", "run", "-d", "--rm", "--name", "ship-redis-dst",
		"-v", "/tmp/ship-redis-test.rdb:/dump.rdb:ro", image).Run()
	time.Sleep(5 * time.Second)

	execDst := func(cmd string) string {
		out, _ := exec.Command("docker", "exec", "ship-redis-dst",
			"/bin/bash", "-c", cmd).Output()
		return string(out)
	}

	execDst("cp /dump.rdb /var/lib/redis/dump.rdb 2>/dev/null || mkdir -p /var/lib/redis && cp /dump.rdb /var/lib/redis/")
	execDst("redis-server --daemonize yes 2>/dev/null || true")
	time.Sleep(2 * time.Second)

	// Verify target
	dstResult := execDst("redis-cli GET migration_key")
	if strings.Contains(dstResult, "redis-test-value") {
		t.Log("✓ Redis data migrated to target")
	} else {
		t.Errorf("Redis data missing in target: %s", dstResult)
	}

	t.Log("✓ Redis migration: seed → detect → dump → restore → verify")
}
