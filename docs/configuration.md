# Configuration

## ship.toml

```toml
# ship.toml
app = "my-app"
server = "46.224.91.70"             # or use 'ship use <name>'

[build]
dockerfile = "Dockerfile"
args = ["NEXT_PUBLIC_KEY=abc"]     # build-time ARG passed to docker build

# Public, non-sensitive env vars — committed to git
# For secrets use: ship secrets set KEY=value
[env]
NODE_ENV = "production"

[deploy]
port = 8080
domains = ["myapp.com"]             # SSL via Caddy
health_check = "/health"
release_command = "npm run db:migrate"  # runs after container starts

# Sidecar services — run on a per-app bridge network, reachable by name
[services.postgres]
image = "postgres:16-alpine"
port = 5432
volume = "/data/postgres"          # optional, defaults to /data/<name>
env = { POSTGRES_DB = "myapp" }

[services.redis]
image = "redis:7-alpine"
port = 6379

[[volumes]]
path = "/data"
size = "10GB"
```

### Env vars: public vs secrets

- `[env]` — **public** config, committed to git (NODE_ENV, LOG_LEVEL, etc.)
- `ship secrets set` — **secrets**, encrypted in `.env.encrypted` (DATABASE_URL, API keys, passwords)

Use `ship env` to see all resolved env vars. Secrets are hidden by default; use `--reveal` to show values.

### Naming convention

Every Docker resource ship manages is prefixed with `ship-` and segmented by kind, so it can never collide with containers you manage yourself:

| Resource | Name | Example |
|---|---|---|
| App container | `ship-app-<app>` | `ship-app-myapp` |
| Service container | `ship-svc-<app>-<name>` | `ship-svc-myapp-postgres` |
| Standalone Postgres | `ship-pg-<name>` | `ship-pg-analytics` |
| Bridge network | `ship-net-<app>` | `ship-net-myapp` |
| App image | `<app>:latest` | `myapp:latest` |

Every container also carries Docker labels (`com.ship.managed=true`, `com.ship.app=<app>`). On redeploy or `ship down`, ship only ever removes containers with the managed label — if a name is already taken by a container ship didn't create, it refuses and leaves it alone rather than destroying it.

### Services

Sidecar containers defined in `[services.<name>]` are provisioned automatically on deploy. Ship:

- Creates a per-app bridge network (`ship-net-<app>`) and attaches both app and services to it
- Names service containers `ship-svc-<app>-<name>` (e.g. `ship-svc-myapp-postgres`)
- Adds a DNS alias per service, so the app reaches them by name (e.g. `postgres`, `redis`)
- Auto-generates connection strings (see below)
- Persists data at `/opt/ship/data/<app>/data/<name>` (remote) or `.ship-data/<app>/data/<name>` (local)

```toml
[services.postgres]
image = "postgres:16-alpine"
port = 5432
volume = "/data/postgres"   # optional, defaults to /data/<name>
env = { POSTGRES_DB = "myapp" }

[services.redis]
image = "redis:7-alpine"
port = 6379
```

**Auto-generated connection strings** (injected into the app container, unless already set in `[env]` or secrets):

| Service image | Env var |
|---|---|
| `postgres:*` | `DATABASE_URL=postgresql://postgres:<pass>@postgres:5432/<db>` |
| `redis:*` | `REDIS_URL=redis://redis:6379` |

If `POSTGRES_PASSWORD` is not set in the service `env`, ship generates a random one.

Stop everything with `ship down` (add `--local` for local Docker, `--volumes` to also wipe data).

## Server resolution

Commands resolve the target server in this order:

1. `--server` flag (per-command)
2. ship.toml `server` field (per-project)
3. `ship use` default (global)

```bash
ship deploy                        # uses ship.toml server
ship deploy --server staging       # overrides to staging
ship use production                # set global default
```

## State

Server state is cached in `~/.config/ship/servers/<name>.json`:

```json
{
  "name": "my-app",
  "id": "159919881",
  "ip": "46.224.91.70",
  "provider": "hetzner",
  "size": "cx23",
  "region": "nbg1"
}
```

Created automatically by `ship servers create`. Used for name resolution and scaling.

## Providers

Reads existing CLI configurations — no `ship login` needed:

| Provider | Config file |
|---|---|
| Hetzner | `~/.config/hcloud/cli.toml` |
| Linode | `~/.config/linode-cli` (planned) |
| DigitalOcean | `~/.config/doctl/config.yaml` (planned) |
| Vultr | `~/.vultr-cli.yaml` (planned) |

## Provider interface

Adding a new VPS provider is 13 methods:

```go
type Provider interface {
    Name() string
    AuthCommand() string
    SetupInstructions() string
    Validate(ctx) error
    CreateServer(ctx, opts) (*Server, error)
    DeleteServer(ctx, id) error
    ListServers(ctx) ([]Server, error)
    GetServer(ctx, id) (*Server, error)
    ListRegions(ctx) ([]Region, error)
    ListSizes(ctx) ([]Size, error)
    ListImages(ctx) ([]Image, error)
    CreateSSHKey(ctx, name, key) (*SSHKey, error)
    ResizeServer(ctx, id, size) error
    ShutdownServer(ctx, id) error
    PowerOnServer(ctx, id) error
}
```
