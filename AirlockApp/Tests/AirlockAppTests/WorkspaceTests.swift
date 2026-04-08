import XCTest
@testable import AirlockApp

final class WorkspaceTests: XCTestCase {
    func testWorkspaceCreation() {
        let ws = Workspace(name: "my-project", path: "/Users/test/my-project")
        XCTAssertFalse(ws.id.uuidString.isEmpty)
        XCTAssertEqual(ws.name, "my-project")
        XCTAssertNil(ws.envFilePath)
        XCTAssertNil(ws.containerImageOverride)
    }

    func testWorkspaceCodable() throws {
        let ws = Workspace(name: "test", path: "/tmp/test", envFilePath: "/tmp/.env")
        let data = try JSONEncoder().encode(ws)
        let decoded = try JSONDecoder().decode(Workspace.self, from: data)
        XCTAssertEqual(decoded.id, ws.id)
        XCTAssertEqual(decoded.name, ws.name)
        XCTAssertEqual(decoded.envFilePath, ws.envFilePath)
    }

    func testShortIDFormat() {
        let ws = Workspace(name: "test", path: "/tmp")
        XCTAssertEqual(ws.shortID.count, 8)
        XCTAssertEqual(ws.containerName, "airlock-claude-\(ws.shortID)")
        XCTAssertEqual(ws.proxyName, "airlock-proxy-\(ws.shortID)")
    }

    func testRuntimeFieldsNotEncoded() throws {
        var ws = Workspace(name: "test", path: "/tmp")
        ws.isActive = true
        ws.containerId = "abc"
        ws.proxyId = "def"
        let data = try JSONEncoder().encode(ws)
        let decoded = try JSONDecoder().decode(Workspace.self, from: data)
        XCTAssertFalse(decoded.isActive)
        XCTAssertNil(decoded.containerId)
        XCTAssertNil(decoded.proxyId)
    }

    func testOverrideFieldsDefaultToNil() {
        let ws = Workspace(name: "test", path: "/tmp")
        XCTAssertNil(ws.proxyImageOverride)
        XCTAssertNil(ws.passthroughHostsOverride)
        XCTAssertNil(ws.proxyPortOverride)
        XCTAssertNil(ws.enabledMCPServersOverride)
        XCTAssertNil(ws.networkAllowlistOverride)
    }

    func testOverrideFieldsPersisted() throws {
        var ws = Workspace(name: "test", path: "/tmp")
        ws.proxyImageOverride = "custom-proxy:v2"
        ws.passthroughHostsOverride = ["api.example.com"]
        ws.proxyPortOverride = 9090
        ws.enabledMCPServersOverride = ["slack", "github"]
        ws.networkAllowlistOverride = ["api.github.com", "*.stripe.com"]
        let data = try JSONEncoder().encode(ws)
        let decoded = try JSONDecoder().decode(Workspace.self, from: data)
        XCTAssertEqual(decoded.proxyImageOverride, "custom-proxy:v2")
        XCTAssertEqual(decoded.passthroughHostsOverride, ["api.example.com"])
        XCTAssertEqual(decoded.proxyPortOverride, 9090)
        XCTAssertEqual(decoded.enabledMCPServersOverride, ["slack", "github"])
        XCTAssertEqual(decoded.networkAllowlistOverride, ["api.github.com", "*.stripe.com"])
    }

    func testEmptyMCPOverrideMeansNoneEnabled() throws {
        // Empty array is a valid explicit override (none enabled),
        // distinct from nil (inherit global).
        var ws = Workspace(name: "test", path: "/tmp")
        ws.enabledMCPServersOverride = []
        let data = try JSONEncoder().encode(ws)
        let decoded = try JSONDecoder().decode(Workspace.self, from: data)
        XCTAssertNotNil(decoded.enabledMCPServersOverride)
        XCTAssertEqual(decoded.enabledMCPServersOverride, [])
    }

    func testBackwardsCompatDecoding() throws {
        // Simulate old workspaces.json without new fields
        let json = """
        {"id":"12345678-1234-1234-1234-123456789012","name":"old","path":"/tmp"}
        """
        let data = json.data(using: .utf8)!
        let decoded = try JSONDecoder().decode(Workspace.self, from: data)
        XCTAssertEqual(decoded.name, "old")
        XCTAssertNil(decoded.proxyImageOverride)
        XCTAssertNil(decoded.passthroughHostsOverride)
        XCTAssertNil(decoded.proxyPortOverride)
        XCTAssertNil(decoded.enabledMCPServersOverride)
        XCTAssertNil(decoded.networkAllowlistOverride)
    }

    func testTerminalSessionCreation() {
        let session = TerminalSession()
        XCTAssertTrue(session.isActive)
        XCTAssertFalse(session.id.uuidString.isEmpty)
    }
}
