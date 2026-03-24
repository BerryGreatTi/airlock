# Airlock GUI Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a macOS native SwiftUI app that wraps the airlock Go CLI with workspace management, an embedded terminal, a side-by-side diff viewer, and settings.

**Architecture:** SwiftUI app with NavigationSplitView (sidebar + detail). Terminal powered by SwiftTerm's LocalProcessTerminalView (AppKit NSView wrapped for SwiftUI). Go CLI invoked as subprocess for all container/crypto operations. Git diff parsed from `git diff HEAD` output. State managed via @Observable AppState.

**Tech Stack:** Swift 5.9+, SwiftUI (macOS 14+), SwiftTerm (SPM), XCTest

**Prerequisite:** This plan must be executed on a macOS machine with Xcode 15+ installed. The current Go CLI project is at the repository root. The Swift app will live in `AirlockApp/` alongside it.

**Build system:** SPM (Package.swift) for development simplicity. SPM executable targets produce bare binaries, not `.app` bundles. For distribution as `.app`, a separate Xcode project or `xcodebuild` wrapper is needed (post-MVP). All `Bundle.main` APIs are avoided; binary resolution uses PATH lookup + configurable override.

---

## File Structure

```
AirlockApp/
├── Package.swift
├── Sources/
│   └── AirlockApp/
│       ├── AirlockApp.swift                  # @main App, WindowGroup, commands
│       ├── ContentView.swift                 # NavigationSplitView, tab bar, overlays
│       ├── Models/
│       │   ├── Workspace.swift               # Workspace struct
│       │   ├── AppState.swift                # @Observable state + AppSettings
│       │   └── DiffModel.swift               # FileDiff, DiffHunk, SideBySideLine
│       ├── Views/
│       │   ├── Sidebar/
│       │   │   ├── SidebarView.swift         # Workspace list, run/stop/remove actions
│       │   │   └── NewWorkspaceSheet.swift   # Directory picker + env file
│       │   ├── Terminal/
│       │   │   └── TerminalView.swift        # NSViewRepresentable for SwiftTerm
│       │   ├── Diff/
│       │   │   ├── DiffContainerView.swift   # Scrollable file list, refresh
│       │   │   ├── SideBySideDiffView.swift  # Left/right panels for one file
│       │   │   └── DiffLineView.swift        # Single line with number + content
│       │   └── Settings/
│       │       └── SettingsView.swift        # Global + per-workspace form
│       └── Services/
│           ├── CLIService.swift              # Process spawning, binary resolution
│           ├── WorkspaceStore.swift          # JSON persistence
│           └── DiffParser.swift              # Unified diff -> side-by-side parser
├── Tests/
│   └── AirlockAppTests/
│       ├── DiffParserTests.swift
│       ├── WorkspaceStoreTests.swift
│       ├── WorkspaceTests.swift
│       ├── CLIServiceTests.swift
│       └── AppStateTests.swift
```

---

## Task 1: Project Scaffolding

**Files:**
- Create: `AirlockApp/Package.swift`
- Create: `AirlockApp/Sources/AirlockApp/AirlockApp.swift`
- Modify: `.gitignore`
- Modify: `Makefile`

- [ ] **Step 1: Create Package.swift**

```swift
// AirlockApp/Package.swift
// swift-tools-version: 5.9
import PackageDescription

let package = Package(
    name: "AirlockApp",
    platforms: [.macOS(.v14)],
    dependencies: [
        .package(url: "https://github.com/migueldeicaza/SwiftTerm.git", from: "1.0.0"),
    ],
    targets: [
        .executableTarget(
            name: "AirlockApp",
            dependencies: ["SwiftTerm"],
            path: "Sources/AirlockApp"
        ),
        .testTarget(
            name: "AirlockAppTests",
            dependencies: ["AirlockApp"],
            path: "Tests/AirlockAppTests"
        ),
    ]
)
```

- [ ] **Step 2: Create minimal app entry**

```swift
// AirlockApp/Sources/AirlockApp/AirlockApp.swift
import SwiftUI

@main
struct AirlockApp: App {
    var body: some Scene {
        WindowGroup {
            Text("Airlock")
                .frame(width: 800, height: 600)
        }
    }
}
```

- [ ] **Step 3: Update .gitignore**

Append to root `.gitignore`:

```
# Swift
AirlockApp/.build/
AirlockApp/.swiftpm/
*.xcuserstate
```

- [ ] **Step 4: Add Makefile targets**

Append to root `Makefile`:

```makefile
gui-build:
	cd AirlockApp && swift build

gui-test:
	cd AirlockApp && swift test

gui-run:
	cd AirlockApp && swift run
```

- [ ] **Step 5: Resolve dependencies and verify build**

```bash
cd AirlockApp && swift build
```

Expected: BUILD SUCCEEDED

- [ ] **Step 6: Commit**

```bash
git add AirlockApp/Package.swift AirlockApp/Sources/ .gitignore Makefile
git commit -m "feat(gui): scaffold SwiftUI app with SwiftTerm dependency"
```

---

## Task 2: Data Models

**Files:**
- Create: `AirlockApp/Sources/AirlockApp/Models/Workspace.swift`
- Create: `AirlockApp/Sources/AirlockApp/Models/AppState.swift`
- Create: `AirlockApp/Sources/AirlockApp/Models/DiffModel.swift`
- Test: `AirlockApp/Tests/AirlockAppTests/WorkspaceTests.swift`
- Test: `AirlockApp/Tests/AirlockAppTests/AppStateTests.swift`

- [ ] **Step 1: Write model tests**

```swift
// AirlockApp/Tests/AirlockAppTests/WorkspaceTests.swift
import XCTest
@testable import AirlockApp

final class WorkspaceTests: XCTestCase {
    func testWorkspaceCreation() {
        let ws = Workspace(name: "my-project", path: "/Users/test/my-project")
        XCTAssertFalse(ws.id.uuidString.isEmpty)
        XCTAssertEqual(ws.name, "my-project")
        XCTAssertNil(ws.envFilePath)
        XCTAssertNil(ws.containerImageOverride)
    }

    func testWorkspaceCodable() throws {
        let ws = Workspace(name: "test", path: "/tmp/test", envFilePath: "/tmp/.env")
        let data = try JSONEncoder().encode(ws)
        let decoded = try JSONDecoder().decode(Workspace.self, from: data)
        XCTAssertEqual(decoded.id, ws.id)
        XCTAssertEqual(decoded.name, ws.name)
        XCTAssertEqual(decoded.envFilePath, ws.envFilePath)
    }
}
```

```swift
// AirlockApp/Tests/AirlockAppTests/AppStateTests.swift
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
        state.selectedWorkspaceID = UUID() // non-matching
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
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd AirlockApp && swift test
```

Expected: FAIL

- [ ] **Step 3: Create models**

```swift
// AirlockApp/Sources/AirlockApp/Models/Workspace.swift
import Foundation

struct Workspace: Identifiable, Codable, Hashable {
    let id: UUID
    var name: String
    var path: String
    var envFilePath: String?
    var containerImageOverride: String?

    init(name: String, path: String, envFilePath: String? = nil, containerImageOverride: String? = nil) {
        self.id = UUID()
        self.name = name
        self.path = path
        self.envFilePath = envFilePath
        self.containerImageOverride = containerImageOverride
    }
}
```

```swift
// AirlockApp/Sources/AirlockApp/Models/AppState.swift
import Foundation
import Observation

enum SessionStatus: Equatable {
    case stopped
    case running
    case error(String)
}

enum DetailTab: Hashable {
    case terminal
    case diff
    case settings
}

@Observable
final class AppState {
    var workspaces: [Workspace] = []
    var selectedWorkspaceID: UUID?
    var activeWorkspaceID: UUID?
    var sessionStatus: SessionStatus = .stopped
    var selectedTab: DetailTab = .terminal
    var lastError: String?

    var selectedWorkspace: Workspace? {
        workspaces.first { $0.id == selectedWorkspaceID }
    }

    var activeWorkspace: Workspace? {
        workspaces.first { $0.id == activeWorkspaceID }
    }

    var isRunning: Bool {
        sessionStatus == .running
    }
}

struct AppSettings: Codable, Equatable {
    var airlockBinaryPath: String?
    var containerImage: String = "airlock-claude:latest"
    var proxyImage: String = "airlock-proxy:latest"
    var passthroughHosts: [String] = ["api.anthropic.com", "auth.anthropic.com"]
}
```

```swift
// AirlockApp/Sources/AirlockApp/Models/DiffModel.swift
import Foundation

enum DiffChangeType: String {
    case modified = "Modified"
    case added = "Added"
    case deleted = "Deleted"
}

struct FileDiff: Identifiable {
    let id = UUID()
    let oldPath: String
    let newPath: String
    let changeType: DiffChangeType
    let hunks: [DiffHunk]
}

struct DiffHunk: Identifiable {
    let id = UUID()
    let oldStart: Int
    let oldCount: Int
    let newStart: Int
    let newCount: Int
    let lines: [SideBySideLine]
}

enum LineType {
    case context
    case added
    case deleted
}

struct SideBySideLine: Identifiable {
    let id = UUID()
    let leftNumber: Int?
    let leftContent: String?
    let leftType: LineType
    let rightNumber: Int?
    let rightContent: String?
    let rightType: LineType
}
```

- [ ] **Step 4: Run tests**

```bash
cd AirlockApp && swift test
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add AirlockApp/Sources/AirlockApp/Models/ AirlockApp/Tests/
git commit -m "feat(gui): data models with Workspace, AppState, DiffModel, and tests"
```

---

## Task 3: DiffParser (TDD)

**Files:**
- Create: `AirlockApp/Sources/AirlockApp/Services/DiffParser.swift`
- Test: `AirlockApp/Tests/AirlockAppTests/DiffParserTests.swift`

- [ ] **Step 1: Write parser tests**

IMPORTANT: Multi-line diff strings must have NO leading whitespace on content lines. Use a helper to dedent, or start content at column 0.

```swift
// AirlockApp/Tests/AirlockAppTests/DiffParserTests.swift
import XCTest
@testable import AirlockApp

final class DiffParserTests: XCTestCase {

    func testParseSimpleModification() {
        let input = [
            "diff --git a/main.go b/main.go",
            "index abc1234..def5678 100644",
            "--- a/main.go",
            "+++ b/main.go",
            "@@ -10,4 +10,5 @@ func main() {",
            " cfg := config.Load()",
            "-fmt.Println(\"hello\")",
            "+log.Info(\"starting\")",
            "+log.Info(\"version:\", v)",
            " run(cfg)",
        ].joined(separator: "\n")

        let files = DiffParser.parse(input)
        XCTAssertEqual(files.count, 1)
        XCTAssertEqual(files[0].oldPath, "main.go")
        XCTAssertEqual(files[0].newPath, "main.go")
        XCTAssertEqual(files[0].changeType, .modified)
        XCTAssertEqual(files[0].hunks.count, 1)
        XCTAssertGreaterThan(files[0].hunks[0].lines.count, 0)
    }

    func testParseNewFile() {
        let input = [
            "diff --git a/new.go b/new.go",
            "new file mode 100644",
            "index 0000000..abc1234",
            "--- /dev/null",
            "+++ b/new.go",
            "@@ -0,0 +1,3 @@",
            "+package main",
            "+",
            "+func hello() {}",
        ].joined(separator: "\n")

        let files = DiffParser.parse(input)
        XCTAssertEqual(files.count, 1)
        XCTAssertEqual(files[0].changeType, .added)
    }

    func testParseDeletedFile() {
        let input = [
            "diff --git a/old.go b/old.go",
            "deleted file mode 100644",
            "index abc1234..0000000",
            "--- a/old.go",
            "+++ /dev/null",
            "@@ -1,3 +0,0 @@",
            "-package main",
            "-",
            "-func old() {}",
        ].joined(separator: "\n")

        let files = DiffParser.parse(input)
        XCTAssertEqual(files.count, 1)
        XCTAssertEqual(files[0].changeType, .deleted)
    }

    func testParseMultipleFiles() {
        let input = [
            "diff --git a/a.go b/a.go",
            "index abc..def 100644",
            "--- a/a.go",
            "+++ b/a.go",
            "@@ -1,3 +1,3 @@",
            " package main",
            "-var x = 1",
            "+var x = 2",
            "diff --git a/b.go b/b.go",
            "index abc..def 100644",
            "--- a/b.go",
            "+++ b/b.go",
            "@@ -1,3 +1,3 @@",
            " package main",
            "-var y = 1",
            "+var y = 2",
        ].joined(separator: "\n")

        let files = DiffParser.parse(input)
        XCTAssertEqual(files.count, 2)
        XCTAssertEqual(files[0].oldPath, "a.go")
        XCTAssertEqual(files[1].oldPath, "b.go")
    }

    func testParseEmptyInput() {
        XCTAssertEqual(DiffParser.parse("").count, 0)
    }

    func testSideBySideAlignment() {
        let input = [
            "diff --git a/f.go b/f.go",
            "index abc..def 100644",
            "--- a/f.go",
            "+++ b/f.go",
            "@@ -1,3 +1,4 @@",
            " context line",
            "-old line",
            "+new line",
            "+extra line",
            " context end",
        ].joined(separator: "\n")

        let files = DiffParser.parse(input)
        let lines = files[0].hunks[0].lines

        XCTAssertEqual(lines[0].leftType, .context)
        XCTAssertEqual(lines[0].rightType, .context)

        let deletedLine = lines.first { $0.leftType == .deleted }
        XCTAssertNotNil(deletedLine)
        XCTAssertEqual(deletedLine?.leftContent, "old line")
        XCTAssertEqual(deletedLine?.rightContent, "new line")
        XCTAssertEqual(deletedLine?.rightType, .added)
    }
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd AirlockApp && swift test --filter DiffParserTests
```

Expected: FAIL

- [ ] **Step 3: Implement DiffParser**

```swift
// AirlockApp/Sources/AirlockApp/Services/DiffParser.swift
import Foundation

enum DiffParser {
    static func parse(_ input: String) -> [FileDiff] {
        guard !input.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty else {
            return []
        }

        var files: [FileDiff] = []
        let lines = input.components(separatedBy: "\n")
        var i = 0

        while i < lines.count {
            guard lines[i].hasPrefix("diff --git") else { i += 1; continue }

            let (oldPath, newPath) = parseFilePaths(lines[i])
            var changeType: DiffChangeType = .modified
            i += 1

            while i < lines.count && !lines[i].hasPrefix("---") && !lines[i].hasPrefix("diff --git") {
                if lines[i].hasPrefix("new file") { changeType = .added }
                if lines[i].hasPrefix("deleted file") { changeType = .deleted }
                i += 1
            }

            if i < lines.count && lines[i].hasPrefix("---") { i += 1 }
            if i < lines.count && lines[i].hasPrefix("+++") { i += 1 }

            var hunks: [DiffHunk] = []
            while i < lines.count && !lines[i].hasPrefix("diff --git") {
                if lines[i].hasPrefix("@@") {
                    let (hunk, nextIndex) = parseHunk(lines: lines, startIndex: i)
                    hunks.append(hunk)
                    i = nextIndex
                } else {
                    i += 1
                }
            }

            files.append(FileDiff(oldPath: oldPath, newPath: newPath, changeType: changeType, hunks: hunks))
        }
        return files
    }

    private static func parseFilePaths(_ line: String) -> (String, String) {
        let parts = line.components(separatedBy: " ")
        let oldPath = parts.count > 2 ? String(parts[2].dropFirst(2)) : ""
        let newPath = parts.count > 3 ? String(parts[3].dropFirst(2)) : oldPath
        return (oldPath, newPath)
    }

    private static func parseHunk(lines: [String], startIndex: Int) -> (DiffHunk, Int) {
        let (oldStart, oldCount, newStart, newCount) = parseHunkHeader(lines[startIndex])

        var rawLines: [(type: LineType, content: String)] = []
        var i = startIndex + 1

        while i < lines.count {
            let line = lines[i]
            if line.hasPrefix("diff --git") || line.hasPrefix("@@") { break }
            if line.hasPrefix("-") {
                rawLines.append((.deleted, String(line.dropFirst(1))))
            } else if line.hasPrefix("+") {
                rawLines.append((.added, String(line.dropFirst(1))))
            } else if line.hasPrefix(" ") {
                rawLines.append((.context, String(line.dropFirst(1))))
            } else if line.isEmpty {
                rawLines.append((.context, ""))
            }
            i += 1
        }

        let sideBySide = alignSideBySide(rawLines, oldStart: oldStart, newStart: newStart)
        return (DiffHunk(oldStart: oldStart, oldCount: oldCount, newStart: newStart, newCount: newCount, lines: sideBySide), i)
    }

    private static func parseHunkHeader(_ header: String) -> (Int, Int, Int, Int) {
        let regex = try! NSRegularExpression(pattern: #"@@ -(\d+),?(\d*) \+(\d+),?(\d*) @@"#)
        let range = NSRange(header.startIndex..., in: header)
        guard let match = regex.firstMatch(in: header, range: range) else { return (0, 0, 0, 0) }

        func intAt(_ idx: Int) -> Int {
            guard let r = Range(match.range(at: idx), in: header) else { return 1 }
            let s = String(header[r])
            return s.isEmpty ? 1 : (Int(s) ?? 1)
        }
        return (intAt(1), intAt(2), intAt(3), intAt(4))
    }

    private static func alignSideBySide(_ rawLines: [(type: LineType, content: String)], oldStart: Int, newStart: Int) -> [SideBySideLine] {
        var result: [SideBySideLine] = []
        var oldLine = oldStart
        var newLine = newStart
        var i = 0

        while i < rawLines.count {
            let (type, content) = rawLines[i]

            switch type {
            case .context:
                result.append(SideBySideLine(leftNumber: oldLine, leftContent: content, leftType: .context, rightNumber: newLine, rightContent: content, rightType: .context))
                oldLine += 1; newLine += 1; i += 1

            case .deleted:
                var deletions: [String] = []
                var k = i
                while k < rawLines.count && rawLines[k].type == .deleted {
                    deletions.append(rawLines[k].content); k += 1
                }
                var additions: [String] = []
                var m = k
                while m < rawLines.count && rawLines[m].type == .added {
                    additions.append(rawLines[m].content); m += 1
                }
                let maxLen = max(deletions.count, additions.count)
                for idx in 0..<maxLen {
                    result.append(SideBySideLine(
                        leftNumber: idx < deletions.count ? oldLine + idx : nil,
                        leftContent: idx < deletions.count ? deletions[idx] : nil,
                        leftType: idx < deletions.count ? .deleted : .context,
                        rightNumber: idx < additions.count ? newLine + idx : nil,
                        rightContent: idx < additions.count ? additions[idx] : nil,
                        rightType: idx < additions.count ? .added : .context
                    ))
                }
                oldLine += deletions.count; newLine += additions.count; i = m

            case .added:
                result.append(SideBySideLine(leftNumber: nil, leftContent: nil, leftType: .context, rightNumber: newLine, rightContent: content, rightType: .added))
                newLine += 1; i += 1
            }
        }
        return result
    }
}
```

- [ ] **Step 4: Run tests**

```bash
cd AirlockApp && swift test --filter DiffParserTests
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add AirlockApp/Sources/AirlockApp/Services/DiffParser.swift AirlockApp/Tests/AirlockAppTests/DiffParserTests.swift
git commit -m "feat(gui): diff parser converting unified diff to side-by-side model"
```

---

## Task 4: WorkspaceStore + CLIService

**Files:**
- Create: `AirlockApp/Sources/AirlockApp/Services/WorkspaceStore.swift`
- Create: `AirlockApp/Sources/AirlockApp/Services/CLIService.swift`
- Test: `AirlockApp/Tests/AirlockAppTests/WorkspaceStoreTests.swift`
- Test: `AirlockApp/Tests/AirlockAppTests/CLIServiceTests.swift`

- [ ] **Step 1: Write WorkspaceStore tests**

```swift
// AirlockApp/Tests/AirlockAppTests/WorkspaceStoreTests.swift
import XCTest
@testable import AirlockApp

final class WorkspaceStoreTests: XCTestCase {
    var tempDir: URL!

    override func setUp() {
        tempDir = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
        try! FileManager.default.createDirectory(at: tempDir, withIntermediateDirectories: true)
    }
    override func tearDown() { try? FileManager.default.removeItem(at: tempDir) }

    func testSaveAndLoadWorkspaces() throws {
        let store = WorkspaceStore(directory: tempDir)
        let ws = Workspace(name: "test", path: "/tmp/test")
        try store.saveWorkspaces([ws])
        let loaded = try store.loadWorkspaces()
        XCTAssertEqual(loaded.count, 1)
        XCTAssertEqual(loaded[0].id, ws.id)
    }

    func testLoadEmptyReturnsEmpty() throws {
        let store = WorkspaceStore(directory: tempDir)
        XCTAssertEqual(try store.loadWorkspaces().count, 0)
    }

    func testSaveAndLoadSettings() throws {
        let store = WorkspaceStore(directory: tempDir)
        var settings = AppSettings()
        settings.containerImage = "custom:v2"
        try store.saveSettings(settings)
        XCTAssertEqual(try store.loadSettings().containerImage, "custom:v2")
    }

    func testDefaultSettings() throws {
        let store = WorkspaceStore(directory: tempDir)
        XCTAssertEqual(try store.loadSettings().containerImage, "airlock-claude:latest")
    }
}
```

```swift
// AirlockApp/Tests/AirlockAppTests/CLIServiceTests.swift
import XCTest
@testable import AirlockApp

final class CLIServiceTests: XCTestCase {
    func testIsGitRepo() {
        let tempDir = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
        try! FileManager.default.createDirectory(at: tempDir, withIntermediateDirectories: true)
        defer { try? FileManager.default.removeItem(at: tempDir) }

        let cli = CLIService()
        XCTAssertFalse(cli.isGitRepo(path: tempDir.path))

        // Create .git directory
        try! FileManager.default.createDirectory(at: tempDir.appendingPathComponent(".git"), withIntermediateDirectories: true)
        XCTAssertTrue(cli.isGitRepo(path: tempDir.path))
    }

    func testIsAirlockInitialized() {
        let tempDir = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
        try! FileManager.default.createDirectory(at: tempDir, withIntermediateDirectories: true)
        defer { try? FileManager.default.removeItem(at: tempDir) }

        let cli = CLIService()
        XCTAssertFalse(cli.isAirlockInitialized(path: tempDir.path))

        try! FileManager.default.createDirectory(at: tempDir.appendingPathComponent(".airlock"), withIntermediateDirectories: true)
        XCTAssertTrue(cli.isAirlockInitialized(path: tempDir.path))
    }

    func testFindBinaryInPath() {
        // /usr/bin/git should always exist on macOS
        XCTAssertNotNil(CLIService.findInPath("git"))
        XCTAssertNil(CLIService.findInPath("nonexistent_binary_xyz"))
    }
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd AirlockApp && swift test --filter "WorkspaceStoreTests|CLIServiceTests"
```

Expected: FAIL

- [ ] **Step 3: Implement WorkspaceStore**

```swift
// AirlockApp/Sources/AirlockApp/Services/WorkspaceStore.swift
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
```

- [ ] **Step 4: Implement CLIService**

```swift
// AirlockApp/Sources/AirlockApp/Services/CLIService.swift
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

    /// Resolve the airlock binary path.
    func resolveAirlockBinary() -> String {
        if let explicit = binaryPath { return explicit }
        if let found = Self.findInPath("airlock") { return found }
        return "/usr/local/bin/airlock"
    }

    /// Run a one-shot CLI command and capture output.
    func run(args: [String], workingDirectory: String) async throws -> CLIResult {
        let process = Process()
        process.executableURL = URL(filePath: resolveAirlockBinary())
        process.arguments = args
        process.currentDirectoryURL = URL(filePath: workingDirectory)

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

    /// Run `git diff HEAD --unified=3`.
    func gitDiff(workingDirectory: String) async throws -> CLIResult {
        let process = Process()
        process.executableURL = URL(filePath: "/usr/bin/git")
        process.arguments = ["diff", "HEAD", "--unified=3"]
        process.currentDirectoryURL = URL(filePath: workingDirectory)

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

    func isGitRepo(path: String) -> Bool {
        FileManager.default.fileExists(atPath: (path as NSString).appendingPathComponent(".git"))
    }

    func isAirlockInitialized(path: String) -> Bool {
        FileManager.default.fileExists(atPath: (path as NSString).appendingPathComponent(".airlock"))
    }

    /// Search PATH for a binary. Public for testing.
    static func findInPath(_ name: String) -> String? {
        let paths = (ProcessInfo.processInfo.environment["PATH"] ?? "")
            .components(separatedBy: ":")
        for dir in paths {
            let full = (dir as NSString).appendingPathComponent(name)
            if FileManager.default.isExecutableFile(atPath: full) { return full }
        }
        return nil
    }

    /// Build enriched environment for subprocesses (macOS GUI apps have minimal PATH).
    static func enrichedEnvironment() -> [String: String] {
        var env = ProcessInfo.processInfo.environment
        let extraPaths = ["/usr/local/bin", "/opt/homebrew/bin"]
        let currentPath = env["PATH"] ?? ""
        let missing = extraPaths.filter { !currentPath.contains($0) }
        if !missing.isEmpty {
            env["PATH"] = (missing + [currentPath]).joined(separator: ":")
        }
        return env
    }
}
```

- [ ] **Step 4: Run tests**

```bash
cd AirlockApp && swift test --filter "WorkspaceStoreTests|CLIServiceTests"
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add AirlockApp/Sources/AirlockApp/Services/WorkspaceStore.swift AirlockApp/Sources/AirlockApp/Services/CLIService.swift AirlockApp/Tests/AirlockAppTests/WorkspaceStoreTests.swift AirlockApp/Tests/AirlockAppTests/CLIServiceTests.swift
git commit -m "feat(gui): workspace persistence and CLI service with PATH resolution"
```

---

## Task 5: Views (Sidebar, Terminal, Diff, Settings)

**Files:** All view files. This is a large task but views are tightly coupled and best committed together.

- Create: All files under `AirlockApp/Sources/AirlockApp/Views/`

Implement all views as specified in the spec. Key behavioral requirements:

1. **SidebarView**: Run/Stop actions with single-workspace confirmation dialog. Calls `WorkspaceStore().saveWorkspaces()` after every mutation.
2. **NewWorkspaceSheet**: Checks `.airlock/` existence before calling init. Persists after adding.
3. **TerminalView**: NSViewRepresentable wrapping `LocalProcessTerminalView`. Starts process via `startProcess(executable:args:currentDirectory:environment:execName:)` when `activeWorkspaceID` is set. Delegate callbacks: `processTerminated(source: LocalProcessTerminalView, exitCode: Int32?)`.
4. **DiffContainerView**: Auto-refreshes when tab becomes visible (via `onChange(of:)`). Shows "Not a git repository" for non-git dirs.
5. **DiffLineView + SideBySideDiffView**: Pure presentation, no logic.
6. **SettingsView**: Includes per-workspace `containerImageOverride`. Writes back to `.airlock/config.yaml`.

Because SwiftTerm's exact API may differ from documentation, the implementer MUST:
- Check `LocalProcessTerminalView`'s actual method signatures after SPM resolves
- Adjust delegate protocol methods to match the installed version
- The key pattern (NSViewRepresentable + Coordinator + delegate) is standard

- [ ] **Step 1: Create all view files**

Create each file following the spec. For the terminal, the critical code is:

```swift
// In TerminalView Coordinator:
func processTerminated(source: LocalProcessTerminalView, exitCode: Int32?) {
    Task { @MainActor in
        if let code = exitCode, code != 0 {
            appState.sessionStatus = .error("Process exited with code \(code)")
            appState.lastError = "Process exited with code \(code)"
        } else {
            appState.sessionStatus = .stopped
        }
        appState.activeWorkspaceID = nil
    }
}
```

For starting the process, in `updateNSView`:

```swift
func updateNSView(_ terminal: LocalProcessTerminalView, context: Context) {
    let coord = context.coordinator
    if appState.activeWorkspaceID == workspace.id
        && appState.sessionStatus == .running
        && !coord.processStarted
    {
        coord.processStarted = true
        let cli = CLIService()
        let binary = cli.resolveAirlockBinary()
        var args = ["run"]
        if let envFile = workspace.envFilePath {
            args += ["--env", envFile]
        }
        let env = CLIService.enrichedEnvironment().map { "\($0.key)=\($0.value)" }
        terminal.startProcess(
            executable: binary,
            args: args,
            currentDirectory: workspace.path,
            environment: env,
            execName: "airlock"
        )
    }
}
```

For the sidebar stop action:

```swift
private func stopWorkspace(_ workspace: Workspace) {
    Task {
        let cli = CLIService()
        _ = try? await cli.run(args: ["stop"], workingDirectory: workspace.path)
        appState.sessionStatus = .stopped
        appState.activeWorkspaceID = nil
    }
}
```

- [ ] **Step 2: Verify build**

```bash
cd AirlockApp && swift build
```

Expected: BUILD SUCCEEDED (may need SwiftTerm API adjustments)

- [ ] **Step 3: Commit**

```bash
git add AirlockApp/Sources/AirlockApp/Views/
git commit -m "feat(gui): all views - sidebar, terminal, diff viewer, and settings"
```

---

## Task 6: ContentView + App Entry + Menu Commands

**Files:**
- Create: `AirlockApp/Sources/AirlockApp/ContentView.swift`
- Modify: `AirlockApp/Sources/AirlockApp/AirlockApp.swift`

Key requirements:
1. **ContentView**: NavigationSplitView. Tab bar with Terminal/Diff. Error banner overlay when `sessionStatus == .error`. "Session ended" overlay when session exits normally.
2. **AirlockApp**: Menu commands using `@FocusedObject` or notification observers. Settings via SwiftUI `Settings` scene (Cmd+,). Workspace menu with Run (Cmd+R), Stop (Cmd+.).
3. **Persistence**: Load workspaces on appear. Save after mutations.

- [ ] **Step 1: Create ContentView with overlays**

The ContentView must include:
- Error banner (dismissible) above terminal when `appState.sessionStatus == .error`
- "Session ended" overlay with Restart button when session stops
- Tab switching that triggers diff refresh

```swift
// In ContentView, inside the terminal tab case:
ZStack {
    TerminalView(workspace: workspace, appState: appState)

    // Error banner
    if case .error(let msg) = appState.sessionStatus,
       appState.activeWorkspaceID == nil || appState.activeWorkspaceID == workspace.id {
        VStack {
            HStack {
                Image(systemName: "exclamationmark.triangle.fill")
                    .foregroundStyle(.red)
                Text(msg).font(.caption)
                Spacer()
                Button("Dismiss") { appState.sessionStatus = .stopped; appState.lastError = nil }
                Button("Restart") { restartWorkspace(workspace) }
            }
            .padding(8)
            .background(.red.opacity(0.1))
            .clipShape(RoundedRectangle(cornerRadius: 8))
            .padding()
            Spacer()
        }
    }

    // Session ended overlay
    if appState.sessionStatus == .stopped && appState.activeWorkspaceID == nil
        && appState.selectedWorkspaceID == workspace.id {
        VStack {
            Spacer()
            VStack(spacing: 12) {
                Text("Session ended")
                    .font(.headline)
                Button("Restart") { restartWorkspace(workspace) }
            }
            .padding()
            .background(.regularMaterial)
            .clipShape(RoundedRectangle(cornerRadius: 12))
            Spacer()
        }
    }
}
```

- [ ] **Step 2: Update AirlockApp with Settings scene and FocusedObject**

```swift
// Use Settings scene for Cmd+,
var body: some Scene {
    WindowGroup {
        ContentView()
            .frame(minWidth: 800, minHeight: 500)
    }
    .defaultSize(width: 1200, height: 700)
    .commands { appCommands }

    Settings {
        // Settings scene opens with Cmd+,
        Text("Configure in the main window's Settings tab")
            .frame(width: 300, height: 100)
    }
}
```

- [ ] **Step 3: Verify build and run**

```bash
cd AirlockApp && swift build && swift run
```

Expected: Window appears with sidebar and empty content area

- [ ] **Step 4: Commit**

```bash
git add AirlockApp/Sources/AirlockApp/ContentView.swift AirlockApp/Sources/AirlockApp/AirlockApp.swift
git commit -m "feat(gui): main layout with error overlays, tab navigation, and menu commands"
```

---

## Task 7: Integration Test and Polish

- [ ] **Step 1: Run full test suite**

```bash
cd AirlockApp && swift test
```

Expected: All tests pass

- [ ] **Step 2: Build Go CLI**

```bash
make build
```

- [ ] **Step 3: Smoke test**

```bash
cd AirlockApp && swift run
```

Manual verification:
1. "New Workspace" -> pick directory -> workspace appears in sidebar
2. Click workspace -> Terminal tab shows
3. Switch to Diff tab -> shows diff or "No Changes" or "Not a git repository"
4. Settings tab -> form appears, save works
5. Quit and relaunch -> workspaces persist
6. If Docker is available: Run workspace -> terminal connects to airlock session

- [ ] **Step 4: Final commit**

```bash
git add -A AirlockApp/
git commit -m "feat(gui): airlock macOS GUI MVP - terminal, diff, settings, workspace management"
```
