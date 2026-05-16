// LoginView — focused tests for the passkey CTA visibility. The
// component imports the auth store; we provide a Pinia instance plus
// a network stub so onMounted's loadProviders() resolves.
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'

const memory = {}
Object.defineProperty(window, 'localStorage', {
  value: {
    getItem: (k) => (k in memory ? memory[k] : null),
    setItem: (k, v) => { memory[k] = String(v) },
    removeItem: (k) => { delete memory[k] },
    clear: () => { for (const k of Object.keys(memory)) delete memory[k] },
  },
  writable: true, configurable: true,
})

vi.mock('../../../composables/useBridge', () => ({ apiBase: '' }))
vi.mock('@simplewebauthn/browser', () => ({
  startAuthentication: vi.fn(async () => ({ id: 'x' })),
  startRegistration: vi.fn(async () => ({ id: 'x' })),
}))

async function loadView(providersResponse) {
  setActivePinia(createPinia())
  vi.stubGlobal('fetch', vi.fn((url) => {
    if (String(url).includes('/auth/providers')) {
      return Promise.resolve({
        ok: true, status: 200,
        text: () => Promise.resolve(JSON.stringify(providersResponse)),
        json: () => Promise.resolve(providersResponse),
      })
    }
    if (String(url).includes('/auth/passkey/login/begin')) {
      return Promise.resolve({ ok: true, status: 200, text: () => Promise.resolve('{}'), json: () => ({}) })
    }
    return Promise.reject(new Error('unmocked: ' + url))
  }))
  const LoginView = (await import('../LoginView.vue')).default
  const wrapper = mount(LoginView)
  await flushPromises()
  return wrapper
}

describe('LoginView — passkey CTA', () => {
  beforeEach(() => {
    for (const k of Object.keys(memory)) delete memory[k]
    vi.resetModules()
  })
  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('hides the passkey button when passkeyEnabled is false', async () => {
    const wrapper = await loadView({
      providers: [], allowSignup: true, passkeyEnabled: false,
    })
    expect(wrapper.find('[data-test="passkey-button"]').exists()).toBe(false)
    expect(wrapper.find('[data-test="passkey-one-tap"]').exists()).toBe(false)
  })

  it('shows the passkey button when passkeyEnabled is true', async () => {
    const wrapper = await loadView({
      providers: [], allowSignup: true, passkeyEnabled: true,
    })
    expect(wrapper.find('[data-test="passkey-button"]').exists()).toBe(true)
  })

  it('uses "username webauthn" autocomplete when passkeys are enabled', async () => {
    const wrapper = await loadView({
      providers: [], allowSignup: true, passkeyEnabled: true,
    })
    const email = wrapper.find('input[type="email"]')
    expect(email.attributes('autocomplete')).toBe('username webauthn')
  })

  it('shows the one-tap CTA instead of the regular button when lastUsedMethod is passkey', async () => {
    memory['argus.auth.lastMethod'] = JSON.stringify({ kind: 'passkey' })
    const wrapper = await loadView({
      providers: [], allowSignup: true, passkeyEnabled: true,
    })
    expect(wrapper.find('[data-test="passkey-one-tap"]').exists()).toBe(true)
    expect(wrapper.find('[data-test="passkey-button"]').exists()).toBe(false)
  })
})
