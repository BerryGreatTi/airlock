import SwiftUI

/// Reusable host-list editor with an Anthropic-missing guardrail warning.
///
/// Used in four places across `SettingsView` and `WorkspaceSettingsView`:
/// passthrough hosts (global + workspace) and network allow-list (global +
/// workspace). All four share the same structure — caption, monospaced
/// `TextEditor`, inline yellow warning when protected hosts are missing —
/// while differing in copy and which policy computes the missing set.
///
/// The caller supplies `missingHosts` (computed from whichever policy is
/// appropriate) so the editor itself stays policy-agnostic. Mirrors the
/// `MCPAllowListPicker` extraction pattern established in PR1.
struct HostListEditor: View {
    let caption: String
    @Binding var text: String
    let missingHosts: [String]
    let warningText: (String) -> String

    var body: some View {
        Text(caption)
            .font(.caption)
            .foregroundStyle(.secondary)
        TextEditor(text: $text)
            .font(.system(size: 12, design: .monospaced))
            .frame(height: 80)
        if !missingHosts.isEmpty {
            HStack(alignment: .top, spacing: 6) {
                Image(systemName: "exclamationmark.triangle.fill")
                    .foregroundStyle(.yellow)
                Text(warningText(missingHosts.joined(separator: ", ")))
                    .font(.caption)
                    .foregroundStyle(.yellow)
                    .fixedSize(horizontal: false, vertical: true)
            }
            .padding(8)
            .background(Color.yellow.opacity(0.08))
            .clipShape(RoundedRectangle(cornerRadius: 4))
        }
    }
}
