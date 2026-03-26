import SwiftUI

struct AppStateKey: FocusedValueKey {
    typealias Value = AppState
}

struct ContainerServiceKey: FocusedValueKey {
    typealias Value = ContainerSessionService
}

struct TerminalActionKey: FocusedValueKey {
    typealias Value = Binding<TerminalAction?>
}

enum TerminalAction {
    case addPane
    case splitVertical
    case splitHorizontal
}

extension FocusedValues {
    var appState: AppState? {
        get { self[AppStateKey.self] }
        set { self[AppStateKey.self] = newValue }
    }

    var containerService: ContainerSessionService? {
        get { self[ContainerServiceKey.self] }
        set { self[ContainerServiceKey.self] = newValue }
    }

    var terminalAction: Binding<TerminalAction?>? {
        get { self[TerminalActionKey.self] }
        set { self[TerminalActionKey.self] = newValue }
    }
}

@main
struct AirlockApp: App {
    @FocusedValue(\.appState) private var appState
    @FocusedValue(\.containerService) private var containerService
    @FocusedValue(\.terminalAction) private var terminalAction

    var body: some Scene {
        WindowGroup {
            ContentView()
                .frame(minWidth: 800, minHeight: 500)
        }
        .defaultSize(width: 1200, height: 700)
        .commands {
            CommandGroup(replacing: .newItem) {
                Button("New Workspace") {
                    NotificationCenter.default.post(name: .airlockNewWorkspace, object: nil)
                }
                .keyboardShortcut("n")
            }

            CommandMenu("Workspace") {
                Button("Activate") {
                    guard let state = appState, let ws = state.selectedWorkspace,
                          let service = containerService else { return }
                    Task {
                        do {
                            _ = try await service.activate(workspace: ws)
                            state.activeWorkspaceIDs.insert(ws.id)
                            if let idx = state.workspaces.firstIndex(where: { $0.id == ws.id }) {
                                state.workspaces[idx].isActive = true
                            }
                        } catch {
                            state.lastError = error.localizedDescription
                        }
                    }
                }
                .keyboardShortcut("r")

                Button("Deactivate") {
                    guard let state = appState, let ws = state.selectedWorkspace,
                          let service = containerService else { return }
                    Task {
                        await service.deactivate(workspace: ws)
                        state.activeWorkspaceIDs.remove(ws.id)
                        if let idx = state.workspaces.firstIndex(where: { $0.id == ws.id }) {
                            state.workspaces[idx].isActive = false
                        }
                    }
                }
                .keyboardShortcut(".", modifiers: .command)
            }

            CommandMenu("View") {
                Button("Terminal") { appState?.selectedTab = .terminal }
                    .keyboardShortcut("1")

                Button("Secrets") { appState?.selectedTab = .secrets }
                    .keyboardShortcut("2")

                Button("Containers") { appState?.selectedTab = .containers }
                    .keyboardShortcut("3")

                Button("Diff") { appState?.selectedTab = .diff }
                    .keyboardShortcut("4")

                Button("Settings") { appState?.selectedTab = .settings }
                    .keyboardShortcut("5")

                Divider()

                Button("New Terminal") {
                    terminalAction?.wrappedValue = .addPane
                }
                .keyboardShortcut("t")

                Button("Split Vertical") {
                    terminalAction?.wrappedValue = .splitVertical
                }
                .keyboardShortcut("d")

                Button("Split Horizontal") {
                    terminalAction?.wrappedValue = .splitHorizontal
                }
                .keyboardShortcut("d", modifiers: [.command, .shift])
            }
        }

        Settings {
            Text("Use the Settings tab in the main window")
                .frame(width: 300, height: 100)
                .padding()
        }
    }
}

extension Notification.Name {
    static let airlockNewWorkspace = Notification.Name("airlockNewWorkspace")
}
