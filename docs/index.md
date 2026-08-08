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

## Quick example

```bash
ship whoami
ship servers create my-app --size cx23
ship use my-app

cd my-project
ship init
ship keys generate keychain
ship secrets set DATABASE_URL=postgresql://...
ship setup
ship deploy
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
