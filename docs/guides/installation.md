# Installation Guide

## System Requirements

| Requirement | Version | Notes |
|-------------|---------|-------|
| Docker | 20.10+ | Must be running. Used for container isolation. |
| Go | 1.22+ | Only for building from source |
| macOS | 14.0+ (Sonoma) | Only for the GUI app |
| git | 2.x | For diff viewer functionality |

## Install the CLI

### Option A: Pre-built binary (recommended)

Download from [GitHub Releases](https://github.com/BerryGreatTi/airlock/releases):

```bash
# macOS (Apple Silicon + Intel)
curl -L https://github.com/BerryGreatTi/airlock/releases/latest/download/airlock-darwin-universal.tar.gz | tar xz
sudo mv airlock-darwin-universal /usr/local/bin/airlock

# macOS (Apple Silicon only)
curl -L https://github.com/BerryGreatTi/airlock/releases/latest/download/airlock-darwin-arm64.tar.gz | tar xz
sudo mv airlock-darwin-arm64 /usr/local/bin/airlock

# Linux (amd64)
curl -L https://github.com/BerryGreatTi/airlock/releases/latest/download/airlock-linux-amd64.tar.gz | tar xz
sudo mv airlock-linux-amd64 /usr/local/bin/airlock
```

Verify:
```bash
airlock version
# airlock 0.1.0
```

### Option B: Build from source

```bash
git clone https://github.com/BerryGreatTi/airlock.git
cd airlock
make build
# Binary at ./bin/airlock
```

## Build container images

Airlock uses two Docker images. Build them before first use:

```bash
make docker-build
```

This creates:
- `airlock-claude:latest` -- Container with Claude Code installed
- `airlock-proxy:latest` -- mitmproxy sidecar for transparent decryption

## Install the GUI (macOS)

Download `AirlockApp-macOS.zip` from [Releases](https://github.com/BerryGreatTi/airlock/releases).

```bash
# Unzip and move to Applications
unzip AirlockApp-macOS.zip
mv AirlockApp.app /Applications/
```

On first launch, macOS may block the unsigned app. Go to System Settings > Privacy & Security > Open Anyway.

The GUI bundles the CLI binary internally. You do not need a separate CLI installation if you only use the GUI.

## Verify installation

```bash
# Check CLI
airlock version

# Check Docker images exist
docker images | grep airlock

# Check Docker is running
docker info > /dev/null 2>&1 && echo "Docker OK" || echo "Docker not running"
```

## First-time setup

```bash
cd ~/your-project
airlock init
```

This creates `.airlock/` containing:
- `keys/age.key` -- Private key (never leaves the host, never committed)
- `keys/age.pub` -- Public key
- `config.yaml` -- Configuration

Add to your `.gitignore`:
```
.airlock/keys/
```
