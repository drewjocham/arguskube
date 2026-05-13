import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { setActivePinia, createPinia } from 'pinia'
import PrivacyControls from '../../components/setup/PrivacyControls.vue'

function stubFetch(payload, { throws } = {}) {
  if (throws) {
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(throws))
    return
  }
  vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
    ok: true,
    json: vi.fn().mockResolvedValue({ result: payload || {} }),
  }))
}

describe('PrivacyControls', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    if (window.go) delete window.go
    vi.useFakeTimers()
  })
  afterEach(() => {
    vi.restoreAllMocks()
    vi.useRealTimers()
  })

  it('shows the arm button by default', () => {
    stubFetch({})
    const wrapper = mount(PrivacyControls)
    expect(wrapper.find('[data-testid="forget-activity-arm"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="forget-activity-confirm"]').exists()).toBe(false)
  })

  it('arm reveals the confirm button without firing the backend yet', async () => {
    stubFetch({})
    const wrapper = mount(PrivacyControls)
    await wrapper.find('[data-testid="forget-activity-arm"]').trigger('click')
    expect(wrapper.find('[data-testid="forget-activity-confirm"]').exists()).toBe(true)
    // No fetch should have fired — the action only runs on confirm.
    expect(window.fetch).not.toHaveBeenCalled()
  })

  it('auto-reverts to the arm state after the confirm window times out', async () => {
    stubFetch({})
    const wrapper = mount(PrivacyControls)
    await wrapper.find('[data-testid="forget-activity-arm"]').trigger('click')
    vi.advanceTimersByTime(5_100)
    await flushPromises()
    expect(wrapper.find('[data-testid="forget-activity-arm"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="forget-activity-confirm"]').exists()).toBe(false)
  })

  it('confirm fires ClearUserActivity and shows a success message', async () => {
    stubFetch({})
    const wrapper = mount(PrivacyControls)
    await wrapper.find('[data-testid="forget-activity-arm"]').trigger('click')
    await wrapper.find('[data-testid="forget-activity-confirm"]').trigger('click')
    await flushPromises()
    expect(window.fetch).toHaveBeenCalled()
    const url = window.fetch.mock.calls[0][0]
    expect(String(url)).toContain('/api/ClearUserActivity')
    const msg = wrapper.find('[data-testid="forget-activity-result"]')
    expect(msg.exists()).toBe(true)
    expect(msg.text()).toContain('Activity cleared')
  })

  it('shows an error message when the backend fails', async () => {
    stubFetch(null, { throws: new Error('backend down') })
    const wrapper = mount(PrivacyControls)
    await wrapper.find('[data-testid="forget-activity-arm"]').trigger('click')
    await wrapper.find('[data-testid="forget-activity-confirm"]').trigger('click')
    await flushPromises()
    expect(wrapper.find('[data-testid="forget-activity-result"]').text()).toContain('Could not clear')
  })
})
