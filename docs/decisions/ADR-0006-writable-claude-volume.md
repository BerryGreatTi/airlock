# ADR-0006: Writable .claude volume for persistent container state

## Status

Accepted

## Context

Airlock mounts the host's `~/.claude` directory read-only into the agent container (via `BuildClaudeConfig` in `manager.go`). This was a deliberate security choice: prevent the containerized agent from modifying the user's local Claude Code configuration.

However, read-only mounting creates three operational problems:

1. **OAuth tokens cannot persist.** `claude auth login` completes inside the container, but the token write fails. Users must re-authenticate every time a container is recreated.

2. **Session state is lost.** Claude Code accumulates history (`history.jsonl`), project memory (`projects/`), session data (`sessions/`), and plans -- all of which improve subsequent interactions. A read-only mount means every container starts from a frozen snapshot of the host's `.claude`, losing all in-container state.

3. **Cross-workspace context is severed.** On a normal machine, multiple Claude Code sessions share one `~/.claude`, enabling cross-project context. With per-container read-only snapshots, this capability is eliminated.

## Alternatives Considered

### 1. Selective read-write bind mount

Mount specific subdirectories (e.g., `~/.claude/sessions/`, `~/.claude/.credentials/`) as read-write while keeping the rest read-only.

**Rejected because:**
- Claude Code's internal storage layout is not stable across versions -- paths may change
- Requires tracking which subdirectories need RW access as Claude Code evolves
- Multiple fine-grained mounts add container startup complexity
- Still writes directly to the host filesystem, mixing host and container state

### 2. Automatic sync from host ~/.claude

Mount a writable volume but automatically synchronize it with the host's `~/.claude` on every container start.

**Rejected because:**
- Sync direction is ambiguous (host-to-volume? volume-to-host? merge?)
- Conflicts are likely when both host and container modify the same files
- Adds complexity for users who only use Claude Code through Airlock
- Couples container state to host state, making debugging harder

### 3. Read-write bind mount of ~/.claude

Simply change the mount from `:ro` to `:rw`.

**Rejected because:**
- The containerized agent could modify the user's host-side Claude Code configuration
- Settings changes, plugin installations, or corrupted state inside the container would propagate to the host
- Violates the security principle that container actions should not affect the host

### 4. Custom airlock image with pre-baked auth

Let users build a custom `airlock-claude` image with OAuth tokens baked in.

**Rejected because:**
- Tokens in Docker images are a security anti-pattern (visible in layer history)
- Requires image rebuild when tokens expire
- Does not solve session continuity or cross-workspace context

## Decision

Replace the read-only bind mount with a **Docker named volume** (`airlock-claude-home`) that persists independently from the host's `~/.claude`.

### Key properties

1. **Independent from host.** The volume is a self-contained Claude Code home directory. It is not automatically synced with the host's `~/.claude`. A manual `airlock config import` command provides a one-time import path.

2. **Shared across workspaces.** All workspaces mount the same volume, mirroring how Claude Code works on a normal machine. This preserves cross-workspace context and shared OAuth tokens.

3. **Secret encryption preserved.** The existing shadow mount pipeline (ADR-0005) continues to work: settings files are extracted from the volume, scanned for secrets, encrypted, and overlaid as read-only bind mounts. Bind mounts take precedence over volume mounts in Docker, so the container sees encrypted values.

4. **Fallback available.** A `--claude-dir` flag retains the old bind mount behavior for environments where Docker volumes are unavailable.

### Volume lifecycle

- **Created** by `airlock init` or automatically on first `run`/`start`
- **Populated** via `airlock config import` (selective copy from host) or by Claude Code's own initialization (creates `~/.claude` structure on first run)
- **Persists** across container restarts until explicitly destroyed via `airlock volume reset --confirm`
- **Metadata** tracked via `.airlock-volume-meta.json` at volume root for future migration support

## Consequences

### Positive

- OAuth tokens persist across container restarts -- no re-authentication
- Session history, memory, and project context survive container recreation
- Cross-workspace context works as it does on a normal machine
- Host `~/.claude` is never modified by the container
- Security model (shadow mounts for secret encryption) is preserved
- Clean separation: host Claude Code and Airlock Claude Code are independent installations

### Negative

- Users must run `airlock config import` to seed the volume from host settings (one-time cost)
- Changes to host `~/.claude/CLAUDE.md` or `rules/` do not propagate automatically -- requires re-import
- Volume adds Docker state that must be managed (visible via `docker volume ls`, removed via `airlock volume reset`)
- Startup sequence gains one additional step: extracting settings from volume for secret scanning
- A TOCTOU window exists between settings extraction and container start for concurrent workspace activations (accepted risk for Phase 1; mitigated by the small window)

### Security impact

- **No degradation.** Shadow mounts overlay the volume with encrypted settings. The container cannot access plaintext secrets at shadowed paths.
- **Mandatory validation.** Implementation must begin with a PoC test proving bind mounts take precedence over volume mounts on the target Docker version.
- **UID pinning.** The Dockerfile must pin the `airlock` user to UID 1001 to ensure consistent file ownership between the main container and temporary containers used for volume operations.

## References

- [Issue #15](https://github.com/BerryGreatTi/airlock/issues/15): feat: writable .claude directory for OAuth persistence
- [ADR-0005](ADR-0005-settings-secret-protection.md): Settings secret protection with modular Scanner pipeline
- [Spec](../superpowers/specs/2026-03-30-writable-claude-volume-design.md): Full design specification
