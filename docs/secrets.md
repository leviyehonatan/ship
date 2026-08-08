# Secrets & Key Management

## How secrets work

Secrets are encrypted with [`age`](https://age-encryption.org) and stored in `.env.encrypted`. This file is safe to commit to git. At deploy time, ship decrypts in memory and injects secrets into the container.

```
.env.encrypted              ← encrypted (commit-safe)
ship deploy                 ← decrypts in memory → docker run -e
```

Plaintext `.env` is never written to disk during deploy.

## Key storage

The age private key unlocks `.env.encrypted`. You choose where it lives:

```bash
ship keys generate keychain   # macOS Keychain (iCloud sync)
ship keys generate file        # ~/.config/ship/age-key.txt
ship keys generate env         # SHIP_AGE_KEY env var
ship keys status               # check where your key is
```

Resolution order: `SHIP_AGE_KEY` → Keychain → File.

## Syncing across devices

- **macOS Keychain:** auto-syncs via iCloud. Same Apple ID → key appears.
- **File:** copy `~/.config/ship/age-key.txt` manually.
- **1Password / external:** `ship secrets export-key | pbcopy`, paste into 1Password. On new device: `ship secrets import-key`.

## CI / headless

Set `SHIP_AGE_KEY` as a CI secret:

```bash
export SHIP_AGE_KEY="AGE-SECRET-KEY-1..."
ship deploy
```

## Setting secrets

```bash
ship secrets set DATABASE_URL=postgresql://...
ship secrets set COUCHDB_PASSWORD=hunter2
ship secrets set STRIPE_KEY=sk_live_abc123
```

## Viewing secrets

```bash
ship secrets list              # keys only, values hidden
ship secrets show KEY          # single value
ship secrets show              # all values
```

## Removing secrets

```bash
ship secrets unset KEY
```

## Importing from .env

```bash
cat .env
  DATABASE_URL=...
  COUCHDB_PASSWORD=...

ship secrets import .env       # encrypts → .env.encrypted
# Delete .env — it's now in .env.encrypted
```

## Security

- `.env.encrypted` in git → safe (encrypted)
- `.env` in git → dangerous (plaintext, add to .gitignore)
- `~/.config/ship/age-key.txt` → chmod 600, protected by FileVault
- macOS Keychain → encrypted by login password, iCloud sync is end-to-end
- `SHIP_AGE_KEY` → in memory only (don't put in .zshrc)
