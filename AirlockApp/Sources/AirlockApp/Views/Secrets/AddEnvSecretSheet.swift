import SwiftUI

@MainActor
struct AddEnvSecretSheet: View {
    let workspace: Workspace
    let onComplete: () -> Void
    @Environment(\.dismiss) private var dismiss
    @State private var name = ""
    @State private var value = ""
    @State private var isAdding = false
    @State private var errorMessage: String?

    private var nameIsValid: Bool {
        EnvSecret.isValidName(name)
    }

    private var canAdd: Bool {
        nameIsValid && !value.isEmpty && !isAdding
    }

    var body: some View {
        VStack(alignment: .leading, spacing: 12) {
            Text("Add Environment Secret").font(.title3).fontWeight(.semibold)

            Form {
                TextField("NAME", text: $name)
                    .textFieldStyle(.roundedBorder)
                    .font(.system(.body, design: .monospaced))
                if !name.isEmpty && !nameIsValid {
                    Text("Must match ^[A-Za-z_][A-Za-z0-9_]*$")
                        .font(.caption)
                        .foregroundStyle(.red)
                }

                SecureField("Value", text: $value)
                    .textFieldStyle(.roundedBorder)
            }

            Text("Stored encrypted in .airlock/config.yaml. Restart the workspace to apply.")
                .font(.caption)
                .foregroundStyle(.secondary)

            if let error = errorMessage {
                Text(error)
                    .foregroundStyle(.red)
                    .font(.caption)
                    .fixedSize(horizontal: false, vertical: true)
            }

            HStack {
                Spacer()
                Button("Cancel") { dismiss() }
                    .keyboardShortcut(.cancelAction)
                Button("Add") {
                    Task { await add() }
                }
                .keyboardShortcut(.defaultAction)
                .disabled(!canAdd)
            }
        }
        .padding()
        .frame(width: 460)
    }

    private func add() async {
        isAdding = true
        let submittedName = name
        let submittedValue = value
        defer {
            isAdding = false
            // Drop the value reference after the call returns.
            value = ""
        }
        let cli = CLIService()
        let result = try? await cli.run(
            args: ["secret", "env", "add", submittedName, "--value", submittedValue],
            workingDirectory: workspace.path
        )
        if let result, result.exitCode != 0 {
            errorMessage = result.stderr.isEmpty ? "Failed to add env secret" : result.stderr
            return
        }
        onComplete()
        dismiss()
    }
}
