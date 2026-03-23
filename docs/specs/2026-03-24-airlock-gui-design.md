# Airlock GUI Design Spec

## Overview

macOS native SwiftUI application that provides a graphical interface for the airlock CLI tool. The GUI wraps the existing Go CLI binary, offering workspace management, a terminal emulator for containerized Claude Code sessions, a side-by-side diff viewer, and configuration management.

## Tech Stack

- **SwiftUI** (macOS 14+ / Sonoma) - UI framework
- **SwiftTerm** (Swift Package, by Miguel de Icaza) - Terminal emulation
- **Go CLI** (`bin/airlock`) - Existing backend, invoked as subprocess

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

## Features

### 1. Workspace Management

Sidebar displays persisted list of workspaces. Each workspace maps to a directory on disk that has been initialized with `airlock init`.

**New Workspace flow:**
1. User clicks "+" in sidebar
2. macOS `NSOpenPanel` directory picker opens
3. Optional: select `.env` file to encrypt
4. App runs `airlock init` in chosen directory
5. If `.env` selected, runs `airlock encrypt <path>`
6. Workspace added to sidebar, persisted to `~/Library/Application Support/Airlock/workspaces.json`

**Workspace data model:**
```swift
struct Workspace: Identifiable, Codable {
    let id: UUID
    var name: String        // directory basename
    var path: String        // absolute path to project directory
    var envFilePath: String? // .env file if configured
    var isRunning: Bool     // runtime state, not persisted
}
```

**Workspace states:**
- Stopped (gray indicator)
- Running (green indicator) - `airlock run` active
- Error (red indicator) - container crashed

### 2. Terminal

SwiftTerm-based native terminal emulator displayed as a tab in the main area.

**Behavior:**
- When user selects a workspace and clicks "Run" (or double-clicks), start `airlock run --env <envfile>` in the workspace directory
- Allocate a PTY, connect to SwiftTerm view
- Full terminal emulation: ANSI colors, cursor movement, scrollback buffer, text selection, copy/paste
- On session exit, terminal shows exit message and offers "Restart" button

**PTY connection:**
- Use `forkpty()` or `posix_openpt()` to create pseudo-terminal
- Connect master fd to SwiftTerm's `LocalProcess` or custom `DataSource`
- Child process: `bin/airlock run --workspace <path> --env <envfile>`

**SwiftTerm integration:**
- Use `LocalProcessTerminalView` (AppKit) wrapped in `NSViewRepresentable` for SwiftUI
- Configure: font (SF Mono 13pt), color scheme (dark), scrollback (10000 lines)

### 3. Side-by-Side Diff Viewer

Displayed as a second tab next to Terminal. Shows file changes in the workspace.

**Data source:** Run `git diff` in the workspace directory, parse unified diff output.

**Display:**
- Top: file path header (e.g., `src/main.go`)
- Left panel: old version with line numbers
- Right panel: new version with line numbers
- Deleted lines: red background
- Added lines: green background
- Context lines: default background
- Synchronized scrolling between panels

**If multiple files changed:** vertical list of file sections, each with its own side-by-side view. No file picker in MVP (show all changed files in sequence).

**Diff parser:**
- Parse `git diff --unified=3` output
- Extract file paths, hunks, line numbers
- Convert unified diff to side-by-side pairs (align added/removed lines)
- Handle: additions, deletions, modifications, new files, deleted files

**Refresh:** Manual refresh button + auto-refresh when tab becomes visible.

### 4. Settings

Opens as a tab (like a workspace tab) when "Settings" is clicked in sidebar.

**Global settings:**
| Field | Type | Default |
|-------|------|---------|
| Airlock binary path | File picker | `bin/airlock` (relative to app bundle or system PATH) |
| Default container image | Text field | `airlock-claude:latest` |
| Default proxy image | Text field | `airlock-proxy:latest` |
| Passthrough hosts | Text area (one per line) | `api.anthropic.com`, `auth.anthropic.com` |

**Per-workspace settings** (shown when workspace selected):
| Field | Type | Default |
|-------|------|---------|
| .env file path | File picker | None |
| Container image override | Text field | (use global) |

Settings persist to `~/Library/Application Support/Airlock/settings.json`. Per-workspace overrides stored in `workspaces.json`.

On save, settings that map to `.airlock/config.yaml` fields are written back to the workspace's config file.

## GUI to CLI Communication

The GUI never implements container/crypto logic directly. All operations go through the Go CLI binary.

### Subprocess Execution

```swift
// For one-shot commands (init, encrypt, stop)
func runCLI(args: [String], workingDirectory: String) async throws -> CLIResult {
    let process = Process()
    process.executableURL = airlockBinaryURL
    process.arguments = args
    process.currentDirectoryURL = URL(filePath: workingDirectory)
    // capture stdout + stderr via Pipe
    // return CLIResult(exitCode, stdout, stderr)
}
```

### PTY Session (Terminal)

```swift
// For interactive session (run)
func startSession(workspace: Workspace) -> PTYSession {
    // 1. forkpty() to create pseudo-terminal
    // 2. exec airlock run --workspace <path> [--env <envfile>]
    // 3. Return master fd for SwiftTerm to read/write
}
```

### Git Diff

```swift
// For diff viewer
func getWorkspaceDiff(workspace: Workspace) async throws -> [FileDiff] {
    let result = try await runProcess("git", args: ["diff", "--unified=3"], cwd: workspace.path)
    return DiffParser.parse(result.stdout)
}
```

## Project Structure

```
AirlockApp/
├── Package.swift                    # SPM package with SwiftTerm dependency
├── Sources/
│   └── AirlockApp/
│       ├── AirlockApp.swift         # @main App entry, WindowGroup
│       ├── ContentView.swift        # NavigationSplitView (sidebar + detail)
│       ├── Models/
│       │   ├── Workspace.swift      # Workspace data model
│       │   ├── AppState.swift       # @Observable app state
│       │   └── DiffModel.swift      # FileDiff, DiffHunk, DiffLine
│       ├── Views/
│       │   ├── Sidebar/
│       │   │   ├── SidebarView.swift
│       │   │   └── NewWorkspaceSheet.swift
│       │   ├── Terminal/
│       │   │   └── TerminalView.swift    # NSViewRepresentable for SwiftTerm
│       │   ├── Diff/
│       │   │   ├── DiffContainerView.swift   # Tab content, file list
│       │   │   ├── SideBySideDiffView.swift  # Two-panel diff
│       │   │   └── DiffLineView.swift        # Single line rendering
│       │   └── Settings/
│       │       └── SettingsView.swift
│       └── Services/
│           ├── CLIService.swift          # Subprocess management
│           ├── PTYService.swift          # PTY allocation + lifecycle
│           ├── WorkspaceStore.swift      # Persistence (workspaces.json)
│           └── DiffParser.swift          # git diff output parser
```

## Non-Goals (MVP)

- Multiple simultaneous terminal sessions
- Inline diff in terminal output
- File tree browser
- Syntax highlighting in diff (just line-level coloring)
- Auto-update mechanism
- Menu bar icon / background daemon
- Drag-and-drop workspace creation
- Windows/Linux support
