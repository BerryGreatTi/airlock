import SwiftUI

@MainActor
struct AddSecretFileSheet: View {
    let workspace: Workspace
    let onComplete: () -> Void
    @Environment(\.dismiss) private var dismiss
    @State private var selectedPaths: [String] = []
    @State private var formats: [String: SecretFileFormat] = [:]
    @State private var isAdding = false
    @State private var errorMessage: String?

    var body: some View {
        VStack(alignment: .leading, spacing: 12) {
            Text("Add Secret Files").font(.title3).fontWeight(.semibold)

            if selectedPaths.isEmpty {
                Button("Choose Files...") { pickFiles() }
                    .buttonStyle(.borderedProminent)
            } else {
                ForEach(selectedPaths, id: \.self) { path in
                    HStack {
                        Image(systemName: formatFor(path).iconName)
                            .foregroundStyle(.secondary)
                        Text((path as NSString).lastPathComponent)
                        Spacer()
                        Picker("", selection: bindingFor(path)) {
                            ForEach(SecretFileFormat.allCases, id: \.self) { fmt in
                                Text(fmt.displayName).tag(fmt)
                            }
                        }
                        .frame(width: 120)
                    }
                }
            }

            if let error = errorMessage {
                Text(error)
                    .foregroundStyle(.red)
                    .font(.caption)
            }

            HStack {
                Spacer()
                Button("Cancel") { dismiss() }
                    .keyboardShortcut(.cancelAction)
                Button("Add") {
                    Task { await addFiles() }
                }
                .keyboardShortcut(.defaultAction)
                .disabled(selectedPaths.isEmpty || isAdding)
            }
        }
        .padding()
        .frame(width: 500)
    }

    private func pickFiles() {
        let panel = NSOpenPanel()
        panel.canChooseDirectories = false
        panel.canChooseFiles = true
        panel.allowsMultipleSelection = true
        if panel.runModal() == .OK {
            selectedPaths = panel.urls.map(\.path)
            for path in selectedPaths {
                formats[path] = SecretFileFormat.detect(from: path)
            }
        }
    }

    private func formatFor(_ path: String) -> SecretFileFormat {
        formats[path] ?? SecretFileFormat.detect(from: path)
    }

    private func bindingFor(_ path: String) -> Binding<SecretFileFormat> {
        Binding(
            get: { formatFor(path) },
            set: { formats[path] = $0 }
        )
    }

    private func addFiles() async {
        isAdding = true
        defer { isAdding = false }

        let cli = CLIService()
        for path in selectedPaths {
            let fmt = formatFor(path)
            let result = try? await cli.run(
                args: ["secret", "add", path, "--format", fmt.rawValue],
                workingDirectory: workspace.path
            )
            if let result, result.exitCode != 0 {
                errorMessage = result.stderr.isEmpty ? "Failed to add \(path)" : result.stderr
                return
            }
        }
        onComplete()
        dismiss()
    }
}
