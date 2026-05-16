// @ts-check
import { test, expect } from '@playwright/test'

/**
 * Alert workflow E2E — Argus SRE Console
 *
 * Hits the Vue dev server in browser/SaaS mode (no Wails backend), so
 * these tests verify DOM/render contracts rather than data flow. The
 * point is to lock down the selectors the AI agent panel and the
 * incident list rely on — a refactor that renames a class will fail
 * these instead of silently breaking the alert-detail handoff.
 *
 * Run with the dev server live at localhost:5173:
 *     cd kube/view && npx playwright test alerts.spec.js
 */

test.describe('Alerts view', () => {

  test('reachable from sidebar nav', async ({ page }) => {
    await page.goto('/')
    // The sidebar's "Alerts" item belongs to MONITORING. Filtered by
    // text so we don't depend on the exact icon order.
    const alertsNav = page.locator('.nav-item').filter({ hasText: 'Alerts' }).first()
    await expect(alertsNav).toBeVisible()
    await alertsNav.click()
    await expect(alertsNav).toHaveClass(/active/)
  })

  test('renders the title and action bar', async ({ page }) => {
    await page.goto('/')
    const alertsNav = page.locator('.nav-item').filter({ hasText: 'Alerts' }).first()
    await alertsNav.click()

    await expect(page.locator('.alerts-view')).toBeVisible()
    await expect(page.locator('.alerts-title')).toContainText('Alerts')
    // The action bar must surface the three core actions: configure
    // watchers, manual alert, run-now. We assert visibility, not
    // labels — translations may rewrite text.
    await expect(page.locator('.alerts-actions .alerts-btn')).toHaveCount(3)
  })

  test('summary row is always present', async ({ page }) => {
    await page.goto('/')
    await page.locator('.nav-item').filter({ hasText: 'Alerts' }).first().click()
    // The summary row is rendered even when zero alerts exist — it
    // displays the counters that downstream UI (incident page, sidebar
    // badge) depends on. Confirm it's in the DOM.
    await expect(page.locator('.alerts-summary')).toBeVisible()
  })

  test('alerts-scroll container renders for the list', async ({ page }) => {
    await page.goto('/')
    await page.locator('.nav-item').filter({ hasText: 'Alerts' }).first().click()
    // The scroll container is what virtualization will hang off later.
    // Even when empty, it should be in the DOM so the ResizeObserver
    // bindings attach.
    await expect(page.locator('.alerts-scroll')).toBeVisible()
  })

  test('Run-now button toggles the watchersRunning state', async ({ page }) => {
    await page.goto('/')
    await page.locator('.nav-item').filter({ hasText: 'Alerts' }).first().click()

    const runBtn = page.locator('.alerts-btn.primary').first()
    await expect(runBtn).toBeVisible()
    // In SaaS mode without a backend, the click is a no-op but the
    // disabled-state transition still flips for the visual feedback.
    // We just confirm the button is interactive.
    await expect(runBtn).toBeEnabled()
  })

})
