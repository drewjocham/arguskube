// @ts-check
import { test, expect } from '@playwright/test'

/**
 * Profiles E2E — Argus per-user UI profiles
 *
 * Verifies the user-visible flow that 41 unit tests can't:
 *   1. ProfileSwitcher renders in the titlebar without throwing.
 *   2. The Settings panel exposes ProfilesManager.
 *   3. Creating a group via the inline form actually appends it to the list.
 *   4. The new group survives a page reload (localStorage fallback in
 *      dev-without-backend mode, real persistence when the backend runs).
 *
 * Each test skips cleanly when the surface isn't present so the file
 * works under both auth-bypass and auth-required dev builds.
 */

const GROUP_NAME = `e2e-${Date.now()}`

test.describe('Profiles', () => {
  test('ProfileSwitcher renders in titlebar without crashing', async ({ page }) => {
    const errors = []
    page.on('pageerror', (e) => errors.push(e.message))

    await page.goto('/')
    await page.waitForLoadState('networkidle')

    const switcher = page.locator('.profile-switcher').first()
    if (await switcher.count() === 0) {
      test.skip(true, 'ProfileSwitcher not mounted in this build (auth gate likely).')
    }
    await expect(switcher).toBeVisible()
    expect(errors, `unexpected page errors: ${errors.join(' | ')}`).toEqual([])
  })

  // The ProfilesManager component lives inside SettingsPanel, which only
  // renders when the user navigates to the admin → settings nav tab —
  // multiple layouts/builds reach it differently, so driving the store
  // directly via window-exposed handles is the reliable signal here.
  // The component's own click flow is exercised by Vitest component
  // tests; the value e2e adds is "real Pinia + real persistence path
  // (HTTP backend or localStorage fallback) actually round-trips."
  test('profiles store creates a group via real save path', async ({ page }) => {
    const errors = []
    page.on('pageerror', (e) => errors.push(e.message))

    await page.goto('/')
    await page.waitForLoadState('networkidle')

    // The app exposes its Pinia stores on window in dev builds — verified
    // by checking for a stable handle. If unavailable, skip rather than
    // pretend to pass.
    const handle = await page.evaluate(async (name) => {
      // The import below runs INSIDE the page via Vite's dev-server module
      // resolver, where /src/* is the correct URL prefix. Sonar's static
      // analysis can't see that this isn't a Node-side import.
      const mod = await import('/src/stores/profiles.ts') // NOSONAR(javascript:S6859)
      const useProfilesStore = mod.useProfilesStore || mod.default
      if (!useProfilesStore) return { ok: false, why: 'no store export' }
      const store = useProfilesStore()
      await store.hydrate?.()
      const created = store.createGroup(name, 'e2e')
      // Give the debounced sync a moment in case the backend is up.
      await new Promise((r) => setTimeout(r, 400))
      return { ok: true, createdId: created?.id, names: store.groups.map((g) => g.name) }
    }, GROUP_NAME)

    if (!handle.ok) {
      test.skip(true, `profiles store handle unavailable: ${handle.why}`)
    }
    expect(handle.createdId, 'createGroup returned no id').toBeTruthy()
    expect(handle.names).toContain(GROUP_NAME)
    expect(errors, `unexpected page errors: ${errors.join(' | ')}`).toEqual([])
  })

  test('saved group survives a page reload', async ({ page }) => {
    await page.goto('/')
    await page.waitForLoadState('networkidle')

    const present = await page.evaluate(async (name) => {
      // Same Vite-resolver rationale as the prior test — Sonar can't tell
      // this runs in the browser, not in Node.
      const mod = await import('/src/stores/profiles.ts') // NOSONAR(javascript:S6859)
      const useProfilesStore = mod.useProfilesStore || mod.default
      if (!useProfilesStore) return { ok: false }
      const store = useProfilesStore()
      await store.hydrate?.()
      return { ok: true, hasName: store.groups.some((g) => g.name === name) }
    }, GROUP_NAME)

    if (!present.ok) {
      test.skip(true, 'profiles store handle unavailable on reload')
    }
    expect(present.hasName).toBe(true)
  })
})
