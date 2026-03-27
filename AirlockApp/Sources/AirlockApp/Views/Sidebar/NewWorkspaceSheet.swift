import SwiftUI

private struct PreCheck: Identifiable {
    let id: String
    let label: String
    var passed: Bool?
    var detail: String?
}

struct NewWorkspaceSheet: View {
    @Bindable var appState: AppState
    @Environment(\.dismiss) private var dismiss
    @Environment(\.containerService) private var containerService
    @State private var selectedPath: String = ""
    @State private var envFilePath: String = ""
    @State private var statusMessage: String = ""
    @State private var isProcessing = false
    @State private var checks: [PreCheck] = []

    var body: some View {
        VStack(alignment: .leading, spacing: 16) {
            Text("New Workspace")
                .font(.title2)
                .fontWeight(.semibold)

            HStack {
                TextField("Project directory", text: $selectedPath)
                    .textFieldStyle(.roundedBorder)
                Button("Browse...") { pickDirectory() }
            }

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

            if !checks.isEmpty {
                preCheckList
            }

            if !statusMessage.isEmpty {
                Text(statusMessage)
                    .font(.caption)
                    .foregroundStyle(statusMessage.contains("Error") ? .red : .secondary)
            }

            HStack {
                Spacer()
                Button("Cancel") { dismiss() }
                    .keyboardShortcut(.cancelAction)
                Button("Add Workspace") { addWorkspace() }
                    .keyboardShortcut(.defaultAction)
                    .disabled(selectedPath.isEmpty || isProcessing || !directoryExists)
            }
        }
        .padding()
        .frame(width: 500)
        .onChange(of: selectedPath) { _, _ in Task { await runPreChecks() } }
        .onChange(of: envFilePath) { _, _ in Task { await runPreChecks() } }
    }

    private var preCheckList: some View {
        VStack(alignment: .leading, spacing: 6) {
            ForEach(checks) { check in
                HStack(spacing: 6) {
                    if let passed = check.passed {
                        Image(systemName: passed ? "checkmark.circle.fill" : "xmark.circle.fill")
                            .foregroundStyle(passed ? .green : .orange)
                            .font(.caption)
                    } else {
                        ProgressView()
                            .controlSize(.small)
                    }
                    Text(check.label)
                        .font(.caption)
                    if let detail = check.detail {
                        Text(detail)
                            .font(.caption2)
                            .foregroundStyle(.secondary)
                    }
                }
            }
        }
        .padding(8)
        .background(Color(nsColor: .controlBackgroundColor))
        .clipShape(RoundedRectangle(cornerRadius: 6))
    }

    private var directoryExists: Bool {
        checks.first(where: { $0.id == "dir" })?.passed ?? false
    }

    private func runPreChecks() async {
        guard !selectedPath.isEmpty else {
            checks = []
            return
        }
        let fm = FileManager.default
        let cli = CLIService()

        var results: [PreCheck] = []

        let dirExists = fm.fileExists(atPath: selectedPath)
        results.append(PreCheck(
            id: "dir",
            label: "Directory exists",
            passed: dirExists
        ))

        let initialized = cli.isAirlockInitialized(path: selectedPath)
        results.append(PreCheck(
            id: "airlock",
            label: ".airlock/ initialized",
            passed: initialized,
            detail: initialized ? nil : "will run airlock init"
        ))

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

        // Show synchronous checks immediately, Docker check updates async
        results.append(PreCheck(
            id: "docker",
            label: "Docker running",
            passed: nil
        ))
        checks = results

        let dockerOK = await containerService.isDockerRunning()
        if let idx = checks.firstIndex(where: { $0.id == "docker" }) {
            checks[idx].passed = dockerOK
            checks[idx].detail = dockerOK ? nil : "start Docker Desktop"
        }
    }

    private func pickDirectory() {
        let panel = NSOpenPanel()
        panel.canChooseDirectories = true
        panel.canChooseFiles = false
        panel.allowsMultipleSelection = false
        if panel.runModal() == .OK, let url = panel.url {
            selectedPath = url.path
        }
    }

    private func pickEnvFile() {
        let panel = NSOpenPanel()
        panel.canChooseDirectories = false
        panel.canChooseFiles = true
        panel.allowsMultipleSelection = false
        if panel.runModal() == .OK, let url = panel.url {
            envFilePath = url.path
        }
    }

    private func addWorkspace() {
        isProcessing = true
        let cli = CLIService()
        let path = selectedPath

        Task {
            if !cli.isAirlockInitialized(path: path) {
                statusMessage = "Running airlock init..."
                let result = try await cli.run(args: ["init"], workingDirectory: path)
                if result.exitCode != 0 {
                    statusMessage = "Error: \(result.stderr)"
                    isProcessing = false
                    return
                }
            } else {
                statusMessage = "Existing airlock workspace detected"
            }

            let name = URL(filePath: path).lastPathComponent
            let workspace = Workspace(
                name: name,
                path: path,
                envFilePath: envFilePath.isEmpty ? nil : envFilePath
            )
            appState.workspaces.append(workspace)
            appState.selectedWorkspaceID = workspace.id
            try? WorkspaceStore().saveWorkspaces(appState.workspaces)

            isProcessing = false
            dismiss()
        }
    }
}
