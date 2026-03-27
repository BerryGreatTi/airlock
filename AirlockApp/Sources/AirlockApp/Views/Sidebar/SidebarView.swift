import SwiftUI

struct SidebarView: View {
    @Bindable var appState: AppState
    @Environment(\.containerService) private var containerService
    @State private var showingNewWorkspace = false
    @State private var showingDeactivateConfirmation = false
    @State private var pendingDeactivation: Workspace?

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
                            Button("Deactivate") { confirmDeactivate(workspace) }
                            Divider()
                            Button("Stop and Remove", role: .destructive) {
                                confirmDeactivate(workspace, thenRemove: true)
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
        .alert("Deactivate Workspace?", isPresented: $showingDeactivateConfirmation) {
            Button("Deactivate", role: .destructive) {
                if let ws = pendingDeactivation {
                    let shouldRemove = pendingRemove
                    deactivateWorkspace(ws)
                    if shouldRemove { removeWorkspace(ws) }
                }
                pendingDeactivation = nil
                pendingRemove = false
            }
            Button("Cancel", role: .cancel) {
                pendingDeactivation = nil
                pendingRemove = false
            }
        } message: {
            if let ws = pendingDeactivation {
                Text("This will stop all containers for \"\(ws.name)\".")
            }
        }
    }

    @State private var pendingRemove = false

    private func statusColor(for workspace: Workspace) -> Color {
        switch appState.statusFor(workspace) {
        case .running: return .green
        case .error: return .red
        case .stopped: return .gray
        }
    }

    private func confirmDeactivate(_ workspace: Workspace, thenRemove: Bool = false) {
        pendingDeactivation = workspace
        pendingRemove = thenRemove
        showingDeactivateConfirmation = true
    }

    private func activateWorkspace(_ workspace: Workspace) {
        appState.selectedWorkspaceID = workspace.id
        appState.selectedTab = .terminal
        appState.lastError = nil
        Task { @MainActor in
            do {
                _ = try await containerService.activate(workspace: workspace)
                appState.activeWorkspaceIDs.insert(workspace.id)
                if let idx = appState.workspaces.firstIndex(where: { $0.id == workspace.id }) {
                    appState.workspaces[idx].isActive = true
                }
            } catch {
                appState.lastError = error.localizedDescription
            }
        }
    }

    private func deactivateWorkspace(_ workspace: Workspace) {
        Task { @MainActor in
            await containerService.deactivate(workspace: workspace)
            appState.activeWorkspaceIDs.remove(workspace.id)
            if let idx = appState.workspaces.firstIndex(where: { $0.id == workspace.id }) {
                appState.workspaces[idx].isActive = false
            }
        }
    }

    @MainActor
    private func removeWorkspace(_ workspace: Workspace) {
        appState.workspaces.removeAll { $0.id == workspace.id }
        appState.activeWorkspaceIDs.remove(workspace.id)
        try? WorkspaceStore().saveWorkspaces(appState.workspaces)
    }
}
