import SwiftUI

@MainActor
struct ImportConfigSheet: View {
    @Environment(\.dismiss) private var dismiss
    @State private var selectedItems: Set<String> = ["CLAUDE.md", "rules", "settings.json", "settings.local.json"]
    @State private var forceOverwrite = false
    @State private var isImporting = false
    @State private var result: String?

    private let allItems: [(name: String, description: String, isDefault: Bool)] = [
        ("CLAUDE.md", "Global instructions", true),
        ("rules", "Custom rules directory", true),
        ("settings.json", "Claude Code settings", true),
        ("settings.local.json", "Local settings overrides", true),
        ("plugins", "Installed plugins", false),
        ("skills", "Custom skills", false),
        ("history.jsonl", "Command history", false),
        ("projects", "Project-specific memory", false),
    ]

    var body: some View {
        VStack(spacing: 0) {
            Form {
                Section("Select items to import from ~/.claude") {
                    ForEach(allItems, id: \.name) { item in
                        Toggle(isOn: Binding(
                            get: { selectedItems.contains(item.name) },
                            set: { if $0 { selectedItems.insert(item.name) } else { selectedItems.remove(item.name) } }
                        )) {
                            VStack(alignment: .leading) {
                                Text(item.name)
                                    .font(.system(.body, design: .monospaced))
                                Text(item.description)
                                    .font(.caption)
                                    .foregroundStyle(.secondary)
                            }
                        }
                    }
                }

                Section {
                    Toggle("Force overwrite existing files", isOn: $forceOverwrite)
                }
            }
            .formStyle(.grouped)

            if let result {
                ScrollView {
                    Text(result)
                        .font(.system(size: 11, design: .monospaced))
                        .frame(maxWidth: .infinity, alignment: .leading)
                        .padding(.horizontal)
                }
                .frame(height: 80)
            }

            HStack {
                Spacer()
                Button("Cancel") { dismiss() }
                    .keyboardShortcut(.cancelAction)
                Button("Import") { performImport() }
                    .keyboardShortcut(.defaultAction)
                    .disabled(isImporting || selectedItems.isEmpty)
            }
            .padding()
        }
        .frame(width: 500, height: 520)
    }

    private func performImport() {
        isImporting = true
        result = nil
        Task {
            let cli = CLIService()
            var args = ["config", "import", "--items", selectedItems.sorted().joined(separator: ",")]
            if forceOverwrite { args.append("--force") }
            let home = FileManager.default.homeDirectoryForCurrentUser.path
            if let output = try? await cli.run(args: args, workingDirectory: home) {
                result = output.stdout.isEmpty ? output.stderr : output.stdout
            } else {
                result = "Import failed"
            }
            isImporting = false
        }
    }
}
