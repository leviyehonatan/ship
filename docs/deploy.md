# Deploy

## Pipeline

```
ship deploy
  → start services (postgres, redis, etc.)
  → docker build (local, layer-cached)
  → docker save | SSH pipe | docker load (no registry)
  → docker run -e SECRET=... (env from .env.encrypted + ship.toml [env])
  → docker exec <release_command> (migrations, etc.)
  → Caddyfile updated (additive, multi-app safe)
  → pg_dump snapshot taken
```

## Steps

1. **Services** — starts sidecar containers defined in `[services]` if not running
2. **Snapshot** — backs up databases before deploy (safety net)
3. **Build** — `docker build` with args from ship.toml `[build.args]`. Skip with `--skip-build`
4. **Push** — image streams over SSH, no Docker Hub needed
5. **Run** — stops old container, starts new with env vars from `ship.toml [env]` + `.env.encrypted`
6. **Release** — runs `[deploy] release_command` inside the container (e.g. `prisma db push`)
7. **SSL** — updates Caddy config (additive, never overwrites other apps)
8. **Releases** — records deploy timestamp + image reference

## Server setup

Server setup runs automatically on first deploy (`/opt/ship/.setup-complete` marker). To run manually:

```bash
ship setup --server <ip>
```

Installs:
- Docker + Docker Compose
- Caddy (reverse proxy + auto Let's Encrypt)
- Docker log rotation (10MB max per file, 3 files)

## Build args

```toml
[build]
args = ["NEXT_PUBLIC_KEY=abc", "NODE_ENV=production"]
```

Passed as `--build-arg` to `docker build`. Docker's layer cache handles rebuilds — if args haven't changed, build is instant.

To skip building when only secrets changed, use `--skip-build`:

```bash
ship deploy --skip-build
```

## Release command

```toml
[deploy]
release_command = "npm run db:migrate"
```

Runs inside the new container after it starts. Use for database migrations, asset compilation, or cache warming. If it fails, a warning is shown but the deploy continues.

## SSL

```bash
ship ssl on myapp.com     # enable HTTPS
ship ssl off               # remove
ship ssl status            # show certificates
ship ssl renew             # force renewal
```

Caddy auto-fetches Let's Encrypt certificates and auto-renews 30 days before expiry. Set `domains` in ship.toml `[deploy]` to auto-configure on deploy.

Multiple apps on same server: each gets its own Caddy block. Ship never overwrites existing configurations.

## Volumes

```toml
[[volumes]]
path = "/data"
size = "10GB"
```

- **Production:** mounts at `/opt/ship/data/<app>/<path>`
- **Local:** mounts at `.ship-data/<app>/<path>` (gitignored, Docker Desktop compatible)
- **No volumes:** container state lost on redeploy (warning shown)

## Rollback

```bash
ship rollback              # restore latest snapshot
ship rollback 20260809     # restore specific snapshot
ship snapshots             # list available
```

Snapshots cover Postgres + CouchDB + Redis. Created automatically before each deploy.
