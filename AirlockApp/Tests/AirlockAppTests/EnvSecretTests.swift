import XCTest
@testable import AirlockApp

final class EnvSecretTests: XCTestCase {
    func testParsesEnvSecretListJSON() throws {
        let json = """
        [
          {"name":"ALPHA"},
          {"name":"BRAVO"},
          {"name":"GITHUB_TOKEN"}
        ]
        """
        let data = Data(json.utf8)
        let parsed = try EnvSecret.decodeList(from: data)
        XCTAssertEqual(parsed.count, 3)
        XCTAssertEqual(parsed[0].name, "ALPHA")
        XCTAssertEqual(parsed[2].name, "GITHUB_TOKEN")
    }

    func testEnvSecretIDsAreStable() throws {
        let json = #"[{"name":"ALPHA"}]"#
        let a = try EnvSecret.decodeList(from: Data(json.utf8))
        let b = try EnvSecret.decodeList(from: Data(json.utf8))
        XCTAssertEqual(a[0].id, b[0].id, "Same name should produce same UUID")
    }

    func testValidNameRegex() {
        XCTAssertTrue(EnvSecret.isValidName("FOO"))
        XCTAssertTrue(EnvSecret.isValidName("FOO_BAR"))
        XCTAssertTrue(EnvSecret.isValidName("_PRIVATE"))
        XCTAssertTrue(EnvSecret.isValidName("a1"))
    }

    func testInvalidNameRegex() {
        XCTAssertFalse(EnvSecret.isValidName(""))
        XCTAssertFalse(EnvSecret.isValidName("1FOO"))
        XCTAssertFalse(EnvSecret.isValidName("FOO-BAR"))
        XCTAssertFalse(EnvSecret.isValidName("PATH=x"))
        XCTAssertFalse(EnvSecret.isValidName("FOO BAR"))
    }
}
