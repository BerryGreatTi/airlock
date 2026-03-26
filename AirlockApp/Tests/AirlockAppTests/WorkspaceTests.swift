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

    func testTerminalSessionCreation() {
        let session = TerminalSession()
        XCTAssertTrue(session.isActive)
        XCTAssertFalse(session.id.uuidString.isEmpty)
    }
}
