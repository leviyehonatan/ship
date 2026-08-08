# Configuration

## ship.toml

```toml
# ship.toml
app = "my-app"
server = "46.224.91.70"             # or use 'ship use <name>'

[build]
dockerfile = "Dockerfile"
args = ["NEXT_PUBLIC_KEY=abc"]     # build-time env vars

[env]
NODE_ENV = "production"             # public, runtime env vars

[deploy]
port = 8080
domains = ["myapp.com"]             # SSL via Caddy
health_check = "/health"

[[volumes]]
path = "/data"
size = "10GB"
```

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
