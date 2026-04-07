import Foundation

/// Single source of truth for the proxy passthrough hosts that
/// must remain in the list to preserve airlock's privacy property:
/// the model and Anthropic's servers must never see plaintext
/// secrets, which requires the proxy to NOT substitute on outbound
/// requests to api.anthropic.com / auth.anthropic.com.
///
/// Removing either host causes airlock-proxy to begin substituting
/// ENC[age:...] tokens with plaintext on the wire to Anthropic,
/// defeating the privacy property.
///
/// Mirrors the default in proxy/addon/decrypt_proxy.py and ADR-0005.
enum PassthroughPolicy {
    static let protectedHosts: Set<String> = [
        "api.anthropic.com",
        "auth.anthropic.com",
    ]

    /// Returns protected hosts missing from the given list.
    /// Comparison is case-insensitive and whitespace-tolerant.
    /// Returned slice is sorted alphabetically for stable display.
    static func missingProtectedHosts(from list: [String]) -> [String] {
        let normalized = Set(
            list.map { $0.trimmingCharacters(in: .whitespacesAndNewlines).lowercased() }
        )
        return protectedHosts
            .filter { !normalized.contains($0) }
            .sorted()
    }
}
