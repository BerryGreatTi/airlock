# Settings Separation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Separate global app settings (Cmd+, sheet) from per-workspace settings (tab bar Settings tab) so each workspace can override container image, proxy image, passthrough hosts, and proxy port.

**Architecture:** The existing `SettingsView` is split into `GlobalSettingsSheet` (macOS standard settings sheet) and `WorkspaceSettingsView` (workspace tab). `Workspace` struct gains 3 optional override fields. `ResolvedSettings` struct merges global defaults with workspace overrides at activation time.

**Tech Stack:** Swift 5.9+, SwiftUI, macOS 14+

---

### Task 1: Add override fields to Workspace model

**Files:**
- Modify: `AirlockApp/Sources/AirlockApp/Models/Workspace.swift:3-19`
- Modify: `AirlockApp/Tests/AirlockAppTests/WorkspaceTests.swift`

- [ ] **Step 1: Write failing test for new fields**

In `WorkspaceTests.swift`, add:

```swift
func testOverrideFieldsDefaultToNil() {
    let ws = Workspace(name: "test", path: "/tmp")
    XCTAssertNil(ws.proxyImageOverride)
    XCTAssertNil(ws.passthroughHostsOverride)
    XCTAssertNil(ws.proxyPortOverride)
}

func testOverrideFieldsPersisted() throws {
    var ws = Workspace(name: "test", path: "/tmp")
    ws.proxyImageOverride = "custom-proxy:v2"
    ws.passthroughHostsOverride = ["api.example.com"]
    ws.proxyPortOverride = 9090
    let data = try JSONEncoder().encode(ws)
    let decoded = try JSONDecoder().decode(Workspace.self, from: data)
    XCTAssertEqual(decoded.proxyImageOverride, "custom-proxy:v2")
    XCTAssertEqual(decoded.passthroughHostsOverride, ["api.example.com"])
    XCTAssertEqual(decoded.proxyPortOverride, 9090)
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd AirlockApp && swift test --filter WorkspaceTests`
Expected: FAIL -- properties don't exist

- [ ] **Step 3: Add fields to Workspace struct**

In `Workspace.swift`, add after `containerImageOverride` (line 8):

```swift
var proxyImageOverride: String?
var passthroughHostsOverride: [String]?
var proxyPortOverride: Int?
```

Update `CodingKeys` (line 17-19) to:

```swift
enum CodingKeys: String, CodingKey {
    case id, name, path, envFilePath, containerImageOverride
    case proxyImageOverride, passthroughHostsOverride, proxyPortOverride
}
```

Add custom `init(from:)` to handle missing keys in existing data:

```swift
init(from decoder: Decoder) throws {
    let container = try decoder.container(keyedBy: CodingKeys.self)
    id = try container.decode(UUID.self, forKey: .id)
    name = try container.decode(String.self, forKey: .name)
    path = try container.decode(String.self, forKey: .path)
    envFilePath = try container.decodeIfPresent(String.self, forKey: .envFilePath)
    containerImageOverride = try container.decodeIfPresent(String.self, forKey: .containerImageOverride)
    proxyImageOverride = try container.decodeIfPresent(String.self, forKey: .proxyImageOverride)
    passthroughHostsOverride = try container.decodeIfPresent([String].self, forKey: .passthroughHostsOverride)
    proxyPortOverride = try container.decodeIfPresent(Int.self, forKey: .proxyPortOverride)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd AirlockApp && swift test --filter WorkspaceTests`
Expected: ALL PASS

- [ ] **Step 5: Commit**

```bash
git add AirlockApp/Sources/AirlockApp/Models/Workspace.swift AirlockApp/Tests/AirlockAppTests/WorkspaceTests.swift
git commit -m "feat: add workspace override fields for proxy image, passthrough hosts, proxy port"
```

---

### Task 2: Add ResolvedSettings and update activation logic

**Files:**
- Modify: `AirlockApp/Sources/AirlockApp/Models/AppState.swift:74-92`
- Modify: `AirlockApp/Sources/AirlockApp/Services/ContainerSessionService.swift:22-37`

- [ ] **Step 1: Add ResolvedSettings struct**

In `AppState.swift`, add after `AppSettings` (after line 111):

```swift
struct ResolvedSettings {
    let containerImage: String
    let proxyImage: String
    let passthroughHosts: [String]
    let proxyPort: Int

    init(global: AppSettings, workspace: Workspace) {
        self.containerImage = workspace.containerImageOverride ?? global.containerImage
        self.proxyImage = workspace.proxyImageOverride ?? global.proxyImage
        self.passthroughHosts = workspace.passthroughHostsOverride ?? global.passthroughHosts
        self.proxyPort = workspace.proxyPortOverride ?? 8080
    }
}
```

- [ ] **Step 2: Update performActivation to use ResolvedSettings**

Replace `performActivation` (lines 74-92) with:

```swift
func performActivation(
    workspace: Workspace,
    using service: ContainerSessionService
) async {
    activationStates[workspace.id] = .activating
    lastError = nil
    do {
        let store = WorkspaceStore()
        let settings = (try? store.loadSettings()) ?? AppSettings()
        let resolved = ResolvedSettings(global: settings, workspace: workspace)
        _ = try await service.activateAndWaitReady(workspace: workspace, resolved: resolved)
        activationStates[workspace.id] = .active
        if let idx = workspaces.firstIndex(where: { $0.id == workspace.id }) {
            workspaces[idx].isActive = true
        }
    } catch {
        activationStates[workspace.id] = .inactive
        lastError = error.localizedDescription
    }
}
```

- [ ] **Step 3: Update ContainerSessionService to use ResolvedSettings**

Replace `activate` method (lines 22-37) with:

```swift
func activate(workspace: Workspace, resolved: ResolvedSettings) async throws -> CLIResult {
    var args = ["start", "--id", workspace.shortID]
    if let envFile = workspace.envFilePath {
        args += ["--env", envFile]
    }
    args += ["--passthrough-hosts", resolved.passthroughHosts.joined(separator: ",")]
    args += ["--proxy-port", String(resolved.proxyPort)]
    let result = try await cli.run(args: args, workingDirectory: workspace.path)
    if result.exitCode != 0 {
        throw NSError(
            domain: "ContainerSession",
            code: Int(result.exitCode),
            userInfo: [NSLocalizedDescriptionKey: result.stderr.isEmpty ? "activation failed" : result.stderr]
        )
    }
    return result
}
```

Replace `activateAndWaitReady` (line 72-76) with:

```swift
func activateAndWaitReady(workspace: Workspace, resolved: ResolvedSettings) async throws -> CLIResult {
    let result = try await activate(workspace: workspace, resolved: resolved)
    try await waitForContainerReady(containerName: workspace.containerName)
    return result
}
```

- [ ] **Step 4: Build to verify compilation**

Run: `cd AirlockApp && swift build`
Expected: Build succeeds

- [ ] **Step 5: Run all tests**

Run: `cd AirlockApp && swift test`
Expected: ALL PASS

- [ ] **Step 6: Commit**

```bash
git add AirlockApp/Sources/AirlockApp/Models/AppState.swift AirlockApp/Sources/AirlockApp/Services/ContainerSessionService.swift
git commit -m "feat: add ResolvedSettings for global+workspace merge at activation"
```

---

### Task 3: Add --proxy-port flag to CLI

**Files:**
- Modify: `internal/cli/start.go:31,117-122,140,151-157`
- Modify: `internal/cli/run.go:19-23,116-121`

- [ ] **Step 1: Add proxy-port flag to start command**

In `start.go`, add to the var block (after line 121):

```go
startProxyPort int
```

Update `RunStart` signature (line 31) to add `proxyPort int`:

```go
func RunStart(ctx context.Context, runtime container.ContainerRuntime, id, workspace, envFile, airlockDir, passthroughHosts string, passthroughOverride bool, proxyPort int) (*StartResult, error) {
```

After the passthrough override block (after line 52), add:

```go
if proxyPort > 0 {
    cfg.ProxyPort = proxyPort
}
```

Update the cobra RunE call (line 140) to pass the port:

```go
result, err := RunStart(ctx, docker, startID, startWorkspace, startEnvFile, ".airlock", startPassthroughHosts, cmd.Flags().Changed("passthrough-hosts"), startProxyPort)
```

Add flag registration in `init()` (after line 156):

```go
startCmd.Flags().IntVar(&startProxyPort, "proxy-port", 0, "proxy listening port (overrides config, default 8080)")
```

- [ ] **Step 2: Add proxy-port flag to run command**

In `run.go`, add to the var block (after line 22):

```go
runProxyPort int
```

After the passthrough block (after line 55), add:

```go
if cmd.Flags().Changed("proxy-port") && runProxyPort > 0 {
    cfg.ProxyPort = runProxyPort
}
```

Add flag registration in `init()` (after line 119):

```go
runCmd.Flags().IntVar(&runProxyPort, "proxy-port", 0, "proxy listening port (overrides config, default 8080)")
```

- [ ] **Step 3: Build and test**

Run: `make build && make test`
Expected: Build succeeds, all tests pass

- [ ] **Step 4: Verify flag works**

Run: `./bin/airlock start --help | grep proxy-port`
Expected: Shows `--proxy-port int   proxy listening port (overrides config, default 8080)`

- [ ] **Step 5: Commit**

```bash
git add internal/cli/start.go internal/cli/run.go
git commit -m "feat: add --proxy-port flag to start and run commands"
```

---

### Task 4: Convert SettingsView to GlobalSettingsSheet

**Files:**
- Modify: `AirlockApp/Sources/AirlockApp/Views/Settings/SettingsView.swift` (rename to GlobalSettingsSheet)

- [ ] **Step 1: Rewrite SettingsView as GlobalSettingsSheet**

Replace entire `SettingsView.swift` with:

```swift
import SwiftUI

@MainActor
struct GlobalSettingsSheet: View {
    @Bindable var appState: AppState
    @Environment(\.dismiss) private var dismiss
    @State private var settings = AppSettings()
    @State private var passthroughText = ""
    @State private var saved = false

    var body: some View {
        VStack(spacing: 0) {
            Form {
                Section("General") {
                    HStack {
                        TextField("Airlock binary path", text: Binding(
                            get: { settings.airlockBinaryPath ?? "(auto-detect from PATH)" },
                            set: { settings.airlockBinaryPath = $0.contains("auto-detect") ? nil : $0 }
                        ))
                        Button("Browse...") { pickBinary() }
                    }
                }

                Section("Container Defaults") {
                    TextField("Container image", text: $settings.containerImage)
                    TextField("Proxy image", text: $settings.proxyImage)
                }

                Section("Network Defaults") {
                    Text("Default passthrough hosts (skip proxy decryption, one per line)")
                        .font(.caption)
                        .foregroundStyle(.secondary)
                    TextEditor(text: $passthroughText)
                        .font(.system(size: 12, design: .monospaced))
                        .frame(height: 80)
                }
            }
            .formStyle(.grouped)

            HStack {
                Spacer()
                if saved {
                    Text("Saved")
                        .foregroundStyle(.green)
                        .transition(.opacity)
                }
                Button("Cancel") { dismiss() }
                    .keyboardShortcut(.cancelAction)
                Button("Save") { save() }
                    .keyboardShortcut(.defaultAction)
            }
            .padding()
        }
        .frame(width: 500, height: 400)
        .onAppear { load() }
    }

    private func load() {
        let store = WorkspaceStore()
        settings = (try? store.loadSettings()) ?? AppSettings()
        passthroughText = settings.passthroughHosts.joined(separator: "\n")
    }

    private func save() {
        settings.passthroughHosts = passthroughText
            .components(separatedBy: "\n")
            .map { $0.trimmingCharacters(in: .whitespaces) }
            .filter { !$0.isEmpty }

        let store = WorkspaceStore()
        try? store.saveSettings(settings)

        withAnimation { saved = true }
        DispatchQueue.main.asyncAfter(deadline: .now() + 1) {
            withAnimation { saved = false }
            dismiss()
        }
    }

    private func pickBinary() {
        let panel = NSOpenPanel()
        panel.canChooseFiles = true
        panel.canChooseDirectories = false
        if panel.runModal() == .OK, let url = panel.url {
            settings.airlockBinaryPath = url.path
        }
    }
}
```

- [ ] **Step 2: Build to verify compilation**

Run: `cd AirlockApp && swift build`
Expected: FAIL (references to `SettingsView` in other files)

- [ ] **Step 3: Commit (partial -- will fix references in next tasks)**

```bash
git add AirlockApp/Sources/AirlockApp/Views/Settings/SettingsView.swift
git commit -m "refactor: convert SettingsView to GlobalSettingsSheet"
```

---

### Task 5: Create WorkspaceSettingsView

**Files:**
- Create: `AirlockApp/Sources/AirlockApp/Views/Settings/WorkspaceSettingsView.swift`

- [ ] **Step 1: Create WorkspaceSettingsView**

```swift
import SwiftUI

@MainActor
struct WorkspaceSettingsView: View {
    let workspace: Workspace
    @Bindable var appState: AppState
    @State private var globalSettings = AppSettings()

    var body: some View {
        Form {
            Section("Environment") {
                HStack {
                    TextField(".env file path", text: binding(\.envFilePath))
                    Button("Browse...") { pickEnvFile() }
                }
            }

            Section("Container Overrides") {
                TextField(
                    "Container image (\(globalSettings.containerImage))",
                    text: binding(\.containerImageOverride)
                )
                TextField(
                    "Proxy image (\(globalSettings.proxyImage))",
                    text: binding(\.proxyImageOverride)
                )
                TextField(
                    "Proxy port (8080)",
                    text: portBinding()
                )
            }

            Section("Network Overrides") {
                let defaultHint = globalSettings.passthroughHosts.isEmpty
                    ? "No default passthrough hosts"
                    : "Default: \(globalSettings.passthroughHosts.joined(separator: ", "))"
                Text("Passthrough hosts override (\(defaultHint))")
                    .font(.caption)
                    .foregroundStyle(.secondary)
                TextEditor(text: passthroughBinding())
                    .font(.system(size: 12, design: .monospaced))
                    .frame(height: 80)
            }
        }
        .formStyle(.grouped)
        .padding()
        .onAppear { loadGlobalSettings() }
        .onChange(of: appState.workspaces) { _, _ in saveWorkspaces() }
    }

    private func loadGlobalSettings() {
        globalSettings = (try? WorkspaceStore().loadSettings()) ?? AppSettings()
    }

    private func saveWorkspaces() {
        try? WorkspaceStore().saveWorkspaces(appState.workspaces)
    }

    private func binding(_ keyPath: WritableKeyPath<Workspace, String?>) -> Binding<String> {
        Binding(
            get: { workspace[keyPath: keyPath] ?? "" },
            set: { newValue in
                if let idx = appState.workspaces.firstIndex(where: { $0.id == workspace.id }) {
                    appState.workspaces[idx][keyPath: keyPath] = newValue.isEmpty ? nil : newValue
                    saveWorkspaces()
                }
            }
        )
    }

    private func portBinding() -> Binding<String> {
        Binding(
            get: {
                if let port = workspace.proxyPortOverride { return String(port) }
                return ""
            },
            set: { newValue in
                if let idx = appState.workspaces.firstIndex(where: { $0.id == workspace.id }) {
                    appState.workspaces[idx].proxyPortOverride = Int(newValue)
                    saveWorkspaces()
                }
            }
        )
    }

    private func passthroughBinding() -> Binding<String> {
        Binding(
            get: {
                workspace.passthroughHostsOverride?.joined(separator: "\n") ?? ""
            },
            set: { newValue in
                if let idx = appState.workspaces.firstIndex(where: { $0.id == workspace.id }) {
                    let hosts = newValue
                        .components(separatedBy: "\n")
                        .map { $0.trimmingCharacters(in: .whitespaces) }
                        .filter { !$0.isEmpty }
                    appState.workspaces[idx].passthroughHostsOverride = hosts.isEmpty ? nil : hosts
                    saveWorkspaces()
                }
            }
        )
    }

    private func pickEnvFile() {
        let panel = NSOpenPanel()
        panel.canChooseFiles = true
        panel.canChooseDirectories = false
        if panel.runModal() == .OK, let url = panel.url {
            if let idx = appState.workspaces.firstIndex(where: { $0.id == workspace.id }) {
                appState.workspaces[idx].envFilePath = url.path
                saveWorkspaces()
            }
        }
    }
}
```

- [ ] **Step 2: Build to verify compilation**

Run: `cd AirlockApp && swift build`
Expected: May still fail due to ContentView references

- [ ] **Step 3: Commit**

```bash
git add AirlockApp/Sources/AirlockApp/Views/Settings/WorkspaceSettingsView.swift
git commit -m "feat: add WorkspaceSettingsView for per-workspace overrides"
```

---

### Task 6: Update ContentView and SidebarView for new settings flow

**Files:**
- Modify: `AirlockApp/Sources/AirlockApp/ContentView.swift:34-53,55-64,105-107`
- Modify: `AirlockApp/Sources/AirlockApp/Views/Sidebar/SidebarView.swift:52-60`
- Modify: `AirlockApp/Sources/AirlockApp/AirlockApp.swift:93-96,117-121`

- [ ] **Step 1: Update ContentView**

Add `@State private var showingGlobalSettings = false` after `terminalAction` (line 9).

Replace `detailContent` (lines 34-53) with:

```swift
@ViewBuilder
private var detailContent: some View {
    if appState.workspaces.isEmpty {
        WelcomeView(appState: appState)
    } else if let workspace = appState.selectedWorkspace {
        VStack(spacing: 0) {
            tabBar
            Divider()
            tabContent(workspace: workspace)
        }
    } else {
        ContentUnavailableView {
            Label("No Workspace Selected", systemImage: "sidebar.left")
        } description: {
            Text("Select a workspace from the sidebar or create a new one with Cmd+N")
        }
    }
}
```

Add `.sheet` modifier after `.alert` block (after line 31):

```swift
.sheet(isPresented: $showingGlobalSettings) {
    GlobalSettingsSheet(appState: appState)
}
```

Add `showingGlobalSettings` to focusedValues. Add a new `FocusedValueKey`:

In `AirlockApp.swift`, add after `TerminalActionKey`:

```swift
struct ShowGlobalSettingsKey: FocusedValueKey {
    typealias Value = Binding<Bool>
}
```

Add to `FocusedValues` extension:

```swift
var showGlobalSettings: Binding<Bool>? {
    get { self[ShowGlobalSettingsKey.self] }
    set { self[ShowGlobalSettingsKey.self] = newValue }
}
```

In `ContentView`, add to focusedValue chain:

```swift
.focusedValue(\.showGlobalSettings, $showingGlobalSettings)
```

Update `tabBar` (lines 55-64) to add Settings tab:

```swift
private var tabBar: some View {
    HStack(spacing: 0) {
        tabButton("Terminal", tab: .terminal, icon: "terminal")
        tabButton("Secrets", tab: .secrets, icon: "key")
        tabButton("Containers", tab: .containers, icon: "shippingbox")
        tabButton("Diff", tab: .diff, icon: "doc.text.magnifyingglass")
        tabButton("Settings", tab: .settings, icon: "gear")
        Spacer()
    }
    .background(Color(nsColor: .controlBackgroundColor))
}
```

Replace `SettingsView` in `tabContent` (lines 105-107) with:

```swift
if appState.selectedTab == .settings {
    WorkspaceSettingsView(workspace: workspace, appState: appState)
}
```

- [ ] **Step 2: Update SidebarView**

Replace the Settings button (lines 53-60) with:

```swift
Button {
    showingGlobalSettings = true
} label: {
    Label("Settings", systemImage: "gear")
        .frame(maxWidth: .infinity, alignment: .leading)
}
.buttonStyle(.plain)
.padding(.horizontal)
```

Add `@State private var showingGlobalSettings = false` to SidebarView properties.

Add `.sheet` modifier to the List:

```swift
.sheet(isPresented: $showingGlobalSettings) {
    GlobalSettingsSheet(appState: appState)
}
```

- [ ] **Step 3: Update AirlockApp.swift menu bar**

Replace the `Settings` menu button (line 95) to open global settings:

```swift
Button("Settings") {
    showGlobalSettings?.wrappedValue = true
}
.keyboardShortcut("5")
```

Add `@FocusedValue(\.showGlobalSettings) private var showGlobalSettings` to AirlockApp properties.

Replace the `Settings` scene (lines 117-121) with:

```swift
Settings {
    GlobalSettingsSheet(appState: appState ?? AppState())
}
```

Note: The `Settings` scene needs its own state. Since `appState` from FocusedValue is optional, use a fallback. Alternatively, keep the placeholder and rely on the sheet approach. The simpler option:

```swift
Settings {
    Text("Use Cmd+5 or the sidebar Settings button")
        .frame(width: 300, height: 100)
        .padding()
}
```

- [ ] **Step 4: Build and test**

Run: `cd AirlockApp && swift build && swift test`
Expected: Build succeeds, all tests pass

- [ ] **Step 5: Commit**

```bash
git add AirlockApp/Sources/AirlockApp/ContentView.swift AirlockApp/Sources/AirlockApp/Views/Sidebar/SidebarView.swift AirlockApp/Sources/AirlockApp/AirlockApp.swift
git commit -m "feat: wire up GlobalSettingsSheet and WorkspaceSettingsView"
```

---

### Task 7: Build, test, and verify

**Files:** None (verification only)

- [ ] **Step 1: Full Go build and test**

Run: `make build && make test`
Expected: All pass

- [ ] **Step 2: Full GUI build and test**

Run: `make gui-build && make gui-test`
Expected: All pass

- [ ] **Step 3: Manual smoke test**

Run: `make gui-run`

Verify:
1. Sidebar Settings button opens a sheet (not a tab)
2. Cmd+, opens the same sheet
3. Sheet has: binary path, container image, proxy image, passthrough hosts
4. Sheet does NOT have workspace-specific fields
5. Select a workspace -- tab bar shows Settings tab
6. Workspace Settings tab has: .env file, container image override, proxy image override, proxy port override, passthrough hosts override
7. Override fields show global default as placeholder
8. Save workspace settings, deactivate/reactivate -- overrides take effect

- [ ] **Step 4: Commit any fixes from smoke test**

- [ ] **Step 5: Final commit**

```bash
git add -A
git commit -m "feat: separate global and workspace settings (closes #14)"
```
