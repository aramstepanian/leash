import AppKit
import SwiftUI

struct MissionView: View {
    @EnvironmentObject private var app: AppModel

    var body: some View {
        VStack(alignment: .leading, spacing: 0) {
            header
            Hairline().padding(.vertical, LeashSpace.lg)
            inspector
            if let pending = app.state.pending {
                LeashGateBar(
                    pending: pending,
                    deciding: app.deciding,
                    kill: { Task { await app.decide("kill") } },
                    always: { Task { await app.decide("always") } },
                    allow: { Task { await app.decide("allow") } }
                )
                .padding(.top, LeashSpace.xl)
            } else if let failed = app.state.mission?.failed {
                LeashFailureBar(
                    failed: failed,
                    skip: { Task { await app.skipFail() } },
                    retry: { Task { await app.retry() } }
                )
                .padding(.top, LeashSpace.xl)
            }
            LeashSteerBar(
                text: $app.steerDraft,
                canRewind: app.state.burst != nil,
                onSteer: { Task { await app.steer() } },
                onCut: { Task { await app.interrupt() } },
                onRewind: { Task { await app.undo() } }
            )
            .padding(.top, LeashSpace.xl)
            Hairline().padding(.vertical, LeashSpace.lg)
            HStack(alignment: .top, spacing: LeashSpace.xxl) {
                LeashTapeList(events: mission?.timeline ?? [], selectedID: $app.selectedEventID)
                    .frame(width: LeashLayout.timeline)
                LeashScrub(event: selectedEvent)
            }
            .frame(minHeight: LeashLayout.tapeFloor)
        }
        .padding(LeashSpace.panel)
        .frame(width: LeashLayout.mission.width, height: LeashLayout.mission.height)
        .leashChrome(LeashChrome.mission)
        .onAppear { app.start() }
        .onExitCommand {
            Task { await app.interrupt() }
        }
    }

    private var mission: MissionInfo? { app.state.mission }
    private var phase: String { mission?.phase ?? MissionPhase.idle.rawValue }

    private var selectedEvent: TimelineEvent? {
        (mission?.timeline ?? []).first { $0.id == app.selectedEventID } ?? mission?.timeline.last
    }

    private var header: some View {
        HStack(alignment: .center, spacing: LeashSpace.lg) {
            LeashMark(
                filled: LeashFormat.headerMarkFilled(phase: phase, waiting: app.state.waitingCount),
                tint: MissionPhase.headerTint(phase: phase, waiting: app.state.waitingCount)
            )
            LeashKicker(text: LeashCopy.mission)
            Text(mission?.title ?? LeashCopy.idle)
                .font(LeashType.rowStrong)
                .foregroundStyle(LeashPaint.ink)
                .lineLimit(1)
            if let agent = mission?.agent, !agent.isEmpty {
                LeashMono(text: agent, strong: true)
            }
            Spacer(minLength: LeashSpace.md)
            LeashPhaseLights(phase: phase, waiting: app.state.waitingCount)
        }
    }

    private var inspector: some View {
        VStack(alignment: .leading, spacing: LeashSpace.sm) {
            if let goal = mission?.goal, !goal.isEmpty {
                Text(goal)
                    .font(LeashType.body)
                    .foregroundStyle(LeashPaint.muted)
                    .lineLimit(2)
            }
            if let live = mission?.live {
                LeashInspector(live: live)
            } else if let pending = app.state.pending {
                LeashInspector(live: LeashFormat.waitingCall(pending))
            } else {
                LeashEmptyWell(text: LeashCopy.noTool)
            }
        }
    }
}
