import AppKit
import Foundation

final class AppDelegate: NSObject, NSApplicationDelegate {
    private let containerService = ContainerSessionService()

    func applicationShouldTerminate(_ sender: NSApplication) -> NSApplication.TerminateReply {
        Task {
            await self.deactivateAllRunning()
            await MainActor.run {
                sender.reply(toApplicationShouldTerminate: true)
            }
        }
        return .terminateLater
    }

    func applicationShouldTerminateAfterLastWindowClosed(_ sender: NSApplication) -> Bool {
        true
    }

    private func deactivateAllRunning() async {
        guard let result = try? await containerService.status(),
              let data = result.stdout.data(using: .utf8),
              let json = try? JSONSerialization.jsonObject(with: data) as? [String: Any],
              let workspaces = json["workspaces"] as? [[String: Any]] else {
            return
        }

        let runningIDs = workspaces.compactMap { entry -> String? in
            guard let id = entry["id"] as? String,
                  let status = entry["status"] as? String,
                  status == "running" else { return nil }
            return id
        }

        await withTaskGroup(of: Void.self) { group in
            for id in runningIDs {
                group.addTask {
                    await self.containerService.stopByID(id)
                }
            }
        }
    }
}
