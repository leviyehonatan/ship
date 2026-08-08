package deploy

import (
	"os"
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
