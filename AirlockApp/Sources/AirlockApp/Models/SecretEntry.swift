import Foundation

enum SecretStatus: String {
    case encrypted = "Encrypted"
    case plaintext = "Plaintext"
    case notSecret = "Not Secret"

    var color: String {
        switch self {
        case .encrypted: return "green"
        case .plaintext: return "orange"
        case .notSecret: return "secondary"
        }
    }
}

struct SecretEntry: Identifiable {
    let id: UUID = UUID()
    let path: String
    let value: String
    let encrypted: Bool
    let source: String
    let isEditable: Bool

    var status: SecretStatus {
        if encrypted { return .encrypted }
        let sensitivePatterns = ["KEY", "SECRET", "PASSWORD", "TOKEN", "CREDENTIAL", "AUTH"]
        let leaf = path.split(separator: "/").last.map(String.init) ?? path
        if sensitivePatterns.contains(where: { leaf.uppercased().contains($0) }) {
            return .plaintext
        }
        return .notSecret
    }

    var maskedValue: String {
        if encrypted { return "ENC[age:...]" }
        if status == .plaintext { return String(repeating: "*", count: min(value.count, 20)) }
        return value
    }
}
