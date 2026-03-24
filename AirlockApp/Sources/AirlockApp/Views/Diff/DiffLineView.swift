import SwiftUI

struct DiffLineView: View {
    let lineNumber: Int?
    let content: String?
    let type: LineType

    var body: some View {
        HStack(spacing: 0) {
            Text(lineNumber.map { String($0) } ?? "")
                .font(.system(size: 12, design: .monospaced))
                .foregroundStyle(.secondary)
                .frame(width: 44, alignment: .trailing)
                .padding(.trailing, 8)

            Text(content ?? "")
                .font(.system(size: 12, design: .monospaced))
                .frame(maxWidth: .infinity, alignment: .leading)
        }
        .padding(.vertical, 1)
        .padding(.horizontal, 4)
        .background(backgroundColor)
    }

    private var backgroundColor: Color {
        switch type {
        case .added: return Color.green.opacity(0.15)
        case .deleted: return Color.red.opacity(0.15)
        case .context: return .clear
        }
    }
}
