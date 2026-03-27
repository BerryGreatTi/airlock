import SwiftUI

struct TerminalPane: Identifiable {
    let id: UUID = UUID()
}

struct TerminalSplitView: View {
    let containerName: String
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
                splitVertical = true
            } label: {
                Label("Split Vertical", systemImage: "rectangle.split.1x2")
            }
            .disabled(panes.count < 2)

            Button {
                splitVertical = false
            } label: {
                Label("Split Horizontal", systemImage: "rectangle.split.2x1")
            }
            .disabled(panes.count < 2)

            Spacer()

            Text("\(panes.count) terminal\(panes.count == 1 ? "" : "s")")
                .font(.caption)
                .foregroundStyle(.secondary)
        }
        .padding(.horizontal, 8)
        .padding(.vertical, 4)
        .background(Color(nsColor: .controlBackgroundColor))
    }

    @ViewBuilder
    private var terminalGrid: some View {
        if splitVertical {
            HSplitView {
                ForEach(panes) { pane in
                    singleTerminal(pane: pane)
                }
            }
        } else {
            VSplitView {
                ForEach(panes) { pane in
                    singleTerminal(pane: pane)
                }
            }
        }
    }

    private func singleTerminal(pane: TerminalPane) -> some View {
        VStack(spacing: 0) {
            if panes.count > 1 {
                HStack {
                    Spacer()
                    Button {
                        removePane(pane)
                    } label: {
                        Image(systemName: "xmark")
                            .font(.caption2)
                    }
                    .buttonStyle(.plain)
                    .padding(2)
                }
                .background(Color(nsColor: .controlBackgroundColor).opacity(0.5))
            }
            TerminalView(containerName: containerName) {
                removePane(pane)
            }
        }
    }

    private func handleAction(_ terminalAction: TerminalAction) {
        switch terminalAction {
        case .addPane:
            addPane()
        case .splitVertical:
            addPane()
            splitVertical = true
        case .splitHorizontal:
            addPane()
            splitVertical = false
        }
    }

    private func addPane() {
        guard panes.count < maxPanes else { return }
        panes.append(TerminalPane())
    }

    private func removePane(_ pane: TerminalPane) {
        guard panes.count > 1 else { return }
        panes.removeAll { $0.id == pane.id }
    }
}
