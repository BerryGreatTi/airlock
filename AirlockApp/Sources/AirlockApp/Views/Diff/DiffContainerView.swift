import SwiftUI

@MainActor
struct DiffContainerView: View {
    let workspace: Workspace
    @Bindable var appState: AppState
    @State private var fileDiffs: [FileDiff] = []
    @State private var errorMessage: String?
    @State private var isLoading = false

    var body: some View {
        Group {
            if let error = errorMessage {
                ContentUnavailableView {
                    Label("Diff Unavailable", systemImage: "exclamationmark.triangle")
                } description: {
                    Text(error)
                }
            } else if isLoading {
                ProgressView("Loading diff...")
            } else if fileDiffs.isEmpty {
                ContentUnavailableView {
                    Label("No Changes", systemImage: "checkmark.circle")
                } description: {
                    Text("No uncommitted changes in this workspace")
                }
            } else {
                ScrollView {
                    VStack(spacing: 16) {
                        ForEach(fileDiffs) { diff in
                            SideBySideDiffView(fileDiff: diff)
                        }
                    }
                    .padding()
                }
            }
        }
        .task { await loadDiff() }
        .onChange(of: appState.selectedTab) { _, newTab in
            if newTab == .diff {
                Task { await loadDiff() }
            }
        }
        .toolbar {
            Button {
                Task { await loadDiff() }
            } label: {
                Label("Refresh", systemImage: "arrow.clockwise")
            }
            .keyboardShortcut("R", modifiers: [.command, .shift])
        }
    }

    private func loadDiff() async {
        let cli = CLIService()
        guard cli.isGitRepo(path: workspace.path) else {
            errorMessage = "Not a git repository. Diff viewer requires git."
            return
        }
        isLoading = true
        errorMessage = nil
        do {
            let result = try await cli.gitDiff(workingDirectory: workspace.path)
            fileDiffs = DiffParser.parse(result.stdout)
        } catch {
            errorMessage = error.localizedDescription
        }
        isLoading = false
    }
}
