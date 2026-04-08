import Foundation

/// Reads the user's Claude settings files to enumerate the MCP server names
/// configured at the global level. The GUI uses this list to populate the
/// per-workspace MCP allow-list picker.
///
/// Note: this reads from the host filesystem (`~/.claude/settings*.json`),
/// not the Docker volume. The Docker volume is the authoritative source at
/// runtime, but the host file is the editable copy users interact with via
/// `claude mcp add` and reflects what will be mounted into the container at
/// session start. MCPs added via `claude mcp add` from inside a running
/// container only land in the volume; they will not appear in this picker
/// until `airlock config export` syncs the volume back to the host.
enum MCPInventoryService {
    static func discoverServerNames(
        homeDirectory: URL = FileManager.default.homeDirectoryForCurrentUser
    ) -> [String] {
        let claudeDir = homeDirectory.appendingPathComponent(".claude")
        let candidates = [
            claudeDir.appendingPathComponent("settings.json"),
            claudeDir.appendingPathComponent("settings.local.json"),
        ]

        var names = Set<String>()
        for url in candidates {
            guard let data = try? Data(contentsOf: url),
                  let json = try? JSONSerialization.jsonObject(with: data) as? [String: Any],
                  let mcpServers = json["mcpServers"] as? [String: Any] else {
                continue
            }
            for name in mcpServers.keys {
                names.insert(name)
            }
        }
        return names.sorted()
    }
}
