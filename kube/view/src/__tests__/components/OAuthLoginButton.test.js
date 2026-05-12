import { describe, it, expect, beforeEach, vi, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { setActivePinia, createPinia } from 'pinia'
import { nextTick } from 'vue'
import OAuthLoginButton from '../../components/auth/OAuthLoginButton.vue'
import { useAuthStore } from '../../stores/auth'

// We don't want LoginView's `auth.startOAuth` to actually hit the network,
// so we replace the store's startOAuth/pollOAuth methods with controllable
// fakes per test. The store is a real Pinia store backed by composables.
// (We aren't using `vi.mock` because the auth store itself is what we
// want to drive the button.)

function setProviders(auth, providers) {
  // The store exposes providers as a ref; assign through the proxy.
  auth.providers = providers
}

function bootBrowserOpenURL() {
  // Suppress accidental window.open noise in jsdom.
  window.runtime = { BrowserOpenURL: vi.fn() }
}

describe('OAuthLoginButton.vue', () => {
  let auth
  beforeEach(() => {
    setActivePinia(createPinia())
    auth = useAuthStore()
    bootBrowserOpenURL()
    vi.useFakeTimers()
  })
  afterEach(() => {
    vi.useRealTimers()
    delete window.runtime
  })

  it('disables the button and shows a hint when no providers are configured', () => {
    setProviders(auth, [])
    const w = mount(OAuthLoginButton)
    const btn = w.find('button.oauth-login-trigger')
    expect(btn.attributes('disabled')).toBeDefined()
    expect(btn.text()).toContain('OAuth login')
    expect(btn.attributes('title')).toMatch(/No OAuth providers/)
  })

  it('renders "Continue with <X>" when exactly one provider is configured', () => {
    setProviders(auth, [{ name: 'google', displayName: 'Google' }])
    const w = mount(OAuthLoginButton)
    expect(w.find('button.oauth-login-trigger').text()).toContain('Continue with Google')
  })

  it('renders "Continue with…" + chevron when multiple providers are configured', () => {
    setProviders(auth, [
      { name: 'google',  displayName: 'Google' },
      { name: 'github',  displayName: 'GitHub' },
    ])
    const w = mount(OAuthLoginButton)
    expect(w.find('button.oauth-login-trigger').text()).toContain('Continue with…')
    expect(w.find('.oauth-chevron').exists()).toBe(true)
  })

  it('opens a menu listing every provider when multiple exist + clicked', async () => {
    setProviders(auth, [
      { name: 'google',  displayName: 'Google' },
      { name: 'github',  displayName: 'GitHub' },
      { name: 'gitlab',  displayName: 'GitLab' },
    ])
    const w = mount(OAuthLoginButton)
    await w.find('button.oauth-login-trigger').trigger('click')
    const items = w.findAll('.oauth-menu-item')
    expect(items.length).toBe(3)
    expect(items[0].text()).toContain('Google')
    expect(items[1].text()).toContain('GitHub')
    expect(items[2].text()).toContain('GitLab')
  })

  it('skips the menu and starts the flow directly when only one provider exists', async () => {
    setProviders(auth, [{ name: 'google', displayName: 'Google' }])
    auth.startOAuth = vi.fn().mockResolvedValue({ authUrl: 'https://x', state: 's1' })
    auth.pollOAuth = vi.fn().mockResolvedValue({ done: true, user: { email: 'u@x' } })

    const w = mount(OAuthLoginButton)
    await w.find('button.oauth-login-trigger').trigger('click')
    await flushPromises()
    expect(auth.startOAuth).toHaveBeenCalledWith('google')
    // No menu element should ever be rendered.
    expect(w.find('.oauth-menu').exists()).toBe(false)
  })

  it('emits "login" with the user payload when pollOAuth resolves done=true', async () => {
    setProviders(auth, [{ name: 'github', displayName: 'GitHub' }])
    const user = { email: 'a@b' }
    auth.startOAuth = vi.fn().mockResolvedValue({ authUrl: 'https://x', state: 's1' })
    auth.pollOAuth = vi.fn().mockResolvedValue({ done: true, user })

    const w = mount(OAuthLoginButton)
    await w.find('button.oauth-login-trigger').trigger('click')
    // Advance through the 1.5s poll delay.
    await vi.advanceTimersByTimeAsync(1500)
    await flushPromises()

    expect(w.emitted('login')).toBeTruthy()
    expect(w.emitted('login')[0][0]).toEqual(user)
  })

  it('emits "error" when startOAuth throws', async () => {
    setProviders(auth, [{ name: 'github', displayName: 'GitHub' }])
    auth.startOAuth = vi.fn().mockRejectedValue(new Error('boom'))

    const w = mount(OAuthLoginButton)
    await w.find('button.oauth-login-trigger').trigger('click')
    await flushPromises()

    const errors = w.emitted('error') || []
    expect(errors.length).toBe(1)
    expect(errors[0][0]).toBe('boom')
  })

  it('emits "error" when pollOAuth rejects', async () => {
    setProviders(auth, [{ name: 'github', displayName: 'GitHub' }])
    auth.startOAuth = vi.fn().mockResolvedValue({ authUrl: 'https://x', state: 's1' })
    auth.pollOAuth = vi.fn().mockRejectedValue(new Error('callback failed'))

    const w = mount(OAuthLoginButton)
    await w.find('button.oauth-login-trigger').trigger('click')
    await vi.advanceTimersByTimeAsync(1500)
    await flushPromises()

    const errors = w.emitted('error') || []
    expect(errors.length).toBe(1)
    expect(errors[0][0]).toBe('callback failed')
  })

  it('emits "cancel" when the user clicks the inline Cancel link mid-flight', async () => {
    setProviders(auth, [{ name: 'github', displayName: 'GitHub' }])
    auth.startOAuth = vi.fn().mockResolvedValue({ authUrl: 'https://x', state: 's1' })
    auth.pollOAuth = vi.fn().mockResolvedValue({ done: false })

    const w = mount(OAuthLoginButton)
    await w.find('button.oauth-login-trigger').trigger('click')
    await flushPromises()
    // Cancel link is rendered after we enter in-flight.
    await w.find('.oauth-link').trigger('click')
    expect(w.emitted('cancel')).toBeTruthy()
    expect(w.emitted('cancel')[0][0]).toBe('github')
  })

  it('rejects malformed startOAuth responses (missing authUrl/state)', async () => {
    setProviders(auth, [{ name: 'github', displayName: 'GitHub' }])
    auth.startOAuth = vi.fn().mockResolvedValue({ authUrl: '', state: 's1' })
    const w = mount(OAuthLoginButton)
    await w.find('button.oauth-login-trigger').trigger('click')
    await flushPromises()
    expect(w.emitted('error')).toBeTruthy()
    expect(w.emitted('error')[0][0]).toMatch(/authUrl/)
  })

  it('opens the URL in the OS browser via window.runtime.BrowserOpenURL', async () => {
    setProviders(auth, [{ name: 'github', displayName: 'GitHub' }])
    auth.startOAuth = vi.fn().mockResolvedValue({ authUrl: 'https://gh', state: 's1' })
    auth.pollOAuth = vi.fn().mockResolvedValue({ done: true, user: {} })

    const w = mount(OAuthLoginButton)
    await w.find('button.oauth-login-trigger').trigger('click')
    await flushPromises()
    expect(window.runtime.BrowserOpenURL).toHaveBeenCalledWith('https://gh')
  })

  it('selects providers via ArrowDown / Enter keyboard navigation', async () => {
    setProviders(auth, [
      { name: 'google',  displayName: 'Google' },
      { name: 'github',  displayName: 'GitHub' },
    ])
    auth.startOAuth = vi.fn().mockResolvedValue({ authUrl: 'https://x', state: 's1' })
    auth.pollOAuth = vi.fn().mockResolvedValue({ done: true })

    const w = mount(OAuthLoginButton, { attachTo: document.body })
    await w.find('button.oauth-login-trigger').trigger('click')
    // Initial focus is index 0 (Google).
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowDown' }))
    await nextTick()
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter' }))
    await flushPromises()
    expect(auth.startOAuth).toHaveBeenCalledWith('github')
    w.unmount()
  })

  it('closes the menu on Escape', async () => {
    setProviders(auth, [
      { name: 'google',  displayName: 'Google' },
      { name: 'github',  displayName: 'GitHub' },
    ])
    const w = mount(OAuthLoginButton, { attachTo: document.body })
    await w.find('button.oauth-login-trigger').trigger('click')
    expect(w.find('.oauth-menu').exists()).toBe(true)
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }))
    await nextTick()
    expect(w.find('.oauth-menu').exists()).toBe(false)
    w.unmount()
  })

  it('renders a custom label when the label prop is provided', () => {
    setProviders(auth, [
      { name: 'google',  displayName: 'Google' },
      { name: 'github',  displayName: 'GitHub' },
    ])
    const w = mount(OAuthLoginButton, { props: { label: 'Sign in with SSO' } })
    expect(w.find('button.oauth-login-trigger').text()).toContain('Sign in with SSO')
  })

  it('exposes startFlow + cancel via defineExpose for parent control', () => {
    setProviders(auth, [{ name: 'github', displayName: 'GitHub' }])
    const w = mount(OAuthLoginButton)
    expect(typeof w.vm.startFlow).toBe('function')
    expect(typeof w.vm.cancel).toBe('function')
  })
})
