import Foundation

struct CLIResult {
    let exitCode: Int32
    let stdout: String
    let stderr: String
}

final class CLIService {
    private let binaryPath: String?

    init(binaryPath: String? = nil) {
        self.binaryPath = binaryPath
    }

    func resolveAirlockBinary() -> String {
        if let explicit = binaryPath { return explicit }
        if let found = Self.findInPath("airlock") { return found }
        return "/usr/local/bin/airlock"
    }

    func run(args: [String], workingDirectory: String) async throws -> CLIResult {
        let process = Process()
        process.executableURL = URL(filePath: resolveAirlockBinary())
        process.arguments = args
        process.currentDirectoryURL = URL(filePath: workingDirectory)
        process.environment = Self.enrichedEnvironment()
        let stdoutPipe = Pipe()
        let stderrPipe = Pipe()
        process.standardOutput = stdoutPipe
        process.standardError = stderrPipe
        try process.run()
        process.waitUntilExit()
        return CLIResult(
            exitCode: process.terminationStatus,
            stdout: String(data: stdoutPipe.fileHandleForReading.readDataToEndOfFile(), encoding: .utf8) ?? "",
            stderr: String(data: stderrPipe.fileHandleForReading.readDataToEndOfFile(), encoding: .utf8) ?? ""
        )
    }

    func isAirlockInitialized(path: String) -> Bool {
        FileManager.default.fileExists(atPath: (path as NSString).appendingPathComponent(".airlock"))
    }

    static func findInPath(_ name: String) -> String? {
        let paths = (ProcessInfo.processInfo.environment["PATH"] ?? "").components(separatedBy: ":")
        for dir in paths {
            let full = (dir as NSString).appendingPathComponent(name)
            if FileManager.default.isExecutableFile(atPath: full) { return full }
        }
        return nil
    }

    static func enrichedEnvironment() -> [String: String] {
        var env = ProcessInfo.processInfo.environment
        let extraPaths = ["/usr/local/bin", "/opt/homebrew/bin"]
        let currentPath = env["PATH"] ?? ""
        let missing = extraPaths.filter { !currentPath.contains($0) }
        if !missing.isEmpty {
            env["PATH"] = (missing + [currentPath]).joined(separator: ":")
        }
        if env["DOCKER_HOST"] == nil {
            let home = FileManager.default.homeDirectoryForCurrentUser.path
            let candidates = [
                "/var/run/docker.sock",
                "\(home)/.rd/docker.sock",
                "\(home)/.colima/docker.sock",
                "\(home)/.docker/run/docker.sock",
            ]
            if let sock = candidates.first(where: { FileManager.default.fileExists(atPath: $0) }) {
                env["DOCKER_HOST"] = "unix://\(sock)"
            }
        }
        return env
    }
}
