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
        XCTAssertEqual(DiffParser.parse(input)[0].changeType, .added)
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
        XCTAssertEqual(DiffParser.parse(input)[0].changeType, .deleted)
    }

    func testParseMultipleFiles() {
        let input = [
            "diff --git a/a.go b/a.go",
            "index abc..def 100644",
            "--- a/a.go", "+++ b/a.go",
            "@@ -1,3 +1,3 @@",
            " package main", "-var x = 1", "+var x = 2",
            "diff --git a/b.go b/b.go",
            "index abc..def 100644",
            "--- a/b.go", "+++ b/b.go",
            "@@ -1,3 +1,3 @@",
            " package main", "-var y = 1", "+var y = 2",
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
            "--- a/f.go", "+++ b/f.go",
            "@@ -1,3 +1,4 @@",
            " context line",
            "-old line",
            "+new line",
            "+extra line",
            " context end",
        ].joined(separator: "\n")
        let lines = DiffParser.parse(input)[0].hunks[0].lines
        XCTAssertEqual(lines[0].leftType, .context)
        XCTAssertEqual(lines[0].rightType, .context)
        let deletedLine = lines.first { $0.leftType == .deleted }
        XCTAssertNotNil(deletedLine)
        XCTAssertEqual(deletedLine?.leftContent, "old line")
        XCTAssertEqual(deletedLine?.rightContent, "new line")
        XCTAssertEqual(deletedLine?.rightType, .added)
    }
}
