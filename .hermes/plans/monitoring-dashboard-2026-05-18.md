# Monitoring Dashboard — Replace Live Log Stream with Rich Metrics

> **Goal:** Remove the Live Log Stream widget from the Alerts dashboard and replace it with a multi-dashboard metrics view featuring categories, expandable metric cards, a free-form canvas for up to 4 chart widgets, and multiple saved dashboard pages.

**Architecture:** New Vue components under `components/center/` — `MonitoringDashboard.vue` (container), `MetricCategoryStrip.vue` (category row), `MetricCategoryPopup.vue` (expandable popup), `MetricWidget.vue` (canvas widget). A new composable `useDashboardMetrics.js` manages metric definitions, dashboard state, and localStorage persistence. CenterPanel wires the new "Dashboards" tab. LogStream is disconnected from the Alerts view.

**Tech Stack:** Vue 3 Composition API, Pinia (sectionTabs for tab routing), localStorage persistence, CSS Grid + CSS custom properties (var(--*) theming from existing design system). PromQL via `QueryTimeSeriesMetrics` backend call. Existing `GetMetrics()` for ClusterMetrics.

---

## Design Summary

### Metrics are organized into categories:
1. **Pod Health** — podHealthPct, podsRunning/pending/failed, restartCount
2. **CPU** — totalCpuMillis, per-node CPU, CPU pressure (via PromQL)
3. **Memory** — totalMemoryBytes, per-node mem, memory pressure (via PromQL)  
4. **Network** — warningEvents, network rx/tx (via PromQL), errorRate
5. **Storage** — PVC usage, PV capacity (via PromQL)
6. **Latency** — p99Latency, SLOStatus, API error rate (via PromQL)

### Layout:
```
┌─────────────────────────────────────────────────────────────┐
│ [Dashboard: Default ▼] [New] [Save] [Edit Layout]            │
├─────────────────────────────────────────────────────────────┤
│ ┌─ Pod Health ──────────────────────── [Expand ▼] ───────┐ │
│ │  ┌──────────┐  ┌──────────┐                             │ │
│ │  │ 95.2%    │  │ 3/42     │  ← 2 toggled metrics        │ │
│ │  │ Pods OK  │  │ Restarts │                             │ │
│ │  └──────────┘  └──────────┘                             │ │
│ └─────────────────────────────────────────────────────────┘ │
│ ┌─ CPU ──────────────────────────────── [Expand ▼] ───────┐ │
│ │  ┌──────────┐  ┌──────────┐                             │ │
│ │  │ 1.4 core │  │ 67% util │                             │ │
│ │  └──────────┘  └──────────┘                             │ │
│ └─────────────────────────────────────────────────────────┘ │
│ ... more categories ...                                      │
├─────────────────────────────────────────────────────────────┤
│                     FREE-FORM CANVAS                         │
│  ┌────────────┐                    ┌────────────┐           │
│  │ CPU Chart  │                    │ Mem Chart  │           │
│  │  ▁▂▃▄▅▆▇   │                    │  ▃▄▅▆▇█▇   │           │
│  └────────────┘                    └────────────┘           │
│       ┌────────────┐                                         │
│       │ Pod Health  │                                        │
│       │  ████████░  │     ← up to 4, free-positioned         │
│       └────────────┘                                         │
└─────────────────────────────────────────────────────────────┘
```

### Dashboard persistence:
- Saved to localStorage as `argus.dashboards.v1`
- Each dashboard has: id, name, categories (which 2 metrics toggled), widgets (which 4 metrics + positions)
- Default dashboard created on first load

---

## Tasks

### Task 1: Define metric categories and definitions
**Files:**
- Create: `kube/view/src/composables/useDashboardMetrics.js`

Defines all metric categories, their metrics, how to fetch/derive each value, and default toggle selections. Exports reactive dashboard state with localStorage persistence.

### Task 2: Create MetricWidget component (canvas widget)
**Files:**
- Create: `kube/view/src/components/center/MetricWidget.vue`

A card widget that renders on the free-form canvas. Shows a metric name, current value, and a miniature time-series sparkline (via QueryTimeSeriesMetrics). In edit mode: draggable via mousedown/mousemove. Has a close button. Props: metric definition, position {x, y}.

### Task 3: Create MetricCategoryPopup component
**Files:**
- Create: `kube/view/src/components/center/MetricCategoryPopup.vue`

Scrollable modal/popup triggered by the "Expand" button on a category strip. Lists ALL metrics in the category as cards. Clicking a card expands it to fill the popup with a detailed chart + sparkline. Close via backdrop click or X button.

### Task 4: Create MetricCategoryStrip component
**Files:**
- Create: `kube/view/src/components/center/MetricCategoryStrip.vue`

Horizontal strip showing a category name, 2 mini metric cards (the "toggled" ones), and an expand button. The user can click on either of the 2 shown metrics to cycle which metrics from the category are displayed. Expand button opens MetricCategoryPopup.

### Task 5: Create MonitoringDashboard component (container)
**Files:**
- Create: `kube/view/src/components/center/MonitoringDashboard.vue`

Top bar: dashboard selector dropdown, new/save/delete dashboard, edit layout toggle. Middle: list of MetricCategoryStrips. Bottom: free-form canvas area showing MetricWidgets (up to 4). Manages dashboard state via useDashboardMetrics composable.

### Task 6: Wire into CenterPanel + sectionTabs, remove LogStream
**Files:**
- Modify: `kube/view/src/lib/sectionTabs.js` — add 'dashboards' tab to monitoring
- Modify: `kube/view/src/components/center/CenterPanel.vue` — import and wire MonitoringDashboard, remove LogStream import and widget from Alerts view

### Task 7: Build and verify
**Files:**
- Run `make build` (or `make lint-go` for Go, frontend check via vite)

Verify no compilation errors, lint passes.
