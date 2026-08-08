# Migrating from Fly.io

Ship can migrate your Fly.io apps — databases, volumes, env vars, and configuration.

## Quick migration

```bash
ship init --from fly              # reads fly.toml → ship.toml
ship migrate fly --to <ip>       # dumps + restores databases
ship secrets import .env.production  # encrypt production secrets
ship deploy
```

## What it migrates

The migration engine auto-detects what's running on your Fly machine:

| Database | Detection | Dump method | Restore method |
|---|---|---|---|
| Postgres | `pg_isready` | `pg_dump` | `psql -f` |
| CouchDB | `test -d /data/couchdb` | `tar czf` | `tar xzf` |
| Redis | `redis-cli ping` | `--rdb SAVE` | `cp dump.rdb` |

Databases that aren't present are silently skipped.

## Step by step

```bash
# In your Fly project directory:
$ ship migrate fly --to 46.224.91.70
  App: my-app
  Detecting databases...
    Postgres
    CouchDB
    Redis
  Dumping Postgres... 8472 bytes
  Dumping CouchDB... 694000 bytes
  Dumping Redis... 1200 bytes
  Reading env vars... 18 vars
  Generating ship.toml... done
  Encrypting secrets... done
  Installing Docker on target...
  Pushing Postgres dump...
  Pushing CouchDB dump...

Done. Next steps:
  1. Review ship.toml and .env.encrypted
  2. ship deploy
  3. Restore databases: (shown with exact commands)
```

## Adding a new database type

Implement the `DBMigrator` interface:

```go
type DBMigrator interface {
    Name() string
    Detect() bool
    Dump() ([]byte, error)
    RestoreCmd(appName string) string
    ConfigHints() map[string]string
}
```

Register in `var migrators` and it's auto-detected.
