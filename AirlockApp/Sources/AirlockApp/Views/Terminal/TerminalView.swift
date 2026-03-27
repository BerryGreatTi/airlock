import SwiftUI
import SwiftTerm

struct TerminalView: NSViewRepresentable {
    let containerName: String
    let onTerminated: (() -> Void)?

    init(containerName: String, onTerminated: (() -> Void)? = nil) {
        self.containerName = containerName
        self.onTerminated = onTerminated
    }

    func makeNSView(context: Context) -> AirlockTerminalView {
        let terminal = AirlockTerminalView(frame: .zero)
        terminal.font = NSFont.monospacedSystemFont(ofSize: 13, weight: .regular)
        terminal.processDelegate = context.coordinator
        return terminal
    }

    func updateNSView(_ terminal: AirlockTerminalView, context: Context) {
        let coord = context.coordinator
        if !coord.processStarted {
            coord.processStarted = true
            let cmd = "docker exec -it \(shellEscape(containerName)) /bin/bash"
            let env = CLIService.enrichedEnvironment().map { "\($0.key)=\($0.value)" }
            terminal.startAfterLayout(cmd: cmd, env: env)
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

/// Defers process start until the view has a non-zero frame from layout.
final class AirlockTerminalView: LocalProcessTerminalView {
    private var pendingStart: (() -> Void)?

    func startAfterLayout(cmd: String, env: [String]) {
        if frame.size.width > 0, frame.size.height > 0 {
            startProcess(executable: "/bin/bash", args: ["-c", cmd], environment: env, execName: "docker")
        } else {
            pendingStart = { [weak self] in
                self?.startProcess(executable: "/bin/bash", args: ["-c", cmd], environment: env, execName: "docker")
            }
        }
    }

    override func layout() {
        super.layout()
        if let start = pendingStart, frame.size.width > 0, frame.size.height > 0 {
            pendingStart = nil
            start()
        }
    }
}
