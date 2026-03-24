import SwiftUI

struct SidebarView: View {
    @Bindable var appState: AppState
    @State private var showingNewWorkspace = false
    @State private var showingStopConfirmation = false
    @State private var pendingWorkspace: Workspace?

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
                        Button("Run") { requestStart(workspace) }
                            .disabled(appState.isRunning && appState.activeWorkspaceID == workspace.id)
                        Button("Stop") { stopWorkspace(workspace) }
                            .disabled(appState.activeWorkspaceID != workspace.id || !appState.isRunning)
                        Divider()
                        Button("Remove", role: .destructive) { removeWorkspace(workspace) }
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
        .alert("Switch Workspace?", isPresented: $showingStopConfirmation) {
            Button("Stop and Switch") {
                if let ws = pendingWorkspace {
                    Task {
                        await forceStartWorkspace(ws)
                    }
                }
            }
            Button("Cancel", role: .cancel) { pendingWorkspace = nil }
        } message: {
            if let active = appState.activeWorkspace {
                Text("Stop \"\(active.name)\" and start \"\(pendingWorkspace?.name ?? "")\"?")
            }
        }
    }

    private func statusColor(for workspace: Workspace) -> Color {
        if appState.activeWorkspaceID == workspace.id {
            switch appState.sessionStatus {
            case .running: return .green
            case .error: return .red
            case .stopped: return .gray
            }
        }
        return .gray
    }

    private func requestStart(_ workspace: Workspace) {
        if appState.isRunning && appState.activeWorkspaceID != workspace.id {
            pendingWorkspace = workspace
            showingStopConfirmation = true
        } else {
            startWorkspace(workspace)
        }
    }

    private func startWorkspace(_ workspace: Workspace) {
        appState.selectedWorkspaceID = workspace.id
        appState.activeWorkspaceID = workspace.id
        appState.sessionStatus = .running
        appState.selectedTab = .terminal
        appState.lastError = nil
    }

    private func forceStartWorkspace(_ workspace: Workspace) async {
        if let activeWs = appState.activeWorkspace {
            let cli = CLIService()
            _ = try? await cli.run(args: ["stop"], workingDirectory: activeWs.path)
        }
        appState.sessionStatus = .stopped
        appState.activeWorkspaceID = nil
        startWorkspace(workspace)
    }

    private func stopWorkspace(_ workspace: Workspace) {
        Task {
            let cli = CLIService()
            _ = try? await cli.run(args: ["stop"], workingDirectory: workspace.path)
            appState.sessionStatus = .stopped
            appState.activeWorkspaceID = nil
        }
    }

    private func removeWorkspace(_ workspace: Workspace) {
        if appState.activeWorkspaceID == workspace.id && appState.isRunning {
            stopWorkspace(workspace)
        }
        appState.workspaces.removeAll { $0.id == workspace.id }
        persistWorkspaces()
    }

    private func persistWorkspaces() {
        try? WorkspaceStore().saveWorkspaces(appState.workspaces)
    }
}
