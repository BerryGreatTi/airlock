# GUI-Primary Redesign Specification

> Status: Approved (2026-03-25)
> ADR: [ADR-0004](../../decisions/ADR-0004-gui-primary-interface.md)

## Overview

Airlock의 인터페이스를 CLI-first에서 GUI-primary로 전환하는 재설계. 터미널이 메인 작업 공간이 되는 코딩 에이전트 트렌드에 맞추어, GUI가 컨테이너화된 작업 환경을 제공하고 사용자가 그 안에서 자유롭게 작업하는 모델로 변경한다.

### Core Model Change

```
Before: GUI가 "airlock run"을 실행하고 종료를 기다림
After:  GUI가 컨테이너 환경을 띄우고, 사용자가 그 안에서 자유롭게 작업
```

### Out of Scope

- Linux GUI (별도 ADR 필요)
- 세션 기록/히스토리
- 구문 강조 (syntax highlighting)

---

## 1. Layout & Navigation

### Structure

```
┌─ Sidebar (220px) ──┬─ Main Area ────────────────────────────────────┐
│                    │                                                │
│ WORKSPACES         │  [Terminal] [Secrets] [Containers] [Settings]
│                    │  ──────────────────────────────────────────────│
│ ● my-api       ▶  │                                                │
│ ● frontend        │           Selected tab content                 │
│   data-pipeline   │                                                │
│                    │                                                │
│ ──────────────     │                                                │
│ [+ New Workspace]  │                                                │
│ Gear icon          │                                                │
└────────────────────┴────────────────────────────────────────────────┘
```

### Sidebar

- Workspace list with color status indicators:
  - Green dot = running (containers active)
  - Red dot = error
  - Gray dot = stopped
- **Multiple workspaces can be green simultaneously** (currently limited to one)
- Selection ≠ activation. Deselecting a running workspace does not stop it.
- Context menu: Activate, Deactivate, Remove

### Tab Bar

| Tab | Shortcut | Role | Default? |
|-----|----------|------|----------|
| Terminal | Cmd+1 | Splittable shells inside container | Yes |
| Secrets | Cmd+2 | .env editing, encryption status | |
| Containers | Cmd+3 | Agent/proxy/network status, proxy activity log | |
| Settings | Cmd+4 | Per-workspace + global config | |

> **Removed (2026-03-31):** The Diff tab (git diff side-by-side) was removed. Git diffs are better viewed in the terminal directly.

---

## 2. Workspace Lifecycle & Terminals

### State Transitions

```
[Created]               [Activated]             [Deactivated]
    │                       │                       │
    ▼                       ▼                       ▼
┌──────────┐           ┌──────────┐           ┌──────────┐
│Configured│  ──────>  │ Running  │  ──────>  │ Stopped  │
│          │           │          │           │          │
│ .airlock/│           │ Containers ON        │ Containers OFF
│ exists   │           │ Proxy ON │           │ Data preserved
│ No       │           │ Terminals│           │          │
│ containers           │ active   │           │          │
└──────────┘           └──────────┘           └──────────┘
                         ▲                       │
                         └──── Reactivate ───────┘
```

### Activation Sequence

```
User clicks "Activate" in sidebar
  │
  ├─ 1. Create network: airlock-net-{workspace-id}
  ├─ 2. Start proxy: airlock-proxy-{workspace-id} (detached)
  │     - Mount mapping.json read-only
  │     - Set passthrough hosts
  ├─ 3. Wait for proxy CA cert & extract
  ├─ 4. Start agent: airlock-claude-{workspace-id} (detached, stays alive)
  │     - Mount project → /workspace
  │     - Set HTTP_PROXY env vars
  │     - Keep-alive entrypoint (not Claude directly)
  ├─ 5. Auto-open first terminal
  │     - docker exec -it airlock-claude-{workspace-id} /bin/bash
  │     - Displayed in terminal tab
  │
  ▼
User works freely inside container shell:
  $ claude            ← Start Claude Code
  $ git status        ← Git operations
  $ make test         ← Build/test
  $ curl api.stripe.com ← Proxy auto-decrypts secrets
```

### Deactivation Sequence

```
User clicks "Deactivate" or app quits
  │
  ├─ 1. Close all terminal sessions
  ├─ 2. Remove airlock-claude-{workspace-id}
  ├─ 3. Remove airlock-proxy-{workspace-id}
  ├─ 4. Remove airlock-net-{workspace-id}
  └─ 5. Clean temp files (mapping.json, etc.)
  │
  ▼
Workspace → Stopped (config preserved, reactivation possible)
```

### App Quit Behavior (2026-03-31)

When the user closes the window or quits the app:

1. `applicationShouldTerminateAfterLastWindowClosed` returns `true` -- closing the window triggers quit
2. `applicationShouldTerminate` queries `airlock status` for all running containers
3. All running containers are stopped in parallel via `airlock stop --id`
4. A 10-second timeout ensures the app always quits, even if Docker is unresponsive
5. Uses `Task.detached` to avoid blocking the main thread during cleanup

### Activation Failure

If Docker is not running when the user tries to activate:
- Show inline error: "Docker is not running. Start Docker Desktop and try again."
- Workspace remains in Configured/Stopped state (no partial containers created)
- Sidebar shows red dot with error tooltip

### Crash Recovery

If the app crashes or is force-quit while workspaces are active:
- Containers keep running (Docker does not stop them)
- On next app launch: call `airlock status` to discover orphaned containers
- Reconcile with saved workspace list:
  - If a workspace's containers are still running → mark as active (green dot), resume
  - If containers are orphaned (no matching workspace) → offer cleanup dialog
- Security: `mapping.json` in temp dir persists across crashes. `airlock stop` cleans it up on next deactivation. If the workspace is removed, `airlock stop --id {id}` is called to ensure cleanup.

### Workspace Removal

- Remove is **disabled** while the workspace is active (green dot)
- User must deactivate first, or the Remove dialog offers "Stop and Remove" as a combined action
- Deactivation runs the full cleanup sequence before removing the workspace record

### Terminal Tab

```
┌─ Terminal toolbar ──────────────────────────────────────────────────┐
│  [+ Terminal]  [Split V]  [Split H]              Term 1 │ Term 2   │
├────────────────────────────────┬────────────────────────────────────┤
│                                │                                    │
│  airlock-claude-a1b2 $         │  airlock-claude-a1b2 $             │
│  claude                        │  git diff                          │
│  Claude Code session...        │  + src/api.go                      │
│                                │  - src/old.go                      │
│                                │                                    │
│  (docker exec session #1)      │  (docker exec session #2)          │
│  (same container, same /workspace)                                  │
└────────────────────────────────┴────────────────────────────────────┘
```

- All terminals are separate shell sessions in the **same container**
- Shared /workspace, filesystem, environment variables
- Add terminal = one more `docker exec -it {container} /bin/bash`
- Close terminal = end that exec session only (container stays)
- Split: vertical (Cmd+D), horizontal (Cmd+Shift+D), max 4 panes
  - These follow iTerm2 conventions. Listed under a "Terminal" menu for discoverability.
- Focus: click or Cmd+[ / Cmd+] to navigate
- Active terminal indicated by subtle border highlight

### Terminal Implementation

Each terminal pane is a **subprocess** managed by SwiftTerm's `LocalProcessTerminalView`, exactly like the current architecture. The GUI does NOT communicate with the Docker API directly — it spawns `docker exec` as a local process:

```
LocalProcessTerminalView.startProcess(
  executable: "/usr/local/bin/docker",
  args: ["exec", "-it", "airlock-claude-{workspace-id}", "/bin/bash"],
  environment: CLIService.enrichedEnvironment()
)
```

This is consistent with ADR-0004 ("no Docker logic in Swift") and follows the same pattern as the current `airlock run` subprocess.

### Keep-Alive Entrypoint

The agent container uses a modified entrypoint to stay alive without running Claude directly:

```bash
#!/bin/bash
set -e

# Load encrypted env file (if mounted)
if [ -f /run/airlock/env.enc ]; then
    set -a
    source /run/airlock/env.enc
    set +a
fi

# Trust proxy CA certificate
if [ -f /usr/local/share/ca-certificates/airlock-proxy.crt ]; then
    update-ca-certificates 2>/dev/null || true
    export NODE_EXTRA_CA_CERTS=/usr/local/share/ca-certificates/airlock-proxy.crt
fi

# Keep container alive — wait for docker exec sessions
exec tail -f /dev/null
```

When `airlock start` is used, the container's `Cmd` is overridden to `["/usr/local/bin/entrypoint-keepalive.sh"]`. The existing `entrypoint.sh` (which runs Claude directly) remains unchanged for `airlock run` backward compatibility.

---

## 3. Secrets Management Tab

### Layout

```
┌─ Secrets Tab ───────────────────────────────────────────────────────┐
│                                                                     │
│  Env File: /Users/me/my-api/.env          [Change File] [Refresh]  │
│                                                                     │
│  ┌──────────────────┬──────────────┬────────────┬─────────────────┐ │
│  │ Name             │ Status       │ Value      │ Actions         │ │
│  ├──────────────────┼──────────────┼────────────┼─────────────────┤ │
│  │ STRIPE_KEY       │ ● Encrypted  │ sk_li••••  │ [View] [Edit]   │ │
│  │ DB_PASSWORD      │ ● Encrypted  │ mypa••••   │ [View] [Edit]   │ │
│  │ AWS_ACCESS_KEY   │ ○ Plaintext  │ AKIA••••   │ [View] [Edit]   │ │
│  │ DEBUG_MODE       │ ─ Not secret │ true       │ [View] [Edit]   │ │
│  └──────────────────┴──────────────┴────────────┴─────────────────┘ │
│                                                                     │
│  [+ Add Entry]                           [Encrypt All] [Export]     │
│                                                                     │
│  ── Key Info ─────────────────────────────────────────────────────  │
│  Public key: age1w8cgh...fqugj       [Copy]                        │
│  Created: 2026-03-24                                                │
│  Encrypted: 2/4 entries                                             │
└─────────────────────────────────────────────────────────────────────┘
```

### Behavior

**Status detection:**
- `● Encrypted` — value matches `ENC[age:...]` pattern
- `○ Plaintext` — name contains KEY, SECRET, PASSWORD, TOKEN (heuristic)
- `─ Not secret` — plain config values (true, localhost, numbers, etc.)

**Actions:**
- [View] — unmask value (plaintext items only; encrypted items show ciphertext)
- [Edit] — modify value, save to .env file
- [+ Add Entry] — new KEY=VALUE pair
- [Encrypt All] — encrypt all plaintext secrets via `airlock encrypt`
- [Export] — save current state to `.env` file (with `ENC[age:...]` values preserved, never plaintext export). Opens save panel defaulting to `{workspace-name}.env`.

**Active workspace interaction:**
- Editing secrets while workspace is running saves to file but does NOT affect running containers
- Banner: "Restart workspace to apply changes"

---

## 4. Container Management Tab

### Layout

```
┌─ Container Tab ─────────────────────────────────────────────────────┐
│                                                                     │
│  Workspace: my-api                          [Restart] [Stop]        │
│                                                                     │
│  ┌─ Agent Container ───────────────────────────────────────────┐   │
│  │  Name:    airlock-claude-a1b2c3                              │   │
│  │  Image:   airlock-claude:latest                              │   │
│  │  Status:  ● Running (2h 14m)                                 │   │
│  │  Mount:   /Users/me/my-api → /workspace                     │   │
│  │  Sessions: 2 active terminals                                │   │
│  └──────────────────────────────────────────────────────────────┘   │
│                                                                     │
│  ┌─ Proxy Container ──────────────────────────────────────────┐    │
│  │  Name:    airlock-proxy-a1b2c3                              │    │
│  │  Image:   airlock-proxy:latest                              │    │
│  │  Status:  ● Running (2h 14m)                                │    │
│  │  Port:    8080 (internal)                                   │    │
│  │  Passthrough: api.anthropic.com, auth.anthropic.com         │    │
│  └─────────────────────────────────────────────────────────────┘    │
│                                                                     │
│  ┌─ Network ──────────────────────────────────────────────────┐    │
│  │  Name:    airlock-net-a1b2c3                                │    │
│  │  Driver:  bridge (Internal)                                 │    │
│  │  Status:  ● Active                                          │    │
│  └─────────────────────────────────────────────────────────────┘    │
│                                                                     │
│  ── Security Summary ──────────────────────────────────────────── │
│  Capabilities: CapDrop=ALL                                         │
│  Secrets:      2 encrypted, mapping.json delivered to proxy only   │
│  Private key:  No container access                                  │
│                                                                     │
│  ── Proxy Activity Log ──────────────────── [Auto-scroll] [Clear]  │
│  ┌─────────────────────────────────────────────────────────────┐   │
│  │ 14:32:01  api.stripe.com    → 1 header decrypted  Authz    │   │
│  │ 14:32:01  api.stripe.com    → 1 body decrypted             │   │
│  │ 14:31:45  api.github.com    → 1 header decrypted  Authz    │   │
│  │ 14:31:12  api.anthropic.com → passthrough                   │   │
│  │ 14:30:58  api.stripe.com    → 1 query decrypted   api_key  │   │
│  │ 14:30:02  cdn.example.com   → no encrypted tokens           │   │
│  └─────────────────────────────────────────────────────────────┘   │
│  Total: 23  │  Decrypted: 14  │  Passthrough: 6  │  None: 3       │
└─────────────────────────────────────────────────────────────────────┘
```

### Container Status

- Inactive workspace: "No containers running. Activate workspace to start."
- Active workspace: real-time status via `docker inspect` (5-second polling)
- [Restart] — stop + restart containers (warns: all terminal sessions will disconnect)
- [Stop] — equivalent to workspace deactivation

### Proxy Activity Log

**Displayed columns:**

| Column | Content | Example |
|--------|---------|---------|
| Time | Request timestamp | `14:32:01` |
| Host | Request target | `api.stripe.com` |
| Result | decrypt/passthrough/none | `→ 1 header decrypted` |
| Location | Where decrypted | `Authorization` (header name), `api_key` (query key) |

**Security rules:**
- Secret values (plaintext) are NEVER displayed
- Shown: hostname, header/query key names, decryption count
- Not shown: secret values, request body content, full ENC tokens

**Implementation:**
- `decrypt_proxy.py` emits structured JSON log lines on decryption events
- Format: `{"time":"...","host":"...","action":"decrypt","location":"header","key":"Authorization"}`
- GUI streams via `docker logs --follow airlock-proxy-{id}`
- Parses JSON and renders in table
- Bottom counters summarize totals

---

## 5. Workspace Creation Flow

### Creation Sheet

```
┌─ New Workspace ──────────────────────────────────────────┐
│                                                          │
│  Project Directory                                       │
│  ┌─────────────────────────────────────┐  [Browse]      │
│  │ /Users/me/Projects/my-api           │                │
│  └─────────────────────────────────────┘                │
│                                                          │
│  Environment File (.env)                   (optional)    │
│  ┌─────────────────────────────────────┐  [Browse]      │
│  │ /Users/me/Projects/my-api/.env      │                │
│  └─────────────────────────────────────┘                │
│                                                          │
│  ── Pre-checks ─────────────────────────────────────     │
│  ✓ Directory exists                                      │
│  ✓ .airlock/ initialized (or auto-run)                  │
│  ✓ Docker daemon running                                 │
│  ✓ airlock-claude:latest image exists                   │
│  ✓ airlock-proxy:latest image exists                    │
│  ✗ .env has 2 plaintext secrets → encrypt after creation │
│                                                          │
│                          [Cancel]  [Create Workspace]    │
└──────────────────────────────────────────────────────────┘
```

### Pre-check Behavior

| Check | On Failure |
|-------|-----------|
| Directory exists | Create button disabled |
| .airlock/ initialized | Auto-run `airlock init` |
| Docker daemon | Warning, creation allowed (activation will fail with inline error if Docker is still not running) |
| Container images | "Build images first: `make docker-build`" guidance, creation allowed |
| .env plaintext secrets | Warning only, encrypt via Secrets tab after creation |

### First Launch Experience

When app opens with no workspaces:

```
┌─────────────────────────────────────────────────────────┐
│                                                         │
│              Welcome to Airlock                         │
│                                                         │
│    Run AI coding agents safely in isolated containers   │
│                                                         │
│         [Create Your First Workspace]                   │
│                                                         │
│  Requirements:                                          │
│  ✓ Docker Desktop running                               │
│  ✗ Airlock images not installed → [How to install]      │
│                                                         │
└─────────────────────────────────────────────────────────┘
```

---

## 6. App Identity & Appearance (2026-03-31)

### Dock Icon

The app uses a programmatic icon generated at runtime via SwiftUI `Canvas` and `ImageRenderer` (512px @ 2x scale). The design is a space airlock hatch viewed from the front: outer rim with 12 protruding tabs, 4 radial spokes with latch clamps, a central hub with handle, and detail elements (equalization valve, observation window). Black background, white glow style. The same icon is shown at 128px in the WelcomeView onboarding screen.

No xcassets catalog is used -- the icon is set at launch via `NSApp.applicationIconImage`.

### Theme Switching

Global settings provide a theme picker with three options:

| Theme | Behavior |
|-------|----------|
| System | Follows macOS appearance (default) |
| Light | Forces light mode |
| Dark | Forces dark mode |

Applied via `.preferredColorScheme()` on the root ContentView. Terminal colors auto-adapt to the effective theme.

### Terminal Rendering Settings

Global settings expose terminal font and size:

- **Font picker**: SF Mono (default), Menlo, Monaco, Courier New, Andale Mono, JetBrains Mono, Fira Code, Source Code Pro
- **Font size slider**: 9-24pt (default 13pt)
- **Live preview** in settings sheet
- Changes apply immediately to all existing terminal panes via `updateNSView` diffing

**Auto-optimized terminal colors per theme:**

| Property | Dark Mode | Light Mode |
|----------|-----------|------------|
| Background | #1e1e2e | #f0f1f5 |
| Foreground | #cdd6f4 | #4c4f69 |
| Caret | #89b4fa | #1e66f5 |
| Selection | rgba(89,180,250,0.25) | rgba(30,102,245,0.18) |
| Bright colors | Enabled | Disabled |

Colors are derived from `appState.settings.theme` directly (not from environment colorScheme) to avoid 1-frame lag on theme switch. Applied via SwiftTerm's `nativeBackgroundColor`, `nativeForegroundColor`, `caretColor`, `selectedTextBackgroundColor`, and `useBrightColors` properties.

---

## 7. CLI Engine Changes (unchanged)

### Command Structure

```
Unchanged:
  airlock init          Initialize .airlock/ directory
  airlock encrypt       Encrypt .env file
  airlock run           Start session with attached terminal (terminal-user compat)

New:
  airlock start         Start containers detached, return immediately
  airlock stop          Stop containers for a specific workspace
  airlock status        Query running container state

Changed:
  airlock stop          Now accepts --id flag for workspace-specific cleanup
```

### New Commands

**`airlock start --id {workspace-id} [--env .env]`**

```
1. Create network:   airlock-net-{id}
2. Start proxy:      airlock-proxy-{id} (RunDetached)
3. Wait for CA cert & extract
4. Start agent:      airlock-claude-{id} (RunDetached, keep-alive entrypoint)
5. Output JSON:      {"status":"running","container":"airlock-claude-{id}"}
6. Return immediately (non-blocking)
```

**`airlock stop --id {workspace-id}`**

```
1. Remove airlock-claude-{id}
2. Remove airlock-proxy-{id}
3. Remove airlock-net-{id}
4. Clean temp files
5. Without --id: legacy behavior (fixed names)
```

**`airlock status [--id {workspace-id}]`**

Full JSON schema:

```json
{
  "workspaces": [
    {
      "id": "a1b2c3",
      "container": "airlock-claude-a1b2c3",
      "proxy": "airlock-proxy-a1b2c3",
      "status": "running",
      "uptime": "2h14m"
    },
    {
      "id": "d4e5f6",
      "container": "airlock-claude-d4e5f6",
      "proxy": "airlock-proxy-d4e5f6",
      "status": "exited",
      "uptime": "",
      "error": "proxy container crashed: exit code 1"
    }
  ]
}
```

Status values: `"running"`, `"exited"`, `"not_found"`. When `status` is `"exited"`, the `"error"` field contains the reason.

**GUI reaction to unexpected container exit:**
- `airlock status` polling (5-second interval) detects `"exited"` status
- Sidebar: workspace shows red dot
- Container tab: error banner "Container exited unexpectedly"
- User can click [Restart] to reactivate

**Container name derivation:** The GUI constructs container names from the workspace UUID directly (`airlock-claude-{uuid-short}`). The `airlock start` JSON output is logged but not parsed for names.

### Container Execution Model Change

| Aspect | Current (airlock run) | New (airlock start) |
|--------|----------------------|---------------------|
| Agent container | RunAttached (blocks CLI) | RunDetached (background) |
| Entrypoint | `claude --dangerouslySkipPermissions` | Keep-alive command |
| Terminal connection | CLI pipes stdin/stdout | GUI manages `docker exec` |
| Exit condition | Claude exits | `airlock stop` called |
| Concurrent sessions | 1 (name collision) | N (ID-based names) |

### GUI → CLI Call Pattern

```
Workspace creation:   GUI → airlock init
Workspace activation: GUI → airlock start --id {id} --env .env
Terminal session:     GUI → docker exec -it airlock-claude-{id} /bin/bash
Workspace deactivation: GUI → airlock stop --id {id}
Status polling:       GUI → airlock status (or docker inspect directly)
Secret encryption:    GUI → airlock encrypt
```

### Backward Compatibility

- `airlock run` unchanged — terminal users can use existing workflow
- `airlock stop` without `--id` — enumerates all running `airlock-*` containers and stops them (replaces legacy fixed-name behavior, which would silently do nothing since fixed-name containers no longer exist)
- New commands (`start`, `status`) are additions only, no breaking changes

---

## 8. Proxy Activity Log

### Purpose

Verify proxy decryption is functioning correctly. Answer the question: "Is the proxy actually decrypting my secrets when Claude makes API calls?"

### Implementation

**Proxy side (`decrypt_proxy.py`):**

Add structured JSON logging on each request:

```python
# On decryption:
{"time":"14:32:01","host":"api.stripe.com","action":"decrypt","location":"header","key":"Authorization"}
{"time":"14:32:01","host":"api.stripe.com","action":"decrypt","location":"body","key":null}

# On passthrough:
{"time":"14:31:12","host":"api.anthropic.com","action":"passthrough"}

# On no match:
{"time":"14:30:02","host":"cdn.example.com","action":"none"}
```

**GUI side:**

- Run `docker logs --follow airlock-proxy-{id}` as a subprocess (same pattern as terminal — no Docker API in Swift)
- Parse JSON lines; skip non-JSON lines (mitmproxy startup messages, etc.)
- Summary counters: Total / Decrypted / Passthrough / None
- [Auto-scroll] toggle, [Clear] button
- Log subprocess lifecycle: started when workspace is activated, killed on deactivation or app termination
- On proxy container restart: restart the log stream subprocess

**Security invariant:** Secret values are NEVER logged or displayed. Only metadata (host, header name, count) is shown.

---

## Data Model Changes

### Workspace (Swift)

```
Workspace {
  id: UUID                    // (existing)
  name: String                // (existing)
  path: String                // (existing)
  envFilePath: String?        // (existing)
  containerImageOverride: String?  // (existing, keep original field name)
  isActive: Bool              // (new) container running state
  containerId: String?        // (new) airlock-claude-{id} when active
  proxyId: String?            // (new) airlock-proxy-{id} when active
  networkId: String?          // (new) airlock-net-{id} when active
  terminalSessions: [TerminalSession]  // (new)
}

TerminalSession {
  id: UUID
  process: Process            // (in-memory only, not persisted) subprocess handle for docker exec
  isActive: Bool
}
```

### AppState Changes

```
AppState {
  workspaces: [Workspace]            // (existing)
  selectedWorkspaceID: UUID?         // (existing)
  activeWorkspaceIDs: Set<UUID>      // (changed) was single activeWorkspaceID
  selectedTab: DetailTab             // (existing)
  lastError: String?                 // (existing)
}
```

Key change: `activeWorkspaceID: UUID?` → `activeWorkspaceIDs: Set<UUID>` to support multiple concurrent active workspaces.

---

## File Changes Summary

### New Files

| File | Purpose |
|------|---------|
| `internal/cli/start.go` | `airlock start` command |
| `internal/cli/status.go` | `airlock status` command |
| `AirlockApp/.../Views/Secrets/SecretsView.swift` | Secrets management tab |
| `AirlockApp/.../Views/Containers/ContainerStatusView.swift` | Container status + proxy log |
| `AirlockApp/.../Views/Terminal/TerminalSplitView.swift` | Multi-terminal split layout |
| `AirlockApp/.../Views/Welcome/WelcomeView.swift` | First-launch experience |
| `AirlockApp/.../Services/ContainerSessionService.swift` | Manages subprocess lifecycle for docker exec terminals and docker logs streaming. Does NOT use Docker API directly — all interaction via subprocess spawning. |

### Modified Files

| File | Change |
|------|--------|
| `internal/cli/stop.go` | Add `--id` flag support |
| `internal/cli/root.go` | Register new commands |
| `internal/orchestrator/session.go` | Support ID-based naming, detached agent mode |
| `internal/container/manager.go` | Dynamic container/network names |
| `proxy/addon/decrypt_proxy.py` | Add structured JSON logging |
| `AirlockApp/.../Models/AppState.swift` | `activeWorkspaceIDs: Set<UUID>` |
| `AirlockApp/.../Models/Workspace.swift` | Add container/terminal state fields |
| `AirlockApp/.../Views/Sidebar/SidebarView.swift` | Multi-activate support |
| `AirlockApp/.../Views/Terminal/TerminalView.swift` | Refactor to docker exec model |
| `AirlockApp/.../ContentView.swift` | Add new tabs |
| `AirlockApp/.../AirlockApp.swift` | Fix menu bar notifications |
