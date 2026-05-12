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
