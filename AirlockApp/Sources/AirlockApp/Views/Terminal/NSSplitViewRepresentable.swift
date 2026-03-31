import SwiftUI
import SwiftTerm

struct NSSplitViewRepresentable: NSViewRepresentable {
    let paneIDs: [UUID]
    let isVertical: Bool
    let containerName: String
    let workDir: String
    let onPaneTerminated: (UUID) -> Void

    func makeCoordinator() -> Coordinator {
        Coordinator(containerName: containerName, workDir: workDir, onPaneTerminated: onPaneTerminated)
    }

    func makeNSView(context: Context) -> NSSplitView {
        let splitView = NSSplitView()
        splitView.isVertical = isVertical
        splitView.dividerStyle = .thin
        splitView.delegate = context.coordinator

        for paneID in paneIDs {
            let terminal = context.coordinator.createTerminal(for: paneID)
            splitView.addArrangedSubview(terminal)
            context.coordinator.startTerminal(terminal)
        }
        context.coordinator.currentPaneIDs = paneIDs

        DispatchQueue.main.async {
            context.coordinator.equalizePanes(in: splitView)
        }

        return splitView
    }

    func updateNSView(_ splitView: NSSplitView, context: Context) {
        let coord = context.coordinator
        coord.onPaneTerminated = onPaneTerminated

        // Toggle direction without destroying subviews (#5)
        if splitView.isVertical != isVertical {
            splitView.isVertical = isVertical
        }

        let oldIDs = coord.currentPaneIDs
        let newIDs = paneIDs

        if oldIDs != newIDs {
            // Remove panes that no longer exist
            let removed = oldIDs.filter { !newIDs.contains($0) }
            for id in removed {
                coord.removeTerminal(for: id, from: splitView)
            }

            // Add new panes
            let added = newIDs.filter { !oldIDs.contains($0) }
            for id in added {
                let terminal = coord.createTerminal(for: id)
                splitView.addArrangedSubview(terminal)
                coord.startTerminal(terminal)
            }

            coord.currentPaneIDs = newIDs

            // Equalize panes after add/remove (#4)
            DispatchQueue.main.async {
                coord.equalizePanes(in: splitView)
            }
        }
    }

    // MARK: - Coordinator

    final class Coordinator: NSObject, NSSplitViewDelegate {
        var terminals: [UUID: AirlockTerminalView] = [:]
        var delegates: [UUID: PaneDelegate] = [:]
        var currentPaneIDs: [UUID] = []
        let containerName: String
        let workDir: String
        var onPaneTerminated: (UUID) -> Void

        init(containerName: String, workDir: String, onPaneTerminated: @escaping (UUID) -> Void) {
            self.containerName = containerName
            self.workDir = workDir
            self.onPaneTerminated = onPaneTerminated
        }

        func createTerminal(for paneID: UUID) -> AirlockTerminalView {
            let terminal = AirlockTerminalView(frame: .zero)
            terminal.font = NSFont.monospacedSystemFont(ofSize: 13, weight: .regular)

            let paneDelegate = PaneDelegate(paneID: paneID) { [weak self] id in
                self?.onPaneTerminated(id)
            }
            terminal.processDelegate = paneDelegate

            terminals[paneID] = terminal
            delegates[paneID] = paneDelegate
            return terminal
        }

        func startTerminal(_ terminal: AirlockTerminalView) {
            let escaped = AirlockTerminalView.shellEscape(containerName)
            let escapedWorkDir = AirlockTerminalView.shellEscape(workDir)
            let cmd = "docker exec -it -w \(escapedWorkDir) \(escaped) /bin/bash"
            let env = CLIService.enrichedEnvironment().map { "\($0.key)=\($0.value)" }
            terminal.startAfterLayout(cmd: cmd, env: env)
        }

        func removeTerminal(for paneID: UUID, from splitView: NSSplitView) {
            if let terminal = terminals[paneID] {
                terminal.removeFromSuperview()
                terminals.removeValue(forKey: paneID)
            }
            delegates.removeValue(forKey: paneID)
        }

        func equalizePanes(in splitView: NSSplitView) {
            let count = splitView.arrangedSubviews.count
            guard count > 1 else { return }
            let dividerThickness = splitView.dividerThickness
            let totalDividers = CGFloat(count - 1) * dividerThickness
            let totalSpace = (splitView.isVertical ? splitView.bounds.width : splitView.bounds.height) - totalDividers
            guard totalSpace > 0 else { return }
            let paneSize = totalSpace / CGFloat(count)

            for i in 0..<(count - 1) {
                let position = paneSize * CGFloat(i + 1) + dividerThickness * CGFloat(i)
                splitView.setPosition(position, ofDividerAt: i)
            }
        }

        // MARK: NSSplitViewDelegate

        func splitView(_ splitView: NSSplitView, constrainMinCoordinate proposedMinimumPosition: CGFloat, ofSubviewAt dividerIndex: Int) -> CGFloat {
            proposedMinimumPosition + 80
        }

        func splitView(_ splitView: NSSplitView, constrainMaxCoordinate proposedMaximumPosition: CGFloat, ofSubviewAt dividerIndex: Int) -> CGFloat {
            proposedMaximumPosition - 80
        }
    }

    // MARK: - PaneDelegate

    final class PaneDelegate: NSObject, LocalProcessTerminalViewDelegate {
        let paneID: UUID
        let onTerminated: (UUID) -> Void

        init(paneID: UUID, onTerminated: @escaping (UUID) -> Void) {
            self.paneID = paneID
            self.onTerminated = onTerminated
        }

        func sizeChanged(source: LocalProcessTerminalView, newCols: Int, newRows: Int) {}
        func setTerminalTitle(source: LocalProcessTerminalView, title: String) {}
        func hostCurrentDirectoryUpdate(source: SwiftTerm.TerminalView, directory: String?) {}

        func processTerminated(source: SwiftTerm.TerminalView, exitCode: Int32?) {
            Task { @MainActor in
                onTerminated(paneID)
            }
        }
    }
}
