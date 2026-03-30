import SwiftUI

@MainActor
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
                        } else if appState.isActivating(workspace) {
                            Button("Activating...") { }
                                .disabled(true)
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
                    NotificationCenter.default.post(name: .airlockOpenGlobalSettings, object: nil)
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
        switch appState.activationState(for: workspace) {
        case .active: return .green
        case .activating: return .yellow
        case .inactive: return .gray
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
        Task { @MainActor in
            await appState.performActivation(workspace: workspace, using: containerService)
        }
    }

    private func deactivateWorkspace(_ workspace: Workspace) {
        Task { @MainActor in
            await appState.performDeactivation(workspace: workspace, using: containerService)
        }
    }

    @MainActor
    private func removeWorkspace(_ workspace: Workspace) {
        appState.workspaces.removeAll { $0.id == workspace.id }
        appState.activationStates.removeValue(forKey: workspace.id)
        try? WorkspaceStore().saveWorkspaces(appState.workspaces)
    }
}
