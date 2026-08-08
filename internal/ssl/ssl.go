package ssl

import (
	"fmt"
	"strings"

	shipssh "github.com/leviyehonatan/ship/internal/ssh"
)

// Configure adds or updates a Caddy reverse proxy entry for a domain.
// It never overwrites existing entries for other domains.
func Configure(client *shipssh.Client, domain string, port int) error {
	if domain == "" {
		return nil
	}

	// Read existing Caddyfile or create default
	existing, err := client.Run("cat /etc/caddy/Caddyfile 2>/dev/null")
	if err != nil {
		existing = ""
	}

	// Build the new Caddy block
	newBlock := fmt.Sprintf(`%s {
    reverse_proxy localhost:%d
}
`, domain, port)

	var updated string
	if strings.Contains(existing, domain) {
		// Replace existing block for this domain
		updated = replaceBlock(existing, domain, newBlock)
	} else {
		// Append new block
		if strings.TrimSpace(existing) == "" {
			updated = fmt.Sprintf("# Managed by ship\n%s", newBlock)
		} else {
			updated = existing + "\n" + newBlock
		}
	}

	// Write updated Caddyfile
	writeCmd := fmt.Sprintf(`cat > /etc/caddy/Caddyfile << 'SHIPEOF'
%s
SHIPEOF`, updated)
	if _, err := client.Run(writeCmd); err != nil {
		return fmt.Errorf("writing Caddyfile: %w", err)
	}

	// Reload Caddy (try systemctl, then service, then caddy reload)
	reload := "systemctl reload caddy 2>/dev/null || service caddy reload 2>/dev/null || caddy reload --config /etc/caddy/Caddyfile 2>/dev/null || true"
	client.Run(reload)

	return nil
}

// Remove removes a domain from the Caddyfile.
func Remove(client *shipssh.Client, domain string) error {
	existing, err := client.Run("cat /etc/caddy/Caddyfile 2>/dev/null")
	if err != nil {
		return nil
	}

	updated := removeBlock(existing, domain)
	if updated == existing {
		return nil
	}

	writeCmd := fmt.Sprintf(`cat > /etc/caddy/Caddyfile << 'SHIPEOF'
%s
SHIPEOF`, updated)
	if _, err := client.Run(writeCmd); err != nil {
		return err
	}

	reload := "systemctl reload caddy 2>/dev/null || service caddy reload 2>/dev/null || true"
	client.Run(reload)
	return nil
}

// Status returns certificate info for all domains.
func Status(client *shipssh.Client) (string, error) {
	out, err := client.Run("cat /etc/caddy/Caddyfile 2>/dev/null")
	if err != nil {
		return "", nil
	}
	if strings.TrimSpace(out) == "" {
		return "No domains configured.", nil
	}

	// List configured domains
	lines := strings.Split(out, "\n")
	var domains []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "}") || line == "{" {
			continue
		}
		if strings.Contains(line, "reverse_proxy") {
			continue
		}
		domains = append(domains, line)
	}

	result := "Configured domains:\n"
	for _, d := range domains {
		result += fmt.Sprintf("  https://%s\n", d)
	}
	return result, nil
}

// ---- internal helpers ----

func replaceBlock(caddyfile, domain, newBlock string) string {
	// Find the block starting with this domain and replace it
	lines := strings.Split(caddyfile, "\n")
	var result []string
	inBlock := false

	for i := 0; i < len(lines); i++ {
		line := lines[i]
		trimmed := strings.TrimSpace(line)

		if trimmed == domain {
			inBlock = true
			result = append(result, strings.Split(newBlock, "\n")...)
			// Skip until we find the closing brace
			for i < len(lines) {
				i++
				if i < len(lines) && strings.TrimSpace(lines[i]) == "}" {
					break
				}
			}
			continue
		}
		if inBlock && trimmed == "}" {
			inBlock = false
			continue
		}
		if !inBlock {
			result = append(result, line)
		}
	}

	return strings.Join(result, "\n")
}

func removeBlock(caddyfile, domain string) string {
	lines := strings.Split(caddyfile, "\n")
	var result []string
	skipMode := false

	for i := 0; i < len(lines); i++ {
		line := lines[i]
		trimmed := strings.TrimSpace(line)

		if trimmed == domain {
			skipMode = true
			continue
		}
		if skipMode && trimmed == "}" {
			skipMode = false
			continue
		}
		if !skipMode {
			result = append(result, line)
		}
	}

	return strings.Join(result, "\n")
}
