import XCTest
@testable import AirlockApp

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

    func testIsRunning() {
        let state = AppState()
        XCTAssertFalse(state.isRunning)
        state.sessionStatus = .running
        XCTAssertTrue(state.isRunning)
    }

    func testSessionStatusEquality() {
        XCTAssertEqual(SessionStatus.stopped, SessionStatus.stopped)
        XCTAssertEqual(SessionStatus.running, SessionStatus.running)
        XCTAssertNotEqual(SessionStatus.stopped, SessionStatus.running)
    }
}
