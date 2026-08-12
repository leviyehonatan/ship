package deploy

import (
	"os/exec"
	"strings"
	"testing"
)

func TestNamingConvention(t *testing.T) {
	cases := []struct {
		desc string
		got  string
		want string
	}{
		{"app container", AppContainer("tunity"), "ship-app-tunity"},
		{"service container", ServiceContainer("tunity", "postgres"), "ship-svc-tunity-postgres"},
		{"pg container", PgContainer("analytics"), "ship-pg-analytics"},
		{"network", Network("tunity"), "ship-net-tunity"},
		{"image ref", ImageRef("tunity"), "tunity:latest"},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("%s = %q, want %q", c.desc, c.got, c.want)
		}
	}
}

func TestNamingDoesNotCollide(t *testing.T) {
	// Two apps on the same server with the same service must not collide.
	if ServiceContainer("a", "postgres") == ServiceContainer("b", "postgres") {
		t.Error("service containers for different apps collide")
	}
	// An app and a service must never share a name.
	if AppContainer("x") == ServiceContainer("x", "x") {
		t.Error("app container collides with a service container")
	}
	// A service and a standalone pg must never share a name.
	if ServiceContainer("x", "y") == PgContainer("y") {
		t.Error("service container collides with pg container")
	}
	// No name may escape the ship- namespace.
	for _, n := range []string{
		AppContainer("t"),
		ServiceContainer("t", "s"),
		PgContainer("p"),
		Network("t"),
	} {
		if !strings.HasPrefix(n, "ship-") {
			t.Errorf("%q does not use the ship- namespace", n)
		}
	}
}

func TestRemoveIfManaged(t *testing.T) {
	if testing.Short() {
		t.Skip("short mode")
	}
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("docker not installed")
	}
	if err := exec.Command("docker", "info").Run(); err != nil {
		t.Skip("docker daemon not running")
	}

	// Foreign container (no ship label) — must NOT be removed.
	foreign := "ship-test-foreign"
	exec.Command("docker", "rm", "-f", foreign).Run()
	if out, err := exec.Command("docker", "run", "-d", "--name", foreign, "alpine:3.21", "sleep", "300").CombinedOutput(); err != nil {
		t.Skipf("cannot start foreign container: %v\n%s", err, out)
	}
	t.Cleanup(func() { exec.Command("docker", "rm", "-f", foreign).Run() })

	err := RemoveIfManaged(nil, foreign)
	if err == nil {
		t.Error("RemoveIfManaged removed a foreign container — should have refused")
	} else if !strings.Contains(err.Error(), "not ship-managed") {
		t.Errorf("unexpected error: %v", err)
	}
	if out, _ := exec.Command("docker", "ps", "-a", "--filter", "name="+foreign, "--format", "{{.Names}}").Output(); !strings.Contains(string(out), foreign) {
		t.Error("foreign container was removed despite refusal")
	}

	// Ship-managed container (with label) — must be removed.
	managed := "ship-test-managed"
	exec.Command("docker", "rm", "-f", managed).Run()
	if out, err := exec.Command("docker", "run", "-d", "--name", managed, "--label", ManagedLabel+"=true", "alpine:3.21", "sleep", "300").CombinedOutput(); err != nil {
		t.Skipf("cannot start managed container: %v\n%s", err, out)
	}
	if err := RemoveIfManaged(nil, managed); err != nil {
		t.Errorf("RemoveIfManaged failed on managed container: %v", err)
	}
	if out, _ := exec.Command("docker", "ps", "-a", "--filter", "name="+managed, "--format", "{{.Names}}").Output(); strings.Contains(string(out), managed) {
		t.Error("managed container was not removed")
	}

	// Non-existent container — must be a no-op.
	if err := RemoveIfManaged(nil, "ship-test-does-not-exist"); err != nil {
		t.Errorf("RemoveIfManaged on non-existent container: %v", err)
	}
}
