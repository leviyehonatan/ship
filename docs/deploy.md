# Deploy

## Pipeline

```
ship deploy
  → docker build (local, layer-cached)
  → docker save | SSH pipe | docker load (no registry)
  → docker run -e SECRET=... (env from .env.encrypted)
  → Caddyfile updated (additive, multi-app safe)
  → pg_dump snapshot taken
```

## Steps

1. **Snapshot** — backs up databases before deploy (safety net)
2. **Build** — `docker build` with args from ship.toml `[build.args]`
3. **Push** — image streams over SSH, no Docker Hub needed
4. **Run** — stops old container, starts new with env vars
5. **SSL** — updates Caddy config (additive, never overwrites other apps)
6. **Releases** — records deploy timestamp + image reference

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
