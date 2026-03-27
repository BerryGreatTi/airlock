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
| Korean/CJK input not supported in container | Minor | #3 |
| Terminal pane ratio not configurable | Minor | #4 |
| Split direction change destroys terminals | Medium | #5 |
| Rapid tab switching causes UI flicker | Minor | #6 |
| Tab bar shifts after deactivate | Minor | #7 |
| mitmproxy CA cert not auto-trusted in agent container | Medium | infra scope |
