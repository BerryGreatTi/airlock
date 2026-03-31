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

struct ShowGlobalSettingsKey: FocusedValueKey {
    typealias Value = Binding<Bool>
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

    var showGlobalSettings: Binding<Bool>? {
        get { self[ShowGlobalSettingsKey.self] }
        set { self[ShowGlobalSettingsKey.self] = newValue }
    }
}

@main
struct AirlockApp: App {
    @NSApplicationDelegateAdaptor(AppDelegate.self) private var appDelegate
    @FocusedValue(\.appState) private var appState
    @FocusedValue(\.containerService) private var containerService
    @FocusedValue(\.terminalAction) private var terminalAction
    @FocusedValue(\.showGlobalSettings) private var showGlobalSettings

    var body: some Scene {
        WindowGroup {
            ContentView()
                .frame(minWidth: 800, minHeight: 500)
                .onAppear {
                    NSApp.setActivationPolicy(.regular)
                    NSApp.activate(ignoringOtherApps: true)
                    NSApp.applicationIconImage = AirlockIconView.makeNSImage()
                }
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
                    Task { @MainActor in
                        await state.performActivation(workspace: ws, using: service)
                    }
                }
                .keyboardShortcut("r")

                Button("Deactivate") {
                    guard let state = appState, let ws = state.selectedWorkspace,
                          let service = containerService else { return }
                    Task { @MainActor in
                        await state.performDeactivation(workspace: ws, using: service)
                    }
                }
                .keyboardShortcut(".", modifiers: .command)
            }

            CommandMenu("View") {
                Button("Terminal") { appState?.switchTab(to: .terminal) }
                    .keyboardShortcut("1")

                Button("Secrets") { appState?.switchTab(to: .secrets) }
                    .keyboardShortcut("2")

                Button("Containers") { appState?.switchTab(to: .containers) }
                    .keyboardShortcut("3")

                Button("Workspace Settings") { appState?.switchTab(to: .settings) }
                    .keyboardShortcut("4")

                Divider()

                Button("Preferences...") {
                    showGlobalSettings?.wrappedValue = true
                }
                .keyboardShortcut(",")


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
            Text("Use Cmd+, or the sidebar Settings button")
                .frame(width: 300, height: 100)
                .padding()
        }
    }
}

extension Notification.Name {
    static let airlockNewWorkspace = Notification.Name("airlockNewWorkspace")
    static let airlockOpenGlobalSettings = Notification.Name("airlockOpenGlobalSettings")
}
