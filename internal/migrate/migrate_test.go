package migrate

import (
	"os"
	"strings"
	"testing"
)

func TestSkipSSHLine(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"Connecting to ...\nreal data", "real data"},
		{"no newline here", "no newline here"},
		{"line1\nline2\nline3", "line2\nline3"},
	}
	for _, tt := range tests {
		got := string(skipSSHLine([]byte(tt.in)))
		if got != tt.want {
			t.Errorf("skipSSHLine(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestSanitize(t *testing.T) {
	tests := []struct{ in, want string }{
		{"Postgres", "postgres"},
		{"CouchDB", "couchdb"},
		{"My SQL", "my_sql"},
	}
	for _, tt := range tests {
		if got := sanitize(tt.in); got != tt.want {
			t.Errorf("sanitize(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestDiscover(t *testing.T) {
	// Save original migrators
	original := migrators
	defer func() { migrators = original }()

	// Replace with test migrators
	migrators = []DBMigrator{
		&testMigrator{name: "Postgres", present: true},
		&testMigrator{name: "CouchDB", present: false},
		&testMigrator{name: "Redis", present: true},
	}

	found := Discover()
	if len(found) != 2 {
		t.Fatalf("expected 2 databases, got %d", len(found))
	}
	if found[0].Name() != "Postgres" {
		t.Errorf("first = %s, want Postgres", found[0].Name())
	}
	if found[1].Name() != "Redis" {
		t.Errorf("second = %s, want Redis", found[1].Name())
	}
}

func TestDiscoverNone(t *testing.T) {
	original := migrators
	defer func() { migrators = original }()

	migrators = []DBMigrator{
		&testMigrator{name: "Postgres", present: false},
		&testMigrator{name: "Redis", present: false},
	}

	found := Discover()
	if len(found) != 0 {
		t.Errorf("expected 0 databases, got %d", len(found))
	}
}

func TestRestoreCommands(t *testing.T) {
	migrators := []DBMigrator{
		&PostgresMigrator{},
		&CouchDBMigrator{},
		&RedisMigrator{},
	}
	tests := []struct {
		name    string
		app     string
		contain string
	}{
		{"Postgres", "my-app", "docker exec -i my-app psql"},
		{"CouchDB", "my-app", "docker exec -i my-app tar xzf"},
		{"Redis", "my-app", "docker cp"},
	}
	for _, tt := range tests {
		for _, m := range migrators {
			if m.Name() == tt.name {
				cmd := m.RestoreCmd(tt.app)
				if !strings.Contains(cmd, tt.contain) {
					t.Errorf("%s restore missing %q: %s", tt.name, tt.contain, cmd)
				}
			}
		}
	}
}

func TestConfigHints(t *testing.T) {
	migrators := []DBMigrator{
		&PostgresMigrator{},
		&CouchDBMigrator{},
		&RedisMigrator{},
	}
	for _, m := range migrators {
		hints := m.ConfigHints()
		if len(hints) == 0 {
			t.Errorf("%s: no config hints", m.Name())
		}
	}
}

func TestGenerateShipTOML(t *testing.T) {
	dir := t.TempDir()

	// Create a minimal fly.toml
	flyToml := `app = "testapp"

[build]
dockerfile = "Dockerfile"

[http_service]
internal_port = 3000

[[mounts]]
source = "data"
destination = "/data"
`
	os.WriteFile(dir+"/fly.toml", []byte(flyToml), 0644)
	cwd, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(cwd)

	if err := generateShipTOML("1.2.3.4", "testapp"); err != nil {
		t.Fatalf("generateShipTOML: %v", err)
	}

	data, err := os.ReadFile("ship.toml")
	if err != nil {
		t.Fatal("ship.toml not created")
	}
	content := string(data)

	checks := []string{
		`app = "testapp"`,
		`server = "1.2.3.4"`,
		`dockerfile = "Dockerfile"`,
		`port = 3000`,
		`path = "/data"`,
	}
	for _, c := range checks {
		if !strings.Contains(content, c) {
			t.Errorf("ship.toml missing %q\n\nGot:\n%s", c, content)
		}
	}
}

func TestWriteMinimalShipTOML(t *testing.T) {
	dir := t.TempDir()
	cwd, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(cwd)

	if err := writeMinimalShipTOML("46.224.91.70", "minimal-app"); err != nil {
		t.Fatalf("writeMinimalShipTOML: %v", err)
	}

	data, _ := os.ReadFile("ship.toml")
	content := string(data)
	if !strings.Contains(content, `app = "minimal-app"`) {
		t.Error("missing app name")
	}
	if !strings.Contains(content, `server = "46.224.91.70"`) {
		t.Error("missing server")
	}
	if !strings.Contains(content, `port = 8080`) {
		t.Error("missing default port")
	}
}

// ---- test migrator (mock) ----

type testMigrator struct {
	name    string
	present bool
}

func (m *testMigrator) Name() string                              { return m.name }
func (m *testMigrator) Detect() bool                               { return m.present }
func (m *testMigrator) Dump() ([]byte, error)                      { return nil, nil }
func (m *testMigrator) RestoreCmd(app string) string               { return "echo " + m.name }
func (m *testMigrator) ConfigHints() map[string]string             { return nil }
