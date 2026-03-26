# Installation Guide

## System Requirements

| Requirement | Version | Notes |
|-------------|---------|-------|
| Docker | 20.10+ | Must be running. Used for container isolation. |
| macOS | 14.0+ (Sonoma) | For the GUI app (recommended) |
| Go | 1.22+ | Only for building CLI from source |
| git | 2.x | For diff viewer functionality |

## Install the GUI App (macOS -- recommended)

### Step 1: Download

Download `AirlockApp-macOS.zip` from [Releases](https://github.com/BerryGreatTi/airlock/releases).

```bash
unzip AirlockApp-macOS.zip
mv AirlockApp.app /Applications/
```

On first launch, macOS may block the unsigned app. Go to System Settings > Privacy & Security > Open Anyway.

### Step 2: Build container images

Airlock uses two Docker images. Build them before first use:

```bash
git clone https://github.com/BerryGreatTi/airlock.git
cd airlock
make docker-build
```

This creates:
- `airlock-claude:latest` -- Container with Claude Code installed
- `airlock-proxy:latest` -- mitmproxy sidecar for transparent decryption

### Step 3: Launch and create a workspace

1. Open AirlockApp
2. The app checks Docker status and image availability
3. Click "Create Your First Workspace"
4. Select your project directory
5. (Optional) Select your `.env` file
6. Click "Create Workspace"
7. Activate the workspace to start containers and open a terminal

The GUI includes the CLI engine internally. No separate CLI installation is needed for GUI users.

## Install the CLI (Linux & advanced users)

For Linux users, SSH/remote environments, and terminal-preference workflows.

### Option A: Pre-built binary

Download from [GitHub Releases](https://github.com/BerryGreatTi/airlock/releases):

```bash
# macOS (Apple Silicon + Intel)
curl -L https://github.com/BerryGreatTi/airlock/releases/latest/download/airlock-darwin-universal.tar.gz | tar xz
sudo mv airlock-darwin-universal /usr/local/bin/airlock

# Linux (amd64)
curl -L https://github.com/BerryGreatTi/airlock/releases/latest/download/airlock-linux-amd64.tar.gz | tar xz
sudo mv airlock-linux-amd64 /usr/local/bin/airlock
```

### Option B: Build from source

```bash
git clone https://github.com/BerryGreatTi/airlock.git
cd airlock
make build
# Binary at ./bin/airlock
```

### Build container images

```bash
make docker-build
```

### CLI first-time setup

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

## Verify installation

```bash
# Check CLI (if installed separately)
airlock version

# Check Docker images exist
docker images | grep airlock

# Check Docker is running
docker info > /dev/null 2>&1 && echo "Docker OK" || echo "Docker not running"
```
