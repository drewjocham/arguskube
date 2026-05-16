import { defineStore } from 'pinia'
import { ref, computed } from 'vue'

const ALL_CATEGORIES = Object.freeze([
  { id: 'getting-started', label: 'Getting Started' },
  { id: 'alerts', label: 'Alerts' },
  { id: 'ai', label: 'AI' },
  { id: 'observability', label: 'Observability' },
  { id: 'security', label: 'Security' },
  { id: 'networking', label: 'Networking' },
  { id: 'storage', label: 'Storage' },
  { id: 'operations', label: 'Operations' },
  { id: 'deployments', label: 'Deployments' },
  { id: 'admin', label: 'Admin' },
])

const GUIDES = Object.freeze([
  {
    sectionId: 'monitoring',
    label: 'Monitoring',
    icon: 'M2 12h4l3-9 4 18 3-9h4',
    categories: ['alerts', 'ai', 'observability'],
    steps: [
      {
        id: 'monitoring-1',
        title: 'View firing alerts',
        instruction: 'The Alerts tab lists every firing alert grouped by severity. Critical alerts appear at the top with a red badge. Click any alert to open the AI diagnostics panel in the right rail.',
        targetSelector: '[data-testid="section-tab-alerts"]',
        navigation: { sectionId: 'monitoring', tabId: 'alerts' },
        durationMs: 5000,
      },
      {
        id: 'monitoring-2',
        title: 'Explore cluster metrics',
        instruction: 'Metrics Explorer shows live CPU, memory, and network graphs for every node, namespace, and pod. Use the time-range picker to zoom in on recent spikes or review historical trends.',
        targetSelector: '[data-testid="section-tab-metrics"]',
        navigation: { sectionId: 'monitoring', tabId: 'metrics' },
        durationMs: 5000,
      },
      {
        id: 'monitoring-3',
        title: 'Run AI diagnostics',
        instruction: 'Argus AI analyzes your cluster state and suggests fixes. Open the chat to ask natural-language questions, or click "Diagnose All" on the Alerts dashboard to batch-analyze every firing alert.',
        targetSelector: '[data-testid="section-tab-argusai"]',
        navigation: { sectionId: 'monitoring', tabId: 'argusai' },
        durationMs: 5000,
      },
    ],
  },
  {
    sectionId: 'cluster',
    label: 'Cluster',
    icon: 'M12 2L2 7l10 5 10-5-10-5zM2 17l10 5 10-5M2 12l10 5 10-5',
    categories: ['observability', 'getting-started'],
    steps: [
      {
        id: 'cluster-1',
        title: 'Browse nodes',
        instruction: 'The Nodes table lists every machine in your cluster with CPU/memory usage, pod count, taints, and kubelet version. Click a node to expand its detail panel with live sparklines and system logs.',
        targetSelector: '[data-testid="section-tab-nodes"]',
        navigation: { sectionId: 'cluster', tabId: 'nodes' },
        durationMs: 5000,
      },
      {
        id: 'cluster-2',
        title: 'Inspect namespaces',
        instruction: 'Namespaces organizes your cluster into logical partitions. Each namespace card shows resource quotas, network policy flow diagrams, and the number of running pods.',
        targetSelector: '[data-testid="section-tab-namespaces"]',
        navigation: { sectionId: 'cluster', tabId: 'namespaces' },
        durationMs: 5000,
      },
      {
        id: 'cluster-3',
        title: 'Review cluster events',
        instruction: 'The Event Stream is a live feed of every Kubernetes event — pod scheduling failures, node condition changes, and volume mount errors. Use the filter bar to narrow by severity or resource type.',
        targetSelector: '[data-testid="section-tab-events"]',
        navigation: { sectionId: 'cluster', tabId: 'events' },
        durationMs: 5000,
      },
    ],
  },
  {
    sectionId: 'workloads',
    label: 'Workloads',
    icon: 'M21 16V8a2 2 0 00-1-1.73l-7-4a2 2 0 00-2 0l-7 4A2 2 0 003 8v8a2 2 0 001 1.73l7 4a2 2 0 002 0l7-4A2 2 0 0021 16z',
    categories: ['deployments', 'observability'],
    steps: [
      {
        id: 'workloads-1',
        title: 'Inspect pods',
        instruction: 'The Pods view lists every pod with live CPU/memory sparklines, restarts, and status. Click a pod to edit its YAML, view logs, or compare resource usage against VPA recommendations.',
        targetSelector: '[data-testid="section-tab-pods"]',
        navigation: { sectionId: 'workloads', tabId: 'pods' },
        durationMs: 5000,
      },
      {
        id: 'workloads-2',
        title: 'Manage deployments',
        instruction: 'Deployments tracks rollout history and revision changes. Click the revision counter to open a timeline showing what changed — image updates, scaling events, or rollbacks.',
        targetSelector: '[data-testid="section-tab-deployments"]',
        navigation: { sectionId: 'workloads', tabId: 'deployments' },
        durationMs: 5000,
      },
    ],
  },
  {
    sectionId: 'config',
    label: 'Config',
    icon: 'M4 6h16M4 12h16M4 18h16',
    categories: ['admin', 'security'],
    steps: [
      {
        id: 'config-1',
        title: 'Review ConfigMaps',
        instruction: 'ConfigMaps stores non-sensitive configuration data. Browse all ConfigMaps by namespace, inspect their key-value pairs, and see which pods mount each config.',
        targetSelector: '[data-testid="sidebar-section-config"]',
        navigation: { sectionId: 'config', tabId: 'configmaps' },
        durationMs: 5000,
      },
      {
        id: 'config-2',
        title: 'Manage secrets',
        instruction: 'Secrets view lists every Kubernetes Secret. Values are masked by default — click to reveal. Use the External Secrets integration to sync from AWS, GCP, Azure, or HashiCorp Vault.',
        targetSelector: '[data-testid="section-tab-secrets"]',
        navigation: { sectionId: 'config', tabId: 'secrets' },
        durationMs: 5000,
      },
    ],
  },
  {
    sectionId: 'network',
    label: 'Network',
    icon: 'M12 2a10 10 0 100 20 10 10 0 000-20zM2 12h20M12 2a15 15 0 014 10 15 15 0 01-4 10 15 15 0 01-4-10A15 15 0 0112 2z',
    categories: ['networking', 'security'],
    steps: [
      {
        id: 'network-1',
        title: 'Browse services',
        instruction: 'The Services table lists every Service with its cluster IP, type, ports, and selector. Click to see the live endpoint topology — a visual map from Service to active backend pods.',
        targetSelector: '[data-testid="section-tab-services"]',
        navigation: { sectionId: 'network', tabId: 'services' },
        durationMs: 5000,
      },
      {
        id: 'network-2',
        title: 'Review network policies',
        instruction: 'Network Policies defines allowed ingress and egress traffic. The policy viewer shows which policies apply to each pod and highlights potential gaps in your zero-trust setup.',
        targetSelector: '[data-testid="section-tab-networkpolicies"]',
        navigation: { sectionId: 'network', tabId: 'networkpolicies' },
        durationMs: 5000,
      },
    ],
  },
  {
    sectionId: 'gateway',
    label: 'Gateway',
    icon: 'M4 10h16v4H4zM8 6l-4 4 4 4M16 6l4 4-4 4',
    categories: ['networking'],
    steps: [
      {
        id: 'gateway-1',
        title: 'Explore route topology',
        instruction: 'The Route Topology graph visualizes how external traffic flows through Gateway API resources — Gateways, HTTPRoutes, and backends. Drag to rearrange, click a node to inspect.',
        targetSelector: '[data-testid="section-tab-topology"]',
        navigation: { sectionId: 'gateway', tabId: 'topology' },
        durationMs: 5000,
      },
      {
        id: 'gateway-2',
        title: 'Check gateway status',
        instruction: 'The Status Dashboard shows the health and readiness of every Gateway and HTTPRoute in your cluster. Programmed vs. pending routes are color-coded for at-a-glance awareness.',
        targetSelector: '[data-testid="section-tab-status"]',
        navigation: { sectionId: 'gateway', tabId: 'status' },
        durationMs: 5000,
      },
    ],
  },
  {
    sectionId: 'storage',
    label: 'Storage',
    icon: 'M4 7v10c0 2 4 4 8 4s8-2 8-4V7M4 7c0 2 4 4 8 4s8-2 8-4M4 7c0-2 4-4 8-4s8 2 8 4',
    categories: ['storage'],
    steps: [
      {
        id: 'storage-1',
        title: 'View volume claims',
        instruction: 'PersistentVolumeClaims (PVCs) request storage for your workloads. Each row shows capacity, access mode, status, and which pod mounts the volume. Click to see live I/O metrics.',
        targetSelector: '[data-testid="section-tab-pvcs"]',
        navigation: { sectionId: 'storage', tabId: 'pvcs' },
        durationMs: 5000,
      },
      {
        id: 'storage-2',
        title: 'Browse volumes',
        instruction: 'PersistentVolumes (PVs) are the backing storage resources. The table shows capacity, reclaim policy, storage class, and which PVC is bound to each volume.',
        targetSelector: '[data-testid="sidebar-section-storage"]',
        navigation: { sectionId: 'storage', tabId: 'pvs' },
        durationMs: 5000,
      },
    ],
  },
  {
    sectionId: 'operations',
    label: 'Operations',
    icon: 'M14.7 6.3a1 1 0 000 1.4l1.6 1.6a1 1 0 001.4 0l3.77-3.77a6 6 0 01-7.94 7.94l-6.91 6.91a2.12 2.12 0 01-3-3l6.91-6.91a6 6 0 017.94-7.94l-3.76 3.76z',
    categories: ['operations', 'alerts'],
    steps: [
      {
        id: 'operations-1',
        title: 'Use runbooks',
        instruction: 'Runbooks are step-by-step operational guides for common incidents. Browse the catalog, search by keyword, or trigger a runbook directly from an alert to auto-remediate.',
        targetSelector: '[data-testid="section-tab-runbooks"]',
        navigation: { sectionId: 'operations', tabId: 'runbooks' },
        durationMs: 5000,
      },
      {
        id: 'operations-2',
        title: 'Log incidents',
        instruction: 'The Incident Log records every alert that required human intervention. Add timestamps, notes, and resolution steps to build a post-mortem trail for your team.',
        targetSelector: '[data-testid="section-tab-incidents"]',
        navigation: { sectionId: 'operations', tabId: 'incidents' },
        durationMs: 5000,
      },
    ],
  },
  {
    sectionId: 'knowledge',
    label: 'Knowledge',
    icon: 'M4 19.5A2.5 2.5 0 016.5 17H20M4 19.5A2.5 2.5 0 016.5 17H20M6.5 2H20v20H6.5A2.5 2.5 0 014 19.5v-15A2.5 2.5 0 016.5 2z',
    categories: ['operations', 'ai'],
    steps: [
      {
        id: 'knowledge-1',
        title: 'Browse documents',
        instruction: 'Documents stores your team wikis and operational notes alongside the cluster view. Create, search, and edit Markdown documents — all changes are synced to your S3 bucket.',
        targetSelector: '[data-testid="section-tab-documents"]',
        navigation: { sectionId: 'knowledge', tabId: 'documents' },
        durationMs: 5000,
      },
      {
        id: 'knowledge-2',
        title: 'Open notebooks',
        instruction: 'S3-backed notebooks give you a writable workspace for live investigations. Run shell commands, inspect YAML, and share results — all persisted to your configured S3 bucket.',
        targetSelector: '[data-testid="section-tab-notebooks"]',
        navigation: { sectionId: 'knowledge', tabId: 'notebooks' },
        durationMs: 5000,
      },
    ],
  },
  {
    sectionId: 'admin',
    label: 'Admin',
    icon: 'M17 21v-2a4 4 0 00-4-4H5a4 4 0 00-4 4v2M9 11a4 4 0 100-8 4 4 0 000 8zM23 21v-2a4 4 0 00-3-3.87M16 3.13a4 4 0 010 7.75',
    categories: ['admin', 'getting-started', 'security'],
    steps: [
      {
        id: 'admin-1',
        title: 'Run setup checklist',
        instruction: 'The Setup Checklist walks through every prerequisite — kubectl, Helm, cluster context, and AI provider keys. Each checked item persists so you can resume where you left off.',
        targetSelector: '[data-testid="section-tab-setup"]',
        navigation: { sectionId: 'admin', tabId: 'setup' },
        durationMs: 5000,
      },
      {
        id: 'admin-2',
        title: 'Configure settings',
        instruction: 'Settings is the control center for every integration: notification channels (Slack, email, webhook), LLM providers, ArgoCD, CI/CD pipelines, Vault credentials, and UI appearance.',
        targetSelector: '[data-testid="section-tab-settings"]',
        navigation: { sectionId: 'admin', tabId: 'settings' },
        durationMs: 5000,
      },
      {
        id: 'admin-3',
        title: 'Manage RBAC',
        instruction: 'RBAC & Permissions shows which roles and bindings exist in your cluster. Review who can do what, spot overly permissive bindings, and audit service account access.',
        targetSelector: '[data-testid="section-tab-rbac"]',
        navigation: { sectionId: 'admin', tabId: 'rbac' },
        durationMs: 5000,
      },
    ],
  },
])

export const useGuidesStore = defineStore('guides', () => {
  const activeGuideId = ref(null)
  const activeStepIndex = ref(0)
  const searchQuery = ref('')
  const activeCategory = ref(null)

  const categories = computed(() => {
    const used = new Set()
    for (const g of GUIDES) {
      for (const c of g.categories) used.add(c)
    }
    return ALL_CATEGORIES.filter((c) => used.has(c.id)).map((c) => ({
      ...c,
      count: GUIDES.filter((g) => g.categories.includes(c.id)).length,
    }))
  })

  const allCategories = computed(() => categories.value)

  const filteredGuides = computed(() => {
    let list = GUIDES
    if (activeCategory.value) {
      list = list.filter((g) => g.categories.includes(activeCategory.value))
    }
    if (searchQuery.value.trim()) {
      const q = searchQuery.value.toLowerCase().trim()
      list = list.filter((g) => {
        if (g.label.toLowerCase().includes(q)) return true
        if (g.steps.some((s) => s.title.toLowerCase().includes(q) || s.instruction.toLowerCase().includes(q))) return true
        return g.categories.some((c) => {
          const cat = ALL_CATEGORIES.find((a) => a.id === c)
          return cat && cat.label.toLowerCase().includes(q)
        })
      })
    }
    return list
  })

  const hasActiveFilter = computed(() => !!searchQuery.value.trim() || !!activeCategory.value)

  const activeGuide = computed(() => {
    if (!activeGuideId.value) return null
    return GUIDES.find((g) => g.sectionId === activeGuideId.value) || null
  })

  const activeStep = computed(() => {
    if (!activeGuide.value) return null
    return activeGuide.value.steps[activeStepIndex.value] || null
  })

  const stepCount = computed(() => activeGuide.value?.steps.length || 0)

  const isLastStep = computed(() => activeStepIndex.value >= stepCount.value - 1)

  const isFirstStep = computed(() => activeStepIndex.value <= 0)

  function setActiveGuide(sectionId) {
    activeGuideId.value = sectionId
    activeStepIndex.value = 0
  }

  function setActiveCategory(categoryId) {
    if (activeCategory.value === categoryId) {
      activeCategory.value = null
    } else {
      activeCategory.value = categoryId
    }
    searchQuery.value = ''
  }

  function setSearchQuery(q) {
    searchQuery.value = q
    if (q) activeCategory.value = null
  }

  function nextStep() {
    if (activeGuide.value && activeStepIndex.value < activeGuide.value.steps.length - 1) {
      activeStepIndex.value++
    }
  }

  function prevStep() {
    if (activeStepIndex.value > 0) {
      activeStepIndex.value--
    }
  }

  function goToStep(index) {
    if (activeGuide.value && index >= 0 && index < activeGuide.value.steps.length) {
      activeStepIndex.value = index
    }
  }

  function reset() {
    activeGuideId.value = null
    activeStepIndex.value = 0
    searchQuery.value = ''
    activeCategory.value = null
  }

  return {
    activeGuideId,
    activeStepIndex,
    searchQuery,
    activeCategory,
    categories,
    allCategories,
    filteredGuides,
    hasActiveFilter,
    activeGuide,
    activeStep,
    stepCount,
    isLastStep,
    isFirstStep,
    setActiveGuide,
    setActiveCategory,
    setSearchQuery,
    nextStep,
    prevStep,
    goToStep,
    reset,
  }
})
