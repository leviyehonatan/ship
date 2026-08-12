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

## Running from source

Add to your shell config (`~/.zshrc` / `~/.bashrc`):

```bash
ship() {
  local bin=/tmp/ship
  go -C /path/to/ship build -o "$bin" ./cmd/ship 2>/dev/null
  "$bin" "$@"
}
```

This rebuilds on every invocation (Go's build cache makes it fast after the first
build) and runs the resulting binary from your current directory — so `ship.toml`
resolution works normally.

```bash
cd my-project
ship status
ship deploy --local
```

## Documentation

- [Getting Started](docs/getting-started.md) — mental model, step-by-step walkthrough
- [Commands](docs/commands.md) — full command reference
- [Secrets & Keys](docs/secrets.md) — encryption, key storage, syncing
- [Deploy](docs/deploy.md) — pipeline, SSL, rollback, snapshots
- [Migration](docs/migrating.md) — migrating from Fly.io
- [Configuration](docs/configuration.md) — ship.toml reference, providers

## Platforms

| Platform | Status |
|---|---|
| macOS | Fully supported — keychain syncs encryption keys via iCloud |
| Linux | Fully supported — use file or env for encryption keys |
| Windows | Works with Docker Desktop, but untested — use file for keys |

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
