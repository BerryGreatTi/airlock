# ADR-0007: GUI theme switching, terminal settings, and Diff removal

## Status

Accepted

## Context

The GUI needed several improvements for daily usability:

1. No way to switch between light and dark themes -- the app always used the system appearance with no override
2. Terminal font and size were hardcoded (SF Mono 13pt) with no user configuration
3. Terminal colors did not adapt to the effective theme -- dark text on dark background was possible
4. The Diff tab (git diff viewer) provided little value since Claude Code shows diffs inline and users prefer `git diff` in the terminal
5. Closing the app window left Docker containers running with no cleanup
6. No app icon in the Dock (default SwiftUI icon)

## Decision

1. **Theme switching**: Add System/Light/Dark picker in global settings, applied via `.preferredColorScheme()`. Terminal colors auto-adapt with optimized presets for each mode.

2. **Terminal settings**: Add font picker (8 monospace fonts) and size slider (9-24pt) in global settings. Changes apply immediately to existing terminal panes via SwiftUI's `updateNSView` diffing.

3. **Remove Diff tab**: Delete DiffContainerView, SideBySideDiffView, DiffLineView, DiffModel, DiffParser, and all related tests. Renumber keyboard shortcuts (Settings becomes Cmd+4).

4. **App quit cleanup**: Add `NSApplicationDelegateAdaptor` with `AppDelegate` that deactivates all running containers on quit. Uses a 10-second timeout and `Task.detached` to avoid blocking.

5. **Dock icon**: Generate a programmatic airlock hatch icon via SwiftUI Canvas at runtime. No xcassets needed for SPM-based app.

## Consequences

**Easier:**
- Users can work comfortably in both light and dark environments
- Terminal readability improves with theme-optimized colors
- Fewer tabs simplifies the interface
- No orphaned containers after app quit

**Harder:**
- Git diff viewing requires switching to a terminal pane (acceptable tradeoff)
- Programmatic icon cannot be customized via asset catalog (acceptable for current stage)

## Alternatives Considered

- **Diff tab enhancement instead of removal**: Considered adding staged/commit views and container-live diffs, but the terminal already provides superior diff tooling. Removed to reduce complexity.
- **xcassets for dock icon**: Requires Xcode project wrapper instead of SPM. Rejected to keep the build simple.
- **Per-workspace theme override**: Added complexity for minimal benefit. Global theme is sufficient.
