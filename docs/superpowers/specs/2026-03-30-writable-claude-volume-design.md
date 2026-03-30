# Writable .claude Volume

## Problem

The `~/.claude` directory is mounted read-only (`:ro`) into the agent container (`BuildClaudeConfig` in `manager.go`). This prevents:

1. **OAuth persistence** -- `claude auth login` succeeds but the token cannot be saved. Users must re-authenticate on every container restart.
2. **Session continuity** -- history, memory, and project context are lost when containers are recreated. Claude Code cannot build on past interactions.
3. **Cross-workspace context** -- multiple workspaces cannot share accumulated knowledge since each container starts from a frozen snapshot of the host's `.claude`.

## Solution

Replace the read-only bind mount of `~/.claude` with a **Docker named volume** that persists across container restarts. The volume is shared by all workspaces, providing a single persistent Claude Code home directory. Host configuration is imported on-demand (not auto-synced), keeping Airlock's Claude Code state independent from the host's.

The existing secret encryption pipeline (shadow mounts) continues to work -- bind mounts overlay specific files on top of the volume.

## Design Decisions

### Airlock .claude is independent from host .claude

The volume is a self-contained Claude Code home. It is NOT automatically synced with the host `~/.claude`. Rationale:

- Airlock containers are treated as a separate "machine" with their own Claude Code installation
- Avoids sync complexity and potential conflicts
- Users who only use Claude Code through Airlock never need host configuration
- A manual import command provides a fast path to seed from host config

### Single shared volume (Phase 1)

All workspaces share one volume named `airlock-claude-home`. This mirrors how Claude Code works on a normal machine where all terminal sessions share one `~/.claude`. Multi-volume support (for account isolation) is deferred to Phase 2.

### Extract-scan-overlay for secret encryption

On every `run`/`start`, settings files are extracted from the volume, scanned for secrets, encrypted, and shadow-mounted back. This ensures secrets added inside the container are caught on next startup.

## Critical Assumptions

### Shadow mount precedence over volume mount

This design depends on Docker resolving file-level bind mounts over named volume mounts at the same path. Per Docker documentation, bind mounts take precedence over volume mounts. However, this behavior is the single most important assumption in this spec.

**Mandatory first implementation task**: Write a proof-of-concept test that:

1. Creates a named volume with a file at a known path
2. Launches a container with both the volume mount and a file-level bind mount targeting the same path
3. Reads the file from inside the container and asserts the bind mount content wins

If this test fails on any supported Docker version, the design must be revised to use an alternative isolation mechanism (e.g., in-container entrypoint encryption). Document the minimum Docker version required.

Additionally, the startup sequence should include a runtime assertion: after starting the claude container, verify that a shadow-mounted file's content matches the expected encrypted version (not the volume's plaintext).

### UID consistency for temp containers

The `airlock` user in the container image has UID 1001 (from `useradd -m` in the Dockerfile, confirmed via `id airlock`). All temporary containers that read from or write to the volume must run as UID 1001 to avoid permission mismatches.

Temp containers must use:
```
docker run --rm --user 1001:1001 alpine ...
```

The airlock-claude image pins the UID implicitly via `useradd` ordering (after node user at UID 1000). To make this stable across image rebuilds, the Dockerfile should be updated to pin the UID explicitly:
```dockerfile
RUN useradd -m -s /bin/bash -u 1001 airlock
```

## Architecture

### Mount layout

```
Before (current):
  ~/.claude:/home/airlock/.claude:ro          full read-only bind
  /tmp/shadow/settings.json:...:ro            encrypted overlay

After:
  airlock-claude-home:/home/airlock/.claude    named volume (RW)
  /tmp/shadow/settings.json:...:ro            encrypted overlay (unchanged)
```

### Container startup sequence

```
airlock run/start
  |
  +-- 1. EnsureVolume("airlock-claude-home")
  |       Create if not exists.
  |       On first creation, the volume is empty. Claude Code handles
  |       missing ~/.claude gracefully (creates its own directory structure).
  |
  +-- 2. ExtractVolumeSettings(volume, tmpDir)
  |       Temp container (alpine, --user 1001:1001, AutoRemove: true)
  |       reads settings.json, settings.local.json from volume.
  |       Uses CopyFromContainer pattern (tar stream extraction)
  |       rather than stdout capture. Missing files are silently skipped.
  |
  +-- 3. ScanAll(scanners, opts)
  |       ClaudeScanner reads extracted files from tmpDir.
  |       Encrypts secrets, creates shadow files + mapping.
  |
  +-- 4. Start proxy (mapping.json bind mount)
  |
  +-- 5. Extract CA cert from proxy
  |
  +-- 6. Start claude container
  |       Volume: airlock-claude-home -> /home/airlock/.claude (RW)
  |       Shadow: encrypted settings overlaid (RO bind mounts)
  |       Bind:   workspace -> /workspace (RW)
  |       Bind:   CA cert (RO)
  |
  +-- 7. Runtime shadow verification (on first run only)
          docker exec <claude-container> cat /home/airlock/.claude/settings.json
          Assert content matches the shadow mount (encrypted), not volume (plaintext).
          Log warning if verification fails.
```

### Import flow

```
airlock config import [--from ~/.claude] [--all] [--items CLAUDE.md,rules] [--force]
  |
  +-- 1. Validate source path exists
  |
  +-- 2. EnsureVolume("airlock-claude-home")
  |
  +-- 3. Run temp container (airlock-claude image, as UID 1001)
  |       Mount: volume at /dst (RW)
  |       Mount: source at /src (RO)
  |       AutoRemove: true
  |       Cmd: for each selected item:
  |            if /dst/<item> exists and --force not set: skip with warning
  |            else: cp -a /src/<item> /dst/<item>
  |
  +-- 4. Print summary of imported/skipped items
```

**Overwrite policy**: by default, existing files in the volume are NOT overwritten. Use `--force` to overwrite. This prevents accidental destruction of OAuth tokens or in-container settings modifications.

**Default import items**: `CLAUDE.md`, `rules/`, `settings.json`, `settings.local.json`.
**Optional items** (via `--all` or `--items`): `plugins/`, `skills/`, `history.jsonl`, `projects/`.

**Post-import note**: after import, the volume may contain plaintext secrets in settings.json. These are encrypted via shadow mounts on the next `run`/`start`. The CLI prints: `"Settings imported. Secrets will be encrypted on next container start."`

### Export flow

```
airlock config export [--to PATH] [--items LIST]
  |
  +-- 1. Default destination: ~/airlock-claude-export/
  |
  +-- 2. Run temp container (as UID 1001, AutoRemove: true)
  |       Mount: volume at /src (RO)
  |       Mount: destination at /dst (RW)
  |       Cmd: cp -a /src/<item> /dst/<item> for each selected item
  |
  +-- 3. Print export path
```

Default exports the same items as import defaults. `--items` for selective export.

## Component Changes

### internal/container/runtime.go

Add to `ContainerRuntime` interface:

```go
EnsureVolume(ctx context.Context, name string) error
RemoveVolume(ctx context.Context, name string) error
ReadFromVolume(ctx context.Context, volumeName, filePath string) ([]byte, error)
```

### internal/container/docker.go

Implement volume operations:

- `EnsureVolume`: calls `VolumeInspect`, creates via `VolumeCreate` if not found
- `RemoveVolume`: calls `VolumeRemove` with force option
- `ReadFromVolume`: runs a temporary container (alpine, `--user 1001:1001`, `AutoRemove: true`) with the volume mounted read-only. Uses the existing `CopyFromContainer` pattern (Docker API tar stream extraction) to retrieve the file. Returns `os.ErrNotExist` if the file does not exist in the volume.

All temporary containers use `AutoRemove: true` (Docker SDK `HostConfig.AutoRemove`) to ensure cleanup even on process interruption.

### internal/container/manager.go

**RunOpts**: replace `ClaudeDir string` with `VolumeName string`. Keep `ClaudeDir` as a deprecated optional field for fallback (see Migration section).

**ContainerConfig**: use Docker SDK's `mount.Mount` API instead of mixing volume names into `Binds`:

```go
type ContainerConfig struct {
    Image      string
    Name       string
    Binds      []string          // bind mounts only
    Mounts     []mount.Mount     // NEW: volume mounts (Docker SDK mount type)
    Env        []string
    Network    string
    CapDrop    []string
    WorkingDir string
    Tty        bool
    Stdin      bool
    Cmd        []string
}
```

**BuildClaudeConfig**: populate `Mounts` with a `mount.Mount{Type: mount.TypeVolume, Source: opts.VolumeName, Target: "/home/airlock/.claude"}`. Remove the `ClaudeDir` bind from `Binds`. Shadow mounts and other bind mounts remain in `Binds`.

**Docker.RunDetached / RunAttached**: convert `cfg.Mounts` to Docker SDK `HostConfig.Mounts` field. This cleanly separates volume mounts from bind mounts with explicit typing.

### internal/secrets/scanner.go

Add `VolumeSettingsDir string` to `ScanOpts`. This is the host tmpdir path where settings extracted from the volume are stored.

### internal/secrets/scanner_claude.go

Change global settings file paths from `HomeDir`-based to `VolumeSettingsDir`-based:

```go
// Before
{filepath.Join(opts.HomeDir, ".claude", "settings.json"), "/home/airlock/.claude/settings.json"}

// After
{filepath.Join(opts.VolumeSettingsDir, "settings.json"), "/home/airlock/.claude/settings.json"}
```

Workspace settings paths remain unchanged (read from host filesystem via `opts.Workspace`).

### internal/orchestrator/session.go

**SessionParams**: replace `ClaudeDir string` with `VolumeName string`.

Add `ExtractVolumeSettings` helper that calls `ReadFromVolume` for each settings file and writes them to tmpdir. Called before `ScanAll` in both `StartSession` and `StartDetachedSession`. Returns the tmpdir path for `VolumeSettingsDir`.

### internal/config/config.go

Add `VolumeName string` to Config with default `"airlock-claude-home"`.

### internal/cli/run.go and start.go

Replace `claudeDir` logic with:

1. Read `VolumeName` from config
2. `EnsureVolume`
3. `ExtractVolumeSettings` to tmpdir
4. Pass `VolumeSettingsDir` to scanner opts
5. Pass `VolumeName` to `SessionParams`

Optional `--claude-dir` flag retained as fallback: if set, uses the old bind mount behavior instead of the volume. This provides a migration escape hatch for environments where Docker volumes are unavailable.

### internal/cli/config_import.go (new)

`airlock config import` command. Flags:

| Flag | Default | Description |
|------|---------|-------------|
| `--from` | `~/.claude` | Source directory path |
| `--all` | false | Import all items including history/projects |
| `--items` | (default set) | Comma-separated items to import |
| `--force` | false | Overwrite existing files in volume |

Uses the airlock-claude image (not alpine) as the temp container to ensure consistent UID/GID context.

### internal/cli/config_export.go (new)

`airlock config export` command. Flags: `--to` (destination path, default `~/airlock-claude-export/`), `--items` (comma-separated). Uses temp container with volume mounted RO.

### internal/cli/volume.go (new)

`airlock volume status` -- shows volume existence and creation date from `VolumeInspect`. Size is omitted in Phase 1 (Docker API does not expose per-volume size without scanning).

`airlock volume reset --confirm` -- calls `RemoveVolume` then `EnsureVolume`. Destructive operation requiring explicit `--confirm` flag. Prints warning about OAuth/history loss before proceeding.

### container/Dockerfile

Pin the airlock UID explicitly for stability across rebuilds:

```dockerfile
RUN useradd -m -s /bin/bash -u 1001 airlock
```

### AirlockApp GUI changes

**GlobalSettingsSheet** (`SettingsView.swift`): add "Claude Code State Volume" section showing volume status (via `airlock volume status` CLI call), "Import from Host" button, and "Reset Volume" button with confirmation alert.

**ImportConfigSheet** (new view): checklist UI for selecting which items to import. "Force overwrite" toggle. Invokes `airlock config import --items <selected> [--force]` via CLIService.

**No changes** to Workspace model, WorkspaceSettingsView, ContainerSessionService, or ResolvedSettings. The volume is transparent to per-workspace configuration.

## Security Analysis

| Concern | Assessment | Risk |
|---------|-----------|------|
| Shadow mount precedence | Bind mounts override volume mounts per Docker docs. Validated by mandatory PoC test + runtime assertion. | Low (after validation) |
| Plaintext secrets in volume | Shadow mounts overlay encrypted versions. Container sees encrypted file, not plaintext. | Low |
| OAuth token in volume | Same exposure as tokens on host `~/.claude`. Volume is Docker-daemon controlled. | Same as host |
| Cross-workspace history | Intentional -- mirrors host behavior. Proxy controls outbound traffic. | Accepted |
| Volume deletion = auth loss | `airlock volume reset` requires `--confirm`. Documented. | Mitigated |
| Concurrent writes | Claude Code handles concurrent multi-session access. Volume mirrors this. The secret scan is point-in-time: a TOCTOU window exists between settings extraction and container start. Acceptable for Phase 1 (small window, low probability). | Low |
| Import copies host secrets | Same secrets, different storage. Volume is not less secure than host. Post-import plaintext exists until next start encrypts via shadow mount. | Low |
| Temp container cleanup | All temp containers use `AutoRemove: true`. No leaked containers on crash. | Low |

## Migration

Existing users upgrading to this version:

1. `config.yaml` has no `volume_name` field -- default `"airlock-claude-home"` is used
2. First `run`/`start` creates the volume automatically (empty)
3. Claude Code creates its own `~/.claude` structure on first run inside the container
4. CLI prints guidance: `"Airlock now uses a persistent volume. Run 'airlock config import' to import your host settings."`
5. `--claude-dir` flag available as fallback for environments where volumes are unavailable

## Volume Metadata

A `.airlock-volume-meta.json` file is written to the volume root on creation:

```json
{
  "version": 1,
  "created": "2026-03-30T12:00:00Z",
  "airlock_version": "0.1.0"
}
```

This enables future migrations if the volume layout changes in Phase 2.

## Testing

### Unit tests

- `EnsureVolume` creates volume when missing, no-ops when exists
- `RemoveVolume` removes existing volume, no-ops for missing
- `ReadFromVolume` returns file content, returns `os.ErrNotExist` for missing files
- `BuildClaudeConfig` produces `mount.Mount` entry instead of bind mount
- `ClaudeScanner` reads from `VolumeSettingsDir` correctly
- `ExtractVolumeSettings` handles missing files gracefully (returns empty dir)
- Config serialization round-trips `VolumeName` field
- Import: `--force` overwrites, default skips existing
- Volume metadata file written on creation

### Integration tests (requires Docker)

- **Shadow mount precedence PoC**: volume with file + bind mount at same path; container sees bind content
- Full startup sequence with volume mount
- Import from host directory populates volume with correct ownership (UID 1001)
- Shadow mounts correctly overlay volume files (encrypted content visible, not plaintext)
- **End-to-end secret lifecycle**:
  1. Start session with empty volume
  2. Write a new secret to settings.json inside the container (via `docker exec`)
  3. Stop session
  4. Start new session
  5. Verify the secret appears in shadow mounts (encrypted) and mapping.json
- `--claude-dir` fallback produces old bind mount behavior
- Volume reset destroys and recreates the volume

## File Inventory

| File | Change |
|------|--------|
| `internal/container/runtime.go` | Add `EnsureVolume`, `RemoveVolume`, `ReadFromVolume` |
| `internal/container/docker.go` | Implement volume operations with `AutoRemove` temp containers |
| `internal/container/manager.go` | `RunOpts.VolumeName`, `ContainerConfig.Mounts`, `BuildClaudeConfig` |
| `internal/container/manager_test.go` | Update tests for volume mount |
| `internal/secrets/scanner.go` | `ScanOpts.VolumeSettingsDir` |
| `internal/secrets/scanner_claude.go` | Read from `VolumeSettingsDir` |
| `internal/secrets/scanner_claude_test.go` | Update test paths |
| `internal/orchestrator/session.go` | `SessionParams.VolumeName`, `ExtractVolumeSettings` |
| `internal/orchestrator/session_test.go` | Update tests |
| `internal/config/config.go` | `VolumeName` field |
| `internal/config/config_test.go` | Serialization test |
| `internal/cli/run.go` | Volume flow, `--claude-dir` fallback |
| `internal/cli/start.go` | Volume flow, `--claude-dir` fallback |
| `internal/cli/config_import.go` | New: import command |
| `internal/cli/config_export.go` | New: export command |
| `internal/cli/volume.go` | New: volume status/reset |
| `container/Dockerfile` | Pin UID: `useradd -u 1001` |
| `AirlockApp/.../SettingsView.swift` | Volume status section |
| `AirlockApp/.../ImportConfigSheet.swift` | New: import UI |

## Phase 2 (Future)

Deferred to a separate issue after Phase 1 feedback:

- **Multi-volume support**: `Config.VolumeName` becomes configurable per-workspace. Volume CRUD commands. GUI volume management.
- **Volume profiles**: named configurations (e.g., "work", "personal") with different OAuth tokens and settings.
- **TOCTOU mitigation**: advisory lock file or atomic settings extraction for concurrent workspace starts.
- **Volume size reporting**: `docker system df -v` integration for `airlock volume status`.
