import { describe, it, expect, beforeEach, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { setActivePinia, createPinia } from 'pinia'
import LoginView from '../../components/auth/LoginView.vue'
import { useAuthStore } from '../../stores/auth'

// LoginView pulls in the auth store on mount; we stub out loadProviders
// and restoreSession so the test isn't waiting on a real network call.

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

describe('LoginView.vue — password reveal', () => {
  let auth
  beforeEach(() => {
    setActivePinia(createPinia())
    for (const k of Object.keys(memory)) delete memory[k]
    auth = useAuthStore()
    auth.loadProviders = vi.fn().mockResolvedValue(undefined)
    auth.restoreSession = vi.fn().mockResolvedValue(false)
  })

  it('renders the password field masked by default', async () => {
    const w = mount(LoginView, { attachTo: document.body })
    // The form has a single visible password input (the confirm-password
    // field is only rendered in signup mode).
    const passInput = w.find('input.ri-input')
    expect(passInput.exists()).toBe(true)
    expect(passInput.attributes('type')).toBe('password')
    w.unmount()
  })

  it('shows an eye toggle next to the password field', () => {
    const w = mount(LoginView, { attachTo: document.body })
    const eye = w.find('button.ri-toggle')
    expect(eye.exists()).toBe(true)
    expect(eye.attributes('aria-label')).toBe('Show value')
    w.unmount()
  })

  it('flips the password field to plaintext on eye click', async () => {
    const w = mount(LoginView, { attachTo: document.body })
    await w.find('input.ri-input').setValue('hunter22long!')
    await w.find('button.ri-toggle').trigger('click')
    expect(w.find('input.ri-input').attributes('type')).toBe('text')
    expect(w.find('input.ri-input').element.value).toBe('hunter22long!')
    w.unmount()
  })

  it('renders TWO password fields (with their own eyes) in signup mode', async () => {
    const w = mount(LoginView, { attachTo: document.body })
    // Switch to signup mode.
    const signupTab = w.findAll('button.tab').find((b) => /Create account/i.test(b.text()))
    expect(signupTab).toBeTruthy()
    await signupTab.trigger('click')

    const passwordFields = w.findAll('input.ri-input')
    expect(passwordFields.length).toBe(2)
    const eyes = w.findAll('button.ri-toggle')
    expect(eyes.length).toBe(2)
    // Each eye toggle is independent.
    await eyes[0].trigger('click')
    expect(passwordFields[0].attributes('type')).toBe('text')
    expect(passwordFields[1].attributes('type')).toBe('password')
    w.unmount()
  })

  it('confirm-password autocomplete is new-password (not current-password)', async () => {
    const w = mount(LoginView, { attachTo: document.body })
    const signupTab = w.findAll('button.tab').find((b) => /Create account/i.test(b.text()))
    await signupTab.trigger('click')
    const inputs = w.findAll('input.ri-input')
    expect(inputs[1].attributes('autocomplete')).toBe('new-password')
    w.unmount()
  })

  it('password field carries minlength=12 for the browser-level signup validation', async () => {
    const w = mount(LoginView, { attachTo: document.body })
    const signupTab = w.findAll('button.tab').find((b) => /Create account/i.test(b.text()))
    await signupTab.trigger('click')
    const inputs = w.findAll('input.ri-input')
    expect(inputs[0].attributes('minlength')).toBe('12')
    expect(inputs[1].attributes('minlength')).toBe('12')
    w.unmount()
  })

  it('password field has the right aria-label for screen readers', () => {
    const w = mount(LoginView, { attachTo: document.body })
    const pw = w.find('input.ri-input')
    expect(pw.attributes('aria-label')).toBe('Password')
    w.unmount()
  })
})

describe('LoginView.vue — detect-&-default returning user', () => {
  let auth
  beforeEach(() => {
    setActivePinia(createPinia())
    for (const k of Object.keys(memory)) delete memory[k]
    auth = useAuthStore()
    auth.loadProviders = vi.fn().mockResolvedValue(undefined)
    auth.restoreSession = vi.fn().mockResolvedValue(false)
  })

  function flush() {
    return new Promise((r) => setTimeout(r, 0))
  }

  it('renders state A (one-tap) when lastMethod=oauth and the provider exists', async () => {
    auth.providers = [{ name: 'google', displayName: 'Google' }]
    auth.lastUsedMethod = { kind: 'oauth', provider: 'google', email: null, at: Math.floor(Date.now()/1000) }
    const w = mount(LoginView, { attachTo: document.body })
    await flush()
    expect(w.find('[data-testid="one-tap"]').exists()).toBe(true)
    const btn = w.find('[data-testid="one-tap-provider"]')
    expect(btn.exists()).toBe(true)
    // Label uses the displayName, not the internal name.
    expect(btn.text()).toContain('Continue with Google')
    expect(btn.text()).not.toContain('google')
    // The tab row from state B should NOT render.
    expect(w.findAll('button.tab').length).toBe(0)
    w.unmount()
  })

  it('renders state B when lastMethod is null', async () => {
    auth.providers = [{ name: 'google', displayName: 'Google' }]
    auth.lastUsedMethod = null
    const w = mount(LoginView, { attachTo: document.body })
    await flush()
    expect(w.find('[data-testid="one-tap"]').exists()).toBe(false)
    expect(w.findAll('button.tab').length).toBeGreaterThan(0)
    w.unmount()
  })

  it('renders state B when the recorded provider is no longer offered', async () => {
    // Google was used before, but operator disabled it.
    auth.providers = [{ name: 'oidc', displayName: 'Acme SSO' }]
    auth.lastUsedMethod = { kind: 'oauth', provider: 'google', email: null, at: Math.floor(Date.now()/1000) }
    const w = mount(LoginView, { attachTo: document.body })
    await flush()
    expect(w.find('[data-testid="one-tap"]').exists()).toBe(false)
    expect(w.findAll('button.tab').length).toBeGreaterThan(0)
    w.unmount()
  })

  it('renders state A for local lastMethod and prefills the email', async () => {
    auth.providers = []
    auth.lastUsedMethod = { kind: 'local', provider: null, email: 'alice@example.com', at: Math.floor(Date.now()/1000) }
    const w = mount(LoginView, { attachTo: document.body })
    await flush()
    const emailInput = w.find('[data-testid="one-tap-email"]')
    expect(emailInput.exists()).toBe(true)
    expect(emailInput.element.value).toBe('alice@example.com')
    w.unmount()
  })

  it('clicking "Sign in with a different account" expands the full UI', async () => {
    auth.providers = [{ name: 'google', displayName: 'Google' }]
    auth.lastUsedMethod = { kind: 'oauth', provider: 'google', email: null, at: Math.floor(Date.now()/1000) }
    const w = mount(LoginView, { attachTo: document.body })
    await flush()
    expect(w.find('[data-testid="one-tap"]').exists()).toBe(true)
    await w.find('[data-testid="expand-different-account"]').trigger('click')
    expect(w.find('[data-testid="one-tap"]').exists()).toBe(false)
    expect(w.findAll('button.tab').length).toBeGreaterThan(0)
    w.unmount()
  })

  it('uses displayName for the OIDC label, not the literal "oidc"', async () => {
    auth.providers = [{ name: 'oidc', displayName: 'Acme Corp SSO' }]
    auth.lastUsedMethod = { kind: 'oauth', provider: 'oidc', email: null, at: Math.floor(Date.now()/1000) }
    const w = mount(LoginView, { attachTo: document.body })
    await flush()
    const btn = w.find('[data-testid="one-tap-provider"]')
    expect(btn.text()).toContain('Continue with Acme Corp SSO')
    w.unmount()
  })
})
