# Commands

## Infrastructure

| Command | Description |
|---|---|
| `whoami` | Show configured providers and platforms |
| `discover` | List existing servers from all providers |
| `servers create [name]` | Provision a VPS |
| `servers list` | List all servers |
| `servers delete [id]` | Delete a server |
| `use` | Pick a server interactively and set as default |
| `use [name]` | Set server by name (or `provider/name`) |
| `scale` | Show or change server size |
| `sizes` | Compare server pricing |
| `regions` | List available regions |
| `dashboard` | Open provider web console |
| `local start/stop` | Manage local SSH test container |

## Deploy

| Command | Description |
|---|---|
| `init` | Generate ship.toml (or --from fly) |
| `setup` | Install Docker + Caddy + log rotation (auto-run on first deploy) |
| `deploy` | Build locally, push over SSH, run on server. `--skip-build` to skip docker build, `--server <name>` to override target |
| `deploy --local` | Build and run on local Docker (no SSH, no SSL) |
| `down` | Stop app + services, remove network |
| `down --local` | Same, for local Docker |
| `down --volumes` | Also remove persistent data |
| `status` | Container health |
| `logs` | Tail container output |
| `ssh` | Server overview |
| `console` | Interactive SSH session |
| `sftp [src] [dst]` | Transfer files |
| `releases` | Deployment history |
| `image` | Show deployed image info |
| `services` | Show exposed ports |
| `doctor` | Diagnose local tools + server health. `--server <name>` to check a specific server without ship.toml |

All commands support `-v` / `--verbose` for detailed output in the terminal.
Logs are also written to `~/.config/ship/logs/` on every invocation.

## Secrets

| Command | Description |
|---|---|
| `env` | Show all resolved env vars (public + secrets). Secrets hidden by default; `--reveal` to show values |
| `keys generate` | Create encryption key (choose storage) |
| `keys status` | Show where key is stored |
| `secrets set KEY=VALUE` | Store encrypted secret |
| `secrets list` | List keys (values hidden) |
| `secrets show [key]` | Show a value |
| `secrets unset KEY` | Remove a secret |
| `secrets import [.env]` | Encrypt existing .env file |
| `secrets export-key` | Print key for 1Password/sync |
| `secrets import-key` | Import key from another device |

## Data

| Command | Description |
|---|---|
| `pg create` | Create Postgres database |
| `pg list` | List databases |
| `pg connect` | Show connection string |
| `snapshot` | Backup databases |
| `snapshots` | List snapshots |
| `rollback [id]` | Restore a snapshot |
| `tunnel <service>` | SSH tunnel to a service from ship.toml (db, couch, redis, or any [services] name) |
| `migrate fly --to <ip>` | Migrate from Fly.io |

## SSL

| Command | Description |
|---|---|
| `ssl on domain.com` | Enable HTTPS |
| `ssl off domain.com` | Remove domain |
| `ssl status` | Certificate status |
| `ssl renew` | Force renewal |
