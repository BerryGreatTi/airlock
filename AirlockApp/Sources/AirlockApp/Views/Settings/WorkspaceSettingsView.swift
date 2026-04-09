import SwiftUI

@MainActor
struct WorkspaceSettingsView: View {
    let workspace: Workspace
    @Bindable var appState: AppState
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

    var body: some View {
        Form {
            Section("Container Overrides") {
                TextField(
                    "Container image (\(globalSettings.containerImage))",
                    text: stringBinding(\.containerImageOverride)
                )
                TextField(
                    "Proxy image (\(globalSettings.proxyImage))",
                    text: stringBinding(\.proxyImageOverride)
                )
                TextField(
                    "Proxy port (8080)",
                    text: portBinding()
                )
            }

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

            Section("MCP Servers Override") {
                MCPAllowListPicker(
                    enabled: $overrideMCPServers,
                    selection: $workspaceMCPSelection,
                    discovered: discoveredMCPServers,
                    toggleLabel: "Override global MCP setting",
                    restrictedCaption: "Only the checked MCP servers will be active in this workspace.",
                    unrestrictedCaption: inheritedMCPDescription,
                    emptyInventoryCaption: "No MCP servers found in ~/.claude/settings.json.",
                    noneSelectedWarning: "No MCP servers will be available in this workspace."
                )
                .onChange(of: overrideMCPServers) { _, newValue in
                    if !newValue {
                        workspaceMCPSelection = []
                    } else if let global = globalSettings.enabledMCPServers {
                        workspaceMCPSelection = Set(global)
                    }
                }
            }

            Section("Network Allow-list Override") {
                Toggle("Override global allow-list", isOn: $overrideNetworkAllowlist)
                    .onChange(of: overrideNetworkAllowlist) { _, newValue in
                        if !newValue {
                            networkAllowlistText = ""
                        } else if let global = globalSettings.networkAllowlist, !global.isEmpty {
                            networkAllowlistText = global.joined(separator: "\n")
                        }
                    }
                if overrideNetworkAllowlist {
                    HostListEditor(
                        caption: "One host per line. Use `*.example.com` for subdomain wildcards. Only HTTP/HTTPS is filtered.",
                        text: $networkAllowlistText,
                        missingHosts: NetworkAllowlistPolicy.missingProtectedHosts(
                            from: NetworkAllowlistPolicy.splitHostLines(networkAllowlistText)
                        ),
                        warningText: { joined in
                            "This workspace would block \(joined). Claude Code will not work here. Add `*.anthropic.com` or the specific hosts."
                        }
                    )
                } else {
                    Text(inheritedAllowlistDescription)
                        .font(.caption)
                        .foregroundStyle(.secondary)
                }
            }

            if appState.isActive(workspace) {
                HStack(spacing: 6) {
                    Image(systemName: "info.circle")
                        .foregroundStyle(.secondary)
                    Text("Changes take effect on next activation")
                        .font(.caption)
                        .foregroundStyle(.secondary)
                }
            }

            HStack {
                Spacer()
                Button("Save") { save() }
                    .keyboardShortcut(.defaultAction)
            }
        }
        .formStyle(.grouped)
        .padding()
        .onAppear { load() }
        .alert("Disable Anthropic passthrough for this workspace?", isPresented: $showRemoveAnthropicConfirm) {
            Button("Cancel", role: .cancel) {}
            Button("Remove anyway", role: .destructive) {
                proceedAfterPassthroughConfirmed()
            }
        } message: {
            let missing = PassthroughPolicy.missingProtectedHosts(
                from: PassthroughPolicy.splitHostLines(passthroughText)
            )
            Text("\(missing.joined(separator: ", ")) will not be in this workspace's passthrough list. Airlock will decrypt secrets in requests to Anthropic, sending your plaintext credentials to Anthropic's servers. Continue?")
        }
        .alert("Allow-list blocks Anthropic in this workspace?", isPresented: $showAllowlistAnthropicConfirm) {
            Button("Cancel", role: .cancel) {}
            Button("Save anyway", role: .destructive) {
                commitSave()
            }
        } message: {
            let missing = NetworkAllowlistPolicy.missingProtectedHosts(
                from: NetworkAllowlistPolicy.splitHostLines(networkAllowlistText)
            )
            Text("This workspace's allow-list does not cover \(missing.joined(separator: ", ")). The agent will be unable to reach Anthropic and Claude Code will stop responding. Continue?")
        }
    }

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

    private var inheritedMCPDescription: String {
        if let global = globalSettings.enabledMCPServers {
            return global.isEmpty
                ? "Inheriting global setting (no MCP servers enabled)."
                : "Inheriting global setting: \(global.joined(separator: ", "))."
        }
        return "Inheriting global setting (all MCP servers enabled)."
    }

    private var inheritedPassthroughDescription: String {
        if globalSettings.passthroughHosts.isEmpty {
            return "Inheriting global setting (no passthrough hosts — proxy decrypts all HTTPS)."
        }
        return "Inheriting global passthrough: \(globalSettings.passthroughHosts.joined(separator: ", "))."
    }

    private var inheritedAllowlistDescription: String {
        if let global = globalSettings.networkAllowlist, !global.isEmpty {
            return "Inheriting global allow-list: \(global.joined(separator: ", "))."
        }
        return "Inheriting global setting (all HTTP/HTTPS hosts allowed)."
    }

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
        if let override = workspace.enabledMCPServersOverride {
            overrideMCPServers = true
            workspaceMCPSelection = Set(override)
        } else {
            overrideMCPServers = false
            workspaceMCPSelection = []
        }
        if let override = workspace.networkAllowlistOverride, !override.isEmpty {
            overrideNetworkAllowlist = true
            networkAllowlistText = override.joined(separator: "\n")
        } else {
            overrideNetworkAllowlist = false
            networkAllowlistText = ""
        }
    }

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

    private func proceedAfterPassthroughConfirmed() {
        if overrideNetworkAllowlist {
            let allowlist = NetworkAllowlistPolicy.splitHostLines(networkAllowlistText)
            if !NetworkAllowlistPolicy.missingProtectedHosts(from: allowlist).isEmpty {
                showAllowlistAnthropicConfirm = true
                return
            }
        }
        commitSave()
    }

    private func commitSave() {
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

    private func stringBinding(_ keyPath: WritableKeyPath<Workspace, String?>) -> Binding<String> {
        Binding(
            get: {
                let ws = appState.workspaces.first { $0.id == workspace.id } ?? workspace
                return ws[keyPath: keyPath] ?? ""
            },
            set: { newValue in
                if let idx = appState.workspaces.firstIndex(where: { $0.id == workspace.id }) {
                    appState.workspaces[idx][keyPath: keyPath] = newValue.isEmpty ? nil : newValue
                    try? WorkspaceStore().saveWorkspaces(appState.workspaces)
                }
            }
        )
    }

    private func portBinding() -> Binding<String> {
        Binding(
            get: {
                let ws = appState.workspaces.first { $0.id == workspace.id } ?? workspace
                if let port = ws.proxyPortOverride { return String(port) }
                return ""
            },
            set: { newValue in
                if let idx = appState.workspaces.firstIndex(where: { $0.id == workspace.id }) {
                    appState.workspaces[idx].proxyPortOverride = Int(newValue)
                    try? WorkspaceStore().saveWorkspaces(appState.workspaces)
                }
            }
        )
    }

}
