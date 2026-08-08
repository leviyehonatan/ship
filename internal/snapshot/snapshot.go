package snapshot

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	shipssh "github.com/leviyehonatan/ship/internal/ssh"
)

type Manager struct {
	client   *shipssh.Client
	appName  string
	snapshotDir string
}

func NewManager(client *shipssh.Client, appName string) *Manager {
	return &Manager{
		client:      client,
		appName:     appName,
		snapshotDir: fmt.Sprintf("/opt/ship/snapshots/%s", appName),
	}
}

func (m *Manager) Create() error {
	ts := time.Now().UTC().Format("20060102-150405")
	dir := fmt.Sprintf("%s/%s", m.snapshotDir, ts)

	// Ensure snapshot directory exists
	m.client.Run(fmt.Sprintf("mkdir -p %s", dir))

	// Dump Postgres
	pgCmd := fmt.Sprintf(
		"docker exec %s pg_dump -U hono -h 127.0.0.1 tunity > %s/pg_dump.sql",
		m.appName, dir,
	)
	if _, err := m.client.Run(pgCmd); err != nil {
		return fmt.Errorf("pg_dump: %w", err)
	}

	// Backup CouchDB data
	couchCmd := fmt.Sprintf(
		"docker exec %s tar czf - /data/couchdb > %s/couchdb.tar.gz",
		m.appName, dir,
	)
	if _, err := m.client.Run(couchCmd); err != nil {
		// CouchDB might not exist, non-fatal
		fmt.Fprintf(os.Stderr, "Warning: CouchDB backup failed: %v\n", err)
	}

	// Backup Redis if it persists to disk
	redisCmd := fmt.Sprintf(
		"docker exec %s redis-cli -h 127.0.0.1 SAVE 2>/dev/null && docker cp %s:/tmp/redis-data %s/redis-data 2>/dev/null || true",
		m.appName, m.appName, dir,
	)
	m.client.Run(redisCmd)

	// Keep last 5 snapshots
	m.client.Run(fmt.Sprintf(
		"ls -1d %s/*/ | sort -r | tail -n +6 | xargs -r rm -rf", m.snapshotDir,
	))

	return nil
}

func (m *Manager) List() ([]string, error) {
	out, err := m.client.Run(fmt.Sprintf("ls -1d %s/*/ 2>/dev/null || true", m.snapshotDir))
	if err != nil {
		return nil, err
	}
	var snapshots []string
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		line = strings.TrimSuffix(line, "/")
		parts := strings.Split(line, "/")
		if len(parts) > 0 {
			snapshots = append(snapshots, parts[len(parts)-1])
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(snapshots)))
	return snapshots, nil
}

func (m *Manager) Restore(snapshotID string) error {
	if snapshotID == "" {
		return fmt.Errorf("snapshot ID required — use 'ship snapshots' to list")
	}

	dir := fmt.Sprintf("%s/%s", m.snapshotDir, snapshotID)

	// Stop container
	m.client.Run(fmt.Sprintf("docker stop %s 2>/dev/null || true", m.appName))

	// Restore Postgres
	restorePG := fmt.Sprintf(
		"docker start %s && sleep 3 && cat %s/pg_dump.sql | docker exec -i %s psql -U hono -h 127.0.0.1 tunity",
		m.appName, dir, m.appName,
	)
	if _, err := m.client.Run(restorePG); err != nil {
		return fmt.Errorf("pg_restore: %w", err)
	}

	// Restore CouchDB
	restoreCouch := fmt.Sprintf(
		"cat %s/couchdb.tar.gz | docker exec -i %s tar xzf - -C /",
		dir, m.appName,
	)
	m.client.Run(restoreCouch)

	return nil
}
