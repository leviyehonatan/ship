# Providers

Ship works with any VPS provider that has a CLI. You install and authenticate the provider's CLI once — ship discovers it automatically and shells out to it.

## Supported providers

| Provider | CLI | Install | Config path |
|---|---|---|---|
| Hetzner | `hcloud` | `brew install hcloud` | `~/.config/hcloud/cli.toml` |
| Linode | `linode-cli` | `brew install linode-cli` | `~/.config/linode-cli` |
| DigitalOcean | `doctl` | `brew install doctl` | `~/.config/doctl/config.yaml` |
| Vultr | `vultr-cli` | `brew install vultr-cli` | `~/.vultr-cli.yaml` |
| AWS | `aws` | `brew install awscli` | `~/.aws/config` |
| GCP | `gcloud` | `brew install google-cloud-sdk` | `~/.config/gcloud/` |

Check which providers are ready:

```bash
ship whoami
```

## How it works

Ship never stores API keys. It reads the provider's own config files and shells out to their CLI:

```
ship servers create my-app --provider hetzner
  → hcloud server create --name my-app --type cx23 --output json
  → parses JSON
  → returns IP, ID, region
```

Adding a new provider means implementing 15 methods that map CLI output JSON to Ship's data structures — no SDKs, no API key management.

## Hetzner

```bash
brew install hcloud
hcloud context create my-project
# paste API token from https://console.hetzner.cloud → Security → API Tokens

ship servers create my-app --provider hetzner --size cx23 --region nbg1
```

## Linode

```bash
brew install linode-cli
linode-cli configure
# paste API token from https://cloud.linode.com/profile/tokens

ship servers create my-app --provider linode --size g6-nanode-1 --region us-east
```

## DigitalOcean

```bash
brew install doctl
doctl auth init
# paste API token from https://cloud.digitalocean.com/account/api/tokens

ship servers create my-app --provider digitalocean --size s-1vcpu-1gb --region nyc1
```

## Vultr

```bash
brew install vultr-cli
export VULTR_API_KEY=<key from https://my.vultr.com/settings/#settingsapi>

ship servers create my-app --provider vultr --size vc2-1c-1gb --region ewr
```

## AWS

```bash
brew install awscli
aws configure
# AWS Access Key ID + Secret Access Key from IAM

ship servers create my-app --provider aws --size t3.micro --region us-east-1 --image ami-0c7217cdde317cfec
```

## GCP

```bash
brew install google-cloud-sdk
gcloud auth login
gcloud config set project my-project-id

ship servers create my-app --provider gcp --size e2-micro --region us-central1-a --image-family ubuntu-2404-lts
```

## Local (Docker)

For testing without any cloud provider:

```bash
ship local start          # starts a local SSH container (your key auto-injected)
ship use local            # target the container
ship deploy               # builds against local Docker
ship local stop           # tear down

# Volumes locally use .ship-data/ (gitignored, Docker Desktop compatible)
# In production, volumes use /opt/ship/data/ on the server
```

No provider CLI needed. SSH key from `~/.ssh/id_rsa.pub` auto-injected.
