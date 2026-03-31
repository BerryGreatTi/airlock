import SwiftUI

@MainActor
struct WelcomeView: View {
    @Bindable var appState: AppState
    @Environment(\.containerService) private var containerService
    @State private var dockerRunning: Bool?
    @State private var volumeReady: Bool?
    @State private var showingNewWorkspace = false

    var body: some View {
        VStack(spacing: 24) {
            Spacer()

            AirlockIconView(size: 128)

            Text("Welcome to Airlock")
                .font(.largeTitle)
                .fontWeight(.bold)

            Text("Secure AI agent sandboxing with encrypted secrets")
                .font(.title3)
                .foregroundStyle(.secondary)

            VStack(alignment: .leading, spacing: 12) {
                preCheckRow(
                    label: "Docker",
                    status: dockerRunning,
                    ok: "Running",
                    fail: "Not running"
                )
                preCheckRow(
                    label: "Claude State Volume",
                    status: volumeReady,
                    ok: "Ready",
                    fail: "Not created"
                )
            }
            .padding()
            .background(Color(nsColor: .controlBackgroundColor))
            .clipShape(RoundedRectangle(cornerRadius: 8))
            .frame(maxWidth: 400)

            Button("Create Your First Workspace") {
                showingNewWorkspace = true
            }
            .buttonStyle(.borderedProminent)
            .controlSize(.large)

            Spacer()
        }
        .frame(maxWidth: .infinity, maxHeight: .infinity)
        .task {
        await checkDocker()
        await checkVolume()
    }
        .sheet(isPresented: $showingNewWorkspace) {
            NewWorkspaceSheet(appState: appState)
        }
    }

    private func preCheckRow(label: String, status: Bool?, ok: String, fail: String) -> some View {
        HStack {
            if let running = status {
                Image(systemName: running ? "checkmark.circle.fill" : "xmark.circle.fill")
                    .foregroundStyle(running ? .green : .red)
            } else {
                ProgressView()
                    .controlSize(.small)
            }
            Text(label)
                .fontWeight(.medium)
            Spacer()
            if let running = status {
                Text(running ? ok : fail)
                    .foregroundStyle(.secondary)
                    .font(.caption)
            }
        }
    }

    private func checkDocker() async {
        dockerRunning = await containerService.isDockerRunning()
    }

    private func checkVolume() async {
        let cli = CLIService()
        let home = FileManager.default.homeDirectoryForCurrentUser.path
        if let result = try? await cli.run(args: ["volume", "status"], workingDirectory: home) {
            volumeReady = result.stdout.contains("ready")
        } else {
            volumeReady = false
        }
    }
}
