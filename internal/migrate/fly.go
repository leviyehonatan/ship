package migrate

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/leviyehonatan/ship/internal/secrets"
	shipssh "github.com/leviyehonatan/ship/internal/ssh"
)

func FromFly(targetIP string) error {
	fmt.Println("Migrating from Fly.io...")

	appName, err := detectFlyApp()
	if err != nil {
		return err
	}
	fmt.Printf("  App: %s\n", appName)

	fmt.Println("  Detecting databases...")
	dbs := Discover()
	if len(dbs) == 0 {
		fmt.Println("    (none detected)")
	} else {
		for _, db := range dbs {
			fmt.Printf("    %s\n", db.Name())
		}
	}

	for _, db := range dbs {
		fmt.Printf("  Dumping %s...", db.Name())
		data, err := db.Dump()
		if err != nil {
			fmt.Printf(" failed: %v\n", err)
			continue
		}
		if len(data) < 100 {
			fmt.Println(" empty")
			continue
		}
		fmt.Printf(" %d bytes\n", len(data))
		os.WriteFile("/tmp/ship_migrate_"+sanitize(db.Name()), data, 0644)
	}

	fmt.Print("  Reading env vars...")
	envVars, _ := readEnvFromFly()
	fmt.Printf(" %d vars\n", len(envVars))

	for _, db := range dbs {
		for k, v := range db.ConfigHints() {
			if _, exists := envVars[k]; !exists {
				envVars[k] = v
			}
		}
	}

	fmt.Print("  Generating ship.toml...")
	if err := generateShipTOML(targetIP, appName); err != nil {
		return err
	}
	fmt.Println(" done")

	if len(envVars) > 0 {
		fmt.Print("  Encrypting secrets...")
		envPath := ".env"
		var buf []byte
		for k, v := range envVars {
			buf = append(buf, []byte(fmt.Sprintf("%s=%s\n", k, v))...)
		}
		os.WriteFile(envPath, buf, 0644)
		secrets.EncryptFile(envPath)
		os.Remove(envPath)
		fmt.Println(" done")
	}

	fmt.Print("  Connecting to target...")
	ssh, err := shipssh.NewClientInsecure(targetIP, "root", "")
	if err != nil {
		return fmt.Errorf("ssh: %w", err)
	}
	fmt.Println(" connected")

	fmt.Println("  Installing Docker on target...")
	ssh.Run("which docker 2>/dev/null || (apt-get update -qq && apt-get install -y -qq docker.io)")
	ssh.Run("mkdir -p /opt/ship/data")

	for _, db := range dbs {
		name := "ship_migrate_" + sanitize(db.Name())
		localPath := filepath.Join("/tmp", name)
		if _, err := os.Stat(localPath); err != nil {
			continue
		}
		fmt.Printf("  Pushing %s dump...\n", db.Name())
		ssh.CopyFile(localPath, filepath.Join("/opt/ship/data", name))
		os.Remove(localPath)
	}

	fmt.Println()
	fmt.Println("Done. Next steps:")
	fmt.Println("  1. Review ship.toml and .env.encrypted")
	fmt.Println("  2. ship deploy")
	fmt.Println("  3. Restore databases:")
	for _, db := range dbs {
		fmt.Printf("     %s\n", db.RestoreCmd(appName))
	}
	return nil
}

func sanitize(s string) string {
	return strings.ToLower(strings.ReplaceAll(s, " ", "_"))
}

func detectFlyApp() (string, error) {
	data, err := os.ReadFile("fly.toml")
	if err != nil {
		return "", fmt.Errorf("fly.toml not found")
	}
	var fly struct {
		App string `toml:"app"`
	}
	toml.Unmarshal(data, &fly)
	if fly.App == "" {
		return "", fmt.Errorf("app name not found in fly.toml")
	}
	return fly.App, nil
}

func readEnvFromFly() (map[string]string, error) {
	out, err := shell("printenv").Output()
	if err != nil {
		return nil, err
	}
	data := string(skipSSHLine(out))
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
	tomlStr := fmt.Sprintf(`# ship.toml
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
