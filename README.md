# ship

Deploy containers to your own VPS. Fly.io DX, Hetzner prices.

```bash
ship servers create my-app           # provision a VPS
ship secrets set DATABASE_URL=...    # set secrets (encrypted, commitable)
ship deploy                          # build → push → run → SSL
```

## Why

Fly.io charges ~$11/month for a 256MB VM. Hetzner charges ~$6.50 for 4GB. But Hetzner doesn't give you `fly deploy`. Ship bridges the gap — same CLI experience, your own hardware.

| | Fly | ship |
|---|---|---|
| Deploy | `fly deploy` | `ship deploy` |
| Secrets | `fly secrets set` | `ship secrets set` |
| Logs | `fly logs` | `ship logs` |
| Provision | automatic | `ship servers create` |
| SSL | automatic | `ship ssl on domain.com` |
| Cost | $2-11/mo for 256MB | $6.50/mo for 4GB |

## Install

```bash
go install github.com/leviyehonatan/ship@latest
```

Or build from source:

```bash
git clone https://github.com/leviyehonatan/ship.git
cd ship && go build ./cmd/ship/
```

## Quickstart

```bash
# 1. Authenticate with your VPS provider (reads existing CLI configs)
ship whoami                # detects hcloud, linode-cli, etc.

# 2. Create a server
ship servers create my-app --region nbg1 --size cx23

# 3. Initialize your project
cd my-project
ship init                  # interactive, or ship init --from fly

# 4. Set secrets
ship secrets set DATABASE_URL=postgresql://...
ship secrets set COUCHDB_PASSWORD=hunter2

# 5. Deploy
ship deploy
#   App:    my-app
#   Host:   46.224.91.70
#   Port:   8080
#   Status: Up 5 seconds
#   Health: http://46.224.91.70:8080/health

# 6. Enable SSL
ship ssl on myapp.com
```

## Commands

### Infrastructure

| Command | Description |
|---|---|
| `ship whoami` | Show configured providers and platforms |
| `ship discover` | List existing servers, volumes, SSH keys |
| `ship servers create [name]` | Provision a VPS |
| `ship servers list` | List all servers |
| `ship servers delete [id]` | Delete a server |
| `ship sizes` | Compare server pricing |
| `ship regions` | List available regions |

### Deploy

| Command | Description |
|---|---|
| `ship init` | Generate ship.toml |
| `ship setup` | Install Docker + Caddy + log rotation on server |
| `ship deploy` | Build → push → run → SSL (auto-snapshots before deploy) |
| `ship status` | Container health |
| `ship logs` | Tail container output |
| `ship ssh` | Server overview |

### Secrets (Fly-compatible)

| Command | Description |
|---|---|
| `ship secrets set KEY=VALUE` | Store encrypted secret |
| `ship secrets list` | List keys (values hidden) |
| `ship secrets show [key]` | Show a value |
| `ship secrets unset KEY` | Remove a secret |
| `ship secrets import [.env]` | Encrypt existing .env file |

### Data

| Command | Description |
|---|---|
| `ship snapshot` | Backup databases before deploy |
| `ship snapshots` | List available snapshots |
| `ship rollback [id]` | Restore a snapshot |
| `ship tunnel db\|couch\|redis` | SSH tunnel to database |
| `ship migrate fly --to <ip>` | Migrate from Fly.io |

### SSL

| Command | Description |
|---|---|
| `ship ssl on domain.com` | Enable HTTPS via Caddy + Let's Encrypt |
| `ship ssl off domain.com` | Remove domain |
| `ship ssl status` | Certificate status |
| `ship ssl renew` | Force renewal |

## Configuration

```toml
# ship.toml
app = "my-app"
server = "46.224.91.70"

[build]
dockerfile = "Dockerfile"
args = ["NEXT_PUBLIC_KEY=abc"]

[env]
NODE_ENV = "production"    # public env vars

[deploy]
port = 8080
domains = ["myapp.com"]    # SSL via Caddy
health_check = "/health"

[[volumes]]
path = "/data"
size = "10GB"
```

## Providers

| Provider | Status | Detection |
|---|---|---|
| Hetzner | ✓ Full | Reads `~/.config/hcloud/cli.toml` |
| Linode | Planned | Reads `~/.config/linode-cli` |
| DigitalOcean | Planned | Reads `~/.config/doctl/config.yaml` |
| Vultr | Planned | Reads `~/.vultr-cli.yaml` |

Adding a provider: implement the `Provider` interface (~40 lines mapping CLI JSON output).

## Testing

```bash
go test ./... -short               # unit + integration, no Docker, 8s
go test ./internal/e2e/ -v         # full pipeline against SSH container, needs Docker + sshpass
```

## Architecture

Ship shells out to existing tools — no SDK dependencies for providers.

```
ship deploy
  → docker build (local)
  → docker save | Go SSH pipe | docker load (no registry)
  → docker run -e SECRET=... (env injected at runtime)
  → Caddyfile updated (SSL)
  → pg_dump snapshot taken
```

Secrets encrypted with [`age`](https://age-encryption.org). Keypair at `~/.config/ship/age-key.txt`. Encrypted `.env.encrypted` is safe to commit.

## Migrating from Fly.io

```bash
ship init --from fly          # reads fly.toml → ship.toml
ship secrets import .env.production  # encrypt production secrets
ship migrate fly --to <ip>    # dump Fly DBs → restore on server
ship deploy                   # build and deploy
```

## What it doesn't do

- No web dashboard — CLI only
- No multi-server orchestration (v2)
- No DNS management — point your A record at the IP manually
- No managed databases — BYO Postgres (ship snapshots + restores)
