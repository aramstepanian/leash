import SwiftUI

struct ApprovalView: View {
    @EnvironmentObject private var app: AppModel
    @Environment(\.dismiss) private var dismiss

    var body: some View {
        Group {
            if let pending = app.state.pending {
                panel(pending)
            } else {
                VStack(spacing: 8) {
                    Image(systemName: "checkmark.seal")
                        .font(.largeTitle)
                        .foregroundStyle(.secondary)
                    Text("Nothing to approve")
                        .foregroundStyle(.secondary)
                }
                .frame(minHeight: 160)
                .padding(24)
            }
        }
        .background(.ultraThinMaterial)
        .onAppear { app.start() }
        .onChange(of: app.state.pending?.id) { _, new in
            if new == nil { dismiss() }
        }
    }

    private func panel(_ pending: PendingApproval) -> some View {
        VStack(alignment: .leading, spacing: 16) {
            HStack(alignment: .firstTextBaseline) {
                Text("Leash")
                    .font(.caption.weight(.semibold))
                    .foregroundStyle(.secondary)
                Spacer()
                Text(kindLabel(pending.kind))
                    .font(.caption.weight(.semibold))
                    .padding(.horizontal, 8)
                    .padding(.vertical, 3)
                    .background(kindColor(pending.kind).opacity(0.15), in: Capsule())
                    .foregroundStyle(kindColor(pending.kind))
            }

            VStack(alignment: .leading, spacing: 6) {
                Text(pending.title)
                    .font(.title3.weight(.semibold))
                if !pending.reasons.isEmpty {
                    Text(pending.reasons.joined(separator: " · "))
                        .font(.callout)
                        .foregroundStyle(.secondary)
                }
            }

            ScrollView {
                Text(pending.detail)
                    .font(.system(.body, design: .monospaced))
                    .textSelection(.enabled)
                    .frame(maxWidth: .infinity, alignment: .leading)
                    .padding(12)
            }
            .frame(maxHeight: 180)
            .background(Color(nsColor: .textBackgroundColor), in: RoundedRectangle(cornerRadius: 10))
            .overlay(
                RoundedRectangle(cornerRadius: 10)
                    .strokeBorder(.separator, lineWidth: 1)
            )

            HStack(spacing: 8) {
                Button("Kill", role: .destructive) {
                    Task { await app.decide("kill") }
                }
                .keyboardShortcut(.cancelAction)

                Spacer()

                Button("Always") {
                    Task { await app.decide("always") }
                }
                .keyboardShortcut(.return, modifiers: [.command])

                Button("Allow") {
                    Task { await app.decide("allow") }
                }
                .keyboardShortcut(.defaultAction)
                .buttonStyle(.borderedProminent)
            }
        }
        .padding(20)
        .frame(minWidth: 400)
    }

    private func kindLabel(_ kind: String) -> String {
        switch kind {
        case "secret": return "SECRET"
        case "outside": return "OUTSIDE PROJECT"
        case "destroy": return "DANGER"
        default: return kind.uppercased()
        }
    }

    private func kindColor(_ kind: String) -> Color {
        switch kind {
        case "secret": return .orange
        case "outside": return .purple
        case "destroy": return .red
        default: return .blue
        }
    }
}
