import SwiftUI

struct AppStateKey: FocusedValueKey {
    typealias Value = AppState
}

extension FocusedValues {
    var appState: AppState? {
        get { self[AppStateKey.self] }
        set { self[AppStateKey.self] = newValue }
    }
}

@main
struct AirlockApp: App {
    @FocusedValue(\.appState) private var appState

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
                    guard let state = appState, let ws = state.selectedWorkspace else { return }
                    state.activeWorkspaceIDs.insert(ws.id)
                    if let idx = state.workspaces.firstIndex(where: { $0.id == ws.id }) {
                        state.workspaces[idx].isActive = true
                    }
                }
                .keyboardShortcut("r")

                Button("Deactivate") {
                    guard let state = appState, let ws = state.selectedWorkspace else { return }
                    state.activeWorkspaceIDs.remove(ws.id)
                    if let idx = state.workspaces.firstIndex(where: { $0.id == ws.id }) {
                        state.workspaces[idx].isActive = false
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
                    NotificationCenter.default.post(name: .airlockNewTerminal, object: nil)
                }
                .keyboardShortcut("t")

                Button("Split Vertical") {
                    NotificationCenter.default.post(name: .airlockSplitVertical, object: nil)
                }
                .keyboardShortcut("d")

                Button("Split Horizontal") {
                    NotificationCenter.default.post(name: .airlockSplitHorizontal, object: nil)
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
    static let airlockNewTerminal = Notification.Name("airlockNewTerminal")
    static let airlockSplitVertical = Notification.Name("airlockSplitVertical")
    static let airlockSplitHorizontal = Notification.Name("airlockSplitHorizontal")
}
