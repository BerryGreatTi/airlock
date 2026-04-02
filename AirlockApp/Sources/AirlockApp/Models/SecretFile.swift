import Foundation

enum SecretFileFormat: String, Codable, CaseIterable {
    case dotenv
    case json
    case yaml
    case ini
    case properties
    case text

    var displayName: String {
        switch self {
        case .dotenv: return "Dotenv"
        case .json: return "JSON"
        case .yaml: return "YAML"
        case .ini: return "INI"
        case .properties: return "Properties"
        case .text: return "Plain Text"
        }
    }

    var iconName: String {
        switch self {
        case .dotenv: return "doc.text"
        case .json: return "curlybraces"
        case .yaml: return "list.bullet.indent"
        case .ini: return "gearshape"
        case .properties: return "list.dash"
        case .text: return "lock.doc"
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
        default: return .text
        }
    }
}

struct SecretFile: Identifiable, Hashable {
    let id: UUID
    let path: String
    let format: SecretFileFormat
    var label: String { (path as NSString).lastPathComponent }

    // Deterministic UUID from path so selection survives reloads
    private static func stableID(for path: String) -> UUID {
        let data = Data(path.utf8)
        var bytes = [UInt8](repeating: 0, count: 16)
        let hashable = data.withUnsafeBytes { ptr -> Int in
            var hasher = Hasher()
            hasher.combine(bytes: UnsafeRawBufferPointer(ptr))
            return hasher.finalize()
        }
        withUnsafeBytes(of: hashable) { src in
            for i in 0..<min(src.count, 16) { bytes[i] = src[i] }
        }
        // Set UUID version 4 bits for format compliance
        bytes[6] = (bytes[6] & 0x0F) | 0x40
        bytes[8] = (bytes[8] & 0x3F) | 0x80
        return UUID(uuid: (bytes[0], bytes[1], bytes[2], bytes[3],
                           bytes[4], bytes[5], bytes[6], bytes[7],
                           bytes[8], bytes[9], bytes[10], bytes[11],
                           bytes[12], bytes[13], bytes[14], bytes[15]))
    }

    init(path: String, format: SecretFileFormat? = nil) {
        self.id = Self.stableID(for: path)
        self.path = path
        self.format = format ?? SecretFileFormat.detect(from: path)
    }

    init(path: String, formatString: String) {
        self.id = Self.stableID(for: path)
        self.path = path
        self.format = SecretFileFormat(rawValue: formatString) ?? SecretFileFormat.detect(from: path)
    }
}
