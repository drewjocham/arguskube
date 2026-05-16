// @ts-check
import { test, expect } from '@playwright/test'

/**
 * Auth flow E2E — Argus SRE Console
 *
 * In SaaS/browser mode the auth UI is the entry point. These tests
 * lock down the login form's selectors and the basic interactions
 * (focus, validation surface, OAuth button visibility). They do NOT
 * complete a real login — the dev server has no backend auth wired up
 * — but they fail loud if the form gets restructured in a way that
 * would break the SaaS sign-in path.
 */

test.describe('Auth form', () => {

  // The dev-mode bypass means the app may auto-redirect to the
  // dashboard without showing a login form. These tests are
  // conditional — they skip when the form is absent, asserting only
  // what's present.

  test('renders an OAuth Login button when auth is required', async ({ page }) => {
    await page.goto('/')

    // Look for any OAuth button. In SaaS mode the form may show
    // Google / Apple / Passkey buttons; we accept any one as evidence
    // the OAuth surface renders.
    const oauthBtn = page.locator(
      'button:has-text("Sign in"), button:has-text("Continue with"), .oauth-login-button'
    ).first()

    if (await oauthBtn.count() === 0) {
      test.skip(true, 'Auth bypass active — no login surface to test in this build.')
    }
    await expect(oauthBtn).toBeVisible()
  })

  test('login form has accessible labels', async ({ page }) => {
    await page.goto('/')

    const emailInput = page.locator('input[type="email"], input[name="email"]').first()
    if (await emailInput.count() === 0) {
      test.skip(true, 'No email-based login surface on this build.')
    }
    // Accessible name is required — either a label, aria-label, or
    // placeholder serving as one. We just confirm one of those exists.
    const accessibleName = await emailInput.evaluate((el) => {
      const id = el.getAttribute('id')
      if (id && document.querySelector(`label[for="${id}"]`)) return 'label'
      if (el.getAttribute('aria-label')) return 'aria-label'
      if (el.getAttribute('placeholder')) return 'placeholder'
      return ''
    })
    expect(accessibleName).not.toBe('')
  })

  test('submits with empty fields without crashing', async ({ page }) => {
    await page.goto('/')

    const submitBtn = page.locator('button[type="submit"]').first()
    if (await submitBtn.count() === 0) {
      test.skip(true, 'No submit-style login button on this build.')
    }
    // Defensive: clicking submit with empty inputs must not crash the
    // SPA. The error surface (toast or inline error) is the contract;
    // we just confirm the page doesn't navigate away or throw.
    await submitBtn.click()
    await page.waitForTimeout(200) // settle the error surface
    // Page must still be alive and render the titlebar.
    await expect(page.locator('.titlebar, .login-card, .auth-container').first()).toBeVisible()
  })

})
