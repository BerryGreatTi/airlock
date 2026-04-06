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

    func testResolveBinaryPrefersExecutableExplicitPath() throws {
        let tempDir = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
        try FileManager.default.createDirectory(at: tempDir, withIntermediateDirectories: true)
        defer { try? FileManager.default.removeItem(at: tempDir) }

        let fakeBinary = tempDir.appendingPathComponent("airlock")
        FileManager.default.createFile(atPath: fakeBinary.path, contents: Data("#!/bin/sh\nexit 0\n".utf8))
        try FileManager.default.setAttributes([.posixPermissions: 0o755], ofItemAtPath: fakeBinary.path)

        let cli = CLIService(binaryPath: fakeBinary.path)
        XCTAssertEqual(cli.resolveAirlockBinary(), fakeBinary.path)
    }

    func testResolveBinaryIgnoresMissingExplicitPath() {
        let cli = CLIService(binaryPath: "/definitely/nonexistent/airlock")
        // Should fall through -- not return the missing path
        XCTAssertNotEqual(cli.resolveAirlockBinary(), "/definitely/nonexistent/airlock")
    }

    func testResolveBinaryIgnoresNonExecutableExplicitPath() throws {
        let tempDir = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
        try FileManager.default.createDirectory(at: tempDir, withIntermediateDirectories: true)
        defer { try? FileManager.default.removeItem(at: tempDir) }

        let notExecutable = tempDir.appendingPathComponent("airlock")
        FileManager.default.createFile(atPath: notExecutable.path, contents: Data("not executable".utf8))
        // No chmod +x

        let cli = CLIService(binaryPath: notExecutable.path)
        XCTAssertNotEqual(cli.resolveAirlockBinary(), notExecutable.path)
    }
}
