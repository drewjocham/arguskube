import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { setActivePinia, createPinia } from 'pinia'
import ArgusSuggestionCard from '../../components/ai/ArgusSuggestionCard.vue'
import { useAppNavStore } from '../../stores/appNav'

// Default to a stub fetch that returns nothing — the card mounts,
// pollSuggestion runs, suggestion stays null, no card body renders.
function stubFetch(payload) {
  vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
    ok: true,
    json: vi.fn().mockResolvedValue({ result: payload || {} }),
  }))
}

function suggestion(overrides = {}) {
  return {
    suggestion: {
      kind: 'inline-tip',
      title: 'Open pods?',
      body: 'After alerts you usually open pods.',
      actionLabel: 'Open pods',
      actionId: 'userprofile.open-view:pods',
      muteKey: 'userprofile.next:alerts->pods',
      expiresInS: 60,
      ...overrides,
    },
  }
}

describe('ArgusSuggestionCard', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    if (window.go) delete window.go
    vi.useFakeTimers()
  })
  afterEach(() => {
    vi.restoreAllMocks()
    vi.useRealTimers()
  })

  it('renders nothing when the backend has no suggestion', async () => {
    stubFetch({})
    const wrapper = mount(ArgusSuggestionCard, { props: { activeView: 'alerts' } })
    await flushPromises()
    expect(wrapper.find('[data-testid="argus-suggestion-card"]').exists()).toBe(false)
  })

  it('renders the title + body + action button when a suggestion arrives', async () => {
    stubFetch(suggestion())
    const wrapper = mount(ArgusSuggestionCard, { props: { activeView: 'alerts' } })
    await flushPromises()
    const card = wrapper.find('[data-testid="argus-suggestion-card"]')
    expect(card.exists()).toBe(true)
    expect(card.text()).toContain('Open pods?')
    expect(card.text()).toContain('After alerts you usually open pods.')
    expect(wrapper.find('[data-testid="suggestion-accept"]').text()).toBe('Open pods')
  })

  it('Accept routes through appNav.requestNav and clears the card', async () => {
    stubFetch(suggestion())
    const wrapper = mount(ArgusSuggestionCard, { props: { activeView: 'alerts' } })
    await flushPromises()
    const nav = useAppNavStore()
    await wrapper.find('[data-testid="suggestion-accept"]').trigger('click')
    expect(nav.pending?.navId).toBe('pods')
    expect(wrapper.find('[data-testid="argus-suggestion-card"]').exists()).toBe(false)
  })

  it('Mute clears the card and fires the backend MuteSuggestion call', async () => {
    stubFetch(suggestion())
    const wrapper = mount(ArgusSuggestionCard, { props: { activeView: 'alerts' } })
    await flushPromises()
    const initialCalls = window.fetch.mock.calls.length
    await wrapper.find('[data-testid="suggestion-mute"]').trigger('click')
    expect(wrapper.find('[data-testid="argus-suggestion-card"]').exists()).toBe(false)
    // Mute fires the bridge call; we don't await it but the request hit fetch.
    await flushPromises()
    expect(window.fetch.mock.calls.length).toBeGreaterThan(initialCalls)
  })

  it('Close button dismisses without muting', async () => {
    stubFetch(suggestion())
    const wrapper = mount(ArgusSuggestionCard, { props: { activeView: 'alerts' } })
    await flushPromises()
    await wrapper.find('[data-testid="suggestion-close"]').trigger('click')
    expect(wrapper.find('[data-testid="argus-suggestion-card"]').exists()).toBe(false)
  })

  it('Card auto-expires after the configured window', async () => {
    stubFetch(suggestion({ expiresInS: 15 }))
    const wrapper = mount(ArgusSuggestionCard, { props: { activeView: 'alerts' } })
    await flushPromises()
    expect(wrapper.find('[data-testid="argus-suggestion-card"]').exists()).toBe(true)
    // 15s × 1000ms — bump past it.
    vi.advanceTimersByTime(16_000)
    await flushPromises()
    expect(wrapper.find('[data-testid="argus-suggestion-card"]').exists()).toBe(false)
  })

  it('Re-polls when activeView changes', async () => {
    // First poll → no suggestion. Second poll → suggestion.
    let call = 0
    vi.stubGlobal('fetch', vi.fn().mockImplementation(() => {
      call++
      const payload = call >= 2 ? { result: suggestion() } : { result: {} }
      return Promise.resolve({ ok: true, json: vi.fn().mockResolvedValue(payload) })
    }))
    const wrapper = mount(ArgusSuggestionCard, { props: { activeView: 'alerts' } })
    await flushPromises()
    expect(wrapper.find('[data-testid="argus-suggestion-card"]').exists()).toBe(false)
    await wrapper.setProps({ activeView: 'pods' })
    await flushPromises()
    expect(wrapper.find('[data-testid="argus-suggestion-card"]').exists()).toBe(true)
  })
})
