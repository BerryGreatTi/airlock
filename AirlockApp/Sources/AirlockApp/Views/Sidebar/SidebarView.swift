import SwiftUI

struct SidebarView: View {
    @Bindable var appState: AppState
    @State private var showingNewWorkspace = false

    var body: some View {
        List(selection: $appState.selectedWorkspaceID) {
            Section("Workspaces") {
                ForEach(appState.workspaces) { workspace in
                    HStack {
                        Circle()
                            .fill(statusColor(for: workspace))
                            .frame(width: 8, height: 8)
                        Text(workspace.name)
                        Spacer()
                    }
                    .tag(workspace.id)
                    .contextMenu {
                        if appState.isActive(workspace) {
                            Button("Deactivate") { deactivateWorkspace(workspace) }
                            Divider()
                            Button("Stop and Remove", role: .destructive) {
                                deactivateWorkspace(workspace)
                                removeWorkspace(workspace)
                            }
                        } else {
                            Button("Activate") { activateWorkspace(workspace) }
                            Divider()
                            Button("Remove", role: .destructive) { removeWorkspace(workspace) }
                        }
                    }
                }
            }
        }
        .safeAreaInset(edge: .bottom) {
            VStack(spacing: 8) {
                Button {
                    showingNewWorkspace = true
                } label: {
                    Label("New Workspace", systemImage: "plus")
                        .frame(maxWidth: .infinity, alignment: .leading)
                }
                .buttonStyle(.plain)
                .padding(.horizontal)

                Button {
                    appState.selectedTab = .settings
                } label: {
                    Label("Settings", systemImage: "gear")
                        .frame(maxWidth: .infinity, alignment: .leading)
                }
                .buttonStyle(.plain)
                .padding(.horizontal)
            }
            .padding(.vertical, 8)
        }
        .sheet(isPresented: $showingNewWorkspace) {
            NewWorkspaceSheet(appState: appState)
        }
    }

    private func statusColor(for workspace: Workspace) -> Color {
        switch appState.statusFor(workspace) {
        case .running: return .green
        case .error: return .red
        case .stopped: return .gray
        }
    }

    private func activateWorkspace(_ workspace: Workspace) {
        appState.selectedWorkspaceID = workspace.id
        appState.activeWorkspaceIDs.insert(workspace.id)
        if let idx = appState.workspaces.firstIndex(where: { $0.id == workspace.id }) {
            appState.workspaces[idx].isActive = true
        }
        appState.selectedTab = .terminal
        appState.lastError = nil
    }

    private func deactivateWorkspace(_ workspace: Workspace) {
        Task {
            let cli = CLIService()
            _ = try? await cli.run(args: ["stop", "--id", workspace.shortID], workingDirectory: workspace.path)
            appState.activeWorkspaceIDs.remove(workspace.id)
            if let idx = appState.workspaces.firstIndex(where: { $0.id == workspace.id }) {
                appState.workspaces[idx].isActive = false
            }
        }
    }

    private func removeWorkspace(_ workspace: Workspace) {
        appState.workspaces.removeAll { $0.id == workspace.id }
        appState.activeWorkspaceIDs.remove(workspace.id)
        try? WorkspaceStore().saveWorkspaces(appState.workspaces)
    }
}
