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

    func testLoadLegacySettingsMissingThemeAndTerminal() throws {
        let store = WorkspaceStore(directory: tempDir)
        let json = """
        {"containerImage":"custom:v1","proxyImage":"proxy:v1","passthroughHosts":["api.example.com"]}
        """
        try json.data(using: .utf8)!.write(to: tempDir.appendingPathComponent("settings.json"))
        let loaded = try store.loadSettings()
        XCTAssertEqual(loaded.containerImage, "custom:v1")
        XCTAssertEqual(loaded.proxyImage, "proxy:v1")
        XCTAssertEqual(loaded.passthroughHosts, ["api.example.com"])
        XCTAssertEqual(loaded.theme, .system)
        XCTAssertEqual(loaded.terminal.fontName, "SF Mono")
        XCTAssertEqual(loaded.terminal.fontSize, 13)
    }

    func testLoadSettingsWithUnknownKeysSucceeds() throws {
        let store = WorkspaceStore(directory: tempDir)
        let json = """
        {"containerImage":"test:v1","futureKey":"ignored"}
        """
        try json.data(using: .utf8)!.write(to: tempDir.appendingPathComponent("settings.json"))
        let loaded = try store.loadSettings()
        XCTAssertEqual(loaded.containerImage, "test:v1")
        XCTAssertEqual(loaded.theme, .system)
    }

    func testSettingsRoundTripWithThemeAndTerminal() throws {
        let store = WorkspaceStore(directory: tempDir)
        var settings = AppSettings()
        settings.theme = .dark
        settings.terminal.fontName = "Menlo"
        settings.terminal.fontSize = 16
        try store.saveSettings(settings)
        let loaded = try store.loadSettings()
        XCTAssertEqual(loaded.theme, .dark)
        XCTAssertEqual(loaded.terminal.fontName, "Menlo")
        XCTAssertEqual(loaded.terminal.fontSize, 16)
    }
}
