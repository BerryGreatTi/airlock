import Foundation
import Observation
import SwiftUI

enum SessionStatus: Equatable {
    case stopped
    case running
    case error(String)
}

enum DetailTab: Hashable {
    case terminal
    case secrets
    case containers
    case settings
}

enum ActivationState: Equatable {
    case inactive
    case activating
    case active
}

@Observable
@MainActor
final class AppState {
    var workspaces: [Workspace] = []
    var selectedWorkspaceID: UUID?
    var activationStates: [UUID: ActivationState] = [:]
    var selectedTab: DetailTab = .terminal
    var lastError: String?
    var settings: AppSettings = AppSettings()

    private var tabSwitchTask: Task<Void, Never>?

    nonisolated init() {}

    var selectedWorkspace: Workspace? {
        workspaces.first { $0.id == selectedWorkspaceID }
    }

    func isActive(_ workspace: Workspace) -> Bool {
        activationStates[workspace.id] == .active
    }

    func isActivating(_ workspace: Workspace) -> Bool {
        activationStates[workspace.id] == .activating
    }

    func activationState(for workspace: Workspace) -> ActivationState {
        activationStates[workspace.id] ?? .inactive
    }

    func statusFor(_ workspace: Workspace) -> SessionStatus {
        guard let ws = workspaces.first(where: { $0.id == workspace.id }) else { return .stopped }
        switch activationStates[ws.id] {
        case .active:
            return ws.isActive ? .running : .error("activation failed")
        case .activating:
            return .running
        case .inactive, .none:
            return .stopped
        }
    }

    func switchTab(to tab: DetailTab) {
        tabSwitchTask?.cancel()
        tabSwitchTask = Task { @MainActor [weak self] in
            try? await Task.sleep(for: .milliseconds(150))
            guard !Task.isCancelled, let self else { return }
            self.selectedTab = tab
        }
    }

    func performActivation(
        workspace: Workspace,
        using service: ContainerSessionService
    ) async {
        activationStates[workspace.id] = .activating
        lastError = nil
        do {
            let store = WorkspaceStore()
            let settings = (try? store.loadSettings()) ?? AppSettings()
            let resolved = ResolvedSettings(global: settings, workspace: workspace)
            _ = try await service.activateAndWaitReady(workspace: workspace, resolved: resolved)
            activationStates[workspace.id] = .active
            if let idx = workspaces.firstIndex(where: { $0.id == workspace.id }) {
                workspaces[idx].isActive = true
            }
        } catch {
            activationStates[workspace.id] = .inactive
            lastError = error.localizedDescription
        }
    }

    func performDeactivation(
        workspace: Workspace,
        using service: ContainerSessionService
    ) async {
        await service.deactivate(workspace: workspace)
        activationStates[workspace.id] = .inactive
        if let idx = workspaces.firstIndex(where: { $0.id == workspace.id }) {
            workspaces[idx].isActive = false
        }
    }
}

enum AppTheme: String, Codable, CaseIterable {
    case system = "System"
    case light = "Light"
    case dark = "Dark"

    var colorScheme: ColorScheme? {
        switch self {
        case .system: return nil
        case .light: return .light
        case .dark: return .dark
        }
    }
}

struct TerminalSettings: Codable, Equatable {
    var fontName: String = "SF Mono"
    var fontSize: Double = 13

    static let availableFonts: [String] = [
        "SF Mono",
        "Menlo",
        "Monaco",
        "Courier New",
        "Andale Mono",
        "JetBrains Mono",
        "Fira Code",
        "Source Code Pro",
    ]
}

#if canImport(AppKit)
import AppKit

struct TerminalColors: Equatable {
    let background: NSColor
    let foreground: NSColor
    let caret: NSColor
    let caretText: NSColor?
    let selection: NSColor
    let useBrightColors: Bool

    static let dark = TerminalColors(
        background: NSColor(red: 0.12, green: 0.12, blue: 0.18, alpha: 1),
        foreground: NSColor(red: 0.80, green: 0.84, blue: 0.96, alpha: 1),
        caret: NSColor(red: 0.54, green: 0.71, blue: 0.98, alpha: 1),
        caretText: NSColor(red: 0.12, green: 0.12, blue: 0.18, alpha: 1),
        selection: NSColor(red: 0.54, green: 0.71, blue: 0.98, alpha: 0.25),
        useBrightColors: true
    )

    static let light = TerminalColors(
        background: NSColor(red: 0.94, green: 0.95, blue: 0.96, alpha: 1),
        foreground: NSColor(red: 0.30, green: 0.31, blue: 0.41, alpha: 1),
        caret: NSColor(red: 0.12, green: 0.40, blue: 0.96, alpha: 1),
        caretText: NSColor.white,
        selection: NSColor(red: 0.12, green: 0.40, blue: 0.96, alpha: 0.18),
        useBrightColors: false
    )

    static func forDarkMode(_ isDark: Bool) -> TerminalColors {
        isDark ? .dark : .light
    }
}
#endif

struct AppSettings: Codable, Equatable {
    var airlockBinaryPath: String?
    var containerImage: String = "airlock-claude:latest"
    var proxyImage: String = "airlock-proxy:latest"
    var passthroughHosts: [String] = []
    var theme: AppTheme = .system
    var terminal: TerminalSettings = TerminalSettings()
}

struct ResolvedSettings: Sendable {
    let containerImage: String
    let proxyImage: String
    let passthroughHosts: [String]
    let proxyPort: Int

    init(global: AppSettings, workspace: Workspace) {
        self.containerImage = workspace.containerImageOverride ?? global.containerImage
        self.proxyImage = workspace.proxyImageOverride ?? global.proxyImage
        self.passthroughHosts = workspace.passthroughHostsOverride ?? global.passthroughHosts
        self.proxyPort = workspace.proxyPortOverride ?? 8080
    }
}
