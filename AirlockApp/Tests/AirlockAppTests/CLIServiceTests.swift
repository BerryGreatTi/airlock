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

    func testToolSearchPathsAddsMacOSCommonDirsWhenAbsent() {
        let paths = CLIService.toolSearchPaths(
            rawPath: "/usr/bin:/bin",
            homeDir: "/Users/test"
        )
        XCTAssertTrue(paths.contains("/usr/local/bin"))
        XCTAssertTrue(paths.contains("/opt/homebrew/bin"))
        XCTAssertTrue(paths.contains("/Users/test/.rd/bin"))
        XCTAssertTrue(paths.contains("/Users/test/.colima/bin"))
    }

    func testToolSearchPathsDeduplicates() {
        let paths = CLIService.toolSearchPaths(
            rawPath: "/opt/homebrew/bin:/usr/local/bin:/usr/bin",
            homeDir: "/Users/test"
        )
        XCTAssertEqual(paths.filter { $0 == "/opt/homebrew/bin" }.count, 1)
        XCTAssertEqual(paths.filter { $0 == "/usr/local/bin" }.count, 1)
    }

    func testToolSearchPathsPutsRawPathBeforeExtras() {
        let paths = CLIService.toolSearchPaths(
            rawPath: "/custom/bin:/usr/bin",
            homeDir: "/Users/test"
        )
        let customIdx = paths.firstIndex(of: "/custom/bin")
        let homebrewIdx = paths.firstIndex(of: "/opt/homebrew/bin")
        XCTAssertNotNil(customIdx)
        XCTAssertNotNil(homebrewIdx)
        XCTAssertLessThan(customIdx!, homebrewIdx!)
    }

    func testToolSearchPathsHandlesEmptyRawPath() {
        let paths = CLIService.toolSearchPaths(rawPath: "", homeDir: "/Users/test")
        XCTAssertEqual(paths, [
            "/usr/local/bin",
            "/opt/homebrew/bin",
            "/Users/test/.rd/bin",
            "/Users/test/.colima/bin",
        ])
    }

    func testFindInPathWithCustomSearchPathsLocatesBinary() throws {
        let tempDir = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
        try FileManager.default.createDirectory(at: tempDir, withIntermediateDirectories: true)
        defer { try? FileManager.default.removeItem(at: tempDir) }

        let fakeBinary = tempDir.appendingPathComponent("fake-docker")
        FileManager.default.createFile(atPath: fakeBinary.path, contents: Data("#!/bin/sh\nexit 0\n".utf8))
        try FileManager.default.setAttributes([.posixPermissions: 0o755], ofItemAtPath: fakeBinary.path)

        let found = CLIService.findInPath("fake-docker", searchPaths: [tempDir.path])
        XCTAssertEqual(found, fakeBinary.path)
    }

    func testFindInPathWithCustomSearchPathsSkipsNonExecutable() throws {
        let tempDir = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
        try FileManager.default.createDirectory(at: tempDir, withIntermediateDirectories: true)
        defer { try? FileManager.default.removeItem(at: tempDir) }

        let nonExec = tempDir.appendingPathComponent("fake-tool")
        FileManager.default.createFile(atPath: nonExec.path, contents: Data("nope".utf8))
        // No chmod +x

        XCTAssertNil(CLIService.findInPath("fake-tool", searchPaths: [tempDir.path]))
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
