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
| `make gui-build` | Build macOS SwiftUI app (requires macOS + Xcode) |
| `make gui-test` | Run Swift tests |
| `make gui-run` | Run the GUI app locally |
| `airlock volume status` | Show persistent volume state |
| `airlock volume reset --confirm` | Destroy and recreate the `.claude` volume |
| `airlock config import` | Import host `~/.claude` into the airlock volume |
| `airlock config export` | Export airlock volume to host directory |

## Architecture

Two-container setup on a Docker bridge network:
- **airlock-claude**: Runs the AI agent. A Docker named volume (`airlock-claude-home`) provides persistent `~/.claude` state (OAuth, history, sessions). Secrets in settings files exist only as `ENC[age:...]` ciphertext via shadow mounts.
- **airlock-proxy**: mitmproxy sidecar. Intercepts outbound HTTPS, replaces `ENC[age:...]` with decrypted values at the network boundary. Claude API traffic passes through untouched.

The Go CLI (`cmd/airlock/`) orchestrates both containers. Container management is behind a `ContainerRuntime` interface (`internal/container/runtime.go`) for testability.

**GUI** (`AirlockApp/`): macOS native SwiftUI app -- the primary user interface (see [ADR-0004](docs/decisions/ADR-0004-gui-primary-interface.md)). Uses SwiftTerm (SPM) for terminal emulation. The GUI invokes the Go CLI as a subprocess engine -- it never implements container/crypto logic directly. All Docker interaction from Swift is via subprocess (`docker exec`, `docker logs`), not Docker API. Tabs: Terminal (Cmd+1), Secrets (Cmd+2), Containers (Cmd+3), Settings (Cmd+4). Theme switching (System/Light/Dark) and terminal font/size are in global settings. App quit deactivates all running containers (10s timeout). Dock icon is programmatic (Canvas-drawn airlock hatch, no xcassets).

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
- SwiftTerm's `startProcess` has no `currentDirectory` parameter -- the GUI uses `bash -c "cd <path> && exec airlock run"` as a workaround
- SwiftTerm delegate methods use `SwiftTerm.TerminalView` (not `LocalProcessTerminalView`) as the `source` parameter type for `hostCurrentDirectoryUpdate` and `processTerminated`
- GUI builds require macOS 14+ -- CI runs on `macos-14` GitHub Actions runner
- Docker named volume `airlock-claude-home` is created as root-owned. Import/export containers run as `root` and `chown 1001:1001` after copy. The `ContainerConfig.User` field controls this.
- Claude Code stores auth in two files: `~/.claude/.credentials.json` (token) and `~/.claude.json` (session metadata). The entrypoint symlinks `~/.claude.json` into the volume so both persist.
- Workspaces mount at `/workspace/<basename>` (not `/workspace`). The GUI passes `docker exec -w /workspace/<basename>` to open the terminal at the correct path.

## Documentation

`docs/` is the authoritative source of truth. Update `docs/` before changing implementation. Check `docs/glossary/` for ambiguous concepts. Record architectural decisions as ADRs in `docs/decisions/`. See [docs/README.md](docs/README.md) for full governance rules and directory guide.

## Development Workspace

`.dev/` is for development-time artifacts only. Production code must never import from `.dev/`. See [.dev/README.md](.dev/README.md) for directory structure and full rules.
