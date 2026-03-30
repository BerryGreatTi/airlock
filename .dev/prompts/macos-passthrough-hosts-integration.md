# macOS GUI: Passthrough Hosts CLI Integration

## Branch

`feat/passthrough-cli-flag` -- Go CLI already has `--passthrough-hosts` flag on `start` and `run` commands.

## Background

The Go CLI now accepts `--passthrough-hosts "host1,host2"` on both `airlock start` and `airlock run`. This overrides `config.yaml` passthrough_hosts at runtime. The default passthrough is now empty (all traffic goes through the decryption proxy).

The GUI has an `AppSettings.passthroughHosts` field and a Settings UI to edit it, but the value is never passed to the CLI. This means editing passthrough hosts in Settings has no effect.

## Tasks

### 1. Change AppSettings default to empty

**File:** `AirlockApp/Sources/AirlockApp/Models/AppState.swift:108`

Change:
```swift
var passthroughHosts: [String] = ["api.anthropic.com", "auth.anthropic.com"]
```
To:
```swift
var passthroughHosts: [String] = []
```

### 2. Pass --passthrough-hosts to CLI on activate

**File:** `AirlockApp/Sources/AirlockApp/Services/ContainerSessionService.swift`

The `activate(workspace:)` method currently builds args as:
```swift
var args = ["start", "--id", workspace.shortID]
if let envFile = workspace.envFilePath {
    args += ["--env", envFile]
}
```

It needs access to `AppSettings.passthroughHosts` to pass the flag. Two options:

**Option A (recommended): Pass settings to activate**

Change the method signature:
```swift
func activate(workspace: Workspace, settings: AppSettings) async throws -> CLIResult {
    var args = ["start", "--id", workspace.shortID]
    if let envFile = workspace.envFilePath {
        args += ["--env", envFile]
    }
    if !settings.passthroughHosts.isEmpty {
        args += ["--passthrough-hosts", settings.passthroughHosts.joined(separator: ",")]
    }
    // ... rest unchanged
}
```

Also update `activateAndWaitReady` to pass settings through:
```swift
func activateAndWaitReady(workspace: Workspace, settings: AppSettings) async throws -> CLIResult {
    let result = try await activate(workspace: workspace, settings: settings)
    try await waitForContainerReady(containerName: workspace.containerName)
    return result
}
```

**Option B: ContainerSessionService loads settings itself**

Inject `WorkspaceStore` and load settings in `activate`. More decoupled but adds a dependency.

### 3. Update AppState.performActivation to pass settings

**File:** `AirlockApp/Sources/AirlockApp/Models/AppState.swift:74-90`

`performActivation` calls `service.activateAndWaitReady(workspace:)`. It needs to load and pass settings:

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
        _ = try await service.activateAndWaitReady(workspace: workspace, settings: settings)
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

### 4. Update SettingsView hint text

**File:** `AirlockApp/Sources/AirlockApp/Views/Settings/SettingsView.swift:28`

Change:
```swift
Text("Passthrough hosts (MITM excluded, one per line)")
```
To:
```swift
Text("Passthrough hosts (skip proxy decryption, one per line)")
```

This better describes the behavior. "MITM excluded" is technically inaccurate since the proxy still sees the traffic, it just doesn't search for ENC patterns.

### 5. Update ContainerStatusView response action color

**File:** `AirlockApp/Sources/AirlockApp/Views/Containers/ContainerStatusView.swift`

The proxy now emits `"response"` action logs (added in commit `032c989`). The color mapping function should handle this:

Find the action-to-color mapping (around line 194) and add:
```swift
case "response": return .purple
```

Also update the summary counter section (around line 169) to show response count alongside decrypt/passthrough/none.

## Verification

```bash
# Build
make gui-build

# Run tests
make gui-test

# Manual verification
make gui-run
# 1. Open Settings, verify passthrough hosts field is empty by default
# 2. Add "api.anthropic.com" to passthrough, save
# 3. Activate a workspace
# 4. Check Container Status tab -- Anthropic traffic should show "passthrough"
# 5. Remove the host, save, reactivate
# 6. Anthropic traffic should now show "decrypt" (or "none" if no ENC patterns)
```

## Files Changed Summary

| File | Change |
|------|--------|
| `Models/AppState.swift` | Default `passthroughHosts` to `[]`, load settings in `performActivation` |
| `Services/ContainerSessionService.swift` | Add `settings` param to `activate`/`activateAndWaitReady`, pass `--passthrough-hosts` flag |
| `Views/Settings/SettingsView.swift` | Update hint text |
| `Views/Containers/ContainerStatusView.swift` | Handle `"response"` action color + counter |

## CLI Flag Format

```
--passthrough-hosts "api.anthropic.com,auth.anthropic.com"
```

Comma-separated, no spaces around commas. Empty string or omitted = use config.yaml default (which is now empty = all traffic through proxy).
