import Foundation
import Observation

enum SessionStatus: Equatable {
    case stopped
    case running
    case error(String)
}

enum DetailTab: Hashable {
    case terminal
    case diff
    case settings
}

@Observable
final class AppState {
    var workspaces: [Workspace] = []
    var selectedWorkspaceID: UUID?
    var activeWorkspaceID: UUID?
    var sessionStatus: SessionStatus = .stopped
    var selectedTab: DetailTab = .terminal
    var lastError: String?

    var selectedWorkspace: Workspace? {
        workspaces.first { $0.id == selectedWorkspaceID }
    }

    var activeWorkspace: Workspace? {
        workspaces.first { $0.id == activeWorkspaceID }
    }

    var isRunning: Bool {
        sessionStatus == .running
    }
}

struct AppSettings: Codable, Equatable {
    var airlockBinaryPath: String?
    var containerImage: String = "airlock-claude:latest"
    var proxyImage: String = "airlock-proxy:latest"
    var passthroughHosts: [String] = ["api.anthropic.com", "auth.anthropic.com"]
}
