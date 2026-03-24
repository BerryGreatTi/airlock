import SwiftUI

struct ContentView: View {
    @State var appState = AppState()

    var body: some View {
        NavigationSplitView {
            SidebarView(appState: appState)
                .frame(minWidth: 200)
        } detail: {
            detailContent
        }
        .onAppear { loadState() }
    }

    @ViewBuilder
    private var detailContent: some View {
        if appState.selectedTab == .settings {
            SettingsView(appState: appState)
        } else if let workspace = appState.selectedWorkspace {
            VStack(spacing: 0) {
                tabBar
                Divider()
                tabContent(workspace: workspace)
            }
        } else {
            ContentUnavailableView {
                Label("No Workspace Selected", systemImage: "sidebar.left")
            } description: {
                Text("Select a workspace from the sidebar or create a new one with Cmd+N")
            }
        }
    }

    private var tabBar: some View {
        HStack(spacing: 0) {
            tabButton("Terminal", tab: .terminal, icon: "terminal")
            tabButton("Diff", tab: .diff, icon: "doc.text.magnifyingglass")
            Spacer()
        }
        .background(Color(nsColor: .controlBackgroundColor))
    }

    @ViewBuilder
    private func tabContent(workspace: Workspace) -> some View {
        ZStack {
            switch appState.selectedTab {
            case .terminal:
                TerminalView(workspace: workspace, appState: appState)
            case .diff:
                DiffContainerView(workspace: workspace, appState: appState)
            case .settings:
                SettingsView(appState: appState)
            }

            // Error banner overlay
            if case .error(let msg) = appState.sessionStatus {
                VStack {
                    HStack {
                        Image(systemName: "exclamationmark.triangle.fill")
                            .foregroundStyle(.red)
                        Text(msg).font(.caption)
                        Spacer()
                        Button("Dismiss") {
                            appState.sessionStatus = .stopped
                            appState.lastError = nil
                        }
                        .buttonStyle(.bordered)
                        .controlSize(.small)
                        Button("Restart") {
                            if let ws = appState.selectedWorkspace {
                                restartWorkspace(ws)
                            }
                        }
                        .buttonStyle(.borderedProminent)
                        .controlSize(.small)
                    }
                    .padding(8)
                    .background(.red.opacity(0.1))
                    .clipShape(RoundedRectangle(cornerRadius: 8))
                    .padding()
                    Spacer()
                }
            }

            // Session ended overlay
            if appState.sessionStatus == .stopped
                && appState.activeWorkspaceID == nil
                && appState.lastError == nil
                && appState.selectedTab == .terminal {
                // Only show if a session previously ran (not on fresh load)
            }
        }
    }

    private func tabButton(_ title: String, tab: DetailTab, icon: String) -> some View {
        Button {
            appState.selectedTab = tab
        } label: {
            Label(title, systemImage: icon)
                .padding(.horizontal, 12)
                .padding(.vertical, 6)
        }
        .buttonStyle(.plain)
        .background(appState.selectedTab == tab ? Color.accentColor.opacity(0.15) : .clear)
    }

    private func restartWorkspace(_ workspace: Workspace) {
        Task {
            let cli = CLIService()
            _ = try? await cli.run(args: ["stop"], workingDirectory: workspace.path)
            appState.sessionStatus = .stopped
            appState.lastError = nil

            // Small delay then restart
            try? await Task.sleep(for: .milliseconds(500))
            appState.activeWorkspaceID = workspace.id
            appState.sessionStatus = .running
        }
    }

    private func loadState() {
        let store = WorkspaceStore()
        appState.workspaces = (try? store.loadWorkspaces()) ?? []
    }
}
