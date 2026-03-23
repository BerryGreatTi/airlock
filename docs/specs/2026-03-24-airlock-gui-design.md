# Airlock GUI Design Spec

## Overview

macOS native SwiftUI application that provides a graphical interface for the airlock CLI tool. The GUI wraps the existing Go CLI binary, offering workspace management, a terminal emulator for containerized Claude Code sessions, a side-by-side diff viewer, and configuration management.

## Tech Stack

- **SwiftUI** (macOS 14+ / Sonoma) - UI framework
- **SwiftTerm** (Swift Package, by Miguel de Icaza) - Terminal emulation
- **Go CLI** (`airlock`) - Existing backend, invoked as subprocess

## Build and Distribution

The SwiftUI app lives in `AirlockApp/` at the project root, alongside the existing Go CLI.

- **Build system**: Xcode project (not pure SPM) -- required for `Info.plist`, entitlements, and app bundle structure
- **SwiftTerm dependency**: Added via SPM in Xcode (File > Add Package Dependencies)
- **Go binary bundling**: Built separately via `make build`, copied into `AirlockApp.app/Contents/Resources/bin/airlock` as a build phase script
- **App Sandbox**: Disabled. The app must spawn subprocesses (`airlock`, `git`), access arbitrary filesystem paths, and connect to the Docker daemon -- all incompatible with App Sandbox
- **Code signing**: Ad-hoc for development (`-` identity). No notarization in MVP.

## Layout

VS Code-style sidebar + tabbed main area.

```
┌──────────┬─────────────────────────────────┐
│ AIRLOCK  │  [Terminal]  [Diff]             │
│          ├─────────────────────────────────┤
│ WORKSPACES│                                │
│ > my-proj│  Terminal or Diff content       │
│   api-srv│  fills this area                │
│   frontend│                                │
│          │                                 │
│ + New    │                                 │
│ Settings │                                 │
└──────────┴─────────────────────────────────┘
```

- **Sidebar** (fixed width ~200pt): workspace list with status indicators, "New Workspace" button, "Settings" link
- **Main area**: tabbed content (Terminal, Diff). One tab active at a time.
- **No status bar in MVP** - container/proxy status shown in sidebar under each workspace

## Menu Bar and Keyboard Shortcuts

Standard macOS menu bar with:

| Shortcut | Action |
|----------|--------|
| Cmd+N | New Workspace |
| Cmd+, | Settings |
| Cmd+1 | Terminal tab |
| Cmd+2 | Diff tab |
| Cmd+R | Run/Restart workspace |
| Cmd+. | Stop workspace |
| Cmd+Shift+R | Refresh diff |

Standard Edit menu (Copy, Paste, Select All) handled by SwiftTerm and system.

## Features

### 1. Workspace Management

Sidebar displays persisted list of workspaces. Each workspace maps to a directory on disk that has been initialized with `airlock init`.

**Constraint: Only one workspace may run at a time.** The Go CLI uses hard-coded container names (`airlock-claude`, `airlock-proxy`). Starting a second workspace would kill the first. The GUI enforces this: starting workspace B while A is running shows a confirmation dialog ("Stop workspace A and start B?").

**New Workspace flow:**
1. User clicks "+" in sidebar
2. macOS `NSOpenPanel` directory picker opens
3. Check if `.airlock/` exists in chosen directory:
   - **Exists**: skip init, show notice "Existing airlock workspace detected"
   - **Does not exist**: run `airlock init` with cwd set to chosen directory
4. Optional: select `.env` file to associate (stored in workspace model for use at `airlock run` time)
5. Workspace added to sidebar, persisted to `~/Library/Application Support/Airlock/workspaces.json`

Note: Encryption happens at `airlock run` time, not at workspace creation. The CLI's `run --env` flag handles encryption internally.

**Workspace deletion:**
Right-click context menu "Remove Workspace". Removes from `workspaces.json` only. The `.airlock/` directory on disk is not deleted. If the workspace is running, stop it first (with confirmation).

**Workspace data model:**
```swift
struct Workspace: Identifiable, Codable {
    let id: UUID
    var name: String        // directory basename
    var path: String        // absolute path to project directory
    var envFilePath: String? // path to plaintext .env file (encrypted at run time)
}
```

Note: `isRunning` is not stored in the model. It is derived from session lifecycle state in `AppState`.

**Workspace states:**
- **Stopped** (gray indicator) - no active session
- **Running** (green indicator) - PTY process alive, container active
- **Error** (red indicator) - PTY process exited with non-zero code

### Session Lifecycle Management

**State tracking:**
- `AppState` holds `activeWorkspaceID: UUID?` and `sessionStatus: SessionStatus` (enum: `.stopped`, `.running`, `.error(String)`)
- When `LocalProcessTerminalView`'s process exits, the delegate callback fires. If exit code == 0: set `.stopped`. If != 0: set `.error(stderr)`.
- Only the active workspace can be in Running state.

**On app launch:**
- All workspaces start as Stopped. The GUI does not attempt to detect pre-existing Docker containers from previous sessions.
- If stale containers exist, the user can run `airlock stop` via the GUI (or start a new session, which triggers `airlock stop` first via the CLI's cleanup logic).

**Error recovery:**
- Error state shows last stderr output in a dismissible banner above the terminal
- "Restart" button calls `airlock stop` (cleanup) then `airlock run`
- Sidebar error indicator clears when session restarts or user dismisses

### 2. Terminal

SwiftTerm-based native terminal emulator displayed as a tab in the main area.

**Behavior:**
- When user clicks "Run" on a workspace, start the CLI in the workspace directory
- Full terminal emulation: ANSI colors, cursor movement, scrollback buffer, text selection, copy/paste
- On session exit (code 0), terminal shows "Session ended" with "Restart" button
- On session exit (code != 0), terminal shows stderr output with "Restart" button

**SwiftTerm integration:**
Use `LocalProcessTerminalView` (AppKit) wrapped in `NSViewRepresentable` for SwiftUI. This view handles PTY creation and process spawning internally.

```swift
// TerminalView wraps SwiftTerm's LocalProcessTerminalView
struct TerminalView: NSViewRepresentable {
    let workspace: Workspace

    func makeNSView(context: Context) -> LocalProcessTerminalView {
        let terminal = LocalProcessTerminalView(frame: .zero)
        terminal.font = NSFont.monospacedSystemFont(ofSize: 13, weight: .regular)
        // Configure scrollback, colors
        return terminal
    }

    func startSession(terminal: LocalProcessTerminalView) {
        let airlockPath = Bundle.main.url(forResource: "airlock", withExtension: nil, subdirectory: "bin")?.path
            ?? "/usr/local/bin/airlock"

        var args = ["run"]
        if let envFile = workspace.envFilePath {
            args += ["--env", envFile]
        }

        terminal.startProcess(
            executable: airlockPath,
            args: args,
            environment: nil,  // inherit
            execName: "airlock"
        )
        // cwd is set via LocalProcessTerminalView's currentDirectoryURL or
        // by chdir in the child process. The CLI defaults workspace to os.Getwd().
    }
}
```

**Critical: working directory.** The `airlock` CLI resolves `.airlock/` relative to `cwd`. The `LocalProcessTerminalView` process must be started with its working directory set to `workspace.path`. Use `Process.currentDirectoryURL` or set cwd before exec.

- Configure: SF Mono 13pt, dark color scheme, 10000-line scrollback

### 3. Side-by-Side Diff Viewer

Displayed as a second tab next to Terminal. Shows uncommitted file changes in the workspace.

**Data source:** Run `git diff HEAD` in the workspace directory. This shows all uncommitted changes (both staged and unstaged) relative to the last commit.

**Non-git workspace:** If the directory is not a git repository, show a centered message: "Not a git repository. Diff viewer requires git."

**Display:**
- Top: file path header (e.g., `src/main.go`) with change type badge (Modified, Added, Deleted)
- Left panel: old version with line numbers
- Right panel: new version with line numbers
- Deleted lines: red background
- Added lines: green background
- Context lines: default background
- Synchronized scrolling between panels

**If multiple files changed:** vertical list of file sections, each with its own side-by-side view. No file picker in MVP (show all changed files in sequence).

**Diff parser:**
- Parse `git diff HEAD --unified=3` output
- Extract file paths, hunks, line numbers
- Convert unified diff to side-by-side pairs (align added/removed lines, pad with blank lines)
- Handle: additions, deletions, modifications, new files, deleted files

**Refresh:** Manual refresh button (Cmd+Shift+R) + auto-refresh when Diff tab becomes visible.

### 4. Settings

Opens as a tab (like a workspace tab) when "Settings" is clicked in sidebar.

**Global settings:**
| GUI Field | Type | Default | config.yaml field |
|-----------|------|---------|-------------------|
| Airlock binary path | File picker | Bundled (`Contents/Resources/bin/airlock`) or `$PATH` lookup | N/A (GUI-only) |
| Default container image | Text field | `airlock-claude:latest` | `container_image` |
| Default proxy image | Text field | `airlock-proxy:latest` | `proxy_image` |
| Passthrough hosts | Text area (one per line) | `api.anthropic.com`, `auth.anthropic.com` | `passthrough_hosts` |

**Not exposed in MVP:** `network_name` (always `airlock-net`), `proxy_port` (always `8080`). These use defaults from the Go CLI.

**Per-workspace settings** (shown when workspace selected):
| Field | Type | Default |
|-------|------|---------|
| .env file path | File picker | None |
| Container image override | Text field | (use global) |

**Binary path resolution:**
1. First: check `Bundle.main.resourceURL/bin/airlock` (bundled binary)
2. Fallback: search `$PATH` for `airlock`
3. Manual override via settings file picker (validated as executable)

**Persistence:**
- Global settings: `~/Library/Application Support/Airlock/settings.json`
- Per-workspace overrides: stored in `workspaces.json` alongside workspace entries
- On save: settings that map to `config.yaml` fields are written back to the workspace's `.airlock/config.yaml`

## GUI to CLI Communication

The GUI never implements container/crypto logic directly. All operations go through the Go CLI binary.

**Critical rule:** All CLI commands must set `currentDirectoryURL` to `workspace.path`. The CLI resolves `.airlock/` relative to the working directory.

### Subprocess Execution

```swift
// For one-shot commands (init, stop)
func runCLI(args: [String], workingDirectory: String) async throws -> CLIResult {
    let process = Process()
    process.executableURL = airlockBinaryURL
    process.arguments = args
    process.currentDirectoryURL = URL(filePath: workingDirectory)
    // capture stdout + stderr via Pipe
    // return CLIResult(exitCode, stdout, stderr)
}
```

### Terminal Session

Managed by SwiftTerm's `LocalProcessTerminalView` which handles PTY internally. See Terminal section above.

### Git Diff

```swift
func getWorkspaceDiff(workspace: Workspace) async throws -> [FileDiff] {
    let result = try await runProcess(
        "git", args: ["diff", "HEAD", "--unified=3"],
        cwd: workspace.path
    )
    return DiffParser.parse(result.stdout)
}
```

## Project Structure

```
AirlockApp/
├── AirlockApp.xcodeproj            # Xcode project
├── AirlockApp/
│   ├── AirlockApp.swift             # @main App entry, WindowGroup
│   ├── ContentView.swift            # NavigationSplitView (sidebar + detail)
│   ├── Models/
│   │   ├── Workspace.swift          # Workspace data model
│   │   ├── AppState.swift           # @Observable app state, session lifecycle
│   │   └── DiffModel.swift          # FileDiff, DiffHunk, DiffLine
│   ├── Views/
│   │   ├── Sidebar/
│   │   │   ├── SidebarView.swift
│   │   │   └── NewWorkspaceSheet.swift
│   │   ├── Terminal/
│   │   │   └── TerminalView.swift   # NSViewRepresentable for SwiftTerm
│   │   ├── Diff/
│   │   │   ├── DiffContainerView.swift   # Tab content, file list
│   │   │   ├── SideBySideDiffView.swift  # Two-panel diff
│   │   │   └── DiffLineView.swift        # Single line rendering
│   │   └── Settings/
│   │       └── SettingsView.swift
│   └── Services/
│       ├── CLIService.swift          # Subprocess management (cwd always set)
│       ├── WorkspaceStore.swift      # Persistence (workspaces.json, settings.json)
│       └── DiffParser.swift          # git diff output parser
├── Resources/
│   └── bin/                          # Go binary copied here by build phase
└── Info.plist
```

Note: `PTYService.swift` from original design is removed. SwiftTerm's `LocalProcessTerminalView` manages PTY internally.

## Non-Goals (MVP)

- Multiple simultaneous terminal sessions (blocked by hard-coded container names in CLI)
- Inline diff in terminal output
- File tree browser
- Syntax highlighting in diff (just line-level coloring)
- Auto-update mechanism
- Menu bar icon / background daemon
- Drag-and-drop workspace creation
- Windows/Linux support
- App Sandbox / notarization
- Detecting pre-existing Docker containers on app launch
