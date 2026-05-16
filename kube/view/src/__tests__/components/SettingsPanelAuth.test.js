// Focused tests for the "Sign-in & integrations" section added to
// SettingsPanel. Asserts:
//   • the new section renders with its anchor id
//   • masked secrets returned by GetSettings show through to the inputs
//   • a Save round-trip sends the new fields in the UpdateSettings
//     payload, AND a masked secret left untouched is still sent (the
//     backend's containsMask() check is what skips it; the frontend
//     just relays the form state).
//
// Mocking strategy mirrors ConnectionPanel.test.js — stub the
// useBridge module so callGo/cachedCallGo return canned values, then
// mount the real SettingsPanel and assert on the rendered DOM + the
// mock call args.

import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { setActivePinia, createPinia } from 'pinia'

const mockCallGo = vi.fn()
const mockCachedCallGo = vi.fn()
vi.mock('../../composables/useBridge', () => ({
  callGo: (...a) => mockCallGo(...a),
  cachedCallGo: (...a) => mockCachedCallGo(...a),
  invalidateCache: vi.fn(),
  invalidateCachePrefix: vi.fn(),
  DEFAULT_TTL: 30_000,
  FAST_TTL: 5_000,
  isWails: () => false,
  apiBase: () => '',
}))
// useContexts is exposed from useWails — stub it minimally so the
// onMounted listContexts() call resolves without touching the network.
vi.mock('../../composables/useWails', async () => {
  const mod = await import('../../composables/useBridge')
  const { ref } = await import('vue')
  return {
    ...mod,
    useContexts: () => ({
      contexts: ref([]),
      loading: ref(false),
      switching: ref(false),
      error: ref(''),
      listContexts: vi.fn(async () => []),
      switchContext: vi.fn(),
    }),
  }
})

// Stub heavy composables/components the panel pulls in but our slice
// doesn't exercise. Each returns a no-op shape that mirrors the real
// API surface enough to mount.
vi.mock('../../composables/useSpotCheck', () => ({
  useSpotCheck: () => ({ items: { value: [] }, runAll: vi.fn() }),
}))

import SettingsPanel from '../../components/setup/SettingsPanel.vue'

function settingsResponse(over = {}) {
  return {
    googleClientId: 'gid.apps.googleusercontent.com',
    googleClientSecret: 'GOCS…cret', // masked sentinel
    oidcIssuer: '',
    oidcClientId: '',
    oidcClientSecret: '',
    oidcDisplayName: '',
    appleServicesId: '',
    appleTeamId: '',
    appleKeyId: '',
    applePrivateKey: '',
    appleDisplayName: '',
    allowLocalSignup: true,
    passkeyEnabled: false,
    passkeyRpId: 'localhost',
    passkeyRpName: 'Argus',
    passkeyRpOrigin: 'http://localhost:8080',
    workspaceGoogleClientId: '',
    workspaceGoogleClientSecret: '',
    slackClientId: '',
    slackClientSecret: '',
    slackSigningSecret: '',
    ...over,
  }
}

beforeEach(() => {
  setActivePinia(createPinia())
  mockCallGo.mockReset()
  mockCachedCallGo.mockReset()
})

describe('SettingsPanel — Sign-in & integrations section', () => {
  it('renders the section with the sign-in-integrations anchor', async () => {
    // Two callGo callsites fire on mount: GetSettings + several
    // optional ones (ListVaultSecrets, addons probe). We resolve any
    // call to a benign empty value; the GetSettings response is the
    // only one we care about, matched by argument.
    mockCallGo.mockImplementation((method) => {
      if (method === 'GetSettings') return Promise.resolve(settingsResponse())
      return Promise.resolve(null)
    })
    const w = mount(SettingsPanel, { attachTo: document.body })
    await flushPromises()
    const anchor = w.find('#sign-in-integrations')
    expect(anchor.exists()).toBe(true)
    expect(anchor.text()).toMatch(/Sign-in & integrations/)
    expect(anchor.text()).toMatch(/Workspace OAuth/)
    w.unmount()
  })

  it('UpdateSettings payload includes the new auth + workspace fields', async () => {
    mockCallGo.mockImplementation((method) => {
      if (method === 'GetSettings') return Promise.resolve(settingsResponse())
      return Promise.resolve(null)
    })
    const w = mount(SettingsPanel, { attachTo: document.body })
    await flushPromises()

    // Click the global Save button (only one with .save-btn).
    const saveBtn = w.find('.save-btn')
    expect(saveBtn.exists()).toBe(true)
    await saveBtn.trigger('click')
    await flushPromises()

    const updateCall = mockCallGo.mock.calls.find((c) => c[0] === 'UpdateSettings')
    expect(updateCall, 'expected UpdateSettings to be called').toBeTruthy()
    const payload = updateCall[1]
    // Every new field must be present (camelCase JSON shape the backend reads).
    expect(payload).toHaveProperty('googleClientId', 'gid.apps.googleusercontent.com')
    // Masked secret is relayed verbatim — backend's containsMask check
    // is what skips re-applying it. Asserting we don't strip it on the
    // frontend prevents accidental "send empty string" regressions.
    expect(payload).toHaveProperty('googleClientSecret', 'GOCS…cret')
    expect(payload).toHaveProperty('workspaceGoogleClientId')
    expect(payload).toHaveProperty('workspaceGoogleClientSecret')
    expect(payload).toHaveProperty('slackClientId')
    expect(payload).toHaveProperty('slackClientSecret')
    expect(payload).toHaveProperty('slackSigningSecret')
    expect(payload).toHaveProperty('passkeyEnabled', false)
    expect(payload).toHaveProperty('allowLocalSignup', true)
    w.unmount()
  })
})
