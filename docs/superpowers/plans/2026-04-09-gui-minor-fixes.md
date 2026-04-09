# GUI Minor Fixes Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add on/off toggles and clearer headers to the passthrough sections in global and workspace settings, remove the legacy `.env file` flow from the New Workspace sheet, and delete the now-redundant "Secrets" section from workspace settings.

**Architecture:** Four independent UI changes in the SwiftUI app. A new `passthroughHostsDraft: [String]?` field on `AppSettings` persists the toggled-OFF editor text across app restarts. Workspace-level passthrough override semantics change: `nil` = inherit, empty array `[]` = explicit "no passthrough hosts for this workspace". Existing `Workspace.envFilePath` model field and CLI `--env` plumbing stay intact so existing workspaces keep activating.

**Tech Stack:** Swift 5.9+, SwiftUI, XCTest. Built via `make gui-build` / `make gui-test`. Runs on macOS 14+.

**Spec:** `docs/superpowers/specs/2026-04-09-gui-minor-fixes-design.md`

---

## File Structure

Files that will be created or modified:

| File | Role | Change |
|---|---|---|
| `AirlockApp/Sources/AirlockApp/Models/AppState.swift` | `AppSettings` struct definition | Add `passthroughHostsDraft: [String]?` field + Codable handling |
| `AirlockApp/Tests/AirlockAppTests/AppStateTests.swift` | `AppSettings` tests | Add round-trip tests for the new field |
| `AirlockApp/Sources/AirlockApp/Views/Settings/SettingsView.swift` | Global settings sheet | Rename section, add toggle, load/save draft |
| `AirlockApp/Sources/AirlockApp/Views/Settings/WorkspaceSettingsView.swift` | Workspace settings sheet | Rename passthrough section, add toggle, delete Secrets section, change empty-override semantics |
| `AirlockApp/Sources/AirlockApp/Views/Sidebar/NewWorkspaceSheet.swift` | New workspace sheet | Remove `.env file` TextField, state, pre-check, picker |

All five files are modifications. No new files. No file splits (all affected files are under the 800-line ceiling).

---

## Task 1: Add `passthroughHostsDraft` field to `AppSettings`

**Files:**
- Modify: `AirlockApp/Sources/AirlockApp/Models/AppState.swift:179-202`
- Test: `AirlockApp/Tests/AirlockAppTests/AppStateTests.swift`

- [ ] **Step 1: Write the failing test**

Append at the end of `AirlockApp/Tests/AirlockAppTests/AppStateTests.swift`, just before the closing brace of `final class AppStateTests`:

```swift
    // MARK: - passthroughHostsDraft round-trip

    func testPassthroughHostsDraftDefaultsToNil() {
        let settings = AppSettings()
        XCTAssertNil(settings.passthroughHostsDraft)
    }

    func testPassthroughHostsDraftEncodeDecodeRoundTrip() throws {
        var settings = AppSettings()
        settings.passthroughHosts = []
        settings.passthroughHostsDraft = ["api.anthropic.com", "auth.anthropic.com"]
        let data = try JSONEncoder().encode(settings)
        let decoded = try JSONDecoder().decode(AppSettings.self, from: data)
        XCTAssertEqual(decoded.passthroughHosts, [])
        XCTAssertEqual(decoded.passthroughHostsDraft, ["api.anthropic.com", "auth.anthropic.com"])
    }

    func testPassthroughHostsDraftMissingKeyDecodesAsNil() throws {
        // Simulate a settings.json written by the previous app version:
        // no passthroughHostsDraft key at all.
        let legacyJSON = """
        {
          "containerImage": "airlock-claude:latest",
          "proxyImage": "airlock-proxy:latest",
          "passthroughHosts": ["api.anthropic.com", "auth.anthropic.com"],
          "theme": "system",
          "terminal": { "fontName": "Menlo", "fontSize": 12 }
        }
        """.data(using: .utf8)!
        let decoded = try JSONDecoder().decode(AppSettings.self, from: legacyJSON)
        XCTAssertNil(decoded.passthroughHostsDraft)
        XCTAssertEqual(decoded.passthroughHosts, ["api.anthropic.com", "auth.anthropic.com"])
    }
```

- [ ] **Step 2: Run test to verify it fails**

Run: `make gui-test`

Expected: Build error — `AppSettings has no member 'passthroughHostsDraft'`.

- [ ] **Step 3: Add the field to `AppSettings`**

In `AirlockApp/Sources/AirlockApp/Models/AppState.swift`, find the `struct AppSettings: Codable, Equatable { ... }` block starting at line 179 and add the new field plus one decode line.

Change lines 179–202 from:

```swift
struct AppSettings: Codable, Equatable {
    var airlockBinaryPath: String?
    var containerImage: String = "airlock-claude:latest"
    var proxyImage: String = "airlock-proxy:latest"
    var passthroughHosts: [String] = ["api.anthropic.com", "auth.anthropic.com"]
    var enabledMCPServers: [String]?
    var networkAllowlist: [String]?
    var theme: AppTheme = .system
    var terminal: TerminalSettings = TerminalSettings()

    init() {}

    init(from decoder: Decoder) throws {
        let container = try decoder.container(keyedBy: CodingKeys.self)
        airlockBinaryPath = try container.decodeIfPresent(String.self, forKey: .airlockBinaryPath)
        containerImage = try container.decodeIfPresent(String.self, forKey: .containerImage) ?? "airlock-claude:latest"
        proxyImage = try container.decodeIfPresent(String.self, forKey: .proxyImage) ?? "airlock-proxy:latest"
        passthroughHosts = try container.decodeIfPresent([String].self, forKey: .passthroughHosts) ?? ["api.anthropic.com", "auth.anthropic.com"]
        enabledMCPServers = try container.decodeIfPresent([String].self, forKey: .enabledMCPServers)
        networkAllowlist = try container.decodeIfPresent([String].self, forKey: .networkAllowlist)
        theme = try container.decodeIfPresent(AppTheme.self, forKey: .theme) ?? .system
        terminal = try container.decodeIfPresent(TerminalSettings.self, forKey: .terminal) ?? TerminalSettings()
    }
}
```

to:

```swift
struct AppSettings: Codable, Equatable {
    var airlockBinaryPath: String?
    var containerImage: String = "airlock-claude:latest"
    var proxyImage: String = "airlock-proxy:latest"
    var passthroughHosts: [String] = ["api.anthropic.com", "auth.anthropic.com"]
    /// Editor text to restore when the global passthrough toggle is OFF.
    /// Never read by the CLI layer; used only by `SettingsView` to preserve
    /// the user's in-progress host list across app restarts while the
    /// toggle is disabled. Nil when the toggle is ON.
    var passthroughHostsDraft: [String]?
    var enabledMCPServers: [String]?
    var networkAllowlist: [String]?
    var theme: AppTheme = .system
    var terminal: TerminalSettings = TerminalSettings()

    init() {}

    init(from decoder: Decoder) throws {
        let container = try decoder.container(keyedBy: CodingKeys.self)
        airlockBinaryPath = try container.decodeIfPresent(String.self, forKey: .airlockBinaryPath)
        containerImage = try container.decodeIfPresent(String.self, forKey: .containerImage) ?? "airlock-claude:latest"
        proxyImage = try container.decodeIfPresent(String.self, forKey: .proxyImage) ?? "airlock-proxy:latest"
        passthroughHosts = try container.decodeIfPresent([String].self, forKey: .passthroughHosts) ?? ["api.anthropic.com", "auth.anthropic.com"]
        passthroughHostsDraft = try container.decodeIfPresent([String].self, forKey: .passthroughHostsDraft)
        enabledMCPServers = try container.decodeIfPresent([String].self, forKey: .enabledMCPServers)
        networkAllowlist = try container.decodeIfPresent([String].self, forKey: .networkAllowlist)
        theme = try container.decodeIfPresent(AppTheme.self, forKey: .theme) ?? .system
        terminal = try container.decodeIfPresent(TerminalSettings.self, forKey: .terminal) ?? TerminalSettings()
    }
}
```

Swift auto-synthesizes `CodingKeys` to include every stored property, so adding the field alone is enough — no explicit `CodingKeys` enum change is needed.

- [ ] **Step 4: Run tests to verify they pass**

Run: `make gui-test`

Expected: All AppState tests pass, including the three new `passthroughHostsDraft` tests.

- [ ] **Step 5: Commit**

```bash
git add AirlockApp/Sources/AirlockApp/Models/AppState.swift AirlockApp/Tests/AirlockAppTests/AppStateTests.swift
git commit -m "feat: add passthroughHostsDraft field to AppSettings"
```

---

## Task 2: Rename global settings section and add passthrough toggle

**Files:**
- Modify: `AirlockApp/Sources/AirlockApp/Views/Settings/SettingsView.swift`

Goal: rename `Section("Network Defaults")` → `Section("Passthrough Hosts")`, introduce a toggle that controls editor visibility and write path, and persist/restore the editor text via `passthroughHostsDraft` when the toggle is OFF.

- [ ] **Step 1: Add the toggle state property**

In `AirlockApp/Sources/AirlockApp/Views/Settings/SettingsView.swift`, find the `@State` declarations at the top of `GlobalSettingsSheet` (lines 6–19). Add one new `@State` line after the existing `@State private var passthroughText = ""` line so the block reads:

```swift
    @Bindable var appState: AppState
    @Environment(\.dismiss) private var dismiss
    @State private var settings = AppSettings()
    @State private var passthroughText = ""
    @State private var enablePassthrough = true
    @State private var saved = false
    @State private var volumeStatus = "Checking..."
    @State private var showImportSheet = false
    @State private var showResetAlert = false
    @State private var showRemoveAnthropicConfirm = false
    @State private var discoveredMCPServers: [String] = []
    @State private var restrictMCPServers = false
    @State private var enabledMCPSelection: Set<String> = []
    @State private var restrictNetworkAllowlist = false
    @State private var networkAllowlistText = ""
    @State private var showAllowlistAnthropicConfirm = false
```

- [ ] **Step 2: Rewrite the "Network Defaults" section**

Find the section block at lines 70–81:

```swift
                Section("Network Defaults") {
                    HostListEditor(
                        caption: "Default passthrough hosts (skip proxy decryption, one per line)",
                        text: $passthroughText,
                        missingHosts: PassthroughPolicy.missingProtectedHosts(
                            from: PassthroughPolicy.splitHostLines(passthroughText)
                        ),
                        warningText: { joined in
                            "Removing \(joined) from passthrough means Airlock will decrypt secrets in requests to Anthropic. Your plaintext credentials will be sent to Anthropic's servers. This defeats the purpose of Airlock — only remove for testing."
                        }
                    )
                }
```

Replace it with:

```swift
                Section("Passthrough Hosts") {
                    Toggle("Enable passthrough hosts", isOn: $enablePassthrough)
                    if enablePassthrough {
                        HostListEditor(
                            caption: "Passthrough hosts skip proxy decryption (one per line). Anthropic endpoints belong here so credentials stay encrypted in transit.",
                            text: $passthroughText,
                            missingHosts: PassthroughPolicy.missingProtectedHosts(
                                from: PassthroughPolicy.splitHostLines(passthroughText)
                            ),
                            warningText: { joined in
                                "Removing \(joined) from passthrough means Airlock will decrypt secrets in requests to Anthropic. Your plaintext credentials will be sent to Anthropic's servers. This defeats the purpose of Airlock — only remove for testing."
                            }
                        )
                    } else {
                        Text("All outbound HTTPS, including Anthropic, will flow through the proxy for secret decryption. Your plaintext credentials will be sent to Anthropic's servers.")
                            .font(.caption)
                            .foregroundStyle(.secondary)
                    }
                }
```

- [ ] **Step 3: Update `load()` to restore the toggle and draft**

Find `private func load()` at lines 211–230. Replace the first four body lines:

```swift
    private func load() {
        let store = WorkspaceStore()
        settings = (try? store.loadSettings()) ?? AppSettings()
        passthroughText = settings.passthroughHosts.joined(separator: "\n")
        discoveredMCPServers = MCPInventoryService.discoverServerNames()
```

with:

```swift
    private func load() {
        let store = WorkspaceStore()
        settings = (try? store.loadSettings()) ?? AppSettings()
        if settings.passthroughHosts.isEmpty {
            // Toggle OFF state — show the persisted draft (if any) so the
            // user sees whatever they were working on before they turned
            // passthrough off. Nil draft => empty editor.
            enablePassthrough = false
            passthroughText = (settings.passthroughHostsDraft ?? []).joined(separator: "\n")
        } else {
            enablePassthrough = true
            passthroughText = settings.passthroughHosts.joined(separator: "\n")
        }
        discoveredMCPServers = MCPInventoryService.discoverServerNames()
```

The rest of `load()` (MCP and network-allowlist branches) stays unchanged.

- [ ] **Step 4: Update `save()` to branch on the toggle**

Find `private func save()` at lines 232–244:

```swift
    private func save() {
        // Guardrails chain: passthrough → allow-list → commit. Each alert's
        // "confirm anyway" button re-enters this chain via the next helper
        // so users see BOTH warnings if they're both violated, instead of
        // silently losing the second alert after confirming the first.
        let parsed = PassthroughPolicy.splitHostLines(passthroughText)
        let missing = PassthroughPolicy.missingProtectedHosts(from: parsed)
        if !missing.isEmpty {
            showRemoveAnthropicConfirm = true
            return
        }
        proceedAfterPassthroughConfirmed()
    }
```

Replace with:

```swift
    private func save() {
        // Guardrails chain: passthrough → allow-list → commit. Each alert's
        // "confirm anyway" button re-enters this chain via the next helper
        // so users see BOTH warnings if they're both violated, instead of
        // silently losing the second alert after confirming the first.
        //
        // When the passthrough toggle is OFF, the stored host list is []
        // (meaning proxy decrypts everything). missingProtectedHosts([])
        // returns the full protected set, so the guardrail still fires —
        // disabling passthrough is always a confirmed action.
        let parsed = enablePassthrough
            ? PassthroughPolicy.splitHostLines(passthroughText)
            : []
        let missing = PassthroughPolicy.missingProtectedHosts(from: parsed)
        if !missing.isEmpty {
            showRemoveAnthropicConfirm = true
            return
        }
        proceedAfterPassthroughConfirmed()
    }
```

- [ ] **Step 5: Update `commitSave()` to write `passthroughHosts` and `passthroughHostsDraft`**

Find `private func commitSave(hosts: [String])` at lines 257–281. Replace the first block that currently reads:

```swift
    private func commitSave(hosts: [String]) {
        settings.passthroughHosts = hosts
        settings.enabledMCPServers = restrictMCPServers
```

with:

```swift
    private func commitSave(hosts: [String]) {
        if enablePassthrough {
            settings.passthroughHosts = hosts
            settings.passthroughHostsDraft = nil
        } else {
            settings.passthroughHosts = []
            // Preserve whatever is currently in the editor so the user can
            // flip the toggle back ON later and see their previous list.
            let draft = PassthroughPolicy.splitHostLines(passthroughText)
            settings.passthroughHostsDraft = draft.isEmpty ? nil : draft
        }
        settings.enabledMCPServers = restrictMCPServers
```

The rest of `commitSave()` (MCP + network allow-list + persist) stays unchanged.

Also find `proceedAfterPassthroughConfirmed()` at lines 246–255. The final line is:

```swift
        commitSave(hosts: PassthroughPolicy.splitHostLines(passthroughText))
```

Change it to:

```swift
        commitSave(hosts: enablePassthrough ? PassthroughPolicy.splitHostLines(passthroughText) : [])
```

And the identical line inside the `showAllowlistAnthropicConfirm` alert block at lines 188–192 — the `"Save anyway"` button body:

```swift
            Button("Save anyway", role: .destructive) {
                commitSave(hosts: PassthroughPolicy.splitHostLines(passthroughText))
            }
```

becomes:

```swift
            Button("Save anyway", role: .destructive) {
                commitSave(hosts: enablePassthrough ? PassthroughPolicy.splitHostLines(passthroughText) : [])
            }
```

- [ ] **Step 6: Build the GUI to verify the changes compile**

Run: `make gui-build`

Expected: Build succeeds with no errors.

- [ ] **Step 7: Run tests**

Run: `make gui-test`

Expected: All tests pass. No new tests for this task — it is a view-layer change; behavior is verified through the `AppSettings` round-trip tests from Task 1 and manual testing.

- [ ] **Step 8: Commit**

```bash
git add AirlockApp/Sources/AirlockApp/Views/Settings/SettingsView.swift
git commit -m "feat: add passthrough hosts toggle to global settings"
```

---

## Task 3: Rename workspace passthrough section and add override toggle

**Files:**
- Modify: `AirlockApp/Sources/AirlockApp/Views/Settings/WorkspaceSettingsView.swift`

Goal: rename `Section("Network Overrides")` → `Section("Passthrough Override")`, add an override toggle whose OFF state inherits global passthrough, and change the empty-editor semantics so that `toggle ON + empty editor = explicit "[]"` (no longer "inherit").

- [ ] **Step 1: Add new state properties**

In `AirlockApp/Sources/AirlockApp/Views/Settings/WorkspaceSettingsView.swift`, find the `@State` block at lines 7–15. Add `overridePassthrough` after `passthroughText`:

```swift
    @State private var globalSettings = AppSettings()
    @State private var passthroughText = ""
    @State private var overridePassthrough = false
    @State private var showRemoveAnthropicConfirm = false
    @State private var discoveredMCPServers: [String] = []
    @State private var overrideMCPServers = false
    @State private var workspaceMCPSelection: Set<String> = []
    @State private var overrideNetworkAllowlist = false
    @State private var networkAllowlistText = ""
    @State private var showAllowlistAnthropicConfirm = false
```

- [ ] **Step 2: Rewrite the passthrough section**

Find the section block at lines 50–62:

```swift
            Section("Network Overrides") {
                let defaultHint = globalSettings.passthroughHosts.isEmpty
                    ? "No default passthrough hosts"
                    : "Default: \(globalSettings.passthroughHosts.joined(separator: ", "))"
                HostListEditor(
                    caption: "Passthrough hosts override (\(defaultHint))",
                    text: $passthroughText,
                    missingHosts: passthroughOverrideMissingHosts,
                    warningText: { joined in
                        "This override would remove \(joined) from passthrough. Airlock would decrypt secrets in requests to Anthropic, sending your plaintext credentials to Anthropic's servers."
                    }
                )
            }
```

Replace with:

```swift
            Section("Passthrough Override") {
                Toggle("Override global passthrough", isOn: $overridePassthrough)
                    .onChange(of: overridePassthrough) { _, newValue in
                        if newValue && passthroughText.isEmpty {
                            // Prefill from global when turning override on,
                            // matching the network allow-list override pattern.
                            passthroughText = globalSettings.passthroughHosts.joined(separator: "\n")
                        }
                    }
                if overridePassthrough {
                    HostListEditor(
                        caption: "Workspace passthrough hosts (one per line). Overrides global passthrough entirely.",
                        text: $passthroughText,
                        missingHosts: passthroughOverrideMissingHosts,
                        warningText: { joined in
                            "This override would remove \(joined) from passthrough. Airlock would decrypt secrets in requests to Anthropic, sending your plaintext credentials to Anthropic's servers."
                        }
                    )
                } else {
                    Text(inheritedPassthroughDescription)
                        .font(.caption)
                        .foregroundStyle(.secondary)
                }
            }
```

- [ ] **Step 3: Add `inheritedPassthroughDescription` computed property**

Find the existing `inheritedAllowlistDescription` computed property (around lines 175–180). Add a new sibling property just above it:

```swift
    private var inheritedPassthroughDescription: String {
        if globalSettings.passthroughHosts.isEmpty {
            return "Inheriting global setting (no passthrough hosts — proxy decrypts all HTTPS)."
        }
        return "Inheriting global passthrough: \(globalSettings.passthroughHosts.joined(separator: ", "))."
    }

    private var inheritedAllowlistDescription: String {
```

- [ ] **Step 4: Change `passthroughOverrideMissingHosts` semantics**

Find `private var passthroughOverrideMissingHosts: [String]` at lines 158–164. The current body is:

```swift
    /// Protected hosts missing from the workspace passthrough override.
    /// Returns an empty list when the editor is empty (empty = inherit
    /// global, not an explicit removal), so the HostListEditor only
    /// shows the warning for explicit non-empty overrides.
    private var passthroughOverrideMissingHosts: [String] {
        let parsed = PassthroughPolicy.splitHostLines(passthroughText)
        if parsed.isEmpty {
            return []
        }
        return PassthroughPolicy.missingProtectedHosts(from: parsed)
    }
```

Replace with:

```swift
    /// Protected hosts missing from the workspace passthrough override.
    /// The override toggle gates this: when the toggle is OFF we return
    /// an empty list (inherit is safe), but when ON we always check —
    /// including the empty-editor case, which now means "explicitly no
    /// passthrough for this workspace".
    private var passthroughOverrideMissingHosts: [String] {
        guard overridePassthrough else { return [] }
        let parsed = PassthroughPolicy.splitHostLines(passthroughText)
        return PassthroughPolicy.missingProtectedHosts(from: parsed)
    }
```

- [ ] **Step 5: Update `load()` to initialize the toggle from the stored override**

Find `private func load()` at lines 182–200. Replace the first three body lines after `globalSettings = ...`:

```swift
    private func load() {
        globalSettings = (try? WorkspaceStore().loadSettings()) ?? AppSettings()
        passthroughText = workspace.passthroughHostsOverride?.joined(separator: "\n") ?? ""
        discoveredMCPServers = MCPInventoryService.discoverServerNames()
```

with:

```swift
    private func load() {
        globalSettings = (try? WorkspaceStore().loadSettings()) ?? AppSettings()
        if let override = workspace.passthroughHostsOverride {
            overridePassthrough = true
            passthroughText = override.joined(separator: "\n")
        } else {
            overridePassthrough = false
            passthroughText = ""
        }
        discoveredMCPServers = MCPInventoryService.discoverServerNames()
```

- [ ] **Step 6: Update `save()` to gate the guardrail on the toggle**

Find `private func save()` at lines 202–216:

```swift
    private func save() {
        // Guardrails chain: passthrough → allow-list → commit. Each alert's
        // "confirm anyway" button re-enters this chain via the next helper
        // so users see BOTH warnings if they're both violated.
        let hosts = PassthroughPolicy.splitHostLines(passthroughText)
        // Empty override = inherit global; not flagged.
        if !hosts.isEmpty {
            let missing = PassthroughPolicy.missingProtectedHosts(from: hosts)
            if !missing.isEmpty {
                showRemoveAnthropicConfirm = true
                return
            }
        }
        proceedAfterPassthroughConfirmed()
    }
```

Replace with:

```swift
    private func save() {
        // Guardrails chain: passthrough → allow-list → commit. Each alert's
        // "confirm anyway" button re-enters this chain via the next helper
        // so users see BOTH warnings if they're both violated.
        //
        // The override toggle gates the passthrough guardrail: OFF = inherit
        // global (safe, no check). ON = check the editor contents, including
        // the empty-editor case which now means "explicitly no passthrough
        // for this workspace".
        if overridePassthrough {
            let hosts = PassthroughPolicy.splitHostLines(passthroughText)
            let missing = PassthroughPolicy.missingProtectedHosts(from: hosts)
            if !missing.isEmpty {
                showRemoveAnthropicConfirm = true
                return
            }
        }
        proceedAfterPassthroughConfirmed()
    }
```

- [ ] **Step 7: Update `commitSave()` to branch on the toggle**

Find `private func commitSave(hosts: [String])` at lines 229–243:

```swift
    private func commitSave(hosts: [String]) {
        if let idx = appState.workspaces.firstIndex(where: { $0.id == workspace.id }) {
            appState.workspaces[idx].passthroughHostsOverride = hosts.isEmpty ? nil : hosts
            appState.workspaces[idx].enabledMCPServersOverride = overrideMCPServers
                ? workspaceMCPSelection.sorted()
                : nil
            if overrideNetworkAllowlist {
                appState.workspaces[idx].networkAllowlistOverride =
                    NetworkAllowlistPolicy.splitHostLines(networkAllowlistText)
            } else {
                appState.workspaces[idx].networkAllowlistOverride = nil
            }
        }
        try? WorkspaceStore().saveWorkspaces(appState.workspaces)
    }
```

Replace the `passthroughHostsOverride` line with a toggle-based branch:

```swift
    private func commitSave(hosts: [String]) {
        if let idx = appState.workspaces.firstIndex(where: { $0.id == workspace.id }) {
            if overridePassthrough {
                // Explicit override — empty array means "no passthrough for
                // this workspace" (not "inherit"). nil is only written when
                // the toggle is OFF.
                appState.workspaces[idx].passthroughHostsOverride =
                    PassthroughPolicy.splitHostLines(passthroughText)
            } else {
                appState.workspaces[idx].passthroughHostsOverride = nil
            }
            appState.workspaces[idx].enabledMCPServersOverride = overrideMCPServers
                ? workspaceMCPSelection.sorted()
                : nil
            if overrideNetworkAllowlist {
                appState.workspaces[idx].networkAllowlistOverride =
                    NetworkAllowlistPolicy.splitHostLines(networkAllowlistText)
            } else {
                appState.workspaces[idx].networkAllowlistOverride = nil
            }
        }
        try? WorkspaceStore().saveWorkspaces(appState.workspaces)
    }
```

The `hosts` parameter is now unused in the passthrough branch. That is intentional — we read straight from `passthroughText` through `splitHostLines` so the explicit-empty case (`[]`) is preserved. All two callers of `commitSave` still pass a value; we leave the signature alone to avoid churning more code.

- [ ] **Step 8: Build the GUI**

Run: `make gui-build`

Expected: Build succeeds with no errors.

- [ ] **Step 9: Run tests**

Run: `make gui-test`

Expected: All tests pass.

- [ ] **Step 10: Commit**

```bash
git add AirlockApp/Sources/AirlockApp/Views/Settings/WorkspaceSettingsView.swift
git commit -m "feat: add passthrough override toggle to workspace settings"
```

---

## Task 4: Remove Secrets section from workspace settings

**Files:**
- Modify: `AirlockApp/Sources/AirlockApp/Views/Settings/WorkspaceSettingsView.swift:19-33`

- [ ] **Step 1: Delete the Secrets section**

Open `AirlockApp/Sources/AirlockApp/Views/Settings/WorkspaceSettingsView.swift`. Find and delete the entire `Section("Secrets") { ... }` block at lines 19–33:

```swift
            Section("Secrets") {
                HStack {
                    Text("Manage secret files in the Secrets tab (Cmd+2)")
                        .font(.caption)
                        .foregroundStyle(.secondary)
                    Spacer()
                }
                if let envPath = workspace.envFilePath {
                    HStack {
                        Text("Legacy .env: \((envPath as NSString).lastPathComponent)")
                            .font(.caption)
                            .foregroundStyle(.secondary)
                    }
                }
            }

```

After removal, `Form { ... }` should start directly with `Section("Container Overrides") { ... }`.

- [ ] **Step 2: Build the GUI**

Run: `make gui-build`

Expected: Build succeeds.

- [ ] **Step 3: Run tests**

Run: `make gui-test`

Expected: All tests pass.

- [ ] **Step 4: Commit**

```bash
git add AirlockApp/Sources/AirlockApp/Views/Settings/WorkspaceSettingsView.swift
git commit -m "refactor: remove redundant Secrets section from workspace settings"
```

---

## Task 5: Remove `.env file` field from New Workspace sheet

**Files:**
- Modify: `AirlockApp/Sources/AirlockApp/Views/Sidebar/NewWorkspaceSheet.swift`

- [ ] **Step 1: Remove the `envFilePath` state property**

In `AirlockApp/Sources/AirlockApp/Views/Sidebar/NewWorkspaceSheet.swift`, find lines 15–19:

```swift
    @State private var selectedPath: String = ""
    @State private var envFilePath: String = ""
    @State private var statusMessage: String = ""
    @State private var isProcessing = false
    @State private var checks: [PreCheck] = []
```

Delete the `envFilePath` line so the block reads:

```swift
    @State private var selectedPath: String = ""
    @State private var statusMessage: String = ""
    @State private var isProcessing = false
    @State private var checks: [PreCheck] = []
```

- [ ] **Step 2: Remove the `.env file` HStack from the body**

Find the HStack at lines 33–44:

```swift
            HStack {
                TextField(".env file (optional)", text: $envFilePath)
                    .textFieldStyle(.roundedBorder)
                Button("Browse...") { pickEnvFile() }
                if !envFilePath.isEmpty {
                    Button { envFilePath = "" } label: {
                        Image(systemName: "xmark.circle.fill")
                            .foregroundStyle(.secondary)
                    }
                    .buttonStyle(.plain)
                }
            }

```

Delete the entire HStack block (including the trailing blank line).

- [ ] **Step 3: Remove the `.onChange(of: envFilePath)` modifier**

Find line 68:

```swift
        .onChange(of: selectedPath) { _, _ in Task { await runPreChecks() } }
        .onChange(of: envFilePath) { _, _ in Task { await runPreChecks() } }
```

Delete the second line so only `selectedPath` has an onChange.

- [ ] **Step 4: Remove the `.env` pre-check branch inside `runPreChecks()`**

Find lines 127–147 inside `runPreChecks()`:

```swift
        if !envFilePath.isEmpty {
            let envContent = (try? String(contentsOfFile: envFilePath, encoding: .utf8)) ?? ""
            let sensitivePatterns = ["KEY", "SECRET", "PASSWORD", "TOKEN"]
            let hasPlaintext = envContent
                .components(separatedBy: .newlines)
                .contains { line in
                    let parts = line.split(separator: "=", maxSplits: 1)
                    guard parts.count == 2 else { return false }
                    let key = String(parts[0]).uppercased()
                    let value = String(parts[1])
                    return sensitivePatterns.contains(where: { key.contains($0) }) && !value.hasPrefix("ENC[age:")
                }
            if hasPlaintext {
                results.append(PreCheck(
                    id: "secrets",
                    label: "Plaintext secrets detected",
                    passed: false,
                    detail: "will be encrypted on activation"
                ))
            }
        }

```

Delete this entire block.

- [ ] **Step 5: Remove `pickEnvFile()`**

Find lines 174–182:

```swift
    private func pickEnvFile() {
        let panel = NSOpenPanel()
        panel.canChooseDirectories = false
        panel.canChooseFiles = true
        panel.allowsMultipleSelection = false
        if panel.runModal() == .OK, let url = panel.url {
            envFilePath = url.path
        }
    }

```

Delete this entire function.

- [ ] **Step 6: Remove `envFilePath` argument from `Workspace(...)` init**

Find the `addWorkspace()` function, specifically the `Workspace(...)` init around lines 203–207:

```swift
            let name = URL(filePath: path).lastPathComponent
            let workspace = Workspace(
                name: name,
                path: path,
                envFilePath: envFilePath.isEmpty ? nil : envFilePath
            )
```

Replace with:

```swift
            let name = URL(filePath: path).lastPathComponent
            let workspace = Workspace(
                name: name,
                path: path
            )
```

`Workspace.init(name:path:envFilePath:containerImageOverride:)` already defaults `envFilePath` to `nil` (see `Workspace.swift:63`), so omitting it is safe.

- [ ] **Step 7: Build the GUI**

Run: `make gui-build`

Expected: Build succeeds. No references to `envFilePath` should remain in `NewWorkspaceSheet.swift`. (They will still exist in `Workspace.swift`, `WorkspaceTests.swift`, and `ContainerSessionService.swift` — those are intentionally preserved for back-compat.)

- [ ] **Step 8: Run tests**

Run: `make gui-test`

Expected: All tests pass. Existing `WorkspaceTests` testing `envFilePath` continue to pass because the model field is unchanged.

- [ ] **Step 9: Commit**

```bash
git add AirlockApp/Sources/AirlockApp/Views/Sidebar/NewWorkspaceSheet.swift
git commit -m "refactor: remove .env file field from New Workspace sheet"
```

---

## Task 6: End-to-end manual smoke test

**Files:** none (manual test only)

- [ ] **Step 1: Launch the GUI**

Run: `make gui-run`

- [ ] **Step 2: Verify global settings section**

1. Open global settings (Cmd+,).
2. Find the section labeled **Passthrough Hosts** (not "Network Defaults").
3. Confirm the `Enable passthrough hosts` toggle is ON and the editor shows current hosts (likely `api.anthropic.com`, `auth.anthropic.com`).
4. Toggle OFF: the editor disappears and is replaced by the caption about plaintext credentials.
5. Click Save. A confirm dialog should appear warning that Anthropic passthrough is being disabled.
6. Click "Remove anyway". Dialog dismisses.
7. Re-open global settings. Confirm toggle is OFF, editor is hidden.
8. Toggle ON. The editor reappears with the previously entered hosts (restored from `passthroughHostsDraft`).
9. Click Save (no dialog since the hosts are restored).

- [ ] **Step 3: Verify workspace settings section**

1. Select a workspace in the sidebar.
2. Open the Settings tab (Cmd+4).
3. Confirm there is NO `Secrets` section.
4. Find the section labeled **Passthrough Override** (not "Network Overrides").
5. Confirm the `Override global passthrough` toggle is OFF and the caption reads "Inheriting global passthrough: ...".
6. Toggle ON. The editor appears prefilled with the global passthrough hosts.
7. Clear the editor. Click Save. A confirm dialog fires warning that the workspace blocks Anthropic (new semantic: empty + ON = explicit block).
8. Click Cancel. Toggle off. Click Save — no dialog (inherit is safe).

- [ ] **Step 4: Verify New Workspace sheet**

1. Click the `+` button in the sidebar.
2. Confirm there is NO `.env file (optional)` TextField.
3. Confirm there is NO `Plaintext secrets detected` pre-check row (even when selecting a directory that contains a `.env` file).
4. Select a project directory and create the workspace. It should appear in the sidebar with no errors.

- [ ] **Step 5: Verify existing workspace with `envFilePath` (if available)**

If you have an existing workspace in `~/Library/Application Support/airlock/workspaces.json` with an `envFilePath` value set:

1. Activate it — it should still start the container with the `--env` flag passed through by `ContainerSessionService`.
2. Open its Settings tab — the Secrets section is gone (so the legacy `.env` path is no longer shown in the UI), but activation continues to work.

If no such workspace exists, skip this step.

- [ ] **Step 6: Mark the manual test as complete**

There is nothing to commit. Notify the user that the manual smoke test passed.

---

## Self-Review Results

**Spec coverage:**
- Item 1 (rename + toggle global): Task 2 ✅
- Item 2 (rename + toggle workspace): Task 3 ✅
- Item 3 (remove `.env` from New Workspace sheet + precheck row): Task 5 ✅
- Item 4 (remove Secrets section from workspace settings): Task 4 ✅
- `AppSettings.passthroughHostsDraft` field: Task 1 ✅
- Draft round-trip tests: Task 1 ✅
- Manual verification: Task 6 ✅

**Placeholder scan:** No TBD/TODO. Every code step shows the concrete before/after. All commit messages are concrete. `PassthroughPolicy.splitHostLines` and `PassthroughPolicy.missingProtectedHosts` are the existing helpers used throughout the repo and require no new code.

**Type consistency:** `passthroughHostsDraft: [String]?` is used consistently in Tasks 1, 2, and verified in the round-trip test. `overridePassthrough: Bool`, `enablePassthrough: Bool`, and `inheritedPassthroughDescription` are each referenced exactly where declared. The `passthroughHostsOverride: [String]?` field on `Workspace` is unchanged at the model level — only the write path changes in Task 3.
