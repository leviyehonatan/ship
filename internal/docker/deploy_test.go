package deploy

import (
	"os"
	"strings"
	"testing"
)

func TestParseEnvFile(t *testing.T) {
	content := `
# comment
DATABASE_URL=postgresql://localhost/db
COUCHDB_PASSWORD=secret123
EMPTY_LINE=

REDIS_URL=redis://localhost:6379
`
	os.WriteFile("test.env", []byte(content), 0644)
	defer os.Remove("test.env")

	env, err := ParseEnvFile("test.env")
	if err != nil {
		t.Fatalf("ParseEnvFile: %v", err)
	}

	if env["DATABASE_URL"] != "postgresql://localhost/db" {
		t.Errorf("DATABASE_URL = %q", env["DATABASE_URL"])
	}
	if env["COUCHDB_PASSWORD"] != "secret123" {
		t.Errorf("COUCHDB_PASSWORD = %q", env["COUCHDB_PASSWORD"])
	}
	if env["EMPTY_LINE"] != "" {
		t.Errorf("EMPTY_LINE should be empty string, got %q", env["EMPTY_LINE"])
	}
	if _, ok := env["NONEXISTENT"]; ok {
		t.Error("NONEXISTENT should not be set")
	}
	if v, ok := env["# comment"]; ok {
		t.Errorf("comment line parsed as key: %q", v)
	}
}

func TestStopRemoveNilClient(t *testing.T) {
	d := NewDeployerWithEnv("test-app", nil, nil)
	err := d.StopRemove()
	if err != nil {
		t.Errorf("StopRemove with nil client should not error, got: %v", err)
	}
}

func TestStopRemoveSvcNilClient(t *testing.T) {
	d := NewDeployerWithEnv("test-app", nil, nil)
	err := d.StopRemoveSvc("postgres")
	if err != nil {
		t.Errorf("StopRemoveSvc with nil client should not error, got: %v", err)
	}
}

func TestPushOverSSHNilClient(t *testing.T) {
	d := NewDeployerWithEnv("test-app", nil, nil)
	err := d.PushOverSSH()
	if err != nil {
		t.Errorf("PushOverSSH with nil client should be no-op, got: %v", err)
	}
}

func TestRunRemoteNilClient(t *testing.T) {
	d := NewDeployerWithEnv("test-app", map[string]string{"FOO": "bar"}, nil)
	err := d.RunRemote(RunOpts{
		Ports:   []string{"3000:3000"},
		Volumes: []string{"/tmp:/data"},
	})
	if err == nil {
		t.Error("RunRemote with nil client and no image should still fail on docker run")
	}
}

func TestStatusNilClient(t *testing.T) {
	d := NewDeployerWithEnv("test-app", nil, nil)
	_, _ = d.Status()
	// Expected: container doesn't exist locally — no crash is success
}

func TestLogsNilClient(t *testing.T) {
	d := NewDeployerWithEnv("test-app", nil, nil)
	var buf strings.Builder
	_ = d.Logs(&buf, "10")
	// Expected: container doesn't exist locally — no crash is success
}
