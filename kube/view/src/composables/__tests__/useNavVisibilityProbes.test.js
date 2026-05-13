import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { useNavVisibilityStore } from '../../stores/navVisibility'
import { useNavVisibilityProbes, __test } from '../useNavVisibilityProbes'

const memory = {}
Object.defineProperty(window, 'localStorage', {
  configurable: true,
  value: {
    getItem: (k) => (k in memory ? memory[k] : null),
    setItem: (k, v) => { memory[k] = String(v) },
    removeItem: (k) => { delete memory[k] },
    clear: () => { for (const k of Object.keys(memory)) delete memory[k] },
  },
})

function stubFetch(byPath) {
  vi.stubGlobal('fetch', vi.fn().mockImplementation((url) => {
    for (const key of Object.keys(byPath)) {
      if (String(url).includes(key)) {
        const payload = byPath[key]
        return Promise.resolve({
          ok: true,
          json: () => Promise.resolve({ result: payload }),
        })
      }
    }
    return Promise.reject(new Error(`unmocked: ${url}`))
  }))
}

describe('useNavVisibilityProbes — safeBool helper', () => {
  it('treats positive numbers as true', () => {
    expect(__test.safeBool(1)).toBe(true)
    expect(__test.safeBool(0)).toBe(false)
  })
  it('treats non-empty strings as true', () => {
    expect(__test.safeBool('s3://bucket')).toBe(true)
    expect(__test.safeBool('')).toBe(false)
    expect(__test.safeBool('   ')).toBe(false)
  })
  it('treats non-empty arrays as true', () => {
    expect(__test.safeBool([1])).toBe(true)
    expect(__test.safeBool([])).toBe(false)
  })
  it('falls through to truthy coercion', () => {
    expect(__test.safeBool(null)).toBe(false)
    expect(__test.safeBool(undefined)).toBe(false)
    expect(__test.safeBool({})).toBe(true)
  })
})

describe('useNavVisibilityProbes — wired probes', () => {
  beforeEach(() => {
    for (const k of Object.keys(memory)) delete memory[k]
    setActivePinia(createPinia())
    if (window.go) delete window.go
  })
  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('storage probe reveals when ListResources returns PVCs', async () => {
    stubFetch({
      ListResources: [{ name: 'data-0' }],
      GetSettings: { s3Bucket: '' },
    })
    const { run } = useNavVisibilityProbes()
    await run()
    const vis = useNavVisibilityStore()
    expect(vis.isVisible('storage')).toBe(true)
  })

  it('storage probe leaves storage hidden when PVC list is empty', async () => {
    stubFetch({
      ListResources: [],
      GetSettings: { s3Bucket: '' },
    })
    const { run } = useNavVisibilityProbes()
    await run()
    const vis = useNavVisibilityStore()
    expect(vis.isVisible('storage')).toBe(false)
  })

  it('knowledge probe reveals when S3 bucket is configured', async () => {
    stubFetch({
      ListResources: [],
      GetSettings: { s3Bucket: 'argus-notebooks' },
    })
    const { run } = useNavVisibilityProbes()
    await run()
    const vis = useNavVisibilityStore()
    expect(vis.isVisible('knowledge')).toBe(true)
  })

  it('does not auto-reveal config or admin (those are opt-in)', async () => {
    stubFetch({
      ListResources: [],
      GetSettings: { s3Bucket: '' },
    })
    const { run } = useNavVisibilityProbes()
    await run()
    const vis = useNavVisibilityStore()
    expect(vis.isVisible('config')).toBe(false)
    expect(vis.isVisible('admin')).toBe(false)
  })

  it('run() is idempotent — second call is a no-op', async () => {
    let storageCalls = 0
    vi.stubGlobal('fetch', vi.fn().mockImplementation((url) => {
      if (String(url).includes('ListResources')) storageCalls++
      return Promise.resolve({
        ok: true,
        json: () => Promise.resolve({ result: [] }),
      })
    }))
    const probes = useNavVisibilityProbes()
    await probes.run()
    await probes.run()
    expect(storageCalls).toBe(1)
  })

  it('handles probe failures silently — section stays hidden, store still initialized', async () => {
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new Error('boom')))
    const { run } = useNavVisibilityProbes()
    await run()
    const vis = useNavVisibilityStore()
    expect(vis.isVisible('storage')).toBe(false)
    expect(vis.isVisible('knowledge')).toBe(false)
    expect(vis.initialized).toBe(true)
  })

  it('accepts the snake_case shape from the HTTP API', async () => {
    stubFetch({
      ListResources: [],
      GetSettings: { s3_bucket: 'snake-bucket' },
    })
    const { run } = useNavVisibilityProbes()
    await run()
    expect(useNavVisibilityStore().isVisible('knowledge')).toBe(true)
  })
})
