import SwiftUI
import SwiftTerm

struct TerminalView: NSViewRepresentable {
    let containerName: String
    let onTerminated: (() -> Void)?

    init(containerName: String, onTerminated: (() -> Void)? = nil) {
        self.containerName = containerName
        self.onTerminated = onTerminated
    }

    func makeNSView(context: Context) -> LocalProcessTerminalView {
        let terminal = LocalProcessTerminalView(frame: .zero)
        terminal.font = NSFont.monospacedSystemFont(ofSize: 13, weight: .regular)
        terminal.processDelegate = context.coordinator
        return terminal
    }

    func updateNSView(_ terminal: LocalProcessTerminalView, context: Context) {
        let coord = context.coordinator
        if !coord.processStarted {
            coord.processStarted = true
            let cmd = "docker exec -it \(shellEscape(containerName)) /bin/bash"
            let env = CLIService.enrichedEnvironment().map { "\($0.key)=\($0.value)" }
            terminal.startProcess(
                executable: "/bin/bash",
                args: ["-c", cmd],
                environment: env,
                execName: "docker"
            )
        }
    }

    func makeCoordinator() -> Coordinator {
        Coordinator(onTerminated: onTerminated)
    }

    private func shellEscape(_ str: String) -> String {
        "'" + str.replacingOccurrences(of: "'", with: "'\\''") + "'"
    }

    class Coordinator: NSObject, LocalProcessTerminalViewDelegate {
        let onTerminated: (() -> Void)?
        var processStarted = false

        init(onTerminated: (() -> Void)?) {
            self.onTerminated = onTerminated
        }

        func sizeChanged(source: LocalProcessTerminalView, newCols: Int, newRows: Int) {}
        func setTerminalTitle(source: LocalProcessTerminalView, title: String) {}
        func hostCurrentDirectoryUpdate(source: SwiftTerm.TerminalView, directory: String?) {}

        func processTerminated(source: SwiftTerm.TerminalView, exitCode: Int32?) {
            Task { @MainActor in
                onTerminated?()
            }
        }
    }
}
