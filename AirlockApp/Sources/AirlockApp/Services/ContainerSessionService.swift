import Foundation
import SwiftUI

private struct ContainerSessionServiceKey: EnvironmentKey {
    static let defaultValue = ContainerSessionService()
}

extension EnvironmentValues {
    var containerService: ContainerSessionService {
        get { self[ContainerSessionServiceKey.self] }
        set { self[ContainerSessionServiceKey.self] = newValue }
    }
}

final class ContainerSessionService {
    private let cli: CLIService

    init(cli: CLIService = CLIService()) {
        self.cli = cli
    }

    func activate(workspace: Workspace, resolved: ResolvedSettings) async throws -> CLIResult {
        var args = ["start", "--id", workspace.shortID]
        if let envFile = workspace.envFilePath {
            args += ["--env", envFile]
        }
        args += ["--passthrough-hosts", resolved.passthroughHosts.joined(separator: ",")]
        args += ["--proxy-port", String(resolved.proxyPort)]
        args += ["--container-image", resolved.containerImage]
        args += ["--proxy-image", resolved.proxyImage]
        // Only pass --enabled-mcps when an explicit allow-list is set; nil
        // means "no filtering" and we leave the flag off so the Go CLI keeps
        // the existing behavior of forwarding all MCPs from settings.json.
        if let mcpAllowlist = resolved.enabledMCPServers {
            args += ["--enabled-mcps", mcpAllowlist.joined(separator: ",")]
        }
        // Same semantic for the network allow-list: absent flag = keep
        // config.yaml value (and back-compat default of allow-all).
        if let netAllowlist = resolved.networkAllowlist {
            args += ["--network-allowlist", netAllowlist.joined(separator: ",")]
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

    func stopByID(_ id: String) async {
        let home = FileManager.default.homeDirectoryForCurrentUser.path
        _ = try? await cli.run(args: ["stop", "--id", id], workingDirectory: home)
    }

    func isDockerRunning() async -> Bool {
        guard let dockerPath = CLIService.findInPath("docker") else { return false }
        return await withCheckedContinuation { continuation in
            let process = Process()
            process.executableURL = URL(fileURLWithPath: dockerPath)
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

    func activateAndWaitReady(workspace: Workspace, resolved: ResolvedSettings) async throws -> CLIResult {
        let result = try await activate(workspace: workspace, resolved: resolved)
        try await waitForContainerReady(containerName: workspace.containerName)
        return result
    }

    func waitForContainerReady(
        containerName: String,
        timeout: Duration = .seconds(10)
    ) async throws {
        let start = ContinuousClock.now
        while ContinuousClock.now - start < timeout {
            if await canExec(containerName: containerName) {
                return
            }
            try await Task.sleep(for: .milliseconds(500))
        }
        throw NSError(
            domain: "ContainerSession",
            code: -1,
            userInfo: [NSLocalizedDescriptionKey: "Container not ready after \(timeout)"]
        )
    }

    private func canExec(containerName: String) async -> Bool {
        guard let dockerPath = CLIService.findInPath("docker") else { return false }
        return await withCheckedContinuation { continuation in
            let process = Process()
            process.executableURL = URL(fileURLWithPath: dockerPath)
            process.arguments = ["exec", containerName, "true"]
            process.environment = CLIService.enrichedEnvironment()
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
