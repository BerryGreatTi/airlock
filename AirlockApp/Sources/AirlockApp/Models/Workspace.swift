import Foundation

struct Workspace: Identifiable, Codable, Hashable {
    let id: UUID
    var name: String
    var path: String
    var envFilePath: String?
    var containerImageOverride: String?

    init(name: String, path: String, envFilePath: String? = nil, containerImageOverride: String? = nil) {
        self.id = UUID()
        self.name = name
        self.path = path
        self.envFilePath = envFilePath
        self.containerImageOverride = containerImageOverride
    }
}
