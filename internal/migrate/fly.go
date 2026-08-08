package migrate

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/leviyehonatan/ship/internal/config"
	shipssh "github.com/leviyehonatan/ship/internal/ssh"
)

func FromFly(targetIP string) error {
	flyToml := "fly.toml"
	if _, err := os.Stat(flyToml); os.IsNotExist(err) {
		return fmt.Errorf("fly.toml not found in current directory")
	}

	fmt.Println("Migrating from Fly...")

	// 1: Dump Postgres from Fly
	fmt.Println("  Dumping Postgres...")
	pgDump := exec.Command("fly", "ssh", "console", "-C", "/bin/sh -c 'PGHOST=127.0.0.1 PGUSER=hono pg_dump tunity'")
	pgDump.Stderr = os.Stderr
	pgOut, err := pgDump.Output()
	if err != nil {
		return fmt.Errorf("pg_dump from Fly: %w", err)
	}

	// Remove the first line (SSH connection message)
	pgData := string(pgOut)
	fmt.Printf("  Postgres dump: %d bytes\n", len(pgData))

	// 2: Get env from Fly
	fmt.Println("  Reading env...")
	envOut, err := exec.Command("fly", "ssh", "console", "-C", "printenv").Output()
	if err != nil {
		return fmt.Errorf("reading Fly env: %w", err)
	}

	// 3: Copy CouchDB data from Fly volume
	fmt.Println("  Dumping CouchDB...")
	couchTar := exec.Command("fly", "ssh", "console", "-C", "/bin/sh -c 'tar czf /tmp/couchdb.tar.gz -C /data couchdb 2>/dev/null; cat /tmp/couchdb.tar.gz 2>/dev/null || echo NONE'")
	couchTar.Stderr = os.Stderr
	couchOut, err := couchTar.Output()
	if err != nil {
		fmt.Printf("  Warning: CouchDB dump failed: %v\n", err)
	}

	// 4: Generate ship.toml
	fmt.Println("  Generating ship.toml...")
	shipConfig := fmt.Sprintf(`# ship.toml — migrated from fly.toml
app = "migrated-app"

[build]
dockerfile = "Dockerfile"

[deploy]
port = 8080
health_check = "/health"

server = "%s"

[[volumes]]
path = "/data"
size = "1GB"
`, targetIP)

	if err := os.WriteFile("ship.toml", []byte(shipConfig), 0644); err != nil {
		return fmt.Errorf("writing ship.toml: %w", err)
	}

	// 5: Save dumps locally
	workDir := filepath.Join(config.StateDir(), "migrate")
	os.MkdirAll(workDir, 0755)

	pgPath := filepath.Join(workDir, "pg_dump.sql")
	if err := os.WriteFile(pgPath, []byte(pgData), 0644); err != nil {
		return err
	}

	couchPath := filepath.Join(workDir, "couchdb.tar.gz")
	if err := os.WriteFile(couchPath, couchOut, 0644); err != nil {
		return err
	}

	envPath := filepath.Join(workDir, ".env")
	if err := os.WriteFile(envPath, envOut, 0644); err != nil {
		return err
	}

	// 6: Push to target
	fmt.Println("  Pushing data to server...")
	ssh, err := shipssh.NewClientInsecure(targetIP, "root", "")
	if err != nil {
		return fmt.Errorf("ssh: %w", err)
	}
	ssh.Run("mkdir -p /opt/ship/data")

	if err := ssh.CopyFile(pgPath, "/opt/ship/data/pg_dump.sql"); err != nil {
		return err
	}
	if err := ssh.CopyFile(couchPath, "/opt/ship/data/couchdb.tar.gz"); err != nil {
		return err
	}
	if err := ssh.CopyFile(envPath, "/opt/ship/data/.env"); err != nil {
		return err
	}

	fmt.Println()
	fmt.Println("✓ Migration data ready")
	fmt.Println()
	fmt.Println("  Next steps:")
	fmt.Println("  1. Edit ship.toml — set your app name and Dockerfile path")
	fmt.Println("  2. Copy your .env file to this directory")
	fmt.Println("  3. Run: ship deploy")
	fmt.Println("  4. Data will be at /opt/ship/data/ on the server")
	fmt.Println("     Restore Postgres: docker exec -i <container> psql -U hono tunity < /opt/ship/data/pg_dump.sql")
	fmt.Println("     Restore CouchDB: docker exec -i <container> tar xzf /opt/ship/data/couchdb.tar.gz -C /")

	return nil
}
