import SwiftUI

struct NewWorkspaceSheet: View {
    @Bindable var appState: AppState
    @Environment(\.dismiss) private var dismiss
    @State private var selectedPath: String = ""
    @State private var envFilePath: String = ""
    @State private var statusMessage: String = ""
    @State private var isProcessing = false

    var body: some View {
        VStack(alignment: .leading, spacing: 16) {
            Text("New Workspace")
                .font(.title2)
                .fontWeight(.semibold)

            HStack {
                TextField("Project directory", text: $selectedPath)
                    .textFieldStyle(.roundedBorder)
                    .disabled(true)
                Button("Browse...") { pickDirectory() }
            }

            HStack {
                TextField(".env file (optional)", text: $envFilePath)
                    .textFieldStyle(.roundedBorder)
                    .disabled(true)
                Button("Browse...") { pickEnvFile() }
                if !envFilePath.isEmpty {
                    Button { envFilePath = "" } label: {
                        Image(systemName: "xmark.circle.fill")
                            .foregroundStyle(.secondary)
                    }
                    .buttonStyle(.plain)
                }
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
                    .disabled(selectedPath.isEmpty || isProcessing)
            }
        }
        .padding()
        .frame(width: 500)
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
