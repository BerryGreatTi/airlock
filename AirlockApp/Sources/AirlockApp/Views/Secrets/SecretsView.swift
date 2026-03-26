import SwiftUI

private enum SecretStatus: String {
    case encrypted = "Encrypted"
    case plaintext = "Plaintext"
    case notSecret = "Not Secret"

    var color: Color {
        switch self {
        case .encrypted: return .green
        case .plaintext: return .orange
        case .notSecret: return .secondary
        }
    }
}

private struct EnvEntry: Identifiable {
    let id: UUID = UUID()
    var key: String
    var value: String

    var status: SecretStatus {
        if value.hasPrefix("ENC[age:") { return .encrypted }
        let sensitivePatterns = ["KEY", "SECRET", "PASSWORD", "TOKEN", "CREDENTIAL", "API_KEY"]
        if sensitivePatterns.contains(where: { key.uppercased().contains($0) }) { return .plaintext }
        return .notSecret
    }

    var maskedValue: String {
        if status == .encrypted { return "ENC[age:...]" }
        if status == .plaintext { return String(repeating: "*", count: min(value.count, 20)) }
        return value
    }
}

struct SecretsView: View {
    let workspace: Workspace
    @Bindable var appState: AppState
    @State private var entries: [EnvEntry] = []
    @State private var errorMessage: String?
    @State private var showingAddEntry = false
    @State private var newKey = ""
    @State private var newValue = ""

    var body: some View {
        VStack(spacing: 0) {
            if appState.isActive(workspace) {
                restartBanner
            }

            if let envFile = workspace.envFilePath {
                secretsTable
                    .task { loadEnvFile(envFile) }
            } else {
                noEnvFileView
            }
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

    private var secretsTable: some View {
        VStack(spacing: 0) {
            toolbar
            Divider()

            if let error = errorMessage {
                ContentUnavailableView {
                    Label("Error", systemImage: "exclamationmark.triangle")
                } description: { Text(error) }
            } else if entries.isEmpty {
                ContentUnavailableView {
                    Label("No Entries", systemImage: "key")
                } description: { Text("No entries found in .env file") }
            } else {
                Table(entries) {
                    TableColumn("Name") { entry in
                        Text(entry.key).fontDesign(.monospaced)
                    }
                    .width(min: 120, ideal: 200)

                    TableColumn("Status") { entry in
                        HStack(spacing: 4) {
                            Circle()
                                .fill(entry.status.color)
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

    private var toolbar: some View {
        HStack(spacing: 8) {
            Button {
                showingAddEntry = true
            } label: {
                Label("Add Entry", systemImage: "plus")
            }

            Button {
                encryptAll()
            } label: {
                Label("Encrypt All", systemImage: "lock")
            }
            .disabled(entries.filter { $0.status == .plaintext }.isEmpty)

            Spacer()
        }
        .padding(.horizontal, 8)
        .padding(.vertical, 4)
        .background(Color(nsColor: .controlBackgroundColor))
        .sheet(isPresented: $showingAddEntry) {
            addEntrySheet
        }
    }

    private var summaryBar: some View {
        HStack {
            let encrypted = entries.filter { $0.status == .encrypted }.count
            let plaintext = entries.filter { $0.status == .plaintext }.count
            Text("\(entries.count) entries")
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

    private var addEntrySheet: some View {
        VStack(alignment: .leading, spacing: 12) {
            Text("Add Entry").font(.title3).fontWeight(.semibold)
            TextField("Key", text: $newKey).textFieldStyle(.roundedBorder)
            TextField("Value", text: $newValue).textFieldStyle(.roundedBorder)
            HStack {
                Spacer()
                Button("Cancel") { showingAddEntry = false }
                    .keyboardShortcut(.cancelAction)
                Button("Add") {
                    entries.append(EnvEntry(key: newKey, value: newValue))
                    saveEntries()
                    newKey = ""
                    newValue = ""
                    showingAddEntry = false
                }
                .keyboardShortcut(.defaultAction)
                .disabled(newKey.isEmpty)
            }
        }
        .padding()
        .frame(width: 400)
    }

    private var noEnvFileView: some View {
        ContentUnavailableView {
            Label("No .env File", systemImage: "doc.text")
        } description: {
            Text("Configure a .env file in workspace settings to manage secrets")
        }
    }

    private func loadEnvFile(_ path: String) {
        do {
            let content = try String(contentsOfFile: path, encoding: .utf8)
            entries = content
                .components(separatedBy: .newlines)
                .compactMap { line -> EnvEntry? in
                    let trimmed = line.trimmingCharacters(in: .whitespaces)
                    guard !trimmed.isEmpty, !trimmed.hasPrefix("#") else { return nil }
                    let parts = trimmed.split(separator: "=", maxSplits: 1)
                    guard parts.count == 2 else { return nil }
                    return EnvEntry(
                        key: String(parts[0]).trimmingCharacters(in: .whitespaces),
                        value: String(parts[1]).trimmingCharacters(in: .whitespaces)
                    )
                }
        } catch {
            errorMessage = error.localizedDescription
        }
    }

    private func encryptAll() {
        Task {
            let cli = CLIService()
            _ = try? await cli.run(args: ["encrypt"], workingDirectory: workspace.path)
            if let envFile = workspace.envFilePath {
                loadEnvFile(envFile)
            }
        }
    }

    private func saveEntries() {
        guard let envFile = workspace.envFilePath else { return }
        let content = entries.map { "\($0.key)=\($0.value)" }.joined(separator: "\n")
        try? content.write(toFile: envFile, atomically: true, encoding: .utf8)
    }
}
