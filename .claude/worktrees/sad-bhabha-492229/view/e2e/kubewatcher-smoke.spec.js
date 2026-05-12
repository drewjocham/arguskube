// @ts-check
import { test, expect } from '@playwright/test'

/**
 * P3 E2E Smoke Tests — KubeWatcher SRE Console
 *
 * These tests require the Vite dev server to be running at localhost:5173.
 *
 * Start it with: `npx vite` from the view/ directory.
 *
 * The app mounts a Vue.js SPA backed by a Wails Go backend. Since the backend
 * is not available during browser-based E2E tests, the frontend will render
 * in SaaS/browser mode. Key functionality (sidebar nav, terminal toggle,
 * titlebar, and ArgusCD view) is exercised from the DOM.
 */

const TITLE = 'KubeWatcher — SRE Console'

test.describe('P3 Smoke: KubeWatcher view', () => {

  test('page loads with correct title', async ({ page }) => {
    await page.goto('/')
    await expect(page).toHaveTitle(TITLE)
  })

  test('titlebar renders with app name and badge(s)', async ({ page }) => {
    await page.goto('/')
    // Titlebar shows "KubeWatcher — SRE Console"
    await expect(page.locator('.titlebar')).toBeVisible()
    await expect(page.locator('.titlebar-title')).toContainText('KubeWatcher')
    await expect(page.locator('.titlebar-title')).toContainText('SRE Console')

    // SaaS-mode traffic lights (app is not running in Wails)
    const trafficLights = page.locator('.traffic-lights')
    await expect(trafficLights).toBeVisible()

    // Env badges (PROD, QA) and health dot should be present
    await expect(page.locator('.env-badge')).toHaveCount(2)
    await expect(page.locator('.health-dot')).toBeVisible()
  })

  test('sidebar navigation renders with all 9 sections', async ({ page }) => {
    await page.goto('/')
    const sidebar = page.locator('.sidebar')
    await expect(sidebar).toBeVisible()

    // 9 sections in the nav tree
    const sectionHeaders = page.locator('.section-header')
    await expect(sectionHeaders).toHaveCount(9)

    // Expected section labels
    const expected = [
      'MONITORING', 'CLUSTER', 'WORKLOADS', 'CONFIG',
      'NETWORK', 'STORAGE', 'OPERATIONS', 'KNOWLEDGE', 'ADMIN',
    ]
    const labels = page.locator('.section-label')
    for (let i = 0; i < expected.length; i++) {
      await expect(labels.nth(i)).toHaveText(expected[i])
    }
  })

  test('sidebar nav items are interactive and switch activeNav', async ({ page }) => {
    await page.goto('/')
    // The first visible nav item under 'MONITORING' is 'Metrics Explorer' (id: metrics)
    const navItems = page.locator('.nav-item').filter({ hasText: /^[A-Za-z]/ })
    const first = navItems.first()
    await expect(first).toBeVisible()

    // Click 'Metrics Explorer'
    await first.click()
    await expect(first).toHaveClass(/active/)

    // Now click the 'Pods' nav item (under Workloads section)
    const podsItem = page.locator('.nav-item').filter({ hasText: 'Pods' })
    await expect(podsItem).toBeVisible()
    await podsItem.click()
    await expect(podsItem).toHaveClass(/active/)
  })

  test('sidebar collapse toggle works', async ({ page }) => {
    await page.goto('/')
    const sidebar = page.locator('.sidebar')
    const toggle = page.locator('.collapse-toggle')

    // Initial state — expanded
    await expect(sidebar).not.toHaveClass(/sidebar-collapsed/)

    // Click to collapse
    await toggle.click()
    await expect(sidebar).toHaveClass(/sidebar-collapsed/)

    // Now the icon-only navigation should be visible
    const iconItems = page.locator('.icon-item')
    await expect(iconItems.first()).toBeVisible()

    // Click again to expand
    await toggle.click()
    await expect(sidebar).not.toHaveClass(/sidebar-collapsed/)
    await expect(page.locator('.nav-item').first()).toBeVisible()
  })

  test('sidebar collapsed shows popover on icon click', async ({ page }) => {
    await page.goto('/')
    const toggle = page.locator('.collapse-toggle')
    await toggle.click()

    // Collapsed: click the first icon item
    const firstIcon = page.locator('.icon-item').first()
    await expect(firstIcon).toBeVisible()
    await firstIcon.click()

    // Popover should appear
    const popover = page.locator('.sidebar-popover')
    await expect(popover).toBeVisible()
    await expect(popover.locator('.popover-header')).toBeVisible()

    // Click an item in the popover
    const popoverItems = popover.locator('.popover-item')
    await expect(popoverItems.first()).toBeVisible()
    await popoverItems.first().click()

    // Popover should close
    await expect(popover).not.toBeVisible()
  })

  test('terminal button visible in SaaS mode', async ({ page }) => {
    await page.goto('/')
    // In SaaS/browser mode, the terminal toggle is shown as
    // "Desktop App" and "Native Terminal" buttons in the titlebar
    await expect(page.locator('.tb-saas-btn').first()).toBeVisible()
    await expect(page.locator('.tb-saas-btn.primary').first()).toBeVisible()
  })

  test('ArgusCD view renders under Operations', async ({ page }) => {
    await page.goto('/')
    // Expand the Operations section if needed
    const arguscdNav = page.locator('.nav-item').filter({ hasText: 'ArgusCD' })
    await expect(arguscdNav).toBeVisible()

    const isProLocked = await arguscdNav.locator('.pro-badge').count()
    if (isProLocked > 0) {
      // PRO-locked feature
      await expect(arguscdNav).toHaveClass(/pro-locked/)
    } else {
      // If allowed, clicking switches to the ArgusCD view
      await arguscdNav.click()
      const centerContent = page.locator('.content')
      await expect(centerContent).toContainText('ArgusCD')
    }
  })

  test('cluster selector dropdown opens and closes', async ({ page }) => {
    await page.goto('/')
    const clusterSelector = page.locator('.cluster-selector')
    await expect(clusterSelector).toBeVisible()

    // Open the dropdown
    await clusterSelector.click()
    const dropdown = page.locator('.ctx-dropdown')
    await expect(dropdown).toBeVisible()
    await expect(dropdown.locator('.ctx-dropdown-header')).toHaveText('Kubernetes Contexts')

    // Close by clicking elsewhere
    await page.locator('.titlebar').click()
    await expect(dropdown).not.toBeVisible()
  })
})
