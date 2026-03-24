import SwiftUI

struct SideBySideDiffView: View {
    let fileDiff: FileDiff

    var body: some View {
        VStack(alignment: .leading, spacing: 0) {
            HStack {
                Text(fileDiff.newPath)
                    .font(.system(size: 13, weight: .medium, design: .monospaced))
                Text(fileDiff.changeType.rawValue)
                    .font(.caption)
                    .padding(.horizontal, 6)
                    .padding(.vertical, 2)
                    .background(badgeColor.opacity(0.2))
                    .foregroundStyle(badgeColor)
                    .clipShape(RoundedRectangle(cornerRadius: 4))
                Spacer()
            }
            .padding(8)
            .background(Color(nsColor: .controlBackgroundColor))

            ForEach(fileDiff.hunks) { hunk in
                HStack(spacing: 1) {
                    VStack(spacing: 0) {
                        ForEach(hunk.lines) { line in
                            DiffLineView(lineNumber: line.leftNumber, content: line.leftContent, type: line.leftType)
                        }
                    }
                    .frame(maxWidth: .infinity)

                    Rectangle()
                        .fill(Color.gray.opacity(0.3))
                        .frame(width: 1)

                    VStack(spacing: 0) {
                        ForEach(hunk.lines) { line in
                            DiffLineView(lineNumber: line.rightNumber, content: line.rightContent, type: line.rightType)
                        }
                    }
                    .frame(maxWidth: .infinity)
                }
            }
        }
        .background(Color(nsColor: .textBackgroundColor))
        .clipShape(RoundedRectangle(cornerRadius: 6))
        .overlay(RoundedRectangle(cornerRadius: 6).stroke(Color.gray.opacity(0.2)))
    }

    private var badgeColor: Color {
        switch fileDiff.changeType {
        case .modified: return .blue
        case .added: return .green
        case .deleted: return .red
        }
    }
}
