// Workspace feature manifest.
//
// Section-routed: appears in the sidebar (eventually — currently the
// sidebar still reads its workspace entry from lib/sectionTabs.js until
// that file is consumed by the registry too) and renders inside the
// center panel via <FeatureSection sectionId="workspace" />.
//
// The tab's component is lazy-imported, so workspace code (and any
// service-specific deps like OAuth client logic) doesn't ship in the
// startup chunk.

/** @type {import('../registry').FeatureManifest} */
const manifest = {
  id: 'workspace',
  section: {
    id: 'workspace',
    label: 'Workspace',
    // Briefcase outline — matches the icon already in lib/sectionTabs.js
    // so sidebar visuals don't change when this section is registry-sourced.
    icon: 'M20 7h-4V5a2 2 0 00-2-2h-4a2 2 0 00-2 2v2H4a2 2 0 00-2 2v10a2 2 0 002 2h16a2 2 0 002-2V9a2 2 0 00-2-2zM10 5h4v2h-4V5z',
    defaultTab: 'connections',
    tabs: [
      {
        id: 'connections',
        label: 'Connections',
        component: () => import('./ConnectionsTab.vue'),
      },
    ],
  },
}

export default manifest
