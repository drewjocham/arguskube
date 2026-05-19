import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import {
  useDashboardMetrics,
  METRIC_CATEGORIES,
  formatBytes,
  formatCPU,
  formatPct,
  formatCount,
} from '../useDashboardMetrics'

// Tests cover the user-visible invariants of the Monitoring Dashboard
// composable: dashboard CRUD, "always exactly 2 toggled per category",
// the 4-widget cap, color thresholds, and the real fetch round-trip
// for cluster metrics + sparklines (callGo falls back to fetch when
// window.go is absent — same path the SaaS browser build uses).

const PERSIST_KEY = 'argus.dashboards.v1'

// Other test files in the suite globally redefine window.localStorage
// to a minimal {getItem,setItem} stub. Since vitest reuses the jsdom
// global across files in the same worker, we install our own complete
// memory-backed localStorage before each test so .clear() / .removeItem
// actually work.
let memoryStorage = {}
function installLocalStorageStub() {
  memoryStorage = {}
  Object.defineProperty(window, 'localStorage', {
    configurable: true,
    value: {
      getItem: (k) => (k in memoryStorage ? memoryStorage[k] : null),
      setItem: (k, v) => { memoryStorage[k] = String(v) },
      removeItem: (k) => { delete memoryStorage[k] },
      clear: () => { memoryStorage = {} },
      key: (i) => Object.keys(memoryStorage)[i] ?? null,
      get length() { return Object.keys(memoryStorage).length },
    },
  })
}

describe('useDashboardMetrics', () => {
  beforeEach(() => {
    installLocalStorageStub()
    if (window.go) delete window.go
    vi.clearAllMocks()
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  // ── Formatters ────────────────────────────────────────────────────
  describe('formatters', () => {
    it('formatBytes scales across Ki/Mi/Gi and handles missing values', () => {
      expect(formatBytes(null)).toBe('—')
      expect(formatBytes(NaN)).toBe('—')
      expect(formatBytes(512)).toBe('1 Ki')
      expect(formatBytes(2 * 1024 * 1024)).toBe('2 Mi')
      expect(formatBytes(3 * 1024 * 1024 * 1024)).toBe('3.0 Gi')
    })

    it('formatCPU switches between millis and cores', () => {
      expect(formatCPU(null)).toBe('—')
      expect(formatCPU(250)).toBe('250m')
      expect(formatCPU(1500)).toBe('1.5 cores')
    })

    it('formatPct returns one decimal and a dash for missing', () => {
      expect(formatPct(null)).toBe('—')
      expect(formatPct(99.4567)).toBe('99.5%')
    })

    it('formatCount rounds to integer string', () => {
      expect(formatCount(null)).toBe('—')
      expect(formatCount(3.7)).toBe('4')
    })
  })

  // ── Default dashboard / hydration ─────────────────────────────────
  describe('defaults and hydration', () => {
    it('starts with one Default dashboard when storage is empty', () => {
      const dm = useDashboardMetrics()
      expect(dm.dashboards.value).toHaveLength(1)
      expect(dm.dashboards.value[0].name).toBe('Default')
      expect(dm.activeDashboard.value.id).toBe('default')
    })

    it('hydrates persisted dashboards on construction', () => {
      const saved = [{ id: 'a', name: 'A', categories: {}, widgets: [] }]
      localStorage.setItem(PERSIST_KEY, JSON.stringify(saved))
      const dm = useDashboardMetrics()
      expect(dm.dashboards.value).toHaveLength(1)
      expect(dm.dashboards.value[0].name).toBe('A')
    })

    it('falls back to default when persisted JSON is malformed', () => {
      localStorage.setItem(PERSIST_KEY, '{not json')
      const dm = useDashboardMetrics()
      expect(dm.dashboards.value[0].id).toBe('default')
    })

    it('seeds each category with exactly 2 default toggled metrics', () => {
      const dm = useDashboardMetrics()
      for (const cat of METRIC_CATEGORIES) {
        expect(dm.getCategoryToggled(cat.id)).toHaveLength(2)
      }
    })
  })

  // ── Dashboard CRUD ────────────────────────────────────────────────
  describe('dashboard CRUD', () => {
    it('createDashboard appends, activates, and persists', () => {
      const dm = useDashboardMetrics()
      dm.createDashboard('Ops')
      expect(dm.dashboards.value).toHaveLength(2)
      expect(dm.activeIndex.value).toBe(1)
      expect(dm.activeDashboard.value.name).toBe('Ops')
      const persisted = JSON.parse(localStorage.getItem(PERSIST_KEY))
      expect(persisted[1].name).toBe('Ops')
    })

    it('createDashboard auto-names when no name given', () => {
      const dm = useDashboardMetrics()
      dm.createDashboard()
      expect(dm.dashboards.value[1].name).toBe('Dashboard 2')
    })

    it('deleteDashboard refuses to remove the last one', () => {
      const dm = useDashboardMetrics()
      dm.deleteDashboard(0)
      expect(dm.dashboards.value).toHaveLength(1)
    })

    it('deleteDashboard adjusts activeIndex when removing the active', () => {
      const dm = useDashboardMetrics()
      dm.createDashboard('B')
      dm.createDashboard('C') // activeIndex = 2
      dm.deleteDashboard(2)
      expect(dm.activeIndex.value).toBe(1)
      expect(dm.activeDashboard.value.name).toBe('B')
    })

    it('renameDashboard updates name and persists', () => {
      const dm = useDashboardMetrics()
      dm.renameDashboard(0, 'Renamed')
      expect(dm.dashboards.value[0].name).toBe('Renamed')
      const persisted = JSON.parse(localStorage.getItem(PERSIST_KEY))
      expect(persisted[0].name).toBe('Renamed')
    })

    it('renameDashboard is a no-op for invalid index', () => {
      const dm = useDashboardMetrics()
      dm.renameDashboard(99, 'Nope')
      expect(dm.dashboards.value[0].name).toBe('Default')
    })
  })

  // ── Category toggling — "always exactly 2" invariant ──────────────
  describe('toggleCategoryMetric', () => {
    it('removes a toggled metric and rotates the next available in', () => {
      const dm = useDashboardMetrics()
      const before = dm.getCategoryToggled('pod-health')
      expect(before).toEqual(['pod-health-pct', 'restart-count'])

      dm.toggleCategoryMetric('pod-health', 'pod-health-pct')
      const after = dm.getCategoryToggled('pod-health')
      expect(after).toHaveLength(2)
      expect(after).not.toContain('pod-health-pct')
    })

    it('persists toggle changes', () => {
      const dm = useDashboardMetrics()
      dm.toggleCategoryMetric('cpu', 'cpu-total')
      const persisted = JSON.parse(localStorage.getItem(PERSIST_KEY))
      expect(persisted[0].categories['cpu']).toHaveLength(2)
      expect(persisted[0].categories['cpu']).not.toContain('cpu-total')
    })

    it('is a no-op for unknown category id', () => {
      const dm = useDashboardMetrics()
      expect(() => dm.toggleCategoryMetric('bogus', 'whatever')).not.toThrow()
    })
  })

  // ── Widget add/remove/move with 4-cap ─────────────────────────────
  describe('widgets', () => {
    it('addWidget appends up to 4 and refuses the 5th', () => {
      const dm = useDashboardMetrics()
      // Default dashboard ships with 2 widgets already.
      expect(dm.activeDashboard.value.widgets).toHaveLength(2)
      expect(dm.addWidget('pod-health-pct')).toBe(true)
      expect(dm.addWidget('restart-count')).toBe(true)
      expect(dm.addWidget('cpu-throttle')).toBe(false)
      expect(dm.activeDashboard.value.widgets).toHaveLength(4)
    })

    it('addWidget skips occupied positions', () => {
      const dm = useDashboardMetrics()
      dm.addWidget('pod-health-pct')
      const positions = dm.activeDashboard.value.widgets.map(w => `${w.x},${w.y}`)
      expect(new Set(positions).size).toBe(positions.length)
    })

    it('removeWidget removes by metricId and persists', () => {
      const dm = useDashboardMetrics()
      dm.removeWidget('cpu-util-pct')
      const ids = dm.activeDashboard.value.widgets.map(w => w.metricId)
      expect(ids).not.toContain('cpu-util-pct')
    })

    it('moveWidget updates coordinates of the matched widget', () => {
      const dm = useDashboardMetrics()
      dm.moveWidget('cpu-util-pct', 2, 3)
      const w = dm.activeDashboard.value.widgets.find(w => w.metricId === 'cpu-util-pct')
      expect(w).toMatchObject({ x: 2, y: 3 })
    })
  })

  // ── findMetric + getMetricValue ──────────────────────────────────
  describe('metric resolution', () => {
    it('findMetric returns category+metric for known id', () => {
      const dm = useDashboardMetrics()
      const found = dm.findMetric('cpu-util-pct')
      expect(found?.category.id).toBe('cpu')
      expect(found?.metric.label).toBe('CPU Utilization')
    })

    it('findMetric returns null for unknown id', () => {
      const dm = useDashboardMetrics()
      expect(dm.findMetric('nope')).toBeNull()
    })

    it('getMetricValue returns null before cluster metrics are loaded', () => {
      const dm = useDashboardMetrics()
      const def = dm.findMetric('cpu-util-pct').metric
      expect(dm.getMetricValue(def)).toBeNull()
    })

    it('getMetricValue reads the matching field once metrics arrive', () => {
      const dm = useDashboardMetrics()
      dm.clusterMetrics.value = { podHealthPct: 92.5, podsRunning: 14 }
      const def = dm.findMetric('pod-health-pct').metric
      expect(dm.getMetricValue(def)).toBe(92.5)
    })
  })

  // ── metricColor thresholds ───────────────────────────────────────
  describe('metricColor', () => {
    it('returns "up" inside green range', () => {
      const dm = useDashboardMetrics()
      expect(dm.metricColor({ colorRange: { amber: 70, red: 90 } }, 30)).toBe('up')
    })

    it('crosses to warn at amber threshold', () => {
      const dm = useDashboardMetrics()
      expect(dm.metricColor({ colorRange: { amber: 70, red: 90 } }, 75)).toBe('warn')
    })

    it('crosses to crit at red threshold', () => {
      const dm = useDashboardMetrics()
      expect(dm.metricColor({ colorRange: { amber: 70, red: 90 } }, 95)).toBe('crit')
    })

    it('treats "below green" as crit (inverse-ranged metrics)', () => {
      const dm = useDashboardMetrics()
      expect(dm.metricColor({ colorRange: { green: 90, amber: 70 } }, 60)).toBe('crit')
    })

    it('returns norm for non-numeric values', () => {
      const dm = useDashboardMetrics()
      expect(dm.metricColor({ colorRange: { amber: 1 } }, 'oops')).toBe('norm')
    })
  })

  // ── fetchClusterMetrics + fetchSparkline via real callGo path ─────
  describe('async fetching', () => {
    it('fetchClusterMetrics stores the result on success', async () => {
      const payload = { podHealthPct: 88.0, podsRunning: 10 }
      vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
        ok: true,
        json: vi.fn().mockResolvedValue({ result: payload }),
      }))
      const dm = useDashboardMetrics()
      await dm.fetchClusterMetrics()
      expect(dm.clusterMetrics.value).toEqual(payload)
    })

    it('fetchClusterMetrics swallows errors and leaves metrics null', async () => {
      vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new Error('boom')))
      const dm = useDashboardMetrics()
      await dm.fetchClusterMetrics()
      expect(dm.clusterMetrics.value).toBeNull()
    })

    it('fetchSparkline populates the metric series and clears loading flag', async () => {
      const series = [1, 2, 3, 4]
      vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
        ok: true,
        json: vi.fn().mockResolvedValue({ result: series }),
      }))
      const dm = useDashboardMetrics()
      const def = dm.findMetric('cpu-util-pct').metric
      await dm.fetchSparkline('cpu-util-pct', def)
      expect(dm.sparklines['cpu-util-pct']).toEqual(
        series.map((v, i) => ({ time: i, value: v })),
      )
      expect(dm.loadingSparklines['cpu-util-pct']).toBe(false)
    })

    it('fetchSparkline ignores re-entry while a fetch is already in flight', async () => {
      const fetchMock = vi.fn().mockImplementation(
        () => new Promise(() => { /* never resolves */ }),
      )
      vi.stubGlobal('fetch', fetchMock)
      const dm = useDashboardMetrics()
      const def = dm.findMetric('cpu-util-pct').metric
      dm.fetchSparkline('cpu-util-pct', def)
      dm.fetchSparkline('cpu-util-pct', def)
      expect(fetchMock).toHaveBeenCalledTimes(1)
    })

    it('refreshSparklines fans out to every visible widget + toggled metric', async () => {
      const fetchMock = vi.fn().mockResolvedValue({
        ok: true,
        json: vi.fn().mockResolvedValue({ result: [] }),
      })
      vi.stubGlobal('fetch', fetchMock)
      const dm = useDashboardMetrics()
      await dm.refreshSparklines()
      // Default dashboard: 5 categories × 2 toggled = 10 metric ids, plus
      // the two seeded widgets (cpu-util-pct + mem-util-pct) already
      // appear in their categories so the Set dedupes them.
      const expectedUnique = new Set([
        // pod-health
        'pod-health-pct', 'restart-count',
        // cpu
        'cpu-total', 'cpu-util-pct',
        // memory
        'mem-total', 'mem-util-pct',
        // network
        'error-rate', 'warning-events',
        // latency
        'p99-latency', 'slo-status',
      ])
      expect(fetchMock).toHaveBeenCalledTimes(expectedUnique.size)
    })
  })
})
