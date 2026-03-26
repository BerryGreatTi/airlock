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
        .focusedValue(\.appState, appState)
        .onAppear {
            loadState()
            reconcileRunningContainers()
        }
        .alert("Orphaned Containers Found", isPresented: $showingOrphanCleanup) {
            Button("Clean Up") { cleanupOrphans() }
            Button("Ignore", role: .cancel) { orphanedContainers = [] }
        } message: {
            Text("\(orphanedContainers.count) container(s) running without a matching workspace. Clean them up?")
        }
    }

    @State private var orphanedContainers: [String] = []
    @State private var showingOrphanCleanup = false

    @ViewBuilder
    private var detailContent: some View {
        if appState.workspaces.isEmpty {
            WelcomeView(appState: appState)
        } else if appState.selectedTab == .settings {
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
            tabButton("Secrets", tab: .secrets, icon: "key")
            tabButton("Containers", tab: .containers, icon: "shippingbox")
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
                if appState.isActive(workspace) {
                    TerminalSplitView(containerName: workspace.containerName)
                } else {
                    ContentUnavailableView {
                        Label("Workspace Inactive", systemImage: "terminal")
                    } description: {
                        Text("Activate this workspace from the sidebar to open a terminal")
                    }
                }
            case .secrets:
                SecretsView(workspace: workspace, appState: appState)
            case .containers:
                ContainerStatusView(workspace: workspace, appState: appState)
            case .diff:
                DiffContainerView(workspace: workspace, appState: appState)
            case .settings:
                SettingsView(appState: appState)
            }

            if let error = appState.lastError {
                VStack {
                    HStack {
                        Image(systemName: "exclamationmark.triangle.fill")
                            .foregroundStyle(.red)
                        Text(error).font(.caption)
                        Spacer()
                        Button("Dismiss") {
                            appState.lastError = nil
                        }
                        .buttonStyle(.bordered)
                        .controlSize(.small)
                    }
                    .padding(8)
                    .background(.red.opacity(0.1))
                    .clipShape(RoundedRectangle(cornerRadius: 8))
                    .padding()
                    Spacer()
                }
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

    private func loadState() {
        let store = WorkspaceStore()
        appState.workspaces = (try? store.loadWorkspaces()) ?? []
    }

    private func reconcileRunningContainers() {
        Task {
            let service = ContainerSessionService()
            guard let result = try? await service.status() else { return }
            guard let data = result.stdout.data(using: .utf8),
                  let json = try? JSONSerialization.jsonObject(with: data) as? [String: Any],
                  let workspaces = json["workspaces"] as? [[String: Any]] else { return }

            var orphans: [String] = []
            for entry in workspaces {
                guard let entryID = entry["id"] as? String,
                      let status = entry["status"] as? String,
                      status == "running" else { continue }

                let matched = appState.workspaces.first { $0.shortID == entryID }
                if let ws = matched, let idx = appState.workspaces.firstIndex(where: { $0.id == ws.id }) {
                    appState.activeWorkspaceIDs.insert(ws.id)
                    appState.workspaces[idx].isActive = true
                } else {
                    orphans.append(entryID)
                }
            }

            if !orphans.isEmpty {
                orphanedContainers = orphans
                showingOrphanCleanup = true
            }
        }
    }

    private func cleanupOrphans() {
        Task {
            let cli = CLIService()
            let home = FileManager.default.homeDirectoryForCurrentUser.path
            for id in orphanedContainers {
                _ = try? await cli.run(args: ["stop", "--id", id], workingDirectory: home)
            }
            orphanedContainers = []
        }
    }
}
