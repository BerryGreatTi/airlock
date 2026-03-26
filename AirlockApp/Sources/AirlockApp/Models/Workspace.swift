import Foundation

struct Workspace: Identifiable, Codable, Hashable {
    let id: UUID
    var name: String
    var path: String
    var envFilePath: String?
    var containerImageOverride: String?

    // Runtime state (not persisted)
    var isActive: Bool = false
    var containerId: String?
    var proxyId: String?
    var networkId: String?
    var terminalSessions: [TerminalSession] = []

    enum CodingKeys: String, CodingKey {
        case id, name, path, envFilePath, containerImageOverride
    }

    var shortID: String {
        String(id.uuidString.prefix(8)).lowercased()
    }

    var containerName: String {
        "airlock-claude-\(shortID)"
    }

    var proxyName: String {
        "airlock-proxy-\(shortID)"
    }

    init(name: String, path: String, envFilePath: String? = nil, containerImageOverride: String? = nil) {
        self.id = UUID()
        self.name = name
        self.path = path
        self.envFilePath = envFilePath
        self.containerImageOverride = containerImageOverride
    }

    // Hashable conformance excluding runtime fields
    static func == (lhs: Workspace, rhs: Workspace) -> Bool {
        lhs.id == rhs.id
    }

    func hash(into hasher: inout Hasher) {
        hasher.combine(id)
    }
}

struct TerminalSession: Identifiable, Hashable {
    let id: UUID = UUID()
    var isActive: Bool = true
}
