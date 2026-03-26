import Foundation

final class ContainerSessionService {
    private let cli: CLIService

    init(cli: CLIService = CLIService()) {
        self.cli = cli
    }

    func activate(workspace: Workspace) async throws -> CLIResult {
        var args = ["start", "--id", workspace.shortID]
        if let envFile = workspace.envFilePath {
            args += ["--env", envFile]
        }
        let result = try await cli.run(args: args, workingDirectory: workspace.path)
        if result.exitCode != 0 {
            throw NSError(
                domain: "ContainerSession",
                code: Int(result.exitCode),
                userInfo: [NSLocalizedDescriptionKey: result.stderr.isEmpty ? "activation failed" : result.stderr]
            )
        }
        return result
    }

    func deactivate(workspace: Workspace) async {
        _ = try? await cli.run(args: ["stop", "--id", workspace.shortID], workingDirectory: workspace.path)
    }

    func status() async throws -> CLIResult {
        let home = FileManager.default.homeDirectoryForCurrentUser.path
        return try await cli.run(args: ["status"], workingDirectory: home)
    }

    func isDockerRunning() async -> Bool {
        await withCheckedContinuation { continuation in
            let process = Process()
            process.executableURL = URL(fileURLWithPath: "/usr/local/bin/docker")
            process.arguments = ["info"]
            process.standardOutput = FileHandle.nullDevice
            process.standardError = FileHandle.nullDevice
            process.terminationHandler = { p in
                continuation.resume(returning: p.terminationStatus == 0)
            }
            do {
                try process.run()
            } catch {
                continuation.resume(returning: false)
            }
        }
    }
}
