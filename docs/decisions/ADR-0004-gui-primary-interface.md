# ADR-0004: GUI as primary interface, CLI as engine

## Status

Accepted

## Context

Airlock was originally designed with a CLI-first architecture: the Go CLI binary is the primary user interface, and the macOS GUI (AirlockApp) is an optional wrapper that spawns the CLI as a subprocess. Documentation, onboarding, and README all lead with CLI commands.

However, an architecture review (2026-03-25) revealed a mismatch between this design and the actual use case:

1. **Coding agents are interactive by nature.** Claude Code, Codex, and similar tools require a developer sitting at a computer, guiding the agent through conversation. This is fundamentally a desktop activity, not a server or CI/CD activity.

2. **The CI/CD justification is weak.** In CI/CD pipelines, coding agents are used for code review or automated fixes -- tasks that rarely require secret API keys with proxy-based decryption. CI/CD platforms already have their own secrets management (GitHub Secrets, AWS Secrets Manager). The scenario of running a coding agent in CI with encrypted secrets through a transparent proxy is not a realistic use case.

3. **The target user sits at a computer.** Whether an individual developer or an enterprise team member, the person using airlock is at their desk (macOS, Linux desktop, or SSH to a dev machine), running an interactive session with a coding agent.

4. **GUI provides a better onboarding experience.** "Install app, click Run" is simpler than "install binary, run `airlock init`, then `airlock run --env .env`" -- especially for non-developer personas (e.g., product managers using coding agents).

## Decision

**Reposition the GUI as the primary user interface and the CLI as the internal engine.**

### What changes

| Aspect | Before (CLI-first) | After (GUI-primary) |
|--------|--------------------|--------------------|
| Product identity | CLI tool with optional GUI | Desktop app with terminal fallback |
| Default onboarding | `airlock init && airlock run` | Install app, create workspace, click Run |
| README first section | CLI commands | App screenshot + download link |
| Documentation order | CLI usage → GUI section | GUI walkthrough → CLI advanced usage |
| Development priority | CLI features first, then GUI | GUI UX first, CLI keeps parity |
| CLI's role | Primary user interface | Engine (invoked by GUI) + alternative for terminal users |

### What does NOT change

- **Go binary stays.** The CLI/engine binary is still necessary. Swift has no age encryption library and no Docker SDK. The Go binary performs all encryption, Docker orchestration, and proxy management.
- **CLI commands stay.** `airlock init`, `airlock run`, `airlock stop`, `airlock encrypt` remain functional. Terminal users and Linux users still use them directly.
- **Architecture stays.** Two-container model, transparent proxy, ENC[age:...] pattern -- none of this changes.
- **GUI subprocess model stays.** The GUI still spawns the CLI as a subprocess via SwiftTerm. This is a proven pattern (Docker Desktop, GitHub Desktop) and avoids reimplementing Docker/crypto logic in Swift.

### How the CLI's role changes

```
Before:  CLI = product        GUI = accessory
After:   GUI = product        CLI = engine + terminal alternative
```

The CLI binary is still distributed, still works standalone, still has the same commands. But the product narrative, documentation, and development priorities shift to GUI-first.

## Consequences

### Easier

- **Onboarding**: New users install an app and click a button instead of learning CLI commands
- **Non-developer adoption**: Product managers, designers, and other non-developer coding agent users get an accessible entry point
- **Feature design**: GUI constraints force simpler, more opinionated workflows (fewer flags, more defaults)
- **Marketing**: "App" is easier to pitch than "CLI tool" for most audiences

### Harder

- **Linux GUI**: SwiftUI is macOS-only. Linux users must continue using CLI. Cross-platform GUI (Electron, Tauri) is a future consideration, not addressed by this ADR.
- **Headless usage**: SSH users and remote dev machines still need CLI. This is the "terminal alternative" role.
- **Two codebases**: GUI (Swift) and engine (Go) remain separate. No consolidation benefit from this decision.
- **CI/CD deprioritized**: If a legitimate CI/CD use case emerges later, it will need to be re-evaluated. This ADR does not permanently exclude CI/CD -- it deprioritizes it based on current evidence.

## Alternatives Considered

### Keep CLI-first (status quo)

Rejected. The CLI-first design assumes enterprise/CI/CD deployment that does not match the interactive coding agent use case. It adds complexity to onboarding and documentation without serving the actual user.

### GUI-only (remove CLI)

Rejected. The Go binary is technically necessary (age encryption, Docker SDK). Additionally, Linux users, SSH users, and terminal-preference developers need a non-GUI option. Removing CLI would exclude these users.

### Rewrite engine in Swift (eliminate Go dependency)

Rejected. Swift has no age encryption library (filippo.io/age is Go-only) and no official Docker SDK. Implementing either from scratch would be months of work with security risk (crypto) and maintenance burden (Docker API surface). The subprocess model is proven and sufficient.

### Cross-platform GUI (Electron/Tauri) instead of SwiftUI

Deferred. This is a valid future direction for Linux GUI support, but is a separate decision. This ADR only changes the priority of GUI vs CLI, not the GUI technology.

## Revision (2026-03-27): GUI-primary implementation complete (Tasks 6-15)

PR #1 merged to main. The GUI-primary redesign is now implemented across 14 Swift files (+1262/-151 lines). Three rounds of code review identified and resolved issues before merge.

### What was implemented

- **Multi-workspace support**: `activeWorkspaceIDs: Set<UUID>` replaces single `activeWorkspaceID`. Multiple workspaces can run simultaneously with independent green status dots.
- **Container-based terminals**: `TerminalView` now spawns `docker exec -it` into running containers instead of `airlock run`. Split-pane support (up to 4 panes) with Cmd+T/D/Shift+D.
- **ContainerSessionService**: New service layer wrapping CLIService for activate/deactivate/status/Docker health check. Shared as a single instance via `@Environment`.
- **Secrets management tab**: Table view of .env entries with encrypted/plaintext/not-secret status. Add Entry, Encrypt All actions. Preserves .env file comments on save.
- **Container status tab**: Agent/proxy/network cards with live proxy activity log streamed from `docker logs --follow` with JSON parsing.
- **Welcome screen**: First-launch experience with Docker status check and create-workspace button.
- **Pre-checks on workspace creation**: Directory exists, .airlock initialized, Docker running, plaintext secrets warning.
- **Crash recovery**: On app launch, reconcile running containers via `airlock status`, orphan cleanup dialog.
- **Menu bar**: FocusedValue-based commands with Cmd+1-5 tab shortcuts, Cmd+R activate, Cmd+. deactivate.

### Design decisions made during implementation

1. **FocusedValue over NotificationCenter for menu commands.** Tab switching (Cmd+1-5), Activate/Deactivate, and terminal split commands all use SwiftUI's `FocusedValue` pattern. NotificationCenter was initially used for terminal actions but was replaced during review for consistency. Only `airlockNewWorkspace` notification remains (sidebar-scoped).

2. **ContainerSessionService shared via @Environment.** A single instance is created in `ContentView` and propagated via both `@Environment(\.containerService)` (for child views) and `@FocusedValue(\.containerService)` (for menu bar commands). This prevents race conditions from concurrent activations.

3. **@MainActor on AppState.** `AppState` is annotated `@Observable @MainActor` to guarantee all UI state mutations happen on the main actor. This was a code review finding -- without it, post-`await` mutations in `Task { }` blocks had no main actor guarantee.

4. **Async Docker health check.** `isDockerRunning()` uses `Process.terminationHandler` with `withCheckedContinuation` instead of synchronous `waitUntilExit()`, preventing UI freezes during workspace creation pre-checks.

### Spec gaps remaining

The following spec features are not yet implemented and are tracked for future work:

- Secrets tab: Export button, per-row View/Edit actions, Key Info section (public key, creation date)
- Pre-checks: Container image existence check (`airlock-claude:latest`, `airlock-proxy:latest`)
- Terminal split: Cmd+D/Shift+D adds a pane and sets direction (implemented), but no 2x2 grid layout for 4 panes

## Revision (2026-03-27): macOS E2E test -- 18/20 PASS

First manual E2E test on macOS 26.2 / Xcode 26.4 / Swift 6.2.3. Unit tests: 26/26 PASS. E2E: 18/20 PASS after fixing 9 bugs in-session.

### Bugs fixed during E2E

| Fix | Files | Root cause |
|-----|-------|------------|
| Docker path hardcoded to `/usr/local/bin/docker` | ContainerSessionService, ContainerStatusView | Rancher Desktop/Colima use `~/.rd/bin/docker` etc. Fixed via `CLIService.findInPath("docker")` |
| `DOCKER_HOST` not set for Go Docker SDK | CLIService | Go SDK uses `client.FromEnv` which reads `DOCKER_HOST`, not Docker contexts. Added auto-detection of common socket paths |
| App doesn't receive keyboard focus when CLI-launched | AirlockApp | `swift run` subprocess doesn't set activation policy. Fixed with `setActivationPolicy(.regular)` + `activate(ignoringOtherApps:)` |
| TextField disabled in NewWorkspaceSheet | NewWorkspaceSheet | Users couldn't type paths directly. Removed `.disabled(true)` |
| Pre-check secrets not showing | NewWorkspaceSheet | Race condition: secrets check ran after async Docker await, concurrent calls reset the `checks` array. Moved secrets check before await |
| Terminal resets on tab switch | ContentView | `switch` statement destroyed/recreated views. Changed to opacity-based persistence for terminal tab |
| Terminal resets on pane add | TerminalSplitView | Single-pane used direct view, multi-pane used `VSplitView` -- different view identity. Unified to always use split view |
| VSplitView/HSplitView swapped | TerminalSplitView | "Vertical split" should create a vertical divider (side-by-side), which is `HSplitView` in SwiftUI |
| Encrypt All silent failure | SecretsView | Missing envfile arg and `-o` flag. Also, `airlock encrypt` outputs single-quoted values that the parser didn't strip |
| Swift 6 concurrency error in tests | AppStateTests | `@MainActor` on `AppState` required test class to also be `@MainActor` |

### Docker runtime compatibility

The GUI now supports non-default Docker installations via auto-detection:

| Runtime | Socket path | Status |
|---------|-------------|--------|
| Docker Desktop | `/var/run/docker.sock` | Supported |
| Rancher Desktop | `~/.rd/docker.sock` | Supported (tested) |
| Colima | `~/.colima/docker.sock` | Supported (untested) |
| Docker Desktop (newer) | `~/.docker/run/docker.sock` | Supported (untested) |

### Known issues (tracked as GitHub issues)

| Issue | Severity | GitHub |
|-------|----------|--------|
| Terminal rendering unstable on initial load | Minor | #2 |
| Korean/CJK input not supported in container | ~~Minor~~ Resolved | #3 -- Fixed in PR #11 (LANG=C.UTF-8) |
| Terminal pane ratio not configurable | ~~Minor~~ Resolved | #4 -- Fixed in PR #12 (NSSplitViewRepresentable) |
| Split direction change destroys terminals | ~~Medium~~ Resolved | #5 -- Fixed in PR #12 (NSSplitViewRepresentable) |
| Rapid tab switching causes UI flicker | ~~Minor~~ Resolved | #6 -- Fixed in PR #13 (tab debounce) |
| Tab bar shifts after deactivate | Minor | #7 |
| mitmproxy CA cert not auto-trusted in agent container | ~~Medium~~ Resolved | Fixed in revision below |
| Activate from Terminal tab causes transient error | ~~Minor~~ Resolved | #10 -- Fixed in PR #13 (ActivationState) |

## Revision (2026-03-27): Proxy E2E verification -- 9/9 automated, 10/10 GUI manual

End-to-end verification of the secret encryption/decryption proxy pipeline. Resolved 6 blocking issues, added automated E2E test suite (`make test-e2e`), and validated the full flow through the GUI.

### Issues resolved

| Fix | Files | Root cause |
|-----|-------|------------|
| Empty MappingPath causes invalid Docker bind mount | `manager.go` | Proxy config always created a mapping bind mount, even when no env file was provided. Now conditional. |
| CA cert not system-trusted in agent container | `entrypoint.sh`, `entrypoint-keepalive.sh` | `update-ca-certificates` failed silently (non-root). Replaced with combined CA bundle (`system + proxy cert`) exported via `SSL_CERT_FILE`, `CURL_CA_BUNDLE`, `REQUESTS_CA_BUNDLE`. |
| Entrypoint inconsistency between attached/detached | `entrypoint.sh`, `entrypoint-keepalive.sh` | Attached mode skipped `update-ca-certificates`; detached mode called it but failed. Both now use identical combined bundle approach. |
| `docker exec` doesn't inherit PID 1 env vars | `entrypoint-keepalive.sh`, `airlock-exec.sh` | Docker exec starts a fresh process. Added `.airlock-env.sh` sourced via `.bashrc` for interactive shells, plus `airlock-exec.sh` wrapper for non-interactive exec. |
| Mapping not hot-reloaded | `decrypt_proxy.py` | Proxy loaded mapping once at startup. Now checks file mtime on each request and reloads if changed. |
| Double encryption when "Encrypt All" then "Activate" | `mapping.go`, CLI callers | `EncryptEntries` re-encrypted already-encrypted values. Now detects `ENC[age:...]` values, skips re-encryption, and builds the mapping by decrypting the existing ciphertext. |
| `.env` quote stripping (Go + Swift) | `envfile.go`, `SecretsView.swift` | `KEY="value"` was encrypted with quotes intact. Both Go `ParseEnvFile` and Swift `loadEnvFile` now strip surrounding single/double quotes. |

### Architectural decisions

1. **Combined CA bundle over system trust store.** Non-root containers cannot write to `/etc/ssl/certs/`. Instead, the entrypoint concatenates the system CA bundle with the proxy cert into `/tmp/airlock-ca-bundle.crt` and exports `SSL_CERT_FILE`/`CURL_CA_BUNDLE`/`REQUESTS_CA_BUNDLE`. Node.js uses `NODE_EXTRA_CA_CERTS` which only needs the extra cert. This works for curl, python, node, and most TLS-aware tools without requiring root.

2. **`.bashrc` + wrapper script for docker exec env.** Docker container-level env vars (set via API `Env` field) are inherited by exec sessions, but entrypoint-exported vars (like `SSL_CERT_FILE`) are not. Solution: entrypoint writes `.airlock-env.sh` and appends a source line to `.bashrc`. Interactive shells (GUI terminal) pick up env via `.bashrc`. Non-interactive exec uses `airlock-exec.sh` wrapper.

3. **Idempotent encryption with private key.** `EncryptEntries` now accepts both public and private keys. When it encounters an already-encrypted value, it decrypts with the private key to build the proxy mapping, rather than re-encrypting. This makes the "Encrypt All" → "Activate" GUI flow and repeated `airlock start --env` invocations safe.

### E2E test infrastructure

Added `test/e2e-proxy.sh` (9 tests) runnable via `make test-e2e`. Requires Docker and built images. Tests:
- Env vars contain ENC tokens (not plaintext) inside container
- CA bundle and SSL_CERT_FILE correctly configured
- Header decryption verified via httpbin.org
- Body decryption verified via httpbin.org POST
- Single/double quoted .env values stripped and decrypted
- Anthropic API passthrough (no decryption)
- Proxy structured JSON logging

## Revision (2026-03-27): Resolve all remaining GUI issues -- 5/5 closed

Parallel resolution of all 5 open GUI issues across 3 PRs (#11, #12, #13), using agent team with git worktrees for isolated development.

### Architectural changes

1. **NSSplitViewRepresentable replaces HSplitView/VSplitView conditional (PR #12).** The conditional `if splitVertical { HSplitView } else { VSplitView }` caused SwiftUI to destroy and recreate all terminal views on direction change, killing docker exec processes. Replaced with a custom `NSViewRepresentable` wrapping `NSSplitView` directly. Toggling `NSSplitView.isVertical` preserves all subviews and running processes. Also enables programmatic pane equalization via `setPosition(_:ofDividerAt:)`.

2. **ActivationState enum replaces binary activeWorkspaceIDs (PR #13).** `activeWorkspaceIDs: Set<UUID>` was a binary active/inactive model. Replaced with `activationStates: [UUID: ActivationState]` supporting three states: `.inactive`, `.activating`, `.active`. Container readiness is verified by polling `docker exec <name> true` before transitioning to `.active`. A `ProgressView` is shown during the `.activating` phase, preventing the terminal from attempting `docker exec` before the container is ready.

3. **Tab switch debounce (PR #13).** `AppState.switchTab(to:)` replaces direct `selectedTab` assignment. Uses `Task.sleep(for: .milliseconds(150))` with cancellation to coalesce rapid tab changes into a single state update.

4. **Container UTF-8 locale (PR #11).** `LANG=C.UTF-8` set at three levels: Dockerfile `ENV`, `BuildClaudeConfig()` container API env, and `.airlock-env.sh` for docker exec sessions. `C.UTF-8` is built into glibc on Debian bookworm -- no `locales` package needed.

### Issues resolved

| PR | Issues | Change |
|----|--------|--------|
| #11 | #3 (Korean input) | `LANG=C.UTF-8` in Dockerfile, manager.go, entrypoint-keepalive.sh |
| #12 | #5 (split destroys sessions), #4 (pane ratio) | `NSSplitViewRepresentable` wrapping `NSSplitView` |
| #13 | #10 (activation timing), #6 (rapid tab switch) | `ActivationState` enum, readiness polling, tab debounce |

### Spec gaps updated

- ~~Secrets tab: Export button, per-row View/Edit actions, Key Info section~~ (still pending)
- ~~Pre-checks: Container image existence check~~ (still pending)
- ~~Terminal split: no 2x2 grid layout for 4 panes~~ (still pending, but pane equalization now works)

## Revision (2026-03-30): Passthrough hosts GUI integration

Connected the GUI Settings passthrough hosts field to the CLI `--passthrough-hosts` flag. Previously, editing passthrough hosts in Settings had no effect.

### Changes

| File | Change |
|------|--------|
| `Models/AppState.swift` | Default `passthroughHosts` changed from `["api.anthropic.com", "auth.anthropic.com"]` to `[]` (full proxy coverage by default) |
| `Models/AppState.swift` | `performActivation` loads settings from `WorkspaceStore` and passes to service |
| `Services/ContainerSessionService.swift` | `activate`/`activateAndWaitReady` accept `settings: AppSettings`, always pass `--passthrough-hosts` flag |
| `Views/Settings/SettingsView.swift` | Hint text changed from "MITM excluded" to "skip proxy decryption" |
| `Views/Containers/ContainerStatusView.swift` | Added `response` action (purple) color mapping and summary counter |

### CLI fix: empty passthrough override

During GUI testing, discovered that `--passthrough-hosts ""` did not override `config.yaml` defaults because the CLI only checked `if passthroughHosts != ""`. Fixed by adding a `passthroughOverride bool` parameter to `RunStart` and using `cmd.Flags().Changed("passthrough-hosts")` in both `start` and `run` commands. The GUI now always passes the flag, so an empty Settings field correctly clears config.yaml passthrough hosts.

### Container OAuth discovery

Manual testing revealed two container authentication issues (tracked as GitHub issues):

| Issue | Finding |
|-------|---------|
| #14 | Settings UI mixes global and per-workspace settings; needs separation |
| #15 | `~/.claude` mounted read-only prevents OAuth token persistence; `claude auth login` succeeds but token is lost on container restart |

OAuth workaround: users can authenticate via `claude auth login` inside the container terminal and manually paste the callback URL. Authentication works per-session but does not persist across restarts.

### Verification results

| # | Test | Result |
|---|------|--------|
| 1 | Settings default empty | PASS |
| 2 | Save/load passthrough hosts | PASS |
| 3 | CLI flag with hosts (passthrough) | PASS |
| 4 | CLI flag empty (none) | PASS |
| 5 | Response action purple + counter | PASS |
| 6 | Hint text updated | PASS |
| 7 | Claude Code execution | BLOCKED (#15) |
| 8 | Claude Code settings parity | BLOCKED (#15) |
