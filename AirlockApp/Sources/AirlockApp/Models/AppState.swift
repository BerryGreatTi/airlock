import Foundation
import Observation

enum SessionStatus: Equatable {
    case stopped
    case running
    case error(String)
}

enum DetailTab: Hashable {
    case terminal
    case secrets
    case containers
    case diff
    case settings
}

enum ActivationState: Equatable {
    case inactive
    case activating
    case active
}

@Observable
@MainActor
final class AppState {
    var workspaces: [Workspace] = []
    var selectedWorkspaceID: UUID?
    var activationStates: [UUID: ActivationState] = [:]
    var selectedTab: DetailTab = .terminal
    var lastError: String?

    private var tabSwitchTask: Task<Void, Never>?

    nonisolated init() {}

    var selectedWorkspace: Workspace? {
        workspaces.first { $0.id == selectedWorkspaceID }
    }

    func isActive(_ workspace: Workspace) -> Bool {
        activationStates[workspace.id] == .active
    }

    func isActivating(_ workspace: Workspace) -> Bool {
        activationStates[workspace.id] == .activating
    }

    func activationState(for workspace: Workspace) -> ActivationState {
        activationStates[workspace.id] ?? .inactive
    }

    func statusFor(_ workspace: Workspace) -> SessionStatus {
        guard let ws = workspaces.first(where: { $0.id == workspace.id }) else { return .stopped }
        switch activationStates[ws.id] {
        case .active:
            return ws.isActive ? .running : .error("activation failed")
        case .activating:
            return .running
        case .inactive, .none:
            return .stopped
        }
    }

    func switchTab(to tab: DetailTab) {
        tabSwitchTask?.cancel()
        tabSwitchTask = Task { @MainActor [weak self] in
            try? await Task.sleep(for: .milliseconds(150))
            guard !Task.isCancelled, let self else { return }
            self.selectedTab = tab
        }
    }

    func performActivation(
        workspace: Workspace,
        using service: ContainerSessionService
    ) async {
        activationStates[workspace.id] = .activating
        lastError = nil
        do {
            let store = WorkspaceStore()
            let settings = (try? store.loadSettings()) ?? AppSettings()
            _ = try await service.activateAndWaitReady(workspace: workspace, settings: settings)
            activationStates[workspace.id] = .active
            if let idx = workspaces.firstIndex(where: { $0.id == workspace.id }) {
                workspaces[idx].isActive = true
            }
        } catch {
            activationStates[workspace.id] = .inactive
            lastError = error.localizedDescription
        }
    }

    func performDeactivation(
        workspace: Workspace,
        using service: ContainerSessionService
    ) async {
        await service.deactivate(workspace: workspace)
        activationStates[workspace.id] = .inactive
        if let idx = workspaces.firstIndex(where: { $0.id == workspace.id }) {
            workspaces[idx].isActive = false
        }
    }
}

struct AppSettings: Codable, Equatable {
    var airlockBinaryPath: String?
    var containerImage: String = "airlock-claude:latest"
    var proxyImage: String = "airlock-proxy:latest"
    var passthroughHosts: [String] = []
}
