# airlock

## Commands

| Command | Purpose |
|---------|---------|
| `make build` | Build CLI binary to `bin/airlock`. Injects version via `-ldflags` |
| `make test` | Go tests with `-race -cover` |
| `make test-python` | Proxy addon tests (installs deps then runs pytest in `proxy/addon/`) |
| `make test-all` | Both Go and Python test suites |
| `make lint` | Requires `golangci-lint` installed |
| `make docker-build` | Builds both container images: `airlock-claude:latest` and `airlock-proxy:latest` |

## Architecture

Two-container setup on a Docker bridge network:
- **airlock-claude**: Runs the AI agent. Secrets exist only as `ENC[age:...]` ciphertext. No plaintext secrets anywhere in this container.
- **airlock-proxy**: mitmproxy sidecar. Intercepts outbound HTTPS, replaces `ENC[age:...]` with decrypted values at the network boundary. Claude API traffic passes through untouched.

The Go CLI (`cmd/airlock/`) orchestrates both containers. Container management is behind a `ContainerRuntime` interface (`internal/container/runtime.go`) for testability.

## Testing

- Go and Python are separate test worlds: `make test` for Go, `make test-python` for the mitmproxy addon
- Go tests use `-race` flag -- all code must be race-safe
- Proxy addon tests require `pytest` and `mitmproxy` Python packages (installed automatically by `make test-python`)
- Config and crypto packages have tests that use `t.TempDir()` for file I/O -- no cleanup needed

## Gotchas

- `.airlock/keys/` contains age private keys -- never commit, never log, never print in error messages
- `*.age` and `*.key` files are gitignored for the same reason
- `mapping.json` (encrypted-to-plaintext mapping) is gitignored -- it exists only at runtime
- The `ENC[age:...]` wrapper pattern is the contract between the agent container and the proxy. Changing the pattern format breaks decryption.
- Version is injected at build time via `LDFLAGS` -- `cli.Version` defaults to `"dev"` if not set

## Documentation

`docs/` is the authoritative source of truth. Update `docs/` before changing implementation. Check `docs/glossary/` for ambiguous concepts. Record architectural decisions as ADRs in `docs/decisions/`. See [docs/README.md](docs/README.md) for full governance rules and directory guide.

## Development Workspace

`.dev/` is for development-time artifacts only. Production code must never import from `.dev/`. See [.dev/README.md](.dev/README.md) for directory structure and full rules.
