# GUI Minor Fixes Design

**Date:** 2026-04-09
**Branch:** `feat/gui-minor-fixes`
**Status:** Approved

## Summary

Four small GUI adjustments in the SwiftUI app to reduce confusion between "Network Defaults/Overrides" (passthrough) and "Network Allow-list", and to retire the `.env file` flow from the New Workspace sheet now that env secrets have moved into the Secrets tab.

## Motivation

- "Network Defaults" and "Network Overrides" are visually adjacent to "Network Allow-list" but mean a completely different thing (which hosts skip the proxy decrypt pass). Users conflate them.
- The passthrough sections are the only network-area sections without an explicit on/off toggle, breaking consistency with MCP Servers and Network Allow-list.
- The New Workspace sheet still offers a `.env file (optional)` field that predates `airlock secret env add` and the Secrets tab. It is the only place in the GUI that still introduces a legacy `envFilePath` flow to new users.
- The workspace settings "Secrets" section is a pointer to the Secrets tab plus a legacy-path display. Both are clutter.

## Scope

### Item 1 — Global Settings: `Passthrough Hosts` section

Target: `AirlockApp/Sources/AirlockApp/Views/Settings/SettingsView.swift`

- Rename `Section("Network Defaults")` → `Section("Passthrough Hosts")`.
- Add a toggle `Toggle("Enable passthrough hosts", isOn: $enablePassthrough)`.
- Toggle ON: show the existing `HostListEditor` unchanged.
- Toggle OFF: show a caption — "All outbound HTTPS, including Anthropic, will flow through the proxy for secret decryption. Your plaintext credentials will be sent to Anthropic's servers."
- Save semantics:
  - Toggle ON: `settings.passthroughHosts = [parsed hosts]`
  - Toggle OFF: `settings.passthroughHosts = []`
- Guardrail: no change. `PassthroughPolicy.missingProtectedHosts([])` already triggers `showRemoveAnthropicConfirm` on save.

### Item 2 — Workspace Settings: `Passthrough Override` section

Target: `AirlockApp/Sources/AirlockApp/Views/Settings/WorkspaceSettingsView.swift`

- Rename `Section("Network Overrides")` → `Section("Passthrough Override")`.
- Add a toggle `Toggle("Override global passthrough", isOn: $overridePassthrough)`.
- Toggle OFF: caption — "Inheriting global passthrough: `<hosts>`" or "Inheriting global setting (no passthrough hosts)." (mirrors `inheritedAllowlistDescription`).
- Toggle ON: show the existing `HostListEditor`.
- On OFF → ON transition: prefill editor with `globalSettings.passthroughHosts` (mirrors the network allow-list override pattern at `WorkspaceSettingsView.swift:88`).
- Save semantics (meaning change approved by user):
  - Toggle OFF → `workspace.passthroughHostsOverride = nil` (inherit)
  - Toggle ON + any content (including empty editor) → `workspace.passthroughHostsOverride = [parsed hosts]` (possibly an empty array, explicit "no passthrough" for this workspace)
- Guardrail: fire `showRemoveAnthropicConfirm` whenever the toggle is ON and `PassthroughPolicy.missingProtectedHosts(parsed)` is non-empty — this now includes the empty-editor case, which previously was silently treated as inherit.
- Update `passthroughOverrideMissingHosts` computed property: remove the early-return on `parsed.isEmpty`. That early-return was the old "empty = inherit" shortcut.

### Item 3 — New Workspace sheet: remove `.env` flow

Target: `AirlockApp/Sources/AirlockApp/Views/Sidebar/NewWorkspaceSheet.swift`

Remove:
- `@State private var envFilePath: String = ""`
- The `.env file (optional)` `TextField` + Browse button + clear button HStack (lines 33–44)
- `.onChange(of: envFilePath)` modifier (line 68)
- The `if !envFilePath.isEmpty { ... }` block inside `runPreChecks()` that emits the `"Plaintext secrets detected"` PreCheck (lines 127–147)
- `pickEnvFile()` function
- `envFilePath` argument in the `Workspace(...)` init inside `addWorkspace()` (defaults to `nil`)

### Item 4 — Workspace Settings: remove `Secrets` section

Target: `AirlockApp/Sources/AirlockApp/Views/Settings/WorkspaceSettingsView.swift`

- Delete the `Section("Secrets") { ... }` block (lines 19–33). It contains the caption "Manage secret files in the Secrets tab (Cmd+2)" and the conditional `if let envPath = workspace.envFilePath` row.

## Data Model Impact

### `AppSettings` — add `passthroughHostsDraft`

Add `passthroughHostsDraft: [String]?` on `AppSettings`, persisted in `settings.json`. The CLI layer never reads it. Only `SettingsView.load()` and `SettingsView.commitSave()` touch it. Purpose: preserve the editor text when the global passthrough toggle is OFF, across app restarts (the user's explicit A1-on-setting-file requirement).

- Field: `var passthroughHostsDraft: [String]?` on `AppSettings`
- Codable: `decodeIfPresent`, default `nil`
- Write path: `commitSave()` sets `passthroughHostsDraft` to current editor lines when toggle is OFF, and to `nil` when toggle is ON.
- Read path: `load()` — if `passthroughHosts` is non-empty, use it; else fall back to `passthroughHostsDraft ?? []`.
- Tests: add a round-trip test in `AppStateTests.swift` (or wherever `AppSettings` coding is tested) that saves a `draft` with empty `passthroughHosts` and verifies it decodes.

### `Workspace.envFilePath` — preserved

`Workspace.envFilePath: String?` stays. Keeping it means:
- Existing workspaces with `envFilePath` set keep activating with `--env <path>` via `ContainerSessionService.activate()` at `ContainerSessionService.swift:24`.
- `WorkspaceTests.swift` continues to pass.
- Newly created workspaces always have `envFilePath = nil` because the field is removed from `NewWorkspaceSheet`.

### `Workspace.passthroughHostsOverride` — semantic change, no model change

The field is already `[String]?` with `decodeIfPresent`, so nil vs empty array are distinguishable at the JSON level. The old UI collapsed empty to nil on save. The new UI preserves the distinction.

## Architecture / Behavior

### Guardrail chains

Global (`SettingsView.save` → `proceedAfterPassthroughConfirmed` → `commitSave`):
- Unchanged. Toggle OFF just writes `passthroughHosts = []`, which is the same input the existing chain already handles (empty array → missing protected hosts → confirm alert).

Workspace (`WorkspaceSettingsView.save` → `proceedAfterPassthroughConfirmed` → `commitSave`):
- The `if !hosts.isEmpty` guard in `save()` must be replaced. New condition: `if overridePassthrough { check missingProtectedHosts }`. When the toggle is OFF, skip the passthrough guardrail (inherit is safe).
- `commitSave`: branch on `overridePassthrough`, not on `hosts.isEmpty`.

### State restoration

`SettingsView.load()`:
```
if settings.passthroughHosts.isEmpty && settings.passthroughHostsDraft == nil {
    enablePassthrough = false
    passthroughText = ""
} else if settings.passthroughHosts.isEmpty {
    enablePassthrough = false
    passthroughText = draft.joined("\n")
} else {
    enablePassthrough = true
    passthroughText = settings.passthroughHosts.joined("\n")
}
```

`WorkspaceSettingsView.load()`:
```
if let override = workspace.passthroughHostsOverride {
    overridePassthrough = true
    passthroughText = override.joined("\n")
} else {
    overridePassthrough = false
    passthroughText = ""  // not prefilled until the toggle flips ON
}
```

`WorkspaceSettingsView` on toggle change:
```
.onChange(of: overridePassthrough) { _, newValue in
    if !newValue {
        // Leave passthroughText alone in memory so user can flip back without losing work.
    } else if passthroughText.isEmpty {
        passthroughText = globalSettings.passthroughHosts.joined("\n")
    }
}
```

## Testing Plan

### Unit

- `AppSettings` round-trip: new `passthroughHostsDraft` field decodes with legacy JSON (missing key → nil). Saves non-empty draft with empty hosts.
- `Workspace` model tests: unchanged, continue to pass (envFilePath field still present).

### Manual

1. **Global toggle OFF → Save**: expect `showRemoveAnthropicConfirm`. Confirm → `settings.json` has `passthroughHosts: []` and `passthroughHostsDraft: [...]`.
2. **Global toggle OFF, close & reopen app**: editor shows preserved draft, toggle is OFF.
3. **Global toggle ON, Anthropic-free hosts, Save**: same confirm alert, same behavior as today.
4. **Workspace toggle OFF**: caption shows inherited hosts. Save → no alert, `passthroughHostsOverride = nil` in workspaces.json.
5. **Workspace toggle OFF → ON**: editor prefilled with global hosts.
6. **Workspace toggle ON + clear editor → Save**: confirm alert fires (new semantic). Confirm → `passthroughHostsOverride = []`.
7. **New Workspace sheet**: no `.env` field. Create a workspace, open workspace settings — no Secrets section visible.
8. **Existing workspace with legacy `envFilePath`**: still activates correctly (CLI still gets `--env`). No UI exposes the legacy path.

## Out of Scope

- Removing `Workspace.envFilePath` field or `--env` CLI flag.
- Migrating existing workspaces with `envFilePath` set.
- Changing `SecretsView.passthroughBanner`.
- Changing `Section("Network Allow-list")` or `Section("MCP Servers")` in either view.
- Any Go/Python changes.

## References

- `AirlockApp/Sources/AirlockApp/Views/Settings/SettingsView.swift:70-81` — current Network Defaults section
- `AirlockApp/Sources/AirlockApp/Views/Settings/WorkspaceSettingsView.swift:50-62` — current Network Overrides section
- `AirlockApp/Sources/AirlockApp/Views/Settings/WorkspaceSettingsView.swift:19-33` — current Secrets section
- `AirlockApp/Sources/AirlockApp/Views/Sidebar/NewWorkspaceSheet.swift:33-44, 127-147` — current `.env` field and precheck
- `AirlockApp/Sources/AirlockApp/Models/Workspace.swift:7` — `envFilePath` field (preserved)
- `AirlockApp/Sources/AirlockApp/Services/ContainerSessionService.swift:24-26` — `--env` CLI flag passthrough (preserved)
