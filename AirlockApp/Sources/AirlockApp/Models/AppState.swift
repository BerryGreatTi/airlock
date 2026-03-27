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

@Observable
@MainActor
final class AppState {
    var workspaces: [Workspace] = []
    var selectedWorkspaceID: UUID?
    var activeWorkspaceIDs: Set<UUID> = []
    var selectedTab: DetailTab = .terminal
    var lastError: String?

    nonisolated init() {}

    var selectedWorkspace: Workspace? {
        workspaces.first { $0.id == selectedWorkspaceID }
    }

    func isActive(_ workspace: Workspace) -> Bool {
        activeWorkspaceIDs.contains(workspace.id)
    }

    func statusFor(_ workspace: Workspace) -> SessionStatus {
        guard let ws = workspaces.first(where: { $0.id == workspace.id }) else { return .stopped }
        if activeWorkspaceIDs.contains(ws.id) {
            return ws.isActive ? .running : .error("activation failed")
        }
        return .stopped
    }
}

struct AppSettings: Codable, Equatable {
    var airlockBinaryPath: String?
    var containerImage: String = "airlock-claude:latest"
    var proxyImage: String = "airlock-proxy:latest"
    var passthroughHosts: [String] = ["api.anthropic.com", "auth.anthropic.com"]
}
