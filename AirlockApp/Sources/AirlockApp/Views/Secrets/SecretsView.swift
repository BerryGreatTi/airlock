import SwiftUI

@MainActor
struct SecretsView: View {
    let workspace: Workspace
    @Bindable var appState: AppState
    @State private var secretFiles: [SecretFile] = []
    @State private var selectedFileID: UUID?
    @State private var entries: [SecretEntry] = []
    @State private var settingsEntries: [SecretEntry] = []
    @State private var selectedEntryIDs: Set<UUID> = []
    @State private var isProcessing = false
    @State private var errorMessage: String?
    @State private var showingAddFile = false

    private var displayedEntries: [SecretEntry] {
        if selectedFileID == nil {
            return entries + settingsEntries
        }
        if selectedFileID == settingsFileID {
            return settingsEntries
        }
        return entries
    }

    // Stable UUID for the Claude Settings pseudo-file entry.
    // Must not change across view re-creations so selection state is preserved.
    private let settingsFileID = UUID(uuidString: "00000000-0000-0000-0000-000000000001")!

    var body: some View {
        VStack(spacing: 0) {
            if appState.isActive(workspace) {
                restartBanner
            }
            HSplitView {
                fileListPanel
                    .frame(minWidth: 160, idealWidth: 180, maxWidth: 220)
                entriesPanel
            }
        }
        .task {
            await loadSecretFiles()
            loadSettingsSecrets()
        }
        .sheet(isPresented: $showingAddFile) {
            AddSecretFileSheet(workspace: workspace) {
                Task { await loadSecretFiles() }
            }
        }
        .alert("File contains encrypted values",
               isPresented: $showRemoveWarning,
               presenting: fileToRemove) { file in
            Button("Decrypt & Remove") {
                Task {
                    let cli = CLIService()
                    _ = try? await cli.run(
                        args: ["secret", "decrypt", file.path, "--all"],
                        workingDirectory: workspace.path
                    )
                    await removeFile(file)
                }
            }
            Button("Remove Anyway", role: .destructive) {
                Task { await removeFile(file) }
            }
            Button("Cancel", role: .cancel) {}
        } message: { file in
            Text("'\(file.label)' has encrypted values. Removing without decrypting means values stay as ENC[age:...] ciphertext. Decrypt first?")
        }
    }

    private var restartBanner: some View {
        HStack {
            Image(systemName: "exclamationmark.triangle.fill")
                .foregroundStyle(.yellow)
            Text("Restart workspace to apply changes")
                .font(.caption)
            Spacer()
        }
        .padding(8)
        .background(.yellow.opacity(0.1))
    }

    // MARK: - File List Panel

    private var fileListPanel: some View {
        VStack(spacing: 0) {
            List(selection: $selectedFileID) {
                Section("Files") {
                    ForEach(secretFiles) { file in
                        HStack {
                            Image(systemName: file.format.iconName)
                                .foregroundStyle(.secondary)
                            Text(file.label)
                                .lineLimit(1)
                        }
                        .tag(file.id)
                        .contextMenu {
                            Button("Remove", role: .destructive) {
                                confirmRemoveFile(file)
                            }
                        }
                    }
                }
                Section("Claude Settings") {
                    Label("settings.json", systemImage: "gearshape.2")
                        .tag(settingsFileID)
                }
            }
            .listStyle(.sidebar)
            .onChange(of: selectedFileID) { _, newValue in
                Task { await loadEntriesForSelection(newValue) }
            }

            Divider()
            HStack {
                Button { showingAddFile = true } label: {
                    Label("Add File", systemImage: "plus")
                }
                .buttonStyle(.borderless)
                Spacer()
            }
            .padding(8)
        }
    }

    // MARK: - Entries Panel

    private var entriesPanel: some View {
        VStack(spacing: 0) {
            entriesToolbar
            Divider()

            if let error = errorMessage {
                ContentUnavailableView {
                    Label("Error", systemImage: "exclamationmark.triangle")
                } description: { Text(error) }
                .frame(maxHeight: .infinity)
            } else if displayedEntries.isEmpty {
                ContentUnavailableView {
                    Label("No Secrets", systemImage: "key")
                } description: { Text("Select a file or add one to get started") }
                .frame(maxHeight: .infinity)
            } else {
                Table(displayedEntries, selection: $selectedEntryIDs) {
                    TableColumn("Name") { entry in
                        Text(entry.path).fontDesign(.monospaced)
                    }
                    .width(min: 120, ideal: 200)

                    TableColumn("Status") { entry in
                        HStack(spacing: 4) {
                            Circle()
                                .fill(colorForStatus(entry.status))
                                .frame(width: 6, height: 6)
                            Text(entry.status.rawValue)
                                .font(.caption)
                        }
                    }
                    .width(min: 80, ideal: 100)

                    TableColumn("Value") { entry in
                        Text(entry.maskedValue)
                            .fontDesign(.monospaced)
                            .foregroundStyle(.secondary)
                    }
                    .width(min: 200, ideal: 300)
                }
            }

            Divider()
            summaryBar
        }
    }

    private var entriesToolbar: some View {
        HStack(spacing: 8) {
            Button {
                Task { await encryptSelected() }
            } label: {
                Label("Encrypt Selected", systemImage: "lock")
            }
            .disabled(selectedPlaintextEntries.isEmpty || isProcessing)

            Button {
                Task { await decryptSelected() }
            } label: {
                Label("Decrypt Selected", systemImage: "lock.open")
            }
            .disabled(selectedEncryptedEntries.isEmpty || isProcessing)

            if isProcessing {
                ProgressView()
                    .controlSize(.small)
            }

            Spacer()
        }
        .padding(.horizontal, 8)
        .padding(.vertical, 4)
        .background(Color(nsColor: .controlBackgroundColor))
    }

    private var summaryBar: some View {
        HStack {
            let encrypted = displayedEntries.filter { $0.status == .encrypted }.count
            let plaintext = displayedEntries.filter { $0.status == .plaintext }.count
            Text("\(displayedEntries.count) entries")
            Text("\(encrypted) encrypted")
                .foregroundStyle(.green)
            if plaintext > 0 {
                Text("\(plaintext) plaintext")
                    .foregroundStyle(.orange)
                    .fontWeight(.semibold)
            }
            Spacer()
        }
        .font(.caption)
        .padding(.horizontal, 8)
        .padding(.vertical, 4)
        .background(Color(nsColor: .controlBackgroundColor))
    }

    // MARK: - Helpers

    private var selectedPlaintextEntries: [SecretEntry] {
        displayedEntries.filter { selectedEntryIDs.contains($0.id) && $0.status == .plaintext }
    }

    private var selectedEncryptedEntries: [SecretEntry] {
        displayedEntries.filter { selectedEntryIDs.contains($0.id) && $0.status == .encrypted }
    }

    private func colorForStatus(_ status: SecretStatus) -> Color {
        switch status {
        case .encrypted: return .green
        case .plaintext: return .orange
        case .notSecret: return .secondary
        }
    }

    // MARK: - CLI Integration

    private func loadSecretFiles() async {
        let cli = CLIService()
        guard let result = try? await cli.run(args: ["secret", "list", "--json"], workingDirectory: workspace.path) else {
            return
        }
        guard result.exitCode == 0, let data = result.stdout.data(using: .utf8) else { return }

        struct FileInfo: Decodable {
            let path: String
            let format: String
        }

        guard let files = try? JSONDecoder().decode([FileInfo].self, from: data) else { return }
        secretFiles = files.map { SecretFile(path: $0.path, formatString: $0.format) }
    }

    private func loadEntriesForSelection(_ fileID: UUID?) async {
        errorMessage = nil
        selectedEntryIDs = []

        guard let fileID, fileID != settingsFileID else {
            entries = []
            return
        }
        guard let file = secretFiles.first(where: { $0.id == fileID }) else {
            entries = []
            return
        }

        let cli = CLIService()
        guard let result = try? await cli.run(
            args: ["secret", "show", file.path, "--json"],
            workingDirectory: workspace.path
        ) else {
            errorMessage = "Failed to run airlock CLI"
            return
        }

        if result.exitCode != 0 {
            errorMessage = result.stderr.isEmpty ? "Failed to parse file" : result.stderr
            return
        }

        struct ShowOutput: Decodable {
            let format: String
            let entries: [ShowEntry]
        }
        struct ShowEntry: Decodable {
            let path: String
            let value: String
            let encrypted: Bool
            let is_secret: Bool
        }

        guard let data = result.stdout.data(using: .utf8),
              let output = try? JSONDecoder().decode(ShowOutput.self, from: data) else {
            errorMessage = "Failed to parse CLI output"
            return
        }

        entries = output.entries.map { e in
            SecretEntry(
                path: e.path,
                value: e.value,
                encrypted: e.encrypted,
                isSecret: e.is_secret,
                source: file.label,
                isEditable: true
            )
        }
    }

    private func encryptSelected() async {
        guard let file = selectedFile else { return }
        let keys = selectedPlaintextEntries.map(\.path).joined(separator: ",")
        guard !keys.isEmpty else { return }

        isProcessing = true
        defer { isProcessing = false }

        let cli = CLIService()
        let result = try? await cli.run(
            args: ["secret", "encrypt", file.path, "--keys", keys],
            workingDirectory: workspace.path
        )
        if let result, result.exitCode != 0 {
            errorMessage = result.stderr.isEmpty ? "Encryption failed" : result.stderr
            return
        }
        await loadEntriesForSelection(selectedFileID)
    }

    private func decryptSelected() async {
        guard let file = selectedFile else { return }
        let keys = selectedEncryptedEntries.map(\.path).joined(separator: ",")
        guard !keys.isEmpty else { return }

        isProcessing = true
        defer { isProcessing = false }

        let cli = CLIService()
        let result = try? await cli.run(
            args: ["secret", "decrypt", file.path, "--keys", keys],
            workingDirectory: workspace.path
        )
        if let result, result.exitCode != 0 {
            errorMessage = result.stderr.isEmpty ? "Decryption failed" : result.stderr
            return
        }
        await loadEntriesForSelection(selectedFileID)
    }

    @State private var fileToRemove: SecretFile?
    @State private var showRemoveWarning = false

    private func confirmRemoveFile(_ file: SecretFile) {
        Task {
            let hasEncrypted = await checkFileHasEncrypted(file)
            if hasEncrypted {
                fileToRemove = file
                showRemoveWarning = true
            } else {
                await removeFile(file)
            }
        }
    }

    private func checkFileHasEncrypted(_ file: SecretFile) async -> Bool {
        let cli = CLIService()
        guard let result = try? await cli.run(
            args: ["secret", "show", file.path, "--json"],
            workingDirectory: workspace.path
        ), result.exitCode == 0 else { return false }

        struct QuickCheck: Decodable {
            let entries: [QuickEntry]
        }
        struct QuickEntry: Decodable {
            let encrypted: Bool
        }
        guard let data = result.stdout.data(using: .utf8),
              let output = try? JSONDecoder().decode(QuickCheck.self, from: data) else { return false }
        return output.entries.contains { $0.encrypted }
    }

    private func removeFile(_ file: SecretFile) async {
        let cli = CLIService()
        _ = try? await cli.run(
            args: ["secret", "remove", file.path],
            workingDirectory: workspace.path
        )
        await loadSecretFiles()
        if selectedFileID == file.id {
            selectedFileID = nil
            entries = []
        }
    }

    private var selectedFile: SecretFile? {
        guard let id = selectedFileID else { return nil }
        return secretFiles.first { $0.id == id }
    }

    private static let secretKeywords = ["TOKEN", "KEY", "SECRET", "PASSWORD", "CREDENTIAL", "AUTH"]

    private static func looksLikeSecret(_ key: String) -> Bool {
        let upper = key.uppercased()
        return secretKeywords.contains { upper.contains($0) }
    }

    private func loadSettingsSecrets() {
        let settingsFiles: [(path: String, label: String)] = [
            (workspace.path + "/.claude/settings.json", "project"),
            (workspace.path + "/.claude/settings.local.json", "project"),
        ]
        let home = FileManager.default.homeDirectoryForCurrentUser.path
        let globalFiles: [(path: String, label: String)] = [
            (home + "/.claude/settings.json", "global"),
            (home + "/.claude/settings.local.json", "global"),
        ]

        var results: [SecretEntry] = []
        for file in settingsFiles + globalFiles {
            guard let data = try? Data(contentsOf: URL(fileURLWithPath: file.path)),
                  let json = try? JSONSerialization.jsonObject(with: data) as? [String: Any] else {
                continue
            }
            let fileName = (file.path as NSString).lastPathComponent
            let source = "\(file.label) \(fileName)"

            if let envBlock = json["env"] as? [String: String] {
                for (key, value) in envBlock {
                    let enc = value.hasPrefix("ENC[age:")
                    results.append(SecretEntry(
                        path: key, value: value,
                        encrypted: enc, isSecret: enc || Self.looksLikeSecret(key),
                        source: source, isEditable: false
                    ))
                }
            }
            if let mcpServers = json["mcpServers"] as? [String: Any] {
                for (serverName, serverVal) in mcpServers {
                    if let server = serverVal as? [String: Any],
                       let envBlock = server["env"] as? [String: String] {
                        for (key, value) in envBlock {
                            let enc = value.hasPrefix("ENC[age:")
                            results.append(SecretEntry(
                                path: key, value: value,
                                encrypted: enc, isSecret: enc || Self.looksLikeSecret(key),
                                source: "mcp:\(serverName)", isEditable: false
                            ))
                        }
                    }
                }
            }
        }
        settingsEntries = results
    }
}
