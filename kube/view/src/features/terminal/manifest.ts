interface FeatureManifest {
  id: string
  panels?: { id: string; component: () => Promise<{ default: any }> }[]
  tools?: { id: string; invoke: (...args: any[]) => any }[]
  requires?: string[]
  section?: {
    id: string
    label: string
    icon: string
    tabs?: { id: string; label: string; component: () => Promise<{ default: any }>; pro?: boolean }[]
    panel?: () => Promise<{ default: any }>
    defaultTab?: string
  }
}

const manifest: FeatureManifest = {
  id: 'terminal',
  panels: [
    {
      id: 'terminal',
      component: () => import('./TerminalPanel.vue'),
    },
  ],
}

export default manifest
