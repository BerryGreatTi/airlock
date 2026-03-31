import XCTest
@testable import AirlockApp

final class CLIServiceTests: XCTestCase {
    func testIsAirlockInitialized() {
        let tempDir = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
        try! FileManager.default.createDirectory(at: tempDir, withIntermediateDirectories: true)
        defer { try? FileManager.default.removeItem(at: tempDir) }

        let cli = CLIService()
        XCTAssertFalse(cli.isAirlockInitialized(path: tempDir.path))

        try! FileManager.default.createDirectory(at: tempDir.appendingPathComponent(".airlock"), withIntermediateDirectories: true)
        XCTAssertTrue(cli.isAirlockInitialized(path: tempDir.path))
    }

    func testFindBinaryInPath() {
        XCTAssertNotNil(CLIService.findInPath("git"))
        XCTAssertNil(CLIService.findInPath("nonexistent_binary_xyz"))
    }
}
