# ship

Deploy containers to your own VPS. Build, push, run — one command.

```bash
ship servers create my-app           # provision a VPS
ship secrets set DATABASE_URL=...    # set encrypted secrets
ship deploy                          # build → push → run → SSL
```

## Install

```bash
go install github.com/leviyehonatan/ship/cmd/ship@v0.2.0 # → ship
```

## Documentation

- [Getting Started](docs/getting-started.md) — mental model, step-by-step walkthrough
- [Commands](docs/commands.md) — full command reference
- [Secrets & Keys](docs/secrets.md) — encryption, key storage, syncing
- [Deploy](docs/deploy.md) — pipeline, SSL, rollback, snapshots
- [Migration](docs/migrating.md) — migrating from Fly.io
- [Configuration](docs/configuration.md) — ship.toml reference, providers

## Quick example

```bash
ship whoami
ship servers create my-app --size cx23
ship use my-app

cd my-project
ship init
ship keys generate keychain   # choose: keychain, file, or env
ship secrets set DATABASE_URL=postgresql://...
ship deploy                          # auto-sets up server on first run
ship ssl on myapp.com
```

## Architecture

Zero SDK dependencies. Shells out to existing tools (hcloud, docker, ssh).

```
ship deploy
  → docker build (local, cached)
  → docker save | SSH pipe | docker load (no registry)
  → docker run -e SECRET=...
  → Caddyfile updated (additive)
  → snapshot taken
```

## Testing

```bash
go test ./... -short               # 31 tests, ~8s
go test ./internal/e2e/ -v         # full pipeline against SSH container
```
