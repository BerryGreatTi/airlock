import SwiftUI

@MainActor
struct GlobalSettingsSheet: View {
    @Bindable var appState: AppState
    @Environment(\.dismiss) private var dismiss
    @State private var settings = AppSettings()
    @State private var passthroughText = ""
    @State private var saved = false
    @State private var volumeStatus = "Checking..."
    @State private var showImportSheet = false
    @State private var showResetAlert = false

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

                Section("Network Defaults") {
                    Text("Default passthrough hosts (skip proxy decryption, one per line)")
                        .font(.caption)
                        .foregroundStyle(.secondary)
                    TextEditor(text: $passthroughText)
                        .font(.system(size: 12, design: .monospaced))
                        .frame(height: 80)
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
        passthroughText = settings.passthroughHosts.joined(separator: "\n")
    }

    private func save() {
        settings.passthroughHosts = passthroughText
            .components(separatedBy: "\n")
            .map { $0.trimmingCharacters(in: .whitespaces) }
            .filter { !$0.isEmpty }

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
