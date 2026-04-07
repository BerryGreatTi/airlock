import Foundation
import CryptoKit

/// In-memory representation of an environment-variable secret as
/// surfaced by `airlock secret env list --json`. Intentionally does
/// NOT carry the value: the GUI never holds plaintext or ciphertext
/// in long-lived state, only displays truncated prefixes via the
/// `secret env show` CLI when needed.
struct EnvSecret: Identifiable, Hashable {
    let id: UUID
    let name: String

    init(name: String) {
        self.name = name
        // Deterministic UUID derived from a SHA-256 of the name so
        // SwiftUI's selection state is preserved across reloads.
        self.id = EnvSecret.deterministicID(for: name)
    }

    private static func deterministicID(for name: String) -> UUID {
        let digest = SHA256.hash(data: Data(name.utf8))
        var bytes = [UInt8](repeating: 0, count: 16)
        for (i, byte) in digest.enumerated() where i < 16 {
            bytes[i] = byte
        }
        return UUID(uuid: (
            bytes[0], bytes[1], bytes[2], bytes[3],
            bytes[4], bytes[5], bytes[6], bytes[7],
            bytes[8], bytes[9], bytes[10], bytes[11],
            bytes[12], bytes[13], bytes[14], bytes[15]
        ))
    }

    /// Decode a JSON array of `{"name": "..."}` objects from
    /// `airlock secret env list --json`.
    static func decodeList(from data: Data) throws -> [EnvSecret] {
        struct Wire: Decodable { let name: String }
        let wire = try JSONDecoder().decode([Wire].self, from: data)
        return wire.map { EnvSecret(name: $0.name) }
    }

    /// Mirror of the Go-side regex `^[A-Za-z_][A-Za-z0-9_]*$`.
    /// Used by the AddEnvSecretSheet to disable Add until the name
    /// is valid, so the user gets immediate feedback.
    static func isValidName(_ name: String) -> Bool {
        guard !name.isEmpty else { return false }
        let chars = Array(name)
        let first = chars[0]
        let firstOK = first.isLetter || first == "_"
        guard firstOK else { return false }
        for c in chars.dropFirst() {
            let ok = c.isLetter || c.isNumber || c == "_"
            if !ok { return false }
        }
        return true
    }
}
