import XCTest
@testable import AirlockApp

@MainActor
final class AppStateTests: XCTestCase {
    func testSelectedWorkspace() {
        let state = AppState()
        let ws = Workspace(name: "test", path: "/tmp")
        state.workspaces = [ws]
        state.selectedWorkspaceID = ws.id
        XCTAssertEqual(state.selectedWorkspace?.name, "test")
    }

    func testSelectedWorkspaceNilWhenNoMatch() {
        let state = AppState()
        state.workspaces = [Workspace(name: "a", path: "/a")]
        state.selectedWorkspaceID = UUID()
        XCTAssertNil(state.selectedWorkspace)
    }

    func testSessionStatusEquality() {
        XCTAssertEqual(SessionStatus.stopped, SessionStatus.stopped)
        XCTAssertEqual(SessionStatus.running, SessionStatus.running)
        XCTAssertNotEqual(SessionStatus.stopped, SessionStatus.running)
    }

    func testMultipleActiveWorkspaces() {
        let state = AppState()
        let ws1 = Workspace(name: "a", path: "/a")
        var ws2 = Workspace(name: "b", path: "/b")
        ws2.isActive = true
        state.workspaces = [ws1, ws2]
        state.activationStates[ws2.id] = .active
        XCTAssertFalse(state.isActive(ws1))
        XCTAssertTrue(state.isActive(ws2))
    }

    func testStatusForRunningWorkspace() {
        let state = AppState()
        var ws = Workspace(name: "test", path: "/tmp")
        ws.isActive = true
        state.workspaces = [ws]
        state.activationStates[ws.id] = .active
        XCTAssertEqual(state.statusFor(ws), .running)
    }

    func testStatusForStoppedWorkspace() {
        let state = AppState()
        let ws = Workspace(name: "test", path: "/tmp")
        state.workspaces = [ws]
        XCTAssertEqual(state.statusFor(ws), .stopped)
    }

    func testStatusForActivationFailed() {
        let state = AppState()
        var ws = Workspace(name: "test", path: "/tmp")
        ws.isActive = false
        state.workspaces = [ws]
        state.activationStates[ws.id] = .active
        XCTAssertEqual(state.statusFor(ws), .error("activation failed"))
    }

    func testResolvedSettingsUsesWorkspaceOverrides() {
        var global = AppSettings()
        global.containerImage = "default:v1"
        global.proxyImage = "default-proxy:v1"
        global.passthroughHosts = ["host1.com"]
        global.enabledMCPServers = ["slack", "github", "jira"]
        global.networkAllowlist = ["api.github.com"]
        var ws = Workspace(name: "test", path: "/tmp")
        ws.containerImageOverride = "custom:v2"
        ws.proxyImageOverride = "custom-proxy:v2"
        ws.passthroughHostsOverride = ["host2.com"]
        ws.proxyPortOverride = 9090
        ws.enabledMCPServersOverride = ["slack"]
        ws.networkAllowlistOverride = ["api.stripe.com"]
        let resolved = ResolvedSettings(global: global, workspace: ws)
        XCTAssertEqual(resolved.containerImage, "custom:v2")
        XCTAssertEqual(resolved.proxyImage, "custom-proxy:v2")
        XCTAssertEqual(resolved.passthroughHosts, ["host2.com"])
        XCTAssertEqual(resolved.proxyPort, 9090)
        XCTAssertEqual(resolved.enabledMCPServers, ["slack"])
        XCTAssertEqual(resolved.networkAllowlist, ["api.stripe.com"])
    }

    func testResolvedSettingsFallsBackToGlobal() {
        var global = AppSettings()
        global.containerImage = "global:latest"
        global.proxyImage = "global-proxy:latest"
        global.passthroughHosts = ["api.example.com"]
        global.enabledMCPServers = ["slack"]
        global.networkAllowlist = ["api.github.com"]
        let ws = Workspace(name: "test", path: "/tmp")
        let resolved = ResolvedSettings(global: global, workspace: ws)
        XCTAssertEqual(resolved.containerImage, "global:latest")
        XCTAssertEqual(resolved.proxyImage, "global-proxy:latest")
        XCTAssertEqual(resolved.passthroughHosts, ["api.example.com"])
        XCTAssertEqual(resolved.proxyPort, 8080)
        XCTAssertEqual(resolved.enabledMCPServers, ["slack"])
        XCTAssertEqual(resolved.networkAllowlist, ["api.github.com"])
    }

    func testResolvedSettingsNilNetworkAllowlistMeansAllowAll() {
        // nil override + nil global = nil (allow all HTTP traffic)
        let global = AppSettings()
        let ws = Workspace(name: "test", path: "/tmp")
        let resolved = ResolvedSettings(global: global, workspace: ws)
        XCTAssertNil(resolved.networkAllowlist)
    }

    func testEmptyNetworkAllowlistOverrideResolvesToEmptyArray() {
        // An explicit empty-array override wins over a populated global
        // setting. At the enforcement layer (mitmproxy addon), empty means
        // "allow all HTTP" — same as nil — but the override-vs-inherit
        // distinction matters for UI display, so the resolver preserves
        // whichever explicit value the workspace set.
        var global = AppSettings()
        global.networkAllowlist = ["api.github.com"]
        var ws = Workspace(name: "test", path: "/tmp")
        ws.networkAllowlistOverride = []
        let resolved = ResolvedSettings(global: global, workspace: ws)
        XCTAssertEqual(resolved.networkAllowlist, [])
    }

    func testEmptyPassthroughOverrideResolvesToEmptyArray() {
        // An explicit empty-array passthrough override means "no passthrough
        // for this workspace" and must win over a populated global setting.
        // This is the load-bearing distinction that the Passthrough Override
        // toggle exposes in the workspace settings UI.
        var global = AppSettings()
        global.passthroughHosts = ["api.anthropic.com", "auth.anthropic.com"]
        var ws = Workspace(name: "test", path: "/tmp")
        ws.passthroughHostsOverride = []
        let resolved = ResolvedSettings(global: global, workspace: ws)
        XCTAssertEqual(resolved.passthroughHosts, [])
    }

    func testResolvedSettingsNilMCPMeansAllEnabled() {
        // Both global and workspace nil => nil (meaning: do not filter, keep all MCPs)
        let global = AppSettings()
        let ws = Workspace(name: "test", path: "/tmp")
        let resolved = ResolvedSettings(global: global, workspace: ws)
        XCTAssertNil(resolved.enabledMCPServers)
    }

    func testResolvedSettingsEmptyOverrideMeansNoneEnabled() {
        // Workspace explicitly sets empty array => empty (filter out all MCPs)
        var global = AppSettings()
        global.enabledMCPServers = ["slack"]
        var ws = Workspace(name: "test", path: "/tmp")
        ws.enabledMCPServersOverride = []
        let resolved = ResolvedSettings(global: global, workspace: ws)
        XCTAssertEqual(resolved.enabledMCPServers, [])
    }

    func testDetailTabCases() {
        let tabs: [DetailTab] = [.terminal, .secrets, .containers, .settings]
        XCTAssertEqual(tabs.count, 4)
        XCTAssertNotEqual(DetailTab.terminal, DetailTab.secrets)
        XCTAssertNotEqual(DetailTab.containers, DetailTab.settings)
    }

    func testSwitchSelectedWorkspace() {
        let state = AppState()
        let ws1 = Workspace(name: "project-a", path: "/a")
        let ws2 = Workspace(name: "project-b", path: "/b")
        state.workspaces = [ws1, ws2]

        state.selectedWorkspaceID = ws1.id
        XCTAssertEqual(state.selectedWorkspace?.name, "project-a")

        state.selectedWorkspaceID = ws2.id
        XCTAssertEqual(state.selectedWorkspace?.name, "project-b")
    }

    func testSwitchWorkspacePreservesActivationStates() {
        let state = AppState()
        let ws1 = Workspace(name: "a", path: "/a")
        let ws2 = Workspace(name: "b", path: "/b")
        state.workspaces = [ws1, ws2]
        state.activationStates[ws1.id] = .active
        state.activationStates[ws2.id] = .inactive

        state.selectedWorkspaceID = ws1.id
        XCTAssertEqual(state.activationState(for: state.selectedWorkspace!), .active)

        state.selectedWorkspaceID = ws2.id
        XCTAssertEqual(state.activationState(for: state.selectedWorkspace!), .inactive)
        // ws1 state unaffected by selection change
        XCTAssertEqual(state.activationState(for: ws1), .active)
    }

    func testSwitchWorkspaceToNil() {
        let state = AppState()
        let ws = Workspace(name: "test", path: "/tmp")
        state.workspaces = [ws]
        state.selectedWorkspaceID = ws.id
        XCTAssertNotNil(state.selectedWorkspace)

        state.selectedWorkspaceID = nil
        XCTAssertNil(state.selectedWorkspace)
    }

    func testSwitchWorkspaceDuringActivation() {
        let state = AppState()
        let ws1 = Workspace(name: "a", path: "/a")
        let ws2 = Workspace(name: "b", path: "/b")
        state.workspaces = [ws1, ws2]
        state.activationStates[ws1.id] = .activating

        state.selectedWorkspaceID = ws1.id
        XCTAssertTrue(state.isActivating(ws1))

        // Switch away while ws1 is still activating
        state.selectedWorkspaceID = ws2.id
        XCTAssertEqual(state.selectedWorkspace?.name, "b")
        // ws1 activation state must be preserved
        XCTAssertTrue(state.isActivating(ws1))
        XCTAssertEqual(state.activationState(for: ws2), .inactive)
    }

    func testRapidWorkspaceSwitching() {
        let state = AppState()
        let ws1 = Workspace(name: "a", path: "/a")
        let ws2 = Workspace(name: "b", path: "/b")
        let ws3 = Workspace(name: "c", path: "/c")
        state.workspaces = [ws1, ws2, ws3]
        state.activationStates[ws1.id] = .active
        state.activationStates[ws2.id] = .activating
        state.activationStates[ws3.id] = .inactive

        // Rapid switch: A -> B -> C
        state.selectedWorkspaceID = ws1.id
        state.selectedWorkspaceID = ws2.id
        state.selectedWorkspaceID = ws3.id
        XCTAssertEqual(state.selectedWorkspace?.name, "c")

        // All activation states preserved through rapid switching
        XCTAssertEqual(state.activationState(for: ws1), .active)
        XCTAssertEqual(state.activationState(for: ws2), .activating)
        XCTAssertEqual(state.activationState(for: ws3), .inactive)
    }

    func testDirectTabAssignment() {
        let state = AppState()
        state.selectedTab = .secrets
        XCTAssertEqual(state.selectedTab, .secrets)
        state.selectedTab = .terminal
        XCTAssertEqual(state.selectedTab, .terminal)
    }

    func testActivationStateIndependentPerWorkspace() {
        let state = AppState()
        let ws1 = Workspace(name: "a", path: "/a")
        let ws2 = Workspace(name: "b", path: "/b")
        let ws3 = Workspace(name: "c", path: "/c")
        state.workspaces = [ws1, ws2, ws3]

        state.activationStates[ws1.id] = .active
        state.activationStates[ws2.id] = .activating
        // ws3 has no entry -- should default to .inactive

        XCTAssertTrue(state.isActive(ws1))
        XCTAssertFalse(state.isActive(ws2))
        XCTAssertFalse(state.isActive(ws3))

        XCTAssertFalse(state.isActivating(ws1))
        XCTAssertTrue(state.isActivating(ws2))
        XCTAssertFalse(state.isActivating(ws3))

        XCTAssertEqual(state.activationState(for: ws3), .inactive)
    }

    func testRemoveWorkspaceClearsActivationState() {
        let state = AppState()
        let ws1 = Workspace(name: "a", path: "/a")
        let ws2 = Workspace(name: "b", path: "/b")
        state.workspaces = [ws1, ws2]
        state.activationStates[ws1.id] = .active
        state.activationStates[ws2.id] = .active
        state.selectedWorkspaceID = ws1.id

        // Simulate sidebar removeWorkspace behavior
        state.workspaces.removeAll { $0.id == ws1.id }
        state.activationStates.removeValue(forKey: ws1.id)

        XCTAssertNil(state.workspaces.first { $0.id == ws1.id })
        XCTAssertNil(state.activationStates[ws1.id])
        // selectedWorkspace should return nil since ws1 is gone
        XCTAssertNil(state.selectedWorkspace)
        // ws2 unaffected
        XCTAssertEqual(state.activationState(for: ws2), .active)
    }

    // MARK: - passthroughHostsDraft round-trip

    func testPassthroughHostsDraftDefaultsToNil() {
        let settings = AppSettings()
        XCTAssertNil(settings.passthroughHostsDraft)
    }

    func testPassthroughHostsDraftEncodeDecodeRoundTrip() throws {
        var settings = AppSettings()
        settings.passthroughHosts = []
        settings.passthroughHostsDraft = ["api.anthropic.com", "auth.anthropic.com"]
        let data = try JSONEncoder().encode(settings)
        let decoded = try JSONDecoder().decode(AppSettings.self, from: data)
        XCTAssertEqual(decoded.passthroughHosts, [])
        XCTAssertEqual(decoded.passthroughHostsDraft, ["api.anthropic.com", "auth.anthropic.com"])
    }

    func testPassthroughHostsDraftMissingKeyDecodesAsNil() throws {
        // Simulate a settings.json written by the previous app version:
        // no passthroughHostsDraft key at all.
        let legacyJSON = """
        {
          "containerImage": "airlock-claude:latest",
          "proxyImage": "airlock-proxy:latest",
          "passthroughHosts": ["api.anthropic.com", "auth.anthropic.com"],
          "theme": "System",
          "terminal": { "fontName": "Menlo", "fontSize": 12 }
        }
        """.data(using: .utf8)!
        let decoded = try JSONDecoder().decode(AppSettings.self, from: legacyJSON)
        XCTAssertNil(decoded.passthroughHostsDraft)
        XCTAssertEqual(decoded.passthroughHosts, ["api.anthropic.com", "auth.anthropic.com"])
    }
}
