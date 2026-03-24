import XCTest
@testable import AirlockApp

final class WorkspaceStoreTests: XCTestCase {
    var tempDir: URL!

    override func setUp() {
        tempDir = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
        try! FileManager.default.createDirectory(at: tempDir, withIntermediateDirectories: true)
    }
    override func tearDown() { try? FileManager.default.removeItem(at: tempDir) }

    func testSaveAndLoadWorkspaces() throws {
        let store = WorkspaceStore(directory: tempDir)
        let ws = Workspace(name: "test", path: "/tmp/test")
        try store.saveWorkspaces([ws])
        let loaded = try store.loadWorkspaces()
        XCTAssertEqual(loaded.count, 1)
        XCTAssertEqual(loaded[0].id, ws.id)
    }

    func testLoadEmptyReturnsEmpty() throws {
        XCTAssertEqual(try WorkspaceStore(directory: tempDir).loadWorkspaces().count, 0)
    }

    func testSaveAndLoadSettings() throws {
        let store = WorkspaceStore(directory: tempDir)
        var settings = AppSettings()
        settings.containerImage = "custom:v2"
        try store.saveSettings(settings)
        XCTAssertEqual(try store.loadSettings().containerImage, "custom:v2")
    }

    func testDefaultSettings() throws {
        XCTAssertEqual(try WorkspaceStore(directory: tempDir).loadSettings().containerImage, "airlock-claude:latest")
    }
}
