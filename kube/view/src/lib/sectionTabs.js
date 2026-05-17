// Static definitions for the section/tab navigation model.
//
// Argus's sidebar today renders 9 section headers, each containing
// 2-8 sub-items — 37 items in total. The "Tab Navigation" plan
// collapses that into 9 section buttons in the sidebar, with the
// section's items moving to a tab bar inside the center panel. This
// file is the single source of truth for both the sidebar headers
// and the tab rows.
//
// Three exports:
//   - SECTIONS    : the ordered map of section → {label, icon, tabs[]}
//   - DEFAULT_TABS: which tab to open when the user clicks a section
//                   header (or visits the section via Cmd+K without an
//                   explicit tab)
//   - ALL_NAV_ITEMS: a flat list for the Cmd+K palette. Each entry
//                   carries enough context for the palette to navigate
//                   to (section, tab) in a single dispatch.
//
// Section order is preserved from the existing sidebar navTree so the
// switchover doesn't shuffle the user's spatial memory.

/**
 * @typedef {Object} SectionTab
 * @property {string} id     — stable tab id (matches CenterPanel routing)
 * @property {string} label  — human-readable label
 * @property {boolean} [pro] — pro-tier gated; rendered with a badge
 */

/**
 * @typedef {Object} Section
 * @property {string} id     — stable section id
 * @property {string} label  — human-readable label
 * @property {string} icon   — SVG path data for the sidebar icon
 * @property {SectionTab[]} tabs
 */

/** @type {Object.<string, Section>} */
export const SECTIONS = Object.freeze({
  monitoring: {
    id: 'monitoring',
    label: 'Monitoring',
    icon: 'M2 12h4l3-9 4 18 3-9h4',
    tabs: [
      { id: 'argusai', label: 'Argus AI' },
      { id: 'metrics', label: 'Metrics Explorer' },
      { id: 'alerts', label: 'Alerts' },
      { id: 'vulnerabilities', label: 'Vulnerabilities' },
      { id: 'logs', label: 'Logs' },
      { id: 'anomalies', label: 'Argus Alerting' },
      { id: 'analysis', label: 'Analysis' },
      { id: 'correlator', label: 'Log & Event Timeline' },
      { id: 'waste', label: 'Waste Heatmap' },
      { id: 'finops', label: 'Cost Explorer' },
    ],
  },
  cluster: {
    id: 'cluster',
    label: 'Cluster',
    icon: 'M12 2L2 7l10 5 10-5-10-5zM2 17l10 5 10-5M2 12l10 5 10-5',
    tabs: [
      { id: 'nodes', label: 'Nodes' },
      { id: 'namespaces', label: 'Namespaces' },
      { id: 'events', label: 'Events' },
    ],
  },
  workloads: {
    id: 'workloads',
    label: 'Workloads',
    icon: 'M21 16V8a2 2 0 00-1-1.73l-7-4a2 2 0 00-2 0l-7 4A2 2 0 003 8v8a2 2 0 001 1.73l7 4a2 2 0 002 0l7-4A2 2 0 0021 16z',
    tabs: [
      { id: 'pods', label: 'Pods' },
      { id: 'deployments', label: 'Deployments' },
      { id: 'statefulsets', label: 'StatefulSets' },
      { id: 'daemonsets', label: 'DaemonSets' },
      { id: 'replicasets', label: 'ReplicaSets' },
      { id: 'jobs', label: 'Jobs' },
      { id: 'cronjobs', label: 'Cron Jobs' },
      { id: 'builder', label: 'Builder' },
    ],
  },
  config: {
    id: 'config',
    label: 'Config',
    icon: 'M4 6h16M4 12h16M4 18h16',
    tabs: [
      { id: 'configmaps', label: 'Config Maps' },
      { id: 'secrets', label: 'Secrets' },
      { id: 'hpas', label: 'HPAs' },
    ],
  },
  network: {
    id: 'network',
    label: 'Network',
    icon: 'M12 2a10 10 0 100 20 10 10 0 000-20zM2 12h20M12 2a15 15 0 014 10 15 15 0 01-4 10 15 15 0 01-4-10A15 15 0 0112 2z',
    tabs: [
      { id: 'services', label: 'Services' },
      { id: 'endpoints', label: 'Endpoints' },
      { id: 'ingresses', label: 'Ingresses' },
      { id: 'networkpolicies', label: 'Network Policies' },
      { id: 'external-bridges', label: 'External Bridges' },
      { id: 'label-matcher', label: 'Label Matcher' },
      { id: 'endpoint-topology', label: 'Endpoint Topology' },
    ],
  },
  gateway: {
    id: 'gateway',
    label: 'Gateway',
    icon: 'M4 10h16v4H4zM8 6l-4 4 4 4M16 6l4 4-4 4',
    tabs: [
      { id: 'topology', label: 'Route Topology' },
      { id: 'status', label: 'Status Dashboard' },
      { id: 'migration', label: 'Ingress Migration' },
      { id: 'traffic', label: 'Traffic Splitter' },
    ],
  },
  storage: {
    id: 'storage',
    label: 'Storage',
    icon: 'M4 7v10c0 2 4 4 8 4s8-2 8-4V7M4 7c0 2 4 4 8 4s8-2 8-4M4 7c0-2 4-4 8-4s8 2 8 4',
    tabs: [
      { id: 'pvcs', label: 'Volume Claims' },
      { id: 'pvs', label: 'Volumes' },
      { id: 'storageclasses', label: 'Storage Classes' },
    ],
  },
  operations: {
    id: 'operations',
    label: 'Operations',
    icon: 'M14.7 6.3a1 1 0 000 1.4l1.6 1.6a1 1 0 001.4 0l3.77-3.77a6 6 0 01-7.94 7.94l-6.91 6.91a2.12 2.12 0 01-3-3l6.91-6.91a6 6 0 017.94-7.94l-3.76 3.76z',
    tabs: [
      { id: 'runbooks', label: 'Runbooks' },
      { id: 'incidents', label: 'Incident Log' },
      { id: 'audit', label: 'Config Audit' },
      { id: 'arguscd', label: 'ArgusCD', pro: true },
      { id: 'pipelines', label: 'Pipelines' },
      { id: 'distload', label: 'Load Test' },
    ],
  },
  knowledge: {
    id: 'knowledge',
    label: 'Knowledge',
    icon: 'M4 19.5A2.5 2.5 0 016.5 17H20M4 19.5A2.5 2.5 0 016.5 17H20M6.5 2H20v20H6.5A2.5 2.5 0 014 19.5v-15A2.5 2.5 0 016.5 2z',
    tabs: [
      { id: 'documents', label: 'Documents' },
      { id: 'notebooks', label: 'Notebooks & S3' },
    ],
  },
  workspace: {
    id: 'workspace',
    label: 'Workspace',
    // Briefcase outline — distinct enough from the operations wrench
    // and the knowledge book that it reads at a glance in the sidebar.
    icon: 'M20 7h-4V5a2 2 0 00-2-2h-4a2 2 0 00-2 2v2H4a2 2 0 00-2 2v10a2 2 0 002 2h16a2 2 0 002-2V9a2 2 0 00-2-2zM10 5h4v2h-4V5z',
    tabs: [
      { id: 'connections', label: 'Connections' },
      { id: 'slack', label: 'Slack' },
      { id: 'slack-events', label: 'Slack Events' },
      { id: 'gdocs', label: 'Docs' },
      { id: 'gsheets', label: 'Sheets' },
      { id: 'gtasks', label: 'Tasks' },
      { id: 'gchat', label: 'Google Chat' },
      { id: 'gcal', label: 'Calendar' },
      { id: 'icloud', label: 'iCloud' },
    ],
  },
  admin: {
    id: 'admin',
    label: 'Admin',
    icon: 'M17 21v-2a4 4 0 00-4-4H5a4 4 0 00-4 4v2M9 11a4 4 0 100-8 4 4 0 000 8zM23 21v-2a4 4 0 00-3-3.87M16 3.13a4 4 0 010 7.75',
    tabs: [
      { id: 'setup', label: 'Setup & Tools' },
      { id: 'settings', label: 'Settings' },
      { id: 'rbac', label: 'RBAC & Permissions' },
    ],
  },
})

// Iteration order for the sidebar. JS object key order is insertion-
// preserving for string keys, but we keep this explicit so a future
// alphabetical sort of SECTIONS doesn't silently reshuffle the UI.
export const SECTION_ORDER = Object.freeze([
  'monitoring',
  'cluster',
  'workloads',
  'config',
  'network',
  'gateway',
  'storage',
  'operations',
  'knowledge',
  'workspace',
  'admin',
])

// Which tab opens when the user clicks a section header. Picked
// per-section so the most common entry point lands first:
//   - Monitoring → alerts (the SRE's home view)
//   - Workloads  → pods (the most-visited resource type)
//   - everything else → the section's first tab
export const DEFAULT_TABS = Object.freeze({
  monitoring: 'alerts',
  cluster: 'nodes',
  workloads: 'pods',
  config: 'configmaps',
  network: 'services',
  gateway: 'topology',
  storage: 'pvcs',
  operations: 'runbooks',
  knowledge: 'documents',
  workspace: 'connections',
  admin: 'setup',
})

/**
 * Returns true when `tabId` is a known tab under `sectionId`. Used to
 * validate persisted preferences and Cmd+K targets.
 * @param {string} sectionId
 * @param {string} tabId
 * @returns {boolean}
 */
export function isValidTab(sectionId, tabId) {
  const sec = SECTIONS[sectionId]
  if (!sec) return false
  return sec.tabs.some((t) => t.id === tabId)
}

/**
 * Returns the section id that owns the given tab id, or null when the
 * tab id is not known. Useful for migrating old activeNav values that
 * were tab ids (e.g. 'pods') instead of section ids.
 * @param {string} tabId
 * @returns {string|null}
 */
export function sectionForTab(tabId) {
  for (const id of SECTION_ORDER) {
    if (SECTIONS[id].tabs.some((t) => t.id === tabId)) return id
  }
  return null
}

/**
 * Flat list for the Cmd+K palette. Each entry carries both the section
 * and the tab so a single result can be navigated to in one click.
 */
export const ALL_NAV_ITEMS = Object.freeze(
  SECTION_ORDER.flatMap((sectionId) => {
    const sec = SECTIONS[sectionId]
    return sec.tabs.map((tab) => ({
      sectionId,
      sectionLabel: sec.label,
      tabId: tab.id,
      tabLabel: tab.label,
      pro: !!tab.pro,
      // Composite label used by the palette to render "Section › Tab".
      compositeLabel: `${sec.label} › ${tab.label}`,
    }))
  }),
)
