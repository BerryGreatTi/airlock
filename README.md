# Airlock

A security layer for AI coding agents. Run Claude Code (or any AI agent) in an isolated container where secrets are always encrypted -- a transparent proxy decrypts them only at the network boundary.

## Problem

AI coding agents need API keys, tokens, and credentials to interact with external services. But giving an autonomous agent access to plaintext secrets creates risk:

- The agent could accidentally include secrets in LLM context
- Generated code pushed to public repos could contain hardcoded keys
- A compromised agent session could exfiltrate credentials

Airlock solves this by ensuring **secrets never exist in plaintext inside the agent's environment**.

## How It Works

```
┌─────────────────────────────────────────────┐
│ Docker Network (internal, no external access)│
│                                              │
│  ┌────────────────────────┐                 │
│  │  Agent Container        │                 │
│  │  - Workspace mounted    │                 │
│  │  - All secrets encrypted│                 │
│  │  - NO private keys      │                 │
│  │  - HTTP_PROXY=proxy:8080│                 │
│  └──────────┬─────────────┘                 │
│             │ all traffic                    │
│             v                                │
│  ┌────────────────────────┐                 │
│  │  Proxy Container        │                 │
│  │  - Decrypts ENC[age:...] │                │
│  │  - Passes through Claude │                │
│  │    API traffic untouched │                │
│  └──────────┬─────────────┘                 │
└─────────────┼───────────────────────────────┘
              v
       External APIs (Stripe, AWS, GitHub, etc.)
```

1. Secrets in your `.env` are encrypted with [age](https://age-encryption.org/) before entering the container
2. Inside the container, the agent sees only `ENC[age:...]` ciphertext
3. When the agent makes API calls, the proxy replaces encrypted values with real ones
4. Claude API traffic passes through untouched (no MITM on Anthropic)

## Get Started

### GUI App (macOS -- recommended)

Download `AirlockApp-macOS.zip` from [Releases](https://github.com/BerryGreatTi/airlock/releases).

```bash
unzip AirlockApp-macOS.zip
mv AirlockApp.app /Applications/
```

The app provides:
- Workspace management with multiple concurrent sessions
- Split-pane terminals running inside isolated containers
- Secrets management (encrypt, edit, view status)
- Container and proxy status monitoring
- Side-by-side diff viewer

On first launch, the app checks Docker status and guides you through setup.

### CLI (Linux & advanced users)

Prerequisites: Docker (running), Go 1.22+ (build from source) or [pre-built binary](https://github.com/BerryGreatTi/airlock/releases)

```bash
# Install
curl -L https://github.com/BerryGreatTi/airlock/releases/latest/download/airlock-linux-amd64.tar.gz | tar xz
sudo mv airlock-linux-amd64 /usr/local/bin/airlock

# Build container images
make docker-build

# Initialize and run
cd ~/my-project
airlock init
airlock run --env .env
```

## CLI Reference

| Command | Description |
|---------|-------------|
| `airlock init` | Initialize `.airlock/` directory with age keypair and config |
| `airlock encrypt <envfile>` | Encrypt env file values with age. Output: `<envfile>.enc` |
| `airlock run [--env <file>] [--workspace <dir>]` | Launch containerized session with proxy |
| `airlock stop` | Stop running containers and clean up network |
| `airlock version` | Print version |

### Flags

| Flag | Command | Description |
|------|---------|-------------|
| `-e, --env <file>` | run, start | Path to .env file (encrypted at runtime) |
| `-w, --workspace <dir>` | run, start | Workspace directory (default: current dir) |
| `--passthrough-hosts <hosts>` | run, start | Comma-separated hosts to skip proxy decryption (overrides config) |
| `-o, --output <file>` | encrypt | Output path (default: `<input>.enc`) |

## Security Model

### What airlock protects against

- Secrets leaking into LLM context (only ciphertext visible to the agent)
- Secrets in generated code (pushing `ENC[age:...]` to a public repo is safe)
- Container escape blast radius (workspace is the only writable mount)
- Unauthorized network access (all traffic routed through proxy)

### What airlock does NOT protect against

- Secrets used in client-side computation (HMAC signing, AWS Signature V4)
- Non-HTTP protocols (database connections, gRPC without HTTP/2 proxy)
- Kernel-level container escapes (defense-in-depth recommended)
- Compromised host machine (age private key is on the host)

### Security properties

1. No plaintext secrets inside the agent container
2. Age private key never enters the container
3. Proxy mapping (encrypted-to-plaintext) only in proxy container
4. Internal Docker network -- agent has no direct external access
5. `--cap-drop=ALL` on both containers
6. Temp files (mapping.json) deleted on session end
7. Claude API traffic passes through without MITM

## Architecture

See [docs/](docs/) for detailed documentation:
- [Architecture decisions](docs/decisions/)
- [Glossary](docs/glossary/)
- [Specs](docs/specs/)

## Development

```bash
# Go CLI
make build          # Build binary to bin/airlock
make test           # Go tests with -race -cover
make test-python    # Proxy addon tests (pytest)
make test-all       # Both
make lint           # golangci-lint
make docker-build   # Build container images

# macOS GUI
make gui-build      # swift build
make gui-test       # swift test
make gui-run        # swift run
```

## License

MIT
