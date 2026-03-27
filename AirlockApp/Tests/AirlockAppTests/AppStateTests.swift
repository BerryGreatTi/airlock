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

    func testDetailTabCases() {
        let tabs: [DetailTab] = [.terminal, .secrets, .containers, .diff, .settings]
        XCTAssertEqual(tabs.count, 5)
        XCTAssertNotEqual(DetailTab.terminal, DetailTab.secrets)
        XCTAssertNotEqual(DetailTab.containers, DetailTab.diff)
    }
}
