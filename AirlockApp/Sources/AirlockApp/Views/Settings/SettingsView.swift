import SwiftUI

struct SettingsView: View {
    @Bindable var appState: AppState
    @State private var settings = AppSettings()
    @State private var passthroughText = ""
    @State private var saved = false

    var body: some View {
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

            Section("Network") {
                Text("Passthrough hosts (MITM excluded, one per line)")
                    .font(.caption)
                    .foregroundStyle(.secondary)
                TextEditor(text: $passthroughText)
                    .font(.system(size: 12, design: .monospaced))
                    .frame(height: 80)
            }

            if let workspace = appState.selectedWorkspace {
                Section("Workspace: \(workspace.name)") {
                    HStack {
                        TextField(".env file", text: Binding(
                            get: { workspace.envFilePath ?? "" },
                            set: { updateWorkspaceField(workspace) { $0.envFilePath = $1.isEmpty ? nil : $1 }($0) }
                        ))
                        Button("Browse...") { pickEnvFile(for: workspace) }
                    }
                    TextField("Container image override", text: Binding(
                        get: { workspace.containerImageOverride ?? "" },
                        set: { updateWorkspaceField(workspace) { $0.containerImageOverride = $1.isEmpty ? nil : $1 }($0) }
                    ))
                    .textFieldStyle(.roundedBorder)
                }
            }

            HStack {
                Spacer()
                if saved {
                    Text("Saved")
                        .foregroundStyle(.green)
                        .transition(.opacity)
                }
                Button("Save") { save() }
                    .keyboardShortcut(.defaultAction)
            }
        }
        .formStyle(.grouped)
        .padding()
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
        try? store.saveWorkspaces(appState.workspaces)

        withAnimation { saved = true }
        DispatchQueue.main.asyncAfter(deadline: .now() + 2) {
            withAnimation { saved = false }
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

    private func pickEnvFile(for workspace: Workspace) {
        let panel = NSOpenPanel()
        panel.canChooseFiles = true
        panel.canChooseDirectories = false
        if panel.runModal() == .OK, let url = panel.url {
            updateWorkspaceField(workspace) { $0.envFilePath = $1 }(url.path)
        }
    }

    private func updateWorkspaceField(_ workspace: Workspace, _ update: @escaping (inout Workspace, String) -> Void) -> (String) -> Void {
        return { value in
            if let idx = appState.workspaces.firstIndex(where: { $0.id == workspace.id }) {
                update(&appState.workspaces[idx], value)
            }
        }
    }
}
