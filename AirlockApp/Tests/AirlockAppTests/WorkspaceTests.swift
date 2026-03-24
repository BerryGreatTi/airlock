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
}
