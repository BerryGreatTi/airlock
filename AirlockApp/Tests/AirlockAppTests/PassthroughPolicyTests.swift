import XCTest
@testable import AirlockApp

final class PassthroughPolicyTests: XCTestCase {
    func testBothPresentReturnsEmpty() {
        let missing = PassthroughPolicy.missingProtectedHosts(
            from: ["api.anthropic.com", "auth.anthropic.com"]
        )
        XCTAssertEqual(missing, [])
    }

    func testEmptyListReturnsBoth() {
        let missing = PassthroughPolicy.missingProtectedHosts(from: [])
        XCTAssertEqual(missing.sorted(), ["api.anthropic.com", "auth.anthropic.com"])
    }

    func testApiMissing() {
        let missing = PassthroughPolicy.missingProtectedHosts(from: ["auth.anthropic.com"])
        XCTAssertEqual(missing, ["api.anthropic.com"])
    }

    func testAuthMissing() {
        let missing = PassthroughPolicy.missingProtectedHosts(from: ["api.anthropic.com"])
        XCTAssertEqual(missing, ["auth.anthropic.com"])
    }

    func testCaseInsensitive() {
        let missing = PassthroughPolicy.missingProtectedHosts(
            from: ["API.ANTHROPIC.COM", "Auth.Anthropic.Com"]
        )
        XCTAssertEqual(missing, [])
    }

    func testWhitespaceTolerant() {
        let missing = PassthroughPolicy.missingProtectedHosts(
            from: ["  api.anthropic.com  ", "\tauth.anthropic.com"]
        )
        XCTAssertEqual(missing, [])
    }

    func testUnrelatedHostsDoNotMatter() {
        let missing = PassthroughPolicy.missingProtectedHosts(
            from: ["github.com", "slack.com", "api.anthropic.com", "auth.anthropic.com"]
        )
        XCTAssertEqual(missing, [])
    }
}
