# Getting Started

## What you need

- A VPS provider account (Hetzner, Linode, DO, Vultr, AWS, GCP)
- The provider's CLI installed and authenticated (`hcloud`, `doctl`, etc.)
- Docker running on your local machine (for building)
- A project with a Dockerfile

## The mental model

Ship builds images locally (where you have Node.js, npm, and source code)
and pushes them to your server over SSH. The server only needs Docker —
no build toolchain, no source upload, no registry.

```
              ┌──────────┐
 Laptop A ───▶│          │
              │  tunity  │  ← app = "tunity" is the deployment identity
 Laptop B ───▶│  :8080   │
              │          │
     CI  ───▶ └──────────┘
```

Multiple machines can deploy the same app — ship.toml is the source of truth. The CLI is stateless; what matters is the `app` name and the `server` it targets.

## Step-by-step

```bash
# 1. See what's available
ship whoami                  # ✓ hetzner ready
ship sizes                   # compare pricing
ship discover                # existing servers

# 2. Create a server (or use an existing one)
ship servers create my-app --size cx23 --region nbg1
ship use my-app              # set as default

# 3. From your project directory
ship init                    # interactive: app name, port, domain, volumes
# Or from an existing project:
ship init --from fly         # reads fly.toml → generates ship.toml

# 4. Generate encryption key (one-time, choose your storage)
ship keys generate keychain  # macOS Keychain (syncs via iCloud)
ship keys generate file       # cross-platform
ship keys generate env        # CI — set SHIP_AGE_KEY manually

# 5. Set secrets (credentials, API keys — encrypted in .env.encrypted)
ship secrets set DATABASE_URL=postgresql://...
ship secrets set RESEND_API_KEY=re_...

# 5b. Optionally, set public env vars in ship.toml [env]
#     (non-sensitive, committed to git — NODE_ENV, LOG_LEVEL, etc.)

# 6. Review resolved env vars
ship env                     # secrets hidden
ship env --reveal            # show all values

# 7. Add sidecar services (Postgres, Redis, etc.) to ship.toml
#     See docs/configuration.md for [services] section

# 8. Deploy (auto-sets up server on first run)
ship deploy                  # first run: installs Docker + Caddy automatically
                             # subsequent runs: build → push over SSH → run

# 9. Day-to-day
ship logs                    # tail output
ship status                  # is it up?
ship releases                # deployment history
ship snapshot                # backup before changes
ship tunnel db               # connect pgAdmin/DBeaver locally
ship ssl on myapp.com        # enable HTTPS
ship doctor                  # health check
```

## Common patterns

**Multiple apps on one server:**
```toml
# app-a/ship.toml
server = "46.224.91.70"
app = "app-a"
[deploy] port = 8080

# app-b/ship.toml
server = "46.224.91.70"
app = "app-b"
[deploy] port = 8081
```
Docker container names use the app name, so no collision.

**Separate DB server:**
```bash
ship servers create db-server
ship use db-server
ship pg create analytics
```

**Switch between servers:**
```bash
ship use production    # all commands now target production
ship use staging       # switch to staging
ship deploy --server staging  # per-command override
```

**Test locally before deploying:**
```bash
ship deploy --local       # build and run on your machine (no SSH)
ship status               # is it up?
ship logs                 # tail output
ship down --local         # stop and clean up
```

**Test the full SSH pipeline locally:**
```bash
ship local start          # starts local SSH container (fake VPS)
ship use local
ship deploy
ship local stop
```

**Check your setup:**
```bash
ship doctor             # local tools + remote server health
```
