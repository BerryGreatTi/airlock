import SwiftUI

/// Reusable picker for the MCP server allow-list. Used in both global
/// settings (`SettingsView`) and per-workspace overrides
/// (`WorkspaceSettingsView`). The two call sites differ only in label
/// strings, so the picker takes them as parameters and the surrounding
/// `Section` stays at the call site. Parents that need to react to the
/// enable toggle (e.g., to seed the selection set) attach `.onChange(of:)`
/// to the same binding they pass in.
struct MCPAllowListPicker: View {
    @Binding var enabled: Bool
    @Binding var selection: Set<String>
    let discovered: [String]
    let toggleLabel: String
    let restrictedCaption: String
    let unrestrictedCaption: String
    let emptyInventoryCaption: String
    let noneSelectedWarning: String

    var body: some View {
        Toggle(toggleLabel, isOn: $enabled)
        if enabled {
            if discovered.isEmpty {
                Text(emptyInventoryCaption)
                    .font(.caption)
                    .foregroundStyle(.secondary)
            } else {
                Text(restrictedCaption)
                    .font(.caption)
                    .foregroundStyle(.secondary)
                ForEach(discovered, id: \.self) { name in
                    Toggle(name, isOn: Binding(
                        get: { selection.contains(name) },
                        set: { isOn in
                            if isOn { selection.insert(name) }
                            else { selection.remove(name) }
                        }
                    ))
                    .toggleStyle(.checkbox)
                }
                if selection.isEmpty {
                    Text(noneSelectedWarning)
                        .font(.caption)
                        .foregroundStyle(.orange)
                }
            }
        } else {
            Text(unrestrictedCaption)
                .font(.caption)
                .foregroundStyle(.secondary)
        }
    }
}
