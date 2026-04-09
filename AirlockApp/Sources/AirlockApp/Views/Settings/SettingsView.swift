import SwiftUI

@MainActor
struct GlobalSettingsSheet: View {
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

    var body: some View {
        VStack(spacing: 0) {
            Form {
                Section("Appearance") {
                    Picker("Theme", selection: $settings.theme) {
                        ForEach(AppTheme.allCases, id: \.self) { theme in
                            Text(theme.rawValue).tag(theme)
                        }
                    }
                    .pickerStyle(.segmented)
                }

                Section("Terminal") {
                    Picker("Font", selection: $settings.terminal.fontName) {
                        ForEach(TerminalSettings.availableFonts, id: \.self) { font in
                            Text(font).font(.system(size: 12, design: .monospaced)).tag(font)
                        }
                    }
                    HStack {
                        Text("Font size")
                        Slider(value: $settings.terminal.fontSize, in: 9...24, step: 1)
                        Text("\(Int(settings.terminal.fontSize)) pt")
                            .font(.system(.body, design: .monospaced))
                            .frame(width: 44, alignment: .trailing)
                    }
                    // Preview
                    Text("The quick brown fox jumps over the lazy dog")
                        .font(.custom(settings.terminal.fontName, size: settings.terminal.fontSize))
                        .padding(6)
                        .frame(maxWidth: .infinity, alignment: .leading)
                        .background(Color(nsColor: .textBackgroundColor))
                        .clipShape(RoundedRectangle(cornerRadius: 4))
                }

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

                Section("MCP Servers") {
                    MCPAllowListPicker(
                        enabled: $restrictMCPServers,
                        selection: $enabledMCPSelection,
                        discovered: discoveredMCPServers,
                        toggleLabel: "Restrict available MCP servers",
                        restrictedCaption: "Only the checked MCP servers will be active inside the container.",
                        unrestrictedCaption: "All MCP servers from ~/.claude/settings.json will be available.",
                        emptyInventoryCaption: "No MCP servers found in ~/.claude/settings.json. Use `claude mcp add` to install one.",
                        noneSelectedWarning: "No MCP servers will be available — the container will see an empty mcpServers map."
                    )
                    .onChange(of: restrictMCPServers) { _, newValue in
                        if !newValue {
                            enabledMCPSelection = []
                        }
                    }
                }

                Section("Network Allow-list") {
                    Toggle("Restrict outbound hosts", isOn: $restrictNetworkAllowlist)
                    if restrictNetworkAllowlist {
                        HostListEditor(
                            caption: "Only the listed hosts can receive outbound HTTP/HTTPS traffic. Use `*.example.com` for subdomain wildcards. One entry per line.",
                            text: $networkAllowlistText,
                            missingHosts: NetworkAllowlistPolicy.missingProtectedHosts(
                                from: NetworkAllowlistPolicy.splitHostLines(networkAllowlistText)
                            ),
                            warningText: { joined in
                                "Allow-list is missing \(joined). The agent will not be able to reach Anthropic — Claude Code will stop working. Add `*.anthropic.com` or the specific hosts."
                            }
                        )
                    } else {
                        Text("Agent container can reach any HTTP/HTTPS host. Non-HTTP traffic is already blocked by the isolated Docker network.")
                            .font(.caption)
                            .foregroundStyle(.secondary)
                    }
                }

                Section("Claude Code State Volume") {
                    HStack {
                        Text("airlock-claude-home")
                            .font(.system(.body, design: .monospaced))
                        Spacer()
                        Text(volumeStatus)
                            .foregroundStyle(volumeStatus == "Ready" ? .green : .secondary)
                    }

                    Button("Import from Host ~/.claude...") {
                        showImportSheet = true
                    }

                    Button("Reset Volume...") {
                        showResetAlert = true
                    }
                    .foregroundStyle(.red)
                }
            }
            .formStyle(.grouped)
            .task {
                await checkVolumeStatus()
            }

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
        .frame(width: 500, height: 700)
        .onAppear { load() }
        .sheet(isPresented: $showImportSheet) {
            ImportConfigSheet()
        }
        .alert("Reset Volume?", isPresented: $showResetAlert) {
            Button("Cancel", role: .cancel) { }
            Button("Reset", role: .destructive) {
                Task {
                    let cli = CLIService()
                    let home = FileManager.default.homeDirectoryForCurrentUser.path
                    _ = try? await cli.run(args: ["volume", "reset", "--confirm"], workingDirectory: home)
                    await checkVolumeStatus()
                }
            }
        } message: {
            Text("This will delete all Claude Code state including OAuth tokens, history, and memory. This cannot be undone.")
        }
        .alert("Disable Anthropic passthrough?", isPresented: $showRemoveAnthropicConfirm) {
            Button("Cancel", role: .cancel) {}
            Button("Remove anyway", role: .destructive) {
                proceedAfterPassthroughConfirmed()
            }
        } message: {
            Text("\(passthroughMissingProtectedHosts.joined(separator: ", ")) will be removed from passthrough. Airlock will then decrypt secrets in requests to Anthropic, sending your plaintext credentials to Anthropic's servers. Continue?")
        }
        .alert("Allow-list blocks Anthropic?", isPresented: $showAllowlistAnthropicConfirm) {
            Button("Cancel", role: .cancel) {}
            Button("Save anyway", role: .destructive) {
                commitSave()
            }
        } message: {
            let missing = NetworkAllowlistPolicy.missingProtectedHosts(
                from: NetworkAllowlistPolicy.splitHostLines(networkAllowlistText)
            )
            Text("The network allow-list does not cover \(missing.joined(separator: ", ")). The agent will be unable to reach Anthropic and Claude Code will stop responding. Continue?")
        }
    }

    private func checkVolumeStatus() async {
        let cli = CLIService()
        let home = FileManager.default.homeDirectoryForCurrentUser.path
        if let result = try? await cli.run(args: ["volume", "status"], workingDirectory: home) {
            volumeStatus = result.stdout.contains("ready") ? "Ready" : "Not created"
        } else {
            volumeStatus = "Unknown"
        }
    }

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
        if let allowed = settings.enabledMCPServers {
            restrictMCPServers = true
            enabledMCPSelection = Set(allowed)
        } else {
            restrictMCPServers = false
            enabledMCPSelection = []
        }
        if let allowlist = settings.networkAllowlist, !allowlist.isEmpty {
            restrictNetworkAllowlist = true
            networkAllowlistText = allowlist.joined(separator: "\n")
        } else {
            restrictNetworkAllowlist = false
            networkAllowlistText = ""
        }
    }

    /// Protected Anthropic hosts missing from the passthrough list we are
    /// about to save. Uses the toggle state so the alert message and the
    /// guardrail check see the same "parsed" value: toggle OFF means we
    /// are saving `[]` regardless of what the editor shows.
    private var passthroughMissingProtectedHosts: [String] {
        let parsed = enablePassthrough
            ? PassthroughPolicy.splitHostLines(passthroughText)
            : []
        return PassthroughPolicy.missingProtectedHosts(from: parsed)
    }

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
        if !passthroughMissingProtectedHosts.isEmpty {
            showRemoveAnthropicConfirm = true
            return
        }
        proceedAfterPassthroughConfirmed()
    }

    private func proceedAfterPassthroughConfirmed() {
        if restrictNetworkAllowlist {
            let allowlist = NetworkAllowlistPolicy.splitHostLines(networkAllowlistText)
            if !NetworkAllowlistPolicy.missingProtectedHosts(from: allowlist).isEmpty {
                showAllowlistAnthropicConfirm = true
                return
            }
        }
        commitSave()
    }

    private func commitSave() {
        if enablePassthrough {
            settings.passthroughHosts = PassthroughPolicy.splitHostLines(passthroughText)
            settings.passthroughHostsDraft = nil
        } else {
            settings.passthroughHosts = []
            // Preserve whatever is currently in the editor so the user can
            // flip the toggle back ON later and see their previous list.
            let draft = PassthroughPolicy.splitHostLines(passthroughText)
            settings.passthroughHostsDraft = draft.isEmpty ? nil : draft
        }
        settings.enabledMCPServers = restrictMCPServers
            ? enabledMCPSelection.sorted()
            : nil
        if restrictNetworkAllowlist {
            settings.networkAllowlist = NetworkAllowlistPolicy.splitHostLines(networkAllowlistText)
        } else {
            settings.networkAllowlist = nil
        }

        let store = WorkspaceStore()
        do {
            try store.saveSettings(settings)
            appState.settings = settings
            withAnimation { saved = true }
            Task { @MainActor in
                try? await Task.sleep(for: .seconds(1))
                saved = false
                dismiss()
            }
        } catch {
            appState.lastError = "Failed to save settings: \(error.localizedDescription)"
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
