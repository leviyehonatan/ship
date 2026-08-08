package releases

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	shipssh "github.com/leviyehonatan/ship/internal/ssh"
)

type Release struct {
	Version   int    `json:"version"`
	Image     string `json:"image"`
	Timestamp string `json:"timestamp"`
	Commit    string `json:"commit,omitempty"`
}

const releasesPath = "/opt/ship/releases"

func Record(client *shipssh.Client, appName, imageTag string) error {
	client.Run(fmt.Sprintf("mkdir -p %s", releasesPath))

	// Read existing releases
	existing := readAll(client, appName)

	// Create new release
	release := Release{
		Version:   len(existing) + 1,
		Image:     imageTag,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}

	// Try to get git commit
	commit, _ := client.Run("git -C /opt/ship rev-parse --short HEAD 2>/dev/null")
	if commit != "" {
		release.Commit = strings.TrimSpace(commit)
	}

	existing = append(existing, release)

	// Keep last 50 releases
	if len(existing) > 50 {
		existing = existing[len(existing)-50:]
	}

	data, err := json.Marshal(existing)
	if err != nil {
		return err
	}

	client.Run(fmt.Sprintf("cat > %s/%s.json << 'RELEOF'\n%s\nRELEOF",
		releasesPath, appName, string(data)))
	return nil
}

func List(client *shipssh.Client, appName string) ([]Release, error) {
	return readAll(client, appName), nil
}

func Latest(client *shipssh.Client, appName string) (*Release, error) {
	all := readAll(client, appName)
	if len(all) == 0 {
		return nil, fmt.Errorf("no releases for %s", appName)
	}
	return &all[len(all)-1], nil
}

func readAll(client *shipssh.Client, appName string) []Release {
	out, err := client.Run(fmt.Sprintf("cat %s/%s.json 2>/dev/null", releasesPath, appName))
	if err != nil || out == "" {
		return nil
	}
	var releases []Release
	if err := json.Unmarshal([]byte(out), &releases); err != nil {
		return nil
	}
	sort.Slice(releases, func(i, j int) bool {
		return releases[i].Version < releases[j].Version
	})
	return releases
}
