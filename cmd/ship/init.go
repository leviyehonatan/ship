package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/leviyehonatan/ship/internal/config"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Create a ship.toml for your project",
	Long: `Guides you through creating a ship.toml config file for deploying to your VPS.

With --from fly, reads an existing fly.toml and auto-generates ship.toml.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fromFlag, _ := cmd.Flags().GetString("from")

		if fromFlag == "fly" {
			return initFromFly(cmd)
		}

		reader := bufio.NewReader(cmd.InOrStdin())

		cfg := &config.ShipConfig{}

		// App name
		cwd, _ := os.Getwd()
		defaultApp := filepath.Base(cwd)
		fmt.Fprintf(cmd.OutOrStdout(), "App name [%s]: ", defaultApp)
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)
		if input == "" {
			input = defaultApp
		}
		cfg.App = input

	// Dockerfile
	fmt.Fprintf(cmd.OutOrStdout(), "Dockerfile [Dockerfile]: ")
	input, _ = reader.ReadString('\n')
	input = strings.TrimSpace(input)
	if input == "" {
		input = "Dockerfile"
	}
	cfg.Build.Dockerfile = input

	// Build args (for ARG in Dockerfile)
	fmt.Fprintf(cmd.OutOrStdout(), "Build args (ARG=VAL, comma-separated, e.g. NEXT_PUBLIC_KEY=abc): ")
	input, _ = reader.ReadString('\n')
	input = strings.TrimSpace(input)
	if input != "" {
		for _, a := range strings.Split(input, ",") {
			a = strings.TrimSpace(a)
			if a != "" {
				cfg.Build.Args = append(cfg.Build.Args, a)
			}
		}
	}

	// Port
		fmt.Fprintf(cmd.OutOrStdout(), "Port [8080]: ")
		input, _ = reader.ReadString('\n')
		input = strings.TrimSpace(input)
		if input == "" {
			input = "8080"
		}
		fmt.Sscanf(input, "%d", &cfg.Deploy.Port)
		if cfg.Deploy.Port == 0 {
			cfg.Deploy.Port = 8080
		}

		// Health check
		fmt.Fprintf(cmd.OutOrStdout(), "Health check path [/health]: ")
		input, _ = reader.ReadString('\n')
		input = strings.TrimSpace(input)
		if input == "" {
			input = "/health"
		}
		cfg.Deploy.HealthCheck = input

		// Domains
		fmt.Fprintf(cmd.OutOrStdout(), "Domain (optional, press enter to skip): ")
		input, _ = reader.ReadString('\n')
		input = strings.TrimSpace(input)
		if input != "" {
			cfg.Deploy.Domains = []string{input}
		}

		// Env vars (public, commitable)
		fmt.Fprintf(cmd.OutOrStdout(), "Public env vars (KEY=VAL, comma-separated): ")
		input, _ = reader.ReadString('\n')
		input = strings.TrimSpace(input)
		cfg.Env = make(map[string]string)
		if input != "" {
			for _, e := range strings.Split(input, ",") {
				e = strings.TrimSpace(e)
				if e == "" {
					continue
				}
				parts := strings.SplitN(e, "=", 2)
				if len(parts) == 2 {
					cfg.Env[parts[0]] = parts[1]
				}
			}
		}

		// Env file
		fmt.Fprintf(cmd.OutOrStdout(), "Env file [.env]: ")
		input, _ = reader.ReadString('\n')
		input = strings.TrimSpace(input)
		if input == "" {
			input = ".env"
		}
		cfg.EnvFile = input

		// Server address
		fmt.Fprintf(cmd.OutOrStdout(), "Server (IP or hostname, optional): ")
		input, _ = reader.ReadString('\n')
		input = strings.TrimSpace(input)
		cfg.Server = input

		// Volumes
		fmt.Fprintf(cmd.OutOrStdout(), "Volumes (path:size, comma-separated, e.g. /data:1GB): ")
		input, _ = reader.ReadString('\n')
		input = strings.TrimSpace(input)
		for _, v := range strings.Split(input, ",") {
			v = strings.TrimSpace(v)
			if v == "" {
				continue
			}
			parts := strings.SplitN(v, ":", 2)
			vol := config.Volume{Path: parts[0]}
			if len(parts) == 2 {
				vol.Size = parts[1]
			}
			cfg.Volumes = append(cfg.Volumes, vol)
		}

		// Write
		data := fmt.Sprintf(`# ship.toml — deploy config for %s
app = "%s"
`, cfg.App, cfg.App)

		if cfg.Server != "" {
			data += fmt.Sprintf("server = %q\n", cfg.Server)
		}

		if len(cfg.Env) > 0 {
			data += "\n# Public, non-sensitive env vars — committed to git\n# For secrets use: ship secrets set KEY=value\n[env]\n"
			for k, v := range cfg.Env {
				data += fmt.Sprintf("%s = %q\n", k, v)
			}
		} else {
			data += "\n# Public, non-sensitive env vars — committed to git\n# For secrets use: ship secrets set KEY=value\n# [env]\n# NODE_ENV = \"production\"\n"
		}

		data += fmt.Sprintf(`
[build]
dockerfile = "%s"
`, cfg.Build.Dockerfile)

		if len(cfg.Build.Args) > 0 {
			data += "args = ["
			for i, a := range cfg.Build.Args {
				if i > 0 {
					data += ", "
				}
				data += fmt.Sprintf("%q", a)
			}
			data += "]\n"
		}

		data += fmt.Sprintf(`
[deploy]
port = %d
health_check = "%s"
`, cfg.Deploy.Port, cfg.Deploy.HealthCheck)

		if len(cfg.Deploy.Domains) > 0 {
			data += fmt.Sprintf("domains = [%q]\n", cfg.Deploy.Domains[0])
		}

		if cfg.EnvFile != ".env" {
			data += fmt.Sprintf("\nenv_file = %q\n", cfg.EnvFile)
		}

		if len(cfg.Volumes) > 0 {
			data += "\n[[volumes]]\n"
			for _, v := range cfg.Volumes {
				data += fmt.Sprintf("path = %q\n", v.Path)
				if v.Size != "" {
					data += fmt.Sprintf("size = %q\n", v.Size)
				}
			}
		}

		if err := os.WriteFile(config.DefaultPath(), []byte(data), 0644); err != nil {
			return fmt.Errorf("writing ship.toml: %w", err)
		}

		fmt.Fprintf(cmd.OutOrStdout(), "\n✓ Created ship.toml\n")
		fmt.Fprintf(cmd.OutOrStdout(), "  Next: ship servers create (provision a VPS)\n")
		fmt.Fprintf(cmd.OutOrStdout(), "  Then: ship deploy\n")
		return nil
	},
}

// ---- init --from fly --------------------------------------------------------

type flyConfig struct {
	App     string `toml:"app"`
	Build   struct {
		Dockerfile string   `toml:"dockerfile"`
		Args       []string `toml:"args"`
	} `toml:"build"`
	Env map[string]string `toml:"env"`
	HTTPService *struct {
		InternalPort int  `toml:"internal_port"`
		ForceHTTPS   bool `toml:"force_https"`
	} `toml:"http_service"`
	Mounts []struct {
		Source      string `toml:"source"`
		Destination string `toml:"destination"`
	} `toml:"mounts"`
	VM []struct {
		Size   string `toml:"size"`
		Memory string `toml:"memory"`
	} `toml:"vm"`
	Services []struct {
		InternalPort int `toml:"internal_port"`
	} `toml:"services"`
}

func initFromFly(cmd *cobra.Command) error {
	data, err := os.ReadFile("fly.toml")
	if err != nil {
		return fmt.Errorf("reading fly.toml: %w — not a Fly project?", err)
	}

	var fly flyConfig
	if err := toml.Unmarshal(data, &fly); err != nil {
		return fmt.Errorf("parsing fly.toml: %w", err)
	}

	cfg := &config.ShipConfig{}

	// App name
	cfg.App = fly.App
	if cfg.App == "" {
		cwd, _ := os.Getwd()
		cfg.App = filepath.Base(cwd)
	}

	// Dockerfile — resolve relative paths from fly.toml location
	cfg.Build.Dockerfile = fly.Build.Dockerfile
	if cfg.Build.Dockerfile == "" {
		cfg.Build.Dockerfile = "Dockerfile"
	}
	if strings.HasPrefix(cfg.Build.Dockerfile, "../") {
		resolved, err := filepath.Abs(filepath.Join(filepath.Dir("fly.toml"), cfg.Build.Dockerfile))
		if err == nil && fileExists(resolved) {
			cfg.Build.Dockerfile = filepath.Base(resolved)
			fmt.Fprintf(cmd.OutOrStdout(), "  Resolved Dockerfile: %s\n", cfg.Build.Dockerfile)
		} else {
			// Fallback: check common locations
			for _, f := range []string{"Dockerfile", "dockerfile", "Dockerfile.prod"} {
				if fileExists(f) {
					cfg.Build.Dockerfile = f
					fmt.Fprintf(cmd.OutOrStdout(), "  Dockerfile not found at %q — using ./%s\n", fly.Build.Dockerfile, f)
					break
				}
			}
		}
	}

	// Build args
	cfg.Build.Args = fly.Build.Args

	// Port
	cfg.Deploy.Port = 8080
	if fly.HTTPService != nil {
		cfg.Deploy.Port = fly.HTTPService.InternalPort
	}
	cfg.Deploy.HealthCheck = "/health"

	// Domains — detect from fly.toml env or services
	if hostname, ok := fly.Env["PRIMARY_REGION"]; ok && strings.Contains(hostname, ".") {
		cfg.Deploy.Domains = []string{hostname}
	}

	// Public env vars (non-sensitive ones from fly.toml)
	cfg.Env = make(map[string]string)
	for k, v := range fly.Env {
		// Skip secrets (usually set via fly secrets, not in fly.toml)
		if strings.Contains(strings.ToUpper(k), "SECRET") ||
			strings.Contains(strings.ToUpper(k), "PASSWORD") ||
			strings.Contains(strings.ToUpper(k), "KEY") {
			continue
		}
		cfg.Env[k] = v
	}

	// Volumes from mounts
	for _, m := range fly.Mounts {
		cfg.Volumes = append(cfg.Volumes, config.Volume{
			Path: m.Destination,
		})
	}

	// Generate ship.toml
	data2 := fmt.Sprintf(`# ship.toml — migrated from fly.toml
app = "%s"
`, cfg.App)

	if cfg.Server != "" {
		data2 += fmt.Sprintf("server = %q\n", cfg.Server)
	}

	data2 += fmt.Sprintf(`
[build]
dockerfile = "%s"
`, cfg.Build.Dockerfile)

	if len(cfg.Build.Args) > 0 {
		data2 += "args = ["
		for i, a := range cfg.Build.Args {
			if i > 0 {
				data2 += ", "
			}
			data2 += fmt.Sprintf("%q", a)
		}
		data2 += "]\n"
	}

	if len(cfg.Env) > 0 {
		data2 += "\n# Public, non-sensitive env vars — committed to git\n# For secrets use: ship secrets set KEY=value\n[env]\n"
		for k, v := range cfg.Env {
			data2 += fmt.Sprintf("%s = %q\n", k, v)
		}
	} else {
		data2 += "\n# Public, non-sensitive env vars — committed to git\n# For secrets use: ship secrets set KEY=value\n# [env]\n# NODE_ENV = \"production\"\n"
	}

	data2 += fmt.Sprintf(`
[deploy]
port = %d
health_check = "%s"
`, cfg.Deploy.Port, cfg.Deploy.HealthCheck)

	if len(cfg.Volumes) > 0 {
		data2 += "\n[[volumes]]\n"
		for _, v := range cfg.Volumes {
			data2 += fmt.Sprintf("path = %q\n", v.Path)
		}
	}

	if err := os.WriteFile(config.DefaultPath(), []byte(data2), 0644); err != nil {
		return err
	}

	fmt.Fprintf(cmd.OutOrStdout(), "✓ Generated ship.toml from fly.toml\n")
	fmt.Fprintf(cmd.OutOrStdout(), "  App: %s\n", cfg.App)
	fmt.Fprintf(cmd.OutOrStdout(), "  Port: %d\n", cfg.Deploy.Port)
	fmt.Fprintf(cmd.OutOrStdout(), "  Volumes: %d\n", len(cfg.Volumes))
	fmt.Fprintf(cmd.OutOrStdout(), "  Public env vars: %d\n", len(cfg.Env))
	fmt.Fprintln(cmd.OutOrStdout())
	fmt.Fprintln(cmd.OutOrStdout(), "  Next steps:")
	fmt.Fprintln(cmd.OutOrStdout(), "    1. Set server: edit ship.toml and set 'server' to your VPS IP")
	fmt.Fprintln(cmd.OutOrStdout(), "    2. Migrate secrets: ship secrets import .env.production")
	fmt.Fprintln(cmd.OutOrStdout(), "    3. Migrate data: ship migrate fly --to <server-ip>")
	fmt.Fprintln(cmd.OutOrStdout(), "    4. Deploy: ship deploy")
	return nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
