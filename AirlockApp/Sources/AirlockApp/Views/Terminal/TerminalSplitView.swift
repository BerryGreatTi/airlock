import SwiftUI

struct TerminalPane: Identifiable {
    let id: UUID = UUID()
}

struct TerminalSplitView: View {
    let containerName: String
    let workDir: String
    let terminalSettings: TerminalSettings
    let terminalColors: TerminalColors
    @Binding var action: TerminalAction?
    @State private var panes: [TerminalPane] = [TerminalPane()]
    @State private var splitVertical = true

    private let maxPanes = 4

    var body: some View {
        VStack(spacing: 0) {
            toolbar
            Divider()
            terminalGrid
        }
        .onChange(of: action) { _, newAction in
            guard let newAction else { return }
            handleAction(newAction)
            action = nil
        }
    }

    private var toolbar: some View {
        HStack(spacing: 8) {
            Button {
                addPane()
            } label: {
                Label("New Terminal", systemImage: "plus.rectangle")
            }
            .disabled(panes.count >= maxPanes)

            Button {
                if addPane() { splitVertical = true }
            } label: {
                Label("Split Vertical", systemImage: "rectangle.split.1x2")
            }
            .disabled(panes.count >= maxPanes)

            Button {
                if addPane() { splitVertical = false }
            } label: {
                Label("Split Horizontal", systemImage: "rectangle.split.2x1")
            }
            .disabled(panes.count >= maxPanes)

            Divider().frame(height: 16)

            ForEach(Array(panes.enumerated()), id: \.element.id) { index, pane in
                HStack(spacing: 2) {
                    Text("T\(index + 1)")
                        .font(.caption)
                        .fontDesign(.monospaced)
                    if panes.count > 1 {
                        Button {
                            removePane(id: pane.id)
                        } label: {
                            Image(systemName: "xmark.circle.fill")
                                .font(.caption2)
                                .foregroundStyle(.secondary)
                        }
                        .buttonStyle(.plain)
                    }
                }
                .padding(.horizontal, 4)
                .padding(.vertical, 2)
                .background(Color(nsColor: .controlBackgroundColor).opacity(0.5))
                .clipShape(RoundedRectangle(cornerRadius: 4))
            }

            Spacer()

            Text("\(panes.count) terminal\(panes.count == 1 ? "" : "s")")
                .font(.caption)
                .foregroundStyle(.secondary)
        }
        .padding(.horizontal, 8)
        .padding(.vertical, 4)
        .background(Color(nsColor: .controlBackgroundColor))
    }

    private var terminalGrid: some View {
        NSSplitViewRepresentable(
            paneIDs: panes.map(\.id),
            isVertical: splitVertical,
            containerName: containerName,
            workDir: workDir,
            terminalSettings: terminalSettings,
            terminalColors: terminalColors,
            onPaneTerminated: { id in
                removePane(id: id)
            }
        )
    }

    private func handleAction(_ terminalAction: TerminalAction) {
        switch terminalAction {
        case .addPane:
            addPane()
        case .splitVertical:
            if addPane() { splitVertical = true }
        case .splitHorizontal:
            if addPane() { splitVertical = false }
        }
    }

    @discardableResult
    private func addPane() -> Bool {
        guard panes.count < maxPanes else { return false }
        panes.append(TerminalPane())
        return true
    }

    private func removePane(id: UUID) {
        guard panes.count > 1 else { return }
        panes.removeAll { $0.id == id }
    }
}
