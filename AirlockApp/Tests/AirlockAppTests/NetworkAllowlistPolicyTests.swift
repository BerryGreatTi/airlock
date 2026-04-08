import XCTest
@testable import AirlockApp

final class NetworkAllowlistPolicyTests: XCTestCase {
    func testEmptyAllowlistAllowsEverything() {
        XCTAssertTrue(NetworkAllowlistPolicy.isAllowed(host: "api.example.com", allowlist: []))
        XCTAssertTrue(NetworkAllowlistPolicy.isAllowed(host: "anything.goes", allowlist: []))
    }

    func testExactHostMatch() {
        let list = ["api.github.com"]
        XCTAssertTrue(NetworkAllowlistPolicy.isAllowed(host: "api.github.com", allowlist: list))
        XCTAssertFalse(NetworkAllowlistPolicy.isAllowed(host: "api.stripe.com", allowlist: list))
    }

    func testSuffixWildcardMatchesSubdomains() {
        let list = ["*.stripe.com"]
        XCTAssertTrue(NetworkAllowlistPolicy.isAllowed(host: "api.stripe.com", allowlist: list))
        XCTAssertTrue(NetworkAllowlistPolicy.isAllowed(host: "checkout.stripe.com", allowlist: list))
        XCTAssertTrue(NetworkAllowlistPolicy.isAllowed(host: "deeply.nested.stripe.com", allowlist: list))
    }

    func testSuffixWildcardDoesNotMatchBareDomain() {
        let list = ["*.stripe.com"]
        XCTAssertFalse(NetworkAllowlistPolicy.isAllowed(host: "stripe.com", allowlist: list))
    }

    func testSuffixWildcardDoesNotMatchUnrelatedSuffix() {
        let list = ["*.stripe.com"]
        XCTAssertFalse(NetworkAllowlistPolicy.isAllowed(host: "notstripe.com", allowlist: list))
        XCTAssertFalse(NetworkAllowlistPolicy.isAllowed(host: "stripe.com.evil.example", allowlist: list))
    }

    func testCaseInsensitive() {
        let list = ["API.GITHUB.COM"]
        XCTAssertTrue(NetworkAllowlistPolicy.isAllowed(host: "api.github.com", allowlist: list))
    }

    func testMissingProtectedHostsEmptyWhenAllowlistIsEmpty() {
        XCTAssertEqual(NetworkAllowlistPolicy.missingProtectedHosts(from: []), [])
    }

    func testMissingProtectedHostsDetectsBothAnthropicMissing() {
        let list = ["api.github.com"]
        let missing = NetworkAllowlistPolicy.missingProtectedHosts(from: list)
        XCTAssertEqual(missing, ["api.anthropic.com", "auth.anthropic.com"])
    }

    func testMissingProtectedHostsDetectsOneAnthropicMissing() {
        let list = ["api.anthropic.com", "api.github.com"]
        let missing = NetworkAllowlistPolicy.missingProtectedHosts(from: list)
        XCTAssertEqual(missing, ["auth.anthropic.com"])
    }

    func testMissingProtectedHostsWildcardCoversBothAnthropicHosts() {
        // Users who type `*.anthropic.com` should NOT be warned — both
        // protected hosts are covered by the wildcard.
        let list = ["*.anthropic.com"]
        XCTAssertEqual(NetworkAllowlistPolicy.missingProtectedHosts(from: list), [])
    }

    func testSplitHostLinesStripsEmptyAndWhitespace() {
        let input = "api.github.com\n  *.stripe.com  \n\n"
        XCTAssertEqual(
            NetworkAllowlistPolicy.splitHostLines(input),
            ["api.github.com", "*.stripe.com"]
        )
    }
}
