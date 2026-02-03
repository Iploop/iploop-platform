import SwiftUI

/// Main view for IPLoop Node app
public struct NodeView: View {
    @StateObject private var nodeService = NodeService.shared
    @State private var isLoading = false
    @State private var errorMessage: String?
    
    public init() {}
    
    public var body: some View {
        ZStack {
            Color.black.ignoresSafeArea()
            
            VStack(spacing: 24) {
                // Header
                Text("IPLoop Node")
                    .font(.largeTitle)
                    .fontWeight(.bold)
                    .foregroundColor(.white)
                
                if let nodeId = NodeStorage.shared.nodeId {
                    Text("Node: \(String(nodeId.prefix(8)))...")
                        .font(.caption)
                        .foregroundColor(.gray)
                }
                
                // Status
                StatusIndicator(status: nodeService.connectionStatus)
                    .padding(.top, 20)
                
                // Toggle Button
                Button(action: toggleNode) {
                    if isLoading {
                        ProgressView()
                            .progressViewStyle(CircularProgressViewStyle(tint: .white))
                    } else {
                        Text(nodeService.isRunning ? "Stop Sharing" : "Start Sharing")
                            .font(.headline)
                            .foregroundColor(.white)
                    }
                }
                .frame(width: 200, height: 56)
                .background(nodeService.isRunning ? Color.red : Color.green)
                .cornerRadius(28)
                .disabled(isLoading)
                
                if let error = errorMessage {
                    Text(error)
                        .font(.caption)
                        .foregroundColor(.red)
                        .padding()
                }
                
                // Live Stats
                if nodeService.isRunning {
                    LiveStatsCard(stats: nodeService.stats)
                        .transition(.opacity)
                }
                
                // Total Stats
                TotalStatsCard()
                
                Spacer()
                
                // Bottom Buttons
                HStack(spacing: 16) {
                    NavigationButton(title: "Earnings", icon: "dollarsign.circle")
                    NavigationButton(title: "Settings", icon: "gear")
                }
                .padding(.bottom, 24)
            }
            .padding()
        }
        .animation(.default, value: nodeService.isRunning)
    }
    
    private func toggleNode() {
        isLoading = true
        errorMessage = nil
        
        Task {
            do {
                if nodeService.isRunning {
                    nodeService.stop()
                } else {
                    try await nodeService.start()
                }
            } catch {
                errorMessage = error.localizedDescription
            }
            isLoading = false
        }
    }
}

struct StatusIndicator: View {
    let status: ConnectionStatus
    
    var body: some View {
        HStack(spacing: 8) {
            Circle()
                .fill(statusColor)
                .frame(width: 12, height: 12)
            
            Text(statusText)
                .font(.title3)
                .foregroundColor(statusColor)
        }
    }
    
    private var statusColor: Color {
        switch status {
        case .disconnected: return .gray
        case .connecting, .reconnecting: return .yellow
        case .connected: return .green
        }
    }
    
    private var statusText: String {
        switch status {
        case .disconnected: return "Inactive"
        case .connecting: return "Connecting..."
        case .connected: return "Active"
        case .reconnecting: return "Reconnecting..."
        }
    }
}

struct LiveStatsCard: View {
    let stats: NodeStats
    
    var body: some View {
        VStack(alignment: .leading, spacing: 12) {
            Text("This Session")
                .font(.caption)
                .foregroundColor(.gray)
            
            HStack {
                StatItem(title: "Data Shared", value: formatBytes(stats.totalBytesTransferred))
                Spacer()
                StatItem(title: "Requests", value: "\(stats.requestsHandled)")
            }
        }
        .padding()
        .background(Color.white.opacity(0.1))
        .cornerRadius(12)
    }
}

struct TotalStatsCard: View {
    var body: some View {
        VStack(alignment: .leading, spacing: 12) {
            Text("All Time Stats")
                .font(.caption)
                .foregroundColor(.gray)
            
            HStack {
                StatItem(
                    title: "Total Earnings",
                    value: String(format: "$%.4f", NodeStorage.shared.totalEarnings),
                    valueColor: .green
                )
                Spacer()
                StatItem(
                    title: "Total Data",
                    value: formatBytes(NodeStorage.shared.totalBytes)
                )
            }
        }
        .padding()
        .background(Color.white.opacity(0.1))
        .cornerRadius(12)
    }
}

struct StatItem: View {
    let title: String
    let value: String
    var valueColor: Color = .white
    
    var body: some View {
        VStack(alignment: .leading, spacing: 4) {
            Text(title)
                .font(.caption2)
                .foregroundColor(.gray)
            Text(value)
                .font(.title2)
                .fontWeight(.bold)
                .foregroundColor(valueColor)
        }
    }
}

struct NavigationButton: View {
    let title: String
    let icon: String
    
    var body: some View {
        Button(action: {}) {
            HStack {
                Image(systemName: icon)
                Text(title)
            }
            .foregroundColor(.white)
            .frame(maxWidth: .infinity)
            .padding()
            .overlay(
                RoundedRectangle(cornerRadius: 12)
                    .stroke(Color.white.opacity(0.3), lineWidth: 1)
            )
        }
    }
}

// MARK: - Helpers

func formatBytes(_ bytes: Int64) -> String {
    let formatter = ByteCountFormatter()
    formatter.countStyle = .binary
    return formatter.string(fromByteCount: bytes)
}

#Preview {
    NodeView()
}
