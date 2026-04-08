import XCTest
@testable import AirlockApp

final class MCPInventoryServiceTests: XCTestCase {
    private func makeHomeWithSettings(_ settings: [String: Any], local: [String: Any]? = nil) throws -> URL {
        let home = URL(fileURLWithPath: NSTemporaryDirectory())
            .appendingPathComponent("airlock-mcp-test-\(UUID().uuidString)")
        let claudeDir = home.appendingPathComponent(".claude")
        try FileManager.default.createDirectory(at: claudeDir, withIntermediateDirectories: true)
        let data = try JSONSerialization.data(withJSONObject: settings)
        try data.write(to: claudeDir.appendingPathComponent("settings.json"))
        if let local {
            let localData = try JSONSerialization.data(withJSONObject: local)
            try localData.write(to: claudeDir.appendingPathComponent("settings.local.json"))
        }
        return home
    }

    func testDiscoversMCPServersFromGlobalSettings() throws {
        let home = try makeHomeWithSettings([
            "mcpServers": [
                "slack": ["command": "npx"],
                "github": ["command": "npx"],
            ],
        ])
        let names = MCPInventoryService.discoverServerNames(homeDirectory: home)
        XCTAssertEqual(names, ["github", "slack"])
    }

    func testReturnsEmptyArrayWhenNoSettingsFile() {
        let home = URL(fileURLWithPath: NSTemporaryDirectory())
            .appendingPathComponent("airlock-mcp-empty-\(UUID().uuidString)")
        let names = MCPInventoryService.discoverServerNames(homeDirectory: home)
        XCTAssertEqual(names, [])
    }

    func testReturnsEmptyArrayWhenSettingsHasNoMCPServers() throws {
        let home = try makeHomeWithSettings(["env": ["FOO": "bar"]])
        let names = MCPInventoryService.discoverServerNames(homeDirectory: home)
        XCTAssertEqual(names, [])
    }

    func testMergesGlobalAndLocalSettings() throws {
        let home = try makeHomeWithSettings(
            ["mcpServers": ["slack": ["command": "npx"]]],
            local: ["mcpServers": ["jira": ["command": "npx"]]]
        )
        let names = MCPInventoryService.discoverServerNames(homeDirectory: home)
        XCTAssertEqual(names, ["jira", "slack"])
    }

    func testDeduplicatesAcrossSettingsFiles() throws {
        let home = try makeHomeWithSettings(
            ["mcpServers": ["slack": ["command": "npx"]]],
            local: ["mcpServers": ["slack": ["command": "npx-override"]]]
        )
        let names = MCPInventoryService.discoverServerNames(homeDirectory: home)
        XCTAssertEqual(names, ["slack"])
    }
}
