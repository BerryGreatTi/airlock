import Foundation

enum DiffParser {
    static func parse(_ input: String) -> [FileDiff] {
        guard !input.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty else { return [] }

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
                while k < rawLines.count && rawLines[k].type == .deleted { deletions.append(rawLines[k].content); k += 1 }
                var additions: [String] = []
                var m = k
                while m < rawLines.count && rawLines[m].type == .added { additions.append(rawLines[m].content); m += 1 }
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
