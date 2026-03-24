import SwiftUI
import SwiftTerm

struct TerminalView: NSViewRepresentable {
    let workspace: Workspace
    @Bindable var appState: AppState

    func makeNSView(context: Context) -> LocalProcessTerminalView {
        let terminal = LocalProcessTerminalView(frame: .zero)
        terminal.font = NSFont.monospacedSystemFont(ofSize: 13, weight: .regular)
        terminal.processDelegate = context.coordinator
        return terminal
    }

    func updateNSView(_ terminal: LocalProcessTerminalView, context: Context) {
        let coord = context.coordinator
        if appState.activeWorkspaceID == workspace.id
            && appState.sessionStatus == .running
            && !coord.processStarted
        {
            coord.processStarted = true
            let cli = CLIService()
            let binary = cli.resolveAirlockBinary()
            var args = ["run"]
            if let envFile = workspace.envFilePath {
                args += ["--env", envFile]
            }
            let env = CLIService.enrichedEnvironment().map { "\($0.key)=\($0.value)" }
            terminal.startProcess(
                executable: binary,
                args: args,
                environment: env,
                currentDirectory: workspace.path,
                execName: "airlock"
            )
        }
    }

    func makeCoordinator() -> Coordinator {
        Coordinator(appState: appState, workspace: workspace)
    }

    class Coordinator: NSObject, LocalProcessTerminalViewDelegate {
        let appState: AppState
        let workspace: Workspace
        var processStarted = false

        init(appState: AppState, workspace: Workspace) {
            self.appState = appState
            self.workspace = workspace
        }

        func sizeChanged(source: LocalProcessTerminalView, newCols: Int, newRows: Int) {}
        func setTerminalTitle(source: LocalProcessTerminalView, title: String) {}
        func hostCurrentDirectoryUpdate(source: SwiftTerm.TerminalView, directory: String?) {}

        func processTerminated(source: SwiftTerm.TerminalView, exitCode: Int32?) {
            Task { @MainActor in
                if let code = exitCode, code != 0 {
                    appState.sessionStatus = .error("Process exited with code \(code)")
                    appState.lastError = "Process exited with code \(code)"
                } else {
                    appState.sessionStatus = .stopped
                }
                appState.activeWorkspaceID = nil
            }
        }
    }
}
