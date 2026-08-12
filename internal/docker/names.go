package deploy

import (
	"fmt"
	"strings"

	"github.com/leviyehonatan/ship/internal/ssh"
)

// Naming convention: every Docker resource ship manages is prefixed
// with "ship-" and segmented by resource kind. This keeps ship's
// containers, networks, and images from ever colliding with resources
// the user manages themselves.
//
//   ship-app-<app>        app container
//   ship-svc-<app>-<name> sidecar service container
//   ship-pg-<name>        standalone Postgres (server-scoped)
//   ship-net-<app>        per-app bridge network
//   <app>:latest          app image (images are a separate namespace)

const (
	// ManagedLabel marks a container as ship-managed. Ship only ever
	// removes containers carrying this label — never foreign ones.
	ManagedLabel = "com.ship.managed"

	// AppLabel records which app a container belongs to.
	AppLabel = "com.ship.app"
)

// AppContainer returns the container name for an app.
func AppContainer(app string) string { return "ship-app-" + app }

// ServiceContainer returns the container name for a sidecar service.
func ServiceContainer(app, svc string) string { return "ship-svc-" + app + "-" + svc }

// PgContainer returns the container name for a standalone Postgres.
func PgContainer(name string) string { return "ship-pg-" + name }

// Network returns the bridge network name for an app.
func Network(app string) string { return "ship-net-" + app }

// ImageRef returns the image tag for an app's image.
func ImageRef(app string) string { return app + ":latest" }

// RunDocker runs a docker command locally (client == nil) or over SSH.
func RunDocker(client *ssh.Client, cmd string) (string, error) {
	if client == nil {
		return runLocal(cmd)
	}
	return client.Run(cmd)
}

// RemoveIfManaged removes a container only if ship created it.
//
// It inspects the container's com.ship.managed label:
//   - container doesn't exist          → no-op (nothing to remove)
//   - label == "true"                  → docker rm -f (safe, it's ours)
//   - exists but label != "true"       → error, leaves it alone
//
// This protects against silently destroying a container the user
// created themselves that happens to share a name.
func RemoveIfManaged(client *ssh.Client, name string) error {
	format := fmt.Sprintf("docker inspect --format '{{ index .Config.Labels %q }}' %s 2>/dev/null", ManagedLabel, name)
	out, err := RunDocker(client, format)
	if err != nil {
		// inspect failed → container doesn't exist (or daemon unreachable;
		// the subsequent docker run will surface that loudly).
		return nil
	}
	out = strings.TrimSpace(out)
	if out == "true" {
		_, _ = RunDocker(client, fmt.Sprintf("docker rm -f %s 2>/dev/null || true", name))
		return nil
	}
	// Container exists but is not ship-managed. Removing it would destroy
	// someone else's container — refuse and let the caller surface a clear
	// error instead of `docker run` failing with a confusing message.
	return fmt.Errorf("container %q already exists but is not ship-managed (label %q is %q) — refusing to remove it", name, ManagedLabel, out)
}
