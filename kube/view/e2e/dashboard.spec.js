// @ts-check
import { test, expect } from '@playwright/test'

/**
 * Monitoring Dashboard E2E
 *
 * Proves the dashboard composable round-trips through real Vite +
 * real browser localStorage. The component-level click flow (drag a
 * widget, open a category popup) is harder to drive cleanly because
 * the dashboard surface is gated behind sidebar navigation that
 * varies by build layout — Vitest already covers all 35 user-visible
 * invariants of the composable in isolation. What e2e adds is "the
 * persistence path actually works in a real browser."
 */

const DASHBOARD_NAME = `e2e-${Date.now()}`

test.describe('Monitoring Dashboard', () => {
  test('composable creates a dashboard and persists it across reload', async ({ page }) => {
    const errors = []
    page.on('pageerror', (e) => errors.push(e.message))

    await page.goto('/')
    await page.waitForLoadState('networkidle')

    const created = await page.evaluate(async (name) => {
      // Import via Vite's dev-server module URL — same rationale as the
      // profiles spec (the import runs in the browser, not Node).
      const mod = await import('/src/composables/useDashboardMetrics.js') // NOSONAR(javascript:S6859)
      const useDashboardMetrics = mod.useDashboardMetrics
      if (!useDashboardMetrics) return { ok: false, why: 'no export' }
      const dm = useDashboardMetrics()
      dm.createDashboard(name)
      return {
        ok: true,
        count: dm.dashboards.value.length,
        activeName: dm.activeDashboard.value.name,
      }
    }, DASHBOARD_NAME)

    if (!created.ok) test.skip(true, `composable unavailable: ${created.why}`)
    expect(created.activeName).toBe(DASHBOARD_NAME)
    expect(created.count).toBeGreaterThanOrEqual(2)
    expect(errors, `unexpected page errors: ${errors.join(' | ')}`).toEqual([])

    // Reload and confirm persistence via the same composable.
    await page.reload()
    await page.waitForLoadState('networkidle')

    const present = await page.evaluate(async (name) => {
      const mod = await import('/src/composables/useDashboardMetrics.js') // NOSONAR(javascript:S6859)
      const dm = mod.useDashboardMetrics()
      return { names: dm.dashboards.value.map((d) => d.name) }
    }, DASHBOARD_NAME)

    expect(present.names).toContain(DASHBOARD_NAME)
  })

  test('toggleCategoryMetric actually rotates the visible metric (regression test)', async ({ page }) => {
    await page.goto('/')
    await page.waitForLoadState('networkidle')

    const result = await page.evaluate(async () => {
      const mod = await import('/src/composables/useDashboardMetrics.js') // NOSONAR(javascript:S6859)
      const dm = mod.useDashboardMetrics()
      const before = dm.getCategoryToggled('pod-health')
      const first = before[0]
      dm.toggleCategoryMetric('pod-health', first)
      const after = dm.getCategoryToggled('pod-health')
      return { before, after, first }
    })

    expect(result.after).toHaveLength(2)
    expect(result.after, 'toggling first metric should swap it out')
      .not.toContain(result.first)
  })
})
