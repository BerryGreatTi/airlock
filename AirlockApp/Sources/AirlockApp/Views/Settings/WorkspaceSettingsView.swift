import SwiftUI

@MainActor
struct WorkspaceSettingsView: View {
    let workspace: Workspace
    @Bindable var appState: AppState
    @State private var globalSettings = AppSettings()
    @State private var passthroughText = ""
    @State private var showRemoveAnthropicConfirm = false
    @State private var pendingMissingHosts: [String] = []

    var body: some View {
        Form {
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

            Section("Network Overrides") {
                let defaultHint = globalSettings.passthroughHosts.isEmpty
                    ? "No default passthrough hosts"
                    : "Default: \(globalSettings.passthroughHosts.joined(separator: ", "))"
                Text("Passthrough hosts override (\(defaultHint))")
                    .font(.caption)
                    .foregroundStyle(.secondary)
                TextEditor(text: $passthroughText)
                    .font(.system(size: 12, design: .monospaced))
                    .frame(height: 80)

                let parsedNonEmpty = parsedHostLines()
                if !parsedNonEmpty.isEmpty {
                    let missing = PassthroughPolicy.missingProtectedHosts(from: parsedNonEmpty)
                    if !missing.isEmpty {
                        HStack(alignment: .top, spacing: 6) {
                            Image(systemName: "exclamationmark.triangle.fill")
                                .foregroundStyle(.yellow)
                            Text("This override would remove \(missing.joined(separator: ", ")) from passthrough. Airlock would decrypt secrets in requests to Anthropic, sending your plaintext credentials to Anthropic's servers.")
                                .font(.caption)
                                .foregroundStyle(.yellow)
                                .fixedSize(horizontal: false, vertical: true)
                        }
                        .padding(8)
                        .background(Color.yellow.opacity(0.08))
                        .clipShape(RoundedRectangle(cornerRadius: 4))
                    }
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
                commitSave(hosts: parsedHostLines())
            }
        } message: {
            Text("\(pendingMissingHosts.joined(separator: ", ")) will not be in this workspace's passthrough list. Airlock will decrypt secrets in requests to Anthropic, sending your plaintext credentials to Anthropic's servers. Continue?")
        }
    }

    private func parsedHostLines() -> [String] {
        passthroughText
            .components(separatedBy: "\n")
            .map { $0.trimmingCharacters(in: .whitespaces) }
            .filter { !$0.isEmpty }
    }

    private func load() {
        globalSettings = (try? WorkspaceStore().loadSettings()) ?? AppSettings()
        passthroughText = workspace.passthroughHostsOverride?.joined(separator: "\n") ?? ""
    }

    private func save() {
        let hosts = parsedHostLines()
        // Empty override = inherit global; not flagged.
        if !hosts.isEmpty {
            let missing = PassthroughPolicy.missingProtectedHosts(from: hosts)
            if !missing.isEmpty {
                pendingMissingHosts = missing
                showRemoveAnthropicConfirm = true
                return
            }
        }
        commitSave(hosts: hosts)
    }

    private func commitSave(hosts: [String]) {
        if let idx = appState.workspaces.firstIndex(where: { $0.id == workspace.id }) {
            appState.workspaces[idx].passthroughHostsOverride = hosts.isEmpty ? nil : hosts
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
