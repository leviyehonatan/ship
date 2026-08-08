package dockerfile

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

type ServiceHint struct {
	Name     string
	Image    string
	Port     int
	Detected bool // true if auto-detected from Dockerfile
}

func Parse(path string) []ServiceHint {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	var hints []ServiceHint
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		for _, h := range detectService(line) {
			hints = append(hints, h)
		}
	}
	return deduplicate(hints)
}

func detectService(line string) []ServiceHint {
	lower := strings.ToLower(line)
	var hints []ServiceHint

	if (strings.Contains(lower, "postgresql") || strings.Contains(lower, "apk add") && strings.Contains(lower, "postgres")) && !strings.Contains(lower, "postgres_") {
		hints = append(hints, ServiceHint{Name: "postgres", Image: "postgres:16-alpine", Port: 5432, Detected: true})
	}
	if strings.Contains(lower, "redis") && !strings.Contains(lower, "redis_url") && !strings.Contains(lower, "redis_") {
		hints = append(hints, ServiceHint{Name: "redis", Image: "redis:7-alpine", Port: 6379, Detected: true})
	}
	if strings.Contains(lower, "couchdb") && !strings.Contains(lower, "couchdb_") {
		hints = append(hints, ServiceHint{Name: "couchdb", Image: "couchdb:latest", Port: 5984, Detected: true})
	}
	return hints
}

func deduplicate(hints []ServiceHint) []ServiceHint {
	seen := make(map[string]bool)
	var result []ServiceHint
	for _, h := range hints {
		if !seen[h.Name] {
			seen[h.Name] = true
			result = append(result, h)
		}
	}
	return result
}

func (h ServiceHint) TOML() string {
	return fmt.Sprintf(`
[services.%s]
image = %q
port = %d
`, h.Name, h.Image, h.Port)
}
