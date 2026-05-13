import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { mount, flushPromises } from '@vue/test-utils'
import { defineComponent, h } from 'vue'

import { useAutoContextProbe } from '../useAutoContextProbe'
import { __test as autoCtxTest } from '../useAutoContext'
import { useSetupChecklistStore } from '../../stores/setupChecklist'

// Stubs the same global fetch path useAutoContext uses, since the probe
// composable is just a watcher on top of that composable's reactive state.
function stubFetch(payload, { throws } = {}) {
  if (throws) {
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(throws))
    return
  }
  vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
    ok: true,
    json: vi.fn().mockResolvedValue({ result: payload }),
  }))
}

// We need a real component host so the watch() inside the composable runs.
// A throwaway component that calls the composable inside setup() is enough.
function mountProbeHost() {
  return mount(defineComponent({
    setup() { useAutoContextProbe() },
    render() { return h('div') },
  }))
}

describe('useAutoContextProbe', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    autoCtxTest.reset()
    vi.clearAllMocks()
    if (window.go) delete window.go
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('inserts an ok row when active context is reachable', async () => {
    stubFetch({
      chosen: 'prod',
      confidence: 'active-reachable',
      probes: [{ name: 'prod', reachable: true, active: true }],
      reachableCount: 1,
    })
    mountProbeHost()
    // Trigger the auto-resolve flow.
    const { useAutoContext } = await import('../useAutoContext')
    await useAutoContext().resolve()
    await flushPromises()

    const store = useSetupChecklistStore()
    const row = store.items.find(i => i.id === 'kubeconfig.context')
    expect(row).toBeTruthy()
    expect(row.status).toBe('ok')
    expect(row.title).toContain('Connected to prod')
  })

  it('inserts a warn row with a Re-check action on fallback-reachable', async () => {
    stubFetch({
      chosen: 'stage',
      confidence: 'fallback-reachable',
      probes: [
        { name: 'prod', reachable: false, active: true, error: 'i/o timeout' },
        { name: 'stage', reachable: true, active: false },
      ],
      reachableCount: 1,
    })
    mountProbeHost()
    const { useAutoContext } = await import('../useAutoContext')
    await useAutoContext().resolve()
    await flushPromises()

    const row = useSetupChecklistStore().items.find(i => i.id === 'kubeconfig.context')
    expect(row.status).toBe('warn')
    expect(row.title).toContain('Active context unreachable')
    expect(row.title).toContain('stage')
    expect(row.action?.label).toBe('Re-check')
  })

  it('inserts a todo row + per-context error detail on active-unreachable', async () => {
    stubFetch({
      chosen: 'prod',
      confidence: 'active-unreachable',
      probes: [{ name: 'prod', reachable: false, active: true, error: 'dial tcp: i/o timeout' }],
      reachableCount: 0,
    })
    mountProbeHost()
    const { useAutoContext } = await import('../useAutoContext')
    await useAutoContext().resolve()
    await flushPromises()

    const row = useSetupChecklistStore().items.find(i => i.id === 'kubeconfig.context')
    expect(row.status).toBe('todo')
    expect(row.detail).toContain('dial tcp')
    expect(row.action?.label).toBe('Re-check')
  })

  it('inserts an error row when kubeconfig has zero contexts', async () => {
    stubFetch({ chosen: '', confidence: 'none', probes: [], reachableCount: 0 })
    mountProbeHost()
    const { useAutoContext } = await import('../useAutoContext')
    await useAutoContext().resolve()
    await flushPromises()

    const row = useSetupChecklistStore().items.find(i => i.id === 'kubeconfig.context')
    expect(row.status).toBe('error')
    expect(row.title).toContain('No kubeconfig contexts')
  })

  it('inserts an error row when the backend call throws', async () => {
    stubFetch(null, { throws: new Error('boom') })
    mountProbeHost()
    const { useAutoContext } = await import('../useAutoContext')
    await useAutoContext().resolve()
    await flushPromises()

    const row = useSetupChecklistStore().items.find(i => i.id === 'kubeconfig.context')
    expect(row.status).toBe('error')
    expect(row.detail).toContain('boom')
  })

  it('updates the same row id across multiple probes (idempotent in place)', async () => {
    // First: unreachable.
    stubFetch({
      chosen: 'prod', confidence: 'active-unreachable',
      probes: [{ name: 'prod', reachable: false, active: true, error: 'timeout' }],
      reachableCount: 0,
    })
    mountProbeHost()
    const { useAutoContext } = await import('../useAutoContext')
    await useAutoContext().resolve()
    await flushPromises()

    // Then: reachable on reprobe.
    stubFetch({
      chosen: 'prod', confidence: 'active-reachable',
      probes: [{ name: 'prod', reachable: true, active: true }],
      reachableCount: 1,
    })
    await useAutoContext().reprobe()
    await flushPromises()

    const items = useSetupChecklistStore().items
    const ctxRows = items.filter(i => i.id === 'kubeconfig.context')
    expect(ctxRows).toHaveLength(1)
    expect(ctxRows[0].status).toBe('ok')
  })
})
