// @ts-check
import { test, expect } from '@playwright/test'

/**
 * AI chat panel E2E — Argus SRE Console
 *
 * Tests the chat panel's structural contract (selectors, empty state,
 * input affordances). Runs in browser/SaaS mode against the Vite dev
 * server; no LLM is called. The panel renders even without a wired
 * agent — that's the contract this spec locks down so a refactor
 * doesn't silently strand the "Argus AI" entry point.
 */

test.describe('AI chat panel', () => {

  test('reachable from sidebar', async ({ page }) => {
    await page.goto('/')
    // The chat panel is reached via the "Argus AI" or "AI" nav item
    // under MONITORING. We match permissively so a rename to
    // "Argus AI" or "Argus Chat" still passes.
    const aiNav = page.locator('.nav-item').filter({ hasText: /^Argus AI|^AI$|Chat/ }).first()
    await expect(aiNav).toBeVisible()
  })

  test('chat-panel scroll region exists', async ({ page }) => {
    await page.goto('/')
    // The chat panel may live in a popout vs. inline view depending
    // on the layout. The scroll element with data-test="chat-scroll"
    // is the canonical hook — assert it's reachable from at least one
    // of the entry points.
    const aiNav = page.locator('.nav-item').filter({ hasText: /^Argus AI|^AI$|Chat/ }).first()
    if (await aiNav.count() > 0) {
      await aiNav.click()
    }
    // If the inline view didn't open the panel, the popout button does.
    const popoutTrigger = page.locator('[data-test="chat-popout-trigger"]')
    if (await popoutTrigger.count() > 0) {
      await popoutTrigger.first().click()
    }
    const scroll = page.locator('[data-test="chat-scroll"]')
    // At least one chat-scroll surface must exist. If both the inline
    // and popout are absent the test fails — that's the regression
    // signal we want.
    await expect(scroll.first()).toBeVisible({ timeout: 2000 })
  })

  test('empty-state message renders without a configured agent', async ({ page }) => {
    await page.goto('/')
    const aiNav = page.locator('.nav-item').filter({ hasText: /^Argus AI|^AI$|Chat/ }).first()
    if (await aiNav.count() > 0) {
      await aiNav.click()
    }
    const popoutTrigger = page.locator('[data-test="chat-popout-trigger"]')
    if (await popoutTrigger.count() > 0) {
      await popoutTrigger.first().click()
    }
    // The empty state introduces what the agent can see — we don't
    // assert exact text because product copy changes; we assert the
    // element renders so a refactor that drops it fails loud.
    const empty = page.locator('.empty-sub, .empty-state').first()
    await expect(empty).toBeVisible({ timeout: 2000 })
  })

})
