import SwiftUI

private struct ProxyLogEntry: Identifiable {
    let id: UUID = UUID()
    let time: String
    let host: String
    let action: String
    let location: String?
    let key: String?
}

struct ContainerStatusView: View {
    let workspace: Workspace
    @Bindable var appState: AppState
    @State private var logEntries: [ProxyLogEntry] = []
    @State private var logProcess: Process?
    @State private var autoScroll = true
    @State private var decryptCount = 0
    @State private var passthroughCount = 0
    @State private var noneCount = 0

    var body: some View {
        if appState.isActive(workspace) {
            activeContent
                .onAppear { startLogStream() }
                .onDisappear { stopLogStream() }
        } else {
            inactiveContent
        }
    }

    private var activeContent: some View {
        VStack(spacing: 0) {
            containerCards
            Divider()
            proxyLogSection
        }
    }

    private var containerCards: some View {
        ScrollView(.horizontal, showsIndicators: false) {
            HStack(spacing: 12) {
                containerCard(
                    title: "Agent",
                    name: workspace.containerName,
                    icon: "cpu",
                    details: ["Status: Running"]
                )
                containerCard(
                    title: "Proxy",
                    name: workspace.proxyName,
                    icon: "network",
                    details: ["Status: Running", "Port: 8080"]
                )
                containerCard(
                    title: "Network",
                    name: "airlock-net-\(workspace.shortID)",
                    icon: "link",
                    details: ["Driver: bridge"]
                )
            }
            .padding()
        }
        .frame(height: 120)
        .background(Color(nsColor: .controlBackgroundColor))
    }

    private func containerCard(title: String, name: String, icon: String, details: [String]) -> some View {
        VStack(alignment: .leading, spacing: 4) {
            HStack {
                Image(systemName: icon)
                    .foregroundStyle(Color.accentColor)
                Text(title)
                    .fontWeight(.semibold)
            }
            Text(name)
                .font(.caption)
                .fontDesign(.monospaced)
                .foregroundStyle(.secondary)
            ForEach(details, id: \.self) { detail in
                Text(detail)
                    .font(.caption2)
                    .foregroundStyle(.secondary)
            }
        }
        .padding(8)
        .frame(minWidth: 180, alignment: .leading)
        .background(Color(nsColor: .textBackgroundColor))
        .clipShape(RoundedRectangle(cornerRadius: 8))
    }

    private var proxyLogSection: some View {
        VStack(spacing: 0) {
            logToolbar
            Divider()
            logTable
            Divider()
            logSummary
        }
    }

    private var logToolbar: some View {
        HStack {
            Text("Proxy Activity Log")
                .fontWeight(.medium)
            Spacer()
            Toggle("Auto-scroll", isOn: $autoScroll)
                .toggleStyle(.switch)
                .controlSize(.small)
            Button("Clear") {
                logEntries.removeAll()
                decryptCount = 0
                passthroughCount = 0
                noneCount = 0
            }
            .controlSize(.small)
        }
        .padding(.horizontal, 8)
        .padding(.vertical, 4)
        .background(Color(nsColor: .controlBackgroundColor))
    }

    private var logTable: some View {
        Table(logEntries) {
            TableColumn("Time") { entry in
                Text(entry.time)
                    .fontDesign(.monospaced)
                    .font(.caption)
            }
            .width(min: 60, ideal: 70)

            TableColumn("Host") { entry in
                Text(entry.host)
                    .fontDesign(.monospaced)
                    .font(.caption)
            }
            .width(min: 150, ideal: 250)

            TableColumn("Result") { entry in
                HStack(spacing: 4) {
                    Circle()
                        .fill(actionColor(entry.action))
                        .frame(width: 6, height: 6)
                    Text(entry.action)
                        .font(.caption)
                }
            }
            .width(min: 80, ideal: 100)

            TableColumn("Location") { entry in
                Text(entry.location ?? "-")
                    .font(.caption)
                    .foregroundStyle(.secondary)
            }
            .width(min: 60, ideal: 80)
        }
    }

    private var logSummary: some View {
        HStack(spacing: 16) {
            Text("\(logEntries.count) requests")
            HStack(spacing: 4) {
                Circle().fill(.green).frame(width: 6, height: 6)
                Text("\(decryptCount) decrypted")
            }
            HStack(spacing: 4) {
                Circle().fill(.blue).frame(width: 6, height: 6)
                Text("\(passthroughCount) passthrough")
            }
            HStack(spacing: 4) {
                Circle().fill(.secondary).frame(width: 6, height: 6)
                Text("\(noneCount) none")
            }
            Spacer()
        }
        .font(.caption)
        .padding(.horizontal, 8)
        .padding(.vertical, 4)
        .background(Color(nsColor: .controlBackgroundColor))
    }

    private var inactiveContent: some View {
        ContentUnavailableView {
            Label("No Containers Running", systemImage: "shippingbox")
        } description: {
            Text("Activate workspace to start containers")
        }
    }

    private func actionColor(_ action: String) -> Color {
        switch action {
        case "decrypt": return .green
        case "passthrough": return .blue
        default: return .secondary
        }
    }

    @State private var logPipe: Pipe?

    private func startLogStream() {
        guard let dockerPath = CLIService.findInPath("docker") else { return }
        let process = Process()
        process.executableURL = URL(fileURLWithPath: dockerPath)
        process.arguments = ["logs", "--follow", "--tail", "100", workspace.proxyName]
        process.environment = CLIService.enrichedEnvironment()
        let pipe = Pipe()
        process.standardOutput = pipe
        process.standardError = FileHandle.nullDevice

        pipe.fileHandleForReading.readabilityHandler = { [weak pipe] handle in
            let data = handle.availableData
            guard !data.isEmpty else {
                // EOF -- clean up handler to prevent tight loop
                handle.readabilityHandler = nil
                return
            }
            guard let line = String(data: data, encoding: .utf8) else { return }
            for rawLine in line.components(separatedBy: .newlines) where !rawLine.isEmpty {
                if let entry = self.parseLogLine(rawLine) {
                    Task { @MainActor in
                        self.logEntries.append(entry)
                        switch entry.action {
                        case "decrypt": self.decryptCount += 1
                        case "passthrough": self.passthroughCount += 1
                        default: self.noneCount += 1
                        }
                    }
                }
            }
            _ = pipe // prevent unused warning
        }

        do {
            try process.run()
            logProcess = process
            logPipe = pipe
        } catch {
            // Docker not available or container not running
        }
    }

    private func stopLogStream() {
        logPipe?.fileHandleForReading.readabilityHandler = nil
        logPipe = nil
        logProcess?.terminate()
        logProcess = nil
    }

    private func parseLogLine(_ line: String) -> ProxyLogEntry? {
        guard let data = line.data(using: .utf8),
              let json = try? JSONSerialization.jsonObject(with: data) as? [String: Any],
              let host = json["host"] as? String,
              let action = json["action"] as? String else {
            return nil
        }
        return ProxyLogEntry(
            time: json["time"] as? String ?? "",
            host: host,
            action: action,
            location: json["location"] as? String,
            key: json["key"] as? String
        )
    }
}
