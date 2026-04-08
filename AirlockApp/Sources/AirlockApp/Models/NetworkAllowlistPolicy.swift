import Foundation

/// Network allow-list matching logic mirroring proxy/addon/decrypt_proxy.py's
/// `is_allowed`. Supports exact host match and suffix wildcard `*.example.com`
/// (which matches subdomains but NOT the bare `example.com` itself — cookie
/// scope semantics). Used by the GUI to drive the Anthropic guardrail so
/// users typing `*.anthropic.com` aren't nagged for "missing api.anthropic.com".
enum NetworkAllowlistPolicy {
    /// Return true if `host` matches any entry in `allowlist`.
    /// Empty list = allow all (back-compat default).
    static func isAllowed(host: String, allowlist: [String]) -> Bool {
        let normalized = allowlist
            .map { $0.trimmingCharacters(in: .whitespaces).lowercased() }
            .filter { !$0.isEmpty }
        if normalized.isEmpty {
            return true
        }
        let target = host.trimmingCharacters(in: .whitespaces).lowercased()
        for entry in normalized {
            if entry.hasPrefix("*.") {
                let suffix = String(entry.dropFirst()) // ".example.com"
                if target.hasSuffix(suffix) && target != String(suffix.dropFirst()) {
                    return true
                }
            } else if entry == target {
                return true
            }
        }
        return false
    }

    /// Protected hosts whose absence from a non-empty allow-list would mean
    /// the agent cannot reach Anthropic — usually unintended. Returned hosts
    /// are sorted for stable display.
    static func missingProtectedHosts(from allowlist: [String]) -> [String] {
        // Empty list = allow all, so nothing is "missing".
        let parsed = allowlist
            .map { $0.trimmingCharacters(in: .whitespaces) }
            .filter { !$0.isEmpty }
        if parsed.isEmpty {
            return []
        }
        return PassthroughPolicy.protectedHosts
            .filter { !isAllowed(host: $0, allowlist: parsed) }
            .sorted()
    }

    /// Split a newline-delimited allow-list editor string into trimmed,
    /// non-empty entries. Delegates to `PassthroughPolicy.splitHostLines`
    /// because the two editors share the same parsing contract.
    static func splitHostLines(_ text: String) -> [String] {
        PassthroughPolicy.splitHostLines(text)
    }
}
