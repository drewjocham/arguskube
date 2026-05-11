---
name: KubeWatcher SRE Console UX Reference
description: Target UX design for the KubeWatcher desktop app — macOS-native dark theme with AI diagnostics panel, saved as kube_watcher_sre_reference.html in workspace
type: project
originSessionId: 2be8f17e-a440-4ec2-9711-c82c203c4f32
---
The definitive UX reference for the KubeWatcher SRE Console is `kube_watcher_sre_reference.html` in the project workspace.

**Why:** The user designed this as the north-star UI — all future frontend work should match this aesthetic and interaction model.

**How to apply:** Use this file as the design spec when building or reviewing any KubeWatcher frontend component.

## Design Language
- **macOS-native feel**: traffic light window controls, frosted-dark surfaces (#1a1c1e base), 0.5px crisp borders (rgba white at 7-12%)
- **Typography**: Segoe UI Variable (UI) + Cascadia Mono (data/code) — 13px base, 10-11px labels, 22px metric values
- **Layout**: three-column — sidebar (220px, cluster selector + nav + AI context card), center (metrics grid + alert list + log stream + topology map), right panel (310px AI Diagnostics)
- **Color tokens**: accent blue #4f8ef7, red #f05454, amber #f5a623, green #3ecf8e, purple #a78bfa, teal #2dd4bf

## Core Innovation: Context-First AI Diagnostics (Right Panel)
- The AI agent is named **Argus** — use this name in UI labels (panel header, chat role, investigating states)
- Every alert auto-passes full context to the AI panel: pod name, namespace, restart count, memory limits, deploy timestamp, image version
- AI reads from DECISION_LOG.md — honouring AGENTS.md rule 1 natively (shown in purple context block)
- Root cause hypothesis with confidence, not just raw data
- Runbook steps include ready-to-paste `kubectl` commands
- Multi-alert cascade correlation: e.g. CPU throttle → fix DiskPressure first because metrics-server got evicted, breaking HPA autoscaling

## Key Interaction Patterns
- Click an alert → right panel updates with context, hypothesis, and steps
- "Diagnose ↗" buttons on each alert card
- Text input at bottom of AI panel for follow-up questions
- Live log stream with color-coded severity (auto-scrolling, new entries every ~3s)
- Service topology mini-map showing blast radius (color-coded node health)
- Context strip below tabs showing active filters (cluster, namespaces, decision log, node count, last deploy time)
- Sparkline bars in metric cards for trend visualization