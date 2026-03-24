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
