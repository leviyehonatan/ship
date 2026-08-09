# Ship critique — real-world feedback from deploying tunity-fresh

Date: 2026-08-09

## Context

We attempted to use `ship` to deploy [tunity-fresh](https://github.com/leviyehonatan/tunity-fresh) (a Next.js + Hono monorepo with Postgres, Redis) in local mode, then to a colima VM. This document captures everything that worked and everything that didn't.

## What ship does well

- **One-command deploy** (`ship deploy`) is the right goal — build, push over SSH, run, SSL
- **SSH pipe for images** — no Docker registry needed, clever and works
- **`ship.toml`** — clean, readable format, better than fly.toml
- **`--from fly` migration path** — smart way to poach Fly users
- **Caddy auto-SSL** — good integration
- **Build caching works** — second deploy is fast when source hasn't changed

---

## What's broken or missing

### 1. `ship local` only works with Docker Desktop ✅ FIXED

~~`ship local start` bind-mounts the Docker socket. It checks `/var/run/docker.sock` and `$HOME/.docker/run/docker.sock`, but neither exists with colima/Orbstack/Rancher Desktop.~~

Now uses `detectDockerSocket()` which checks `DOCKER_HOST`, `docker context inspect`, and common paths (colima, Orbstack, Docker Desktop).

### 2. `[services]` in ship.toml is silently ignored ✅ FIXED

~~The `ship.toml` on the shipify branch defines `[services.postgres]` and `[services.redis]`, but `internal/config/config.go` had no `Services` field — they were parsed and thrown away.~~

Now `ServiceConfig` is parsed and services are auto-started via `docker run` on deploy if they're not already running.

### 3. `ship use local` doesn't create a server entry ✅ FIXED

~~It sets the current server name to "local" but never writes `~/.config/ship/servers/local.json`.~~

Now `ship use local` auto-creates the server entry with `ip: "localhost:2222"`.

### 4. Go SSH client is too rigid ✅ FIXED

~~`internal/ssh/client.go` uses `golang.org/x/crypto/ssh` directly. It hardcodes user `root`, key `~/.ssh/id_rsa`, port 22.~~

Now shells out to the system `ssh` CLI, giving full `~/.ssh/config`, SSH agent, ControlMaster, and jump host support for free.

### 5. `ship deploy` always rebuilds, even for secret-only changes

Changing a secret with `ship secrets set` triggers a full `docker build` (~5 min for tunity). Env vars are passed as `-e` flags at `docker run` time — the image doesn't change. The rebuild is wasted time.

```
$ ship secrets set FOO=bar   # changes .env.encrypted only
$ ship deploy                # full rebuild, no source changes
```

**Fix**: Separate build from run. Hash the build context and skip if unchanged. Or add `--skip-build` and `--build-only` flags. At minimum, detect that only `.env.encrypted` changed and skip the build.

### 6. Secrets/env workflow has sharp edges

- `ship secrets` encrypts to `.env.encrypted` — `ship deploy` passes them as `-e` flags. Two sources of truth.
- No guidance on which vars are public (`[env]` in ship.toml) vs secret (`ship secrets set`).
- There's a `ship secrets import .env` command in the docs but it's not discoverable.
- No way to see what env vars the container will receive without reading encrypted files.
- We had to SSH into the Fly production instance to discover all required env vars.

**Fix**: Add `ship env` to show all resolved env (public + secret, decrypted). Add `--dry-run` to `ship deploy` to print the full `docker run` command.

### 7. No validation or warnings

- Unknown keys in `ship.toml` (like `[services]`) are silently dropped — no error, no warning.
- When the container crashes immediately, `ship deploy` still prints `✓` and a health URL.
- `DATABASE_URL=127.0.0.1:5432` can't work when Postgres is a separate container, but ship doesn't warn.
- The Dockerfile hardcoded `HOSTNAME=127.0.0.1` making the container unreachable via Docker port mapping — no health check caught this.

**Fix**: Validate `ship.toml` against the struct, warn on unknown keys. Run a health check after deploy and report the result. If the container exits within 5 seconds, surface the logs.

### 8. No post-deploy hooks (release commands)

Fly has `[deploy] release_command` for migrations. Ship has nothing. Tunity needed `prisma db push` before it could work. We had to manually `docker exec` into the container.

**Fix**: Add `[deploy] release_command` in ship.toml. Run it once after the container starts, before marking the deploy as healthy.

### 9. Inconsistent `--server` flag

`ship setup` and `ship deploy` accept `--server`, but `ship doctor` doesn't:

```
$ ship doctor --server colima
Error: unknown flag: --server
```

We had to `cd` into the project directory for every command.

**Fix**: Support `--server` on every command that talks to a server.

### 10. macOS Keychain key generation failed silently

```
$ ship keys generate keychain
✓ Key generated (keychain)
```

But the key wasn't actually stored (Keychain access prompt never appeared in our non-interactive terminal). Later `ship secrets set` worked only because we ran `ship keys generate file` after.

**Fix**: Verify the key is retrievable after storing it. If Keychain fails in a non-interactive terminal, fall back or warn clearly.

---

## Summary: priority ordered changes

| Priority | Change | Status |
|----------|--------|--------|
| 🔴 | Shell out to system `ssh` instead of Go SSH library | ✅ Done |
| 🔴 | Implement `[services]` in config or error on unknown keys | ✅ Done |
| 🔴 | Fix `ship local` for colima/Orbstack/Rancher (detect socket) | ✅ Done |
| 🔴 | `ship use local` → auto-create server JSON | ✅ Done |
| 🟡 | Add `--skip-build` / detect when build is unnecessary | ✅ Done |
| 🟡 | Validate `ship.toml` schema, warn on unknown keys | ✅ Done |
| 🟡 | Add `[deploy] release_command` for migrations | ✅ Done |
| 🟡 | `ship env` to show all resolved env vars | ✅ Done |
| 🟡 | Consistent `--server` flag across all commands | ✅ Done (doctor) |
| 🟢 | Post-deploy health check (don't lie with ✓) | ⬜ TODO |
| 🟢 | `--dry-run` to print the docker run command | ⬜ TODO |
| 🟢 | `ship secrets import .env` surfacing in UX | ⬜ TODO |
| 🟢 | Verify keychain storage actually worked | ⬜ TODO |

## What tunity's shipify branch needs to fix

These are issues in the tunity Dockerfile/config, not ship itself:

1. **Dockerfile CMD**: Change `HOSTNAME=127.0.0.1` → `HOSTNAME=0.0.0.0` so Docker port mapping works
2. **DATABASE_URL / REDIS_URL**: Use the Docker bridge gateway IP (e.g. `172.17.0.1`) instead of `127.0.0.1` when services run as separate containers
3. **Missing Caddy**: The slimmed-down Dockerfile removed Caddy but `[deploy] port = 8080` maps to a port nothing listens on — add a tiny reverse proxy or document that Next.js port should be used instead
