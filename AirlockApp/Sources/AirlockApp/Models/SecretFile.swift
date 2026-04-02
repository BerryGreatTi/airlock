import Foundation

enum SecretFileFormat: String, Codable, CaseIterable {
    case dotenv
    case json
    case yaml
    case ini
    case properties
    case plaintext

    var displayName: String {
        switch self {
        case .dotenv: return "Dotenv"
        case .json: return "JSON"
        case .yaml: return "YAML"
        case .ini: return "INI"
        case .properties: return "Properties"
        case .plaintext: return "Plain Text"
        }
    }

    var iconName: String {
        switch self {
        case .dotenv: return "doc.text"
        case .json: return "curlybraces"
        case .yaml: return "list.bullet.indent"
        case .ini: return "gearshape"
        case .properties: return "list.dash"
        case .plaintext: return "lock.doc"
        }
    }

    static func detect(from path: String) -> SecretFileFormat {
        let name = (path as NSString).lastPathComponent
        let ext = (path as NSString).pathExtension.lowercased()

        if name == ".env" || name.hasPrefix(".env.") || ext == "env" {
            return .dotenv
        }
        switch ext {
        case "json": return .json
        case "yaml", "yml": return .yaml
        case "ini", "cfg": return .ini
        case "properties": return .properties
        default: return .plaintext
        }
    }
}

struct SecretFile: Identifiable, Hashable {
    let id: UUID
    let path: String
    let format: SecretFileFormat
    var label: String { (path as NSString).lastPathComponent }

    init(path: String, format: SecretFileFormat? = nil) {
        self.id = UUID()
        self.path = path
        self.format = format ?? SecretFileFormat.detect(from: path)
    }

    init(path: String, formatString: String) {
        self.id = UUID()
        self.path = path
        self.format = SecretFileFormat(rawValue: formatString) ?? SecretFileFormat.detect(from: path)
    }
}
