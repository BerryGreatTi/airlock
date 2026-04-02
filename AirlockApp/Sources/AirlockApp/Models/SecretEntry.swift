import Foundation

enum SecretStatus: String {
    case encrypted = "Encrypted"
    case plaintext = "Plaintext"
    case notSecret = "Not Secret"
}

struct SecretEntry: Identifiable {
    let id: UUID = UUID()
    let path: String
    let value: String
    let encrypted: Bool
    let isSecret: Bool
    let source: String
    let isEditable: Bool

    var status: SecretStatus {
        if encrypted { return .encrypted }
        if isSecret { return .plaintext }
        return .notSecret
    }

    var maskedValue: String {
        if encrypted { return "ENC[age:...]" }
        if isSecret { return String(repeating: "*", count: min(value.count, 20)) }
        return value
    }
}
