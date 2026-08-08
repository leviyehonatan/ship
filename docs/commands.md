# Commands

## Infrastructure

| Command | Description |
|---|---|
| `whoami` | Show configured providers and platforms |
| `discover` | List existing servers |
| `servers create [name]` | Provision a VPS |
| `servers list` | List all servers |
| `servers delete [id]` | Delete a server |
| `use [name]` | Set default server |
| `scale` | Show or change server size |
| `sizes` | Compare server pricing |
| `regions` | List available regions |
| `dashboard` | Open provider web console |
| `local start/stop` | Run local SSH container for testing |

## Deploy

| Command | Description |
|---|---|
| `init` | Generate ship.toml (or --from fly) |
| `setup` | Install Docker + Caddy + log rotation |
| `deploy` | Build → push → run → SSL (auto-snapshot) |
| `status` | Container health |
| `logs` | Tail container output |
| `ssh` | Server overview |
| `console` | Interactive SSH session |
| `sftp [src] [dst]` | Transfer files |
| `releases` | Deployment history |
| `image` | Show deployed image info |
| `services` | Show exposed ports |
| `doctor` | Diagnose local tools + server health |

## Secrets

| Command | Description |
|---|---|
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
| `tunnel db\|couch\|redis` | SSH tunnel to database |
| `migrate fly --to <ip>` | Migrate from Fly.io |

## SSL

| Command | Description |
|---|---|
| `ssl on domain.com` | Enable HTTPS |
| `ssl off domain.com` | Remove domain |
| `ssl status` | Certificate status |
| `ssl renew` | Force renewal |
