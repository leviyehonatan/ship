package migrate

import (
	"fmt"
	"os/exec"
	"strings"
)

type DBMigrator interface {
	Name() string
	Detect() bool
	Dump() ([]byte, error)
	RestoreCmd(appName string) string
	ConfigHints() map[string]string
}

var migrators = []DBMigrator{
	&PostgresMigrator{},
	&CouchDBMigrator{},
	&RedisMigrator{},
}

// ---- Postgres ----

type PostgresMigrator struct{}

func (m *PostgresMigrator) Name() string { return "Postgres" }

func (m *PostgresMigrator) Detect() bool {
	return shell("pg_isready -h 127.0.0.1").Run() == nil
}

func (m *PostgresMigrator) Dump() ([]byte, error) {
	out, err := shell("/bin/sh -c 'PGHOST=127.0.0.1 PGUSER=postgres pg_dump app 2>/dev/null || PGHOST=127.0.0.1 pg_dumpall 2>/dev/null'").Output()
	if err != nil {
		return nil, err
	}
	return skipSSHLine(out), nil
}

func (m *PostgresMigrator) RestoreCmd(appName string) string {
	return fmt.Sprintf("docker exec -i %s psql -U postgres -h 127.0.0.1 app < /opt/ship/data/ship_migrate_postgres", appName)
}

func (m *PostgresMigrator) ConfigHints() map[string]string {
	return map[string]string{"DATABASE_URL": "postgresql://postgres@127.0.0.1:5432/app"}
}

// ---- CouchDB ----

type CouchDBMigrator struct{}

func (m *CouchDBMigrator) Name() string { return "CouchDB" }

func (m *CouchDBMigrator) Detect() bool {
	return shell("test -d /data/couchdb").Run() == nil
}

func (m *CouchDBMigrator) Dump() ([]byte, error) {
	out, err := shell("tar czf /tmp/couchdb.tar.gz -C /data couchdb 2>/dev/null; cat /tmp/couchdb.tar.gz").Output()
	if err != nil {
		return nil, err
	}
	return skipSSHLine(out), nil
}

func (m *CouchDBMigrator) RestoreCmd(appName string) string {
	return fmt.Sprintf("docker exec -i %s tar xzf /opt/ship/data/ship_migrate_couchdb -C /", appName)
}

func (m *CouchDBMigrator) ConfigHints() map[string]string {
	return map[string]string{"COUCHDB_URL": "http://127.0.0.1:5984"}
}

// ---- Redis ----

type RedisMigrator struct{}

func (m *RedisMigrator) Name() string { return "Redis" }

func (m *RedisMigrator) Detect() bool {
	return shell("redis-cli -h 127.0.0.1 ping 2>/dev/null").Run() == nil
}

func (m *RedisMigrator) Dump() ([]byte, error) {
	out, err := shell("redis-cli -h 127.0.0.1 --rdb /tmp/redis.rdb SAVE 2>/dev/null; cat /tmp/redis.rdb 2>/dev/null").Output()
	if err != nil {
		return nil, nil
	}
	return skipSSHLine(out), nil
}

func (m *RedisMigrator) RestoreCmd(appName string) string {
	return fmt.Sprintf("docker cp /opt/ship/data/ship_migrate_redis %s:/data/dump.rdb && docker restart %s", appName, appName)
}

func (m *RedisMigrator) ConfigHints() map[string]string {
	return map[string]string{"REDIS_URL": "redis://127.0.0.1:6379"}
}

// ---- helpers ----

func Discover() []DBMigrator {
	var found []DBMigrator
	for _, m := range migrators {
		if m.Detect() {
			found = append(found, m)
		}
	}
	return found
}

func skipSSHLine(out []byte) []byte {
	s := string(out)
	if idx := strings.Index(s, "\n"); idx > 0 {
		return []byte(s[idx+1:])
	}
	return out
}

func shell(cmd string) *exec.Cmd {
	c := exec.Command("fly", "ssh", "console", "-C", cmd)
	return c
}
