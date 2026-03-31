import AppKit
import Foundation

final class AppDelegate: NSObject, NSApplicationDelegate {
    private let containerService = ContainerSessionService()

    func applicationShouldTerminate(_ sender: NSApplication) -> NSApplication.TerminateReply {
        Task.detached {
            // Race cleanup against a 10-second timeout so the app always quits
            await withTaskGroup(of: Void.self) { group in
                group.addTask { await self.deactivateAllRunning() }
                group.addTask { try? await Task.sleep(for: .seconds(10)) }
                await group.next()
                group.cancelAll()
            }
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
        let result: CLIResult
        do {
            result = try await containerService.status()
        } catch {
            NSLog("[Airlock] Failed to fetch container status during quit: %@", "\(error)")
            return
        }

        guard let data = result.stdout.data(using: .utf8),
              let json = try? JSONSerialization.jsonObject(with: data) as? [String: Any],
              let workspaces = json["workspaces"] as? [[String: Any]] else {
            NSLog("[Airlock] Could not parse container status output")
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
