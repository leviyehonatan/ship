package migrate

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/leviyehonatan/ship/internal/secrets"
	shipssh "github.com/leviyehonatan/ship/internal/ssh"
)

// FromFly migrates a Fly.io app to a target server.
// It dumps all databases, copies env vars, generates ship.toml,
// and restores everything on the target.
func FromFly(targetIP string) error {
	fmt.Println("🚀 Migrating from Fly.io...")

	// ---- Step 1: Detect fly.toml ----
	appName, err := detectFlyApp()
	if err != nil {
		return err
	}
	fmt.Printf("  App: %s\n", appName)

	// ---- Step 2: Dump Postgres from Fly ----
	fmt.Print("  Dumping Postgres...")
	pgDump, err := dumpPostgresFromFly()
	if err != nil {
		fmt.Printf(" skipped (%v)\n", err)
	} else {
		fmt.Printf(" %d bytes\n", len(pgDump))
	}

	// ---- Step 3: Dump CouchDB from Fly ----
	fmt.Print("  Dumping CouchDB...")
	couchDump, err := dumpCouchDBFromFly()
	if err != nil {
		fmt.Printf(" skipped (%v)\n", err)
	} else if len(couchDump) > 100 {
		fmt.Printf(" %d bytes\n", len(couchDump))
	} else {
		fmt.Println(" no data")
		couchDump = nil
	}

	// ---- Step 4: Read env from Fly ----
	fmt.Print("  Reading env vars...")
	envVars, err := readEnvFromFly()
	if err != nil {
		fmt.Printf(" skipped (%v)\n", err)
	} else {
		fmt.Printf(" %d vars\n", len(envVars))
	}

	// ---- Step 5: Generate ship.toml from fly.toml ----
	fmt.Print("  Generating ship.toml...")
	if err := generateShipTOML(targetIP, appName); err != nil {
		return fmt.Errorf("generating ship.toml: %w", err)
	}
	fmt.Println(" done")

	// ---- Step 6: Encrypt secrets ----
	if len(envVars) > 0 {
		fmt.Print("  Encrypting secrets...")
		envPath := filepath.Join(".", ".env")
		var buf []byte
		for k, v := range envVars {
			buf = append(buf, []byte(fmt.Sprintf("%s=%s\n", k, v))...)
		}
		os.WriteFile(envPath, buf, 0644)
		if err := secrets.EncryptFile(envPath); err != nil {
			fmt.Printf(" warning: %v\n", err)
		} else {
			os.Remove(envPath) // only keep encrypted
			fmt.Println(" done")
		}
	}

	// ---- Step 7: Connect to target ----
	fmt.Print("  Connecting to target...")
	ssh, err := shipssh.NewClientInsecure(targetIP, "root", "")
	if err != nil {
		return fmt.Errorf("ssh to %s: %w", targetIP, err)
	}
	fmt.Println(" connected")

	// ---- Step 8: Setup target (Docker + Caddy) ----
	fmt.Println("  Setting up server...")
	ssh.Run("which docker 2>/dev/null || (apt-get update -qq && apt-get install -y -qq docker.io docker-compose-v2)")
	ssh.Run("which caddy 2>/dev/null || (apt-get install -y -qq debian-keyring debian-archive-keyring apt-transport-https && curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/gpg.key' | gpg --dearmor -o /usr/share/keyrings/caddy-stable-archive-keyring.gpg && curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/debian.deb.txt' | tee /etc/apt/sources.list.d/caddy-stable.list && apt-get update -qq && apt-get install -y -qq caddy)")
	ssh.Run("mkdir -p /opt/ship/data")

	// ---- Step 9: Restore Postgres on target ----
	if len(pgDump) > 0 {
		fmt.Println("  Restoring Postgres on target...")
		remotePath := fmt.Sprintf("/opt/ship/data/pg_dump_%s.sql", appName)
		if err := ssh.CopyFile("/tmp/ship_migrate_pg.sql", remotePath); err != nil {
			// Write temp file first
			os.WriteFile("/tmp/ship_migrate_pg.sql", pgDump, 0644)
			ssh.CopyFile("/tmp/ship_migrate_pg.sql", remotePath)
			os.Remove("/tmp/ship_migrate_pg.sql")
		}
		fmt.Println("    Postgres dump saved. Restore with:")
		fmt.Printf("    ship ssh → docker exec -i %s psql -U postgres -h 127.0.0.1 app < /opt/ship/data/pg_dump_%s.sql\n", appName, appName)
	}

	// ---- Step 10: Restore CouchDB on target ----
	if len(couchDump) > 0 {
		fmt.Println("  Restoring CouchDB on target...")
		os.WriteFile("/tmp/ship_migrate_couch.tar.gz", couchDump, 0644)
		remotePath := fmt.Sprintf("/opt/ship/data/couchdb_%s.tar.gz", appName)
		ssh.CopyFile("/tmp/ship_migrate_couch.tar.gz", remotePath)
		os.Remove("/tmp/ship_migrate_couch.tar.gz")
		fmt.Println("    CouchDB dump saved. Restore with:")
		fmt.Printf("    ship ssh → docker exec -i %s tar xzf /opt/ship/data/couchdb_%s.tar.gz -C /\n", appName, appName)
	}

	// ---- Done ----
	fmt.Println()
	fmt.Println("✅ Migration data ready. Next steps:")
	fmt.Println("  1. Review ship.toml — verify config")
	fmt.Println("  2. ship deploy")
	fmt.Println("  3. Restore databases (if they weren't auto-restored):")
	if len(pgDump) > 0 {
		fmt.Printf("     cat /opt/ship/data/pg_dump_%s.sql | docker exec -i %s psql -U postgres -h 127.0.0.1 app\n", appName, appName)
	}
	if len(couchDump) > 0 {
		fmt.Printf("     cat /opt/ship/data/couchdb_%s.tar.gz | docker exec -i %s tar xzf - -C /\n", appName, appName)
	}
	fmt.Println("  4. ship ssl on your-domain.com")

	return nil
}

// ---- internal helpers ----

func detectFlyApp() (string, error) {
	data, err := os.ReadFile("fly.toml")
	if err != nil {
		return "", fmt.Errorf("fly.toml not found — run from your Fly project directory")
	}
	var fly struct {
		App string `toml:"app"`
	}
	if err := toml.Unmarshal(data, &fly); err != nil {
		return "", fmt.Errorf("parsing fly.toml: %w", err)
	}
	if fly.App == "" {
		return "", fmt.Errorf("app name not found in fly.toml")
	}
	return fly.App, nil
}

func dumpPostgresFromFly() ([]byte, error) {
	cmd := exec.Command("fly", "ssh", "console", "-C",
		"/bin/sh -c 'PGHOST=127.0.0.1 PGUSER=postgres pg_dump app'")
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("pg_dump: %w", err)
	}
	// Skip first line (SSH connection message)
	lines := strings.SplitN(string(out), "\n", 2)
	if len(lines) > 1 {
		return []byte(lines[1]), nil
	}
	return out, nil
}

func dumpCouchDBFromFly() ([]byte, error) {
	cmd := exec.Command("fly", "ssh", "console", "-C",
		"/bin/sh -c 'tar czf /tmp/couchdb.tar.gz -C /data couchdb 2>/dev/null; cat /tmp/couchdb.tar.gz'")
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	// Skip SSH connection line
	lines := strings.SplitN(string(out), "\n", 2)
	if len(lines) > 1 {
		return []byte(lines[1]), nil
	}
	return out, nil
}

func readEnvFromFly() (map[string]string, error) {
	cmd := exec.Command("fly", "ssh", "console", "-C", "printenv")
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	// Skip SSH connection line
	lines := strings.SplitN(string(out), "\n", 2)
	data := string(out)
	if len(lines) > 1 {
		data = lines[1]
	}

	env := make(map[string]string)
	for _, line := range strings.Split(data, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if k, v, ok := strings.Cut(line, "="); ok {
			env[k] = v
		}
	}
	return env, nil
}

func generateShipTOML(targetIP, appName string) error {
	data, err := os.ReadFile("fly.toml")
	if err != nil {
		// Fallback: create minimal ship.toml
		return writeMinimalShipTOML(targetIP, appName)
	}

	var fly struct {
		Build struct {
			Dockerfile string `toml:"dockerfile"`
		} `toml:"build"`
		HTTPService *struct {
			InternalPort int `toml:"internal_port"`
		} `toml:"http_service"`
		Mounts []struct {
			Destination string `toml:"destination"`
		} `toml:"mounts"`
	}
	toml.Unmarshal(data, &fly)

	port := 8080
	if fly.HTTPService != nil && fly.HTTPService.InternalPort > 0 {
		port = fly.HTTPService.InternalPort
	}

	dockerfile := fly.Build.Dockerfile
	if dockerfile == "" {
		dockerfile = "Dockerfile"
	}

	tomlStr := fmt.Sprintf(`# ship.toml — migrated from Fly.io
app = %q
server = %q

[build]
dockerfile = %q

[deploy]
port = %d
health_check = "/health"
`, appName, targetIP, dockerfile, port)

	for _, m := range fly.Mounts {
		if m.Destination != "" {
			tomlStr += fmt.Sprintf("\n[[volumes]]\npath = %q\n", m.Destination)
		}
	}

	return os.WriteFile("ship.toml", []byte(tomlStr), 0644)
}

func writeMinimalShipTOML(targetIP, appName string) error {
	tomlStr := fmt.Sprintf(`# ship.toml — migrated from Fly.io
app = %q
server = %q

[build]
dockerfile = "Dockerfile"

[deploy]
port = 8080
health_check = "/health"
`, appName, targetIP)
	return os.WriteFile("ship.toml", []byte(tomlStr), 0644)
}
