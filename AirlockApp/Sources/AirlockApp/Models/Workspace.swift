import Foundation

struct Workspace: Identifiable, Codable, Hashable {
    let id: UUID
    var name: String
    var path: String
    var envFilePath: String?
    var containerImageOverride: String?
    var proxyImageOverride: String?
    var passthroughHostsOverride: [String]?
    var proxyPortOverride: Int?

    // Runtime state (not persisted)
    var isActive: Bool = false
    var containerId: String?
    var proxyId: String?
    var networkId: String?
    var terminalSessions: [TerminalSession] = []

    enum CodingKeys: String, CodingKey {
        case id, name, path, envFilePath, containerImageOverride
        case proxyImageOverride, passthroughHostsOverride, proxyPortOverride
    }

    init(from decoder: Decoder) throws {
        let container = try decoder.container(keyedBy: CodingKeys.self)
        id = try container.decode(UUID.self, forKey: .id)
        name = try container.decode(String.self, forKey: .name)
        path = try container.decode(String.self, forKey: .path)
        envFilePath = try container.decodeIfPresent(String.self, forKey: .envFilePath)
        containerImageOverride = try container.decodeIfPresent(String.self, forKey: .containerImageOverride)
        proxyImageOverride = try container.decodeIfPresent(String.self, forKey: .proxyImageOverride)
        passthroughHostsOverride = try container.decodeIfPresent([String].self, forKey: .passthroughHostsOverride)
        proxyPortOverride = try container.decodeIfPresent(Int.self, forKey: .proxyPortOverride)
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

    var containerWorkDir: String {
        let basename = (path as NSString).lastPathComponent
        if basename.isEmpty || basename == "." || basename == ".." {
            return "/workspace/workspace"
        }
        return "/workspace/\(basename)"
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
