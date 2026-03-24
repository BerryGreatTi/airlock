import Foundation

final class WorkspaceStore {
    private let directory: URL

    init(directory: URL? = nil) {
        if let dir = directory {
            self.directory = dir
        } else {
            let appSupport = FileManager.default.urls(for: .applicationSupportDirectory, in: .userDomainMask)[0]
            self.directory = appSupport.appendingPathComponent("Airlock")
        }
        try? FileManager.default.createDirectory(at: self.directory, withIntermediateDirectories: true)
    }

    func saveWorkspaces(_ workspaces: [Workspace]) throws {
        let data = try JSONEncoder().encode(workspaces)
        try data.write(to: directory.appendingPathComponent("workspaces.json"))
    }

    func loadWorkspaces() throws -> [Workspace] {
        let path = directory.appendingPathComponent("workspaces.json")
        guard FileManager.default.fileExists(atPath: path.path) else { return [] }
        return try JSONDecoder().decode([Workspace].self, from: Data(contentsOf: path))
    }

    func saveSettings(_ settings: AppSettings) throws {
        let data = try JSONEncoder().encode(settings)
        try data.write(to: directory.appendingPathComponent("settings.json"))
    }

    func loadSettings() throws -> AppSettings {
        let path = directory.appendingPathComponent("settings.json")
        guard FileManager.default.fileExists(atPath: path.path) else { return AppSettings() }
        return try JSONDecoder().decode(AppSettings.self, from: Data(contentsOf: path))
    }
}
