import SwiftUI

@main
struct AirlockApp: App {
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
                Button("Run") {
                    NotificationCenter.default.post(name: .airlockRunWorkspace, object: nil)
                }
                .keyboardShortcut("r")

                Button("Stop") {
                    NotificationCenter.default.post(name: .airlockStopWorkspace, object: nil)
                }
                .keyboardShortcut(".", modifiers: .command)
            }

            CommandMenu("View") {
                Button("Terminal") {
                    NotificationCenter.default.post(name: .airlockShowTerminal, object: nil)
                }
                .keyboardShortcut("1")

                Button("Diff") {
                    NotificationCenter.default.post(name: .airlockShowDiff, object: nil)
                }
                .keyboardShortcut("2")

                Button("Refresh Diff") {
                    NotificationCenter.default.post(name: .airlockRefreshDiff, object: nil)
                }
                .keyboardShortcut("r", modifiers: [.command, .shift])
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
    static let airlockRunWorkspace = Notification.Name("airlockRunWorkspace")
    static let airlockStopWorkspace = Notification.Name("airlockStopWorkspace")
    static let airlockShowTerminal = Notification.Name("airlockShowTerminal")
    static let airlockShowDiff = Notification.Name("airlockShowDiff")
    static let airlockRefreshDiff = Notification.Name("airlockRefreshDiff")
}
