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
