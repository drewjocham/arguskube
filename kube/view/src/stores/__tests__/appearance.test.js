import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { useAppearanceStore } from '../appearance'

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

let matchMediaHandlers = []
const matchMediaMock = (query) => ({
  matches: false,
  media: query,
  addEventListener: (_, h) => matchMediaHandlers.push(h),
  addListener: (h) => matchMediaHandlers.push(h),
  removeEventListener: () => {},
  removeListener: () => {},
})

describe('appearance store', () => {
  beforeEach(() => {
    for (const k of Object.keys(memory)) delete memory[k]
    matchMediaHandlers = []
    vi.stubGlobal('matchMedia', matchMediaMock)
    setActivePinia(createPinia())
  })

  afterEach(() => {
    vi.unstubAllGlobals()
  })

  function fresh() {
    setActivePinia(createPinia())
    return useAppearanceStore()
  }

  it('starts with default values when localStorage is empty', () => {
    const s = fresh()
    expect(s.theme).toBe('dark')
    expect(s.brightness).toBe(100)
    expect(s.contrast).toBe(100)
    expect(s.opacity).toBe(100)
    expect(s.blur).toBe(0)
    expect(s.saturation).toBe(100)
    expect(s.density).toBe('normal')
    expect(s.resolvedTheme).toBe('dark')
  })

  it('persists and restores values from localStorage', () => {
    const a = fresh()
    a.setBrightness(120)
    a.setDensity('compact')
    const b = fresh()
    expect(b.brightness).toBe(120)
    expect(b.density).toBe('compact')
  })

  it('setTheme rejects invalid values', () => {
    const s = fresh()
    s.setTheme('invalid')
    expect(s.theme).toBe('dark')
  })

  it('setBrightness clamps to [50, 150]', () => {
    const s = fresh()
    s.setBrightness(10)
    expect(s.brightness).toBe(50)
    s.setBrightness(200)
    expect(s.brightness).toBe(150)
  })

  it('setContrast clamps to [60, 140]', () => {
    const s = fresh()
    s.setContrast(30)
    expect(s.contrast).toBe(60)
    s.setContrast(200)
    expect(s.contrast).toBe(140)
  })

  it('setOpacity clamps to [60, 100]', () => {
    const s = fresh()
    s.setOpacity(20)
    expect(s.opacity).toBe(60)
    s.setOpacity(120)
    expect(s.opacity).toBe(100)
  })

  it('setBlur clamps to [0, 30]', () => {
    const s = fresh()
    s.setBlur(-5)
    expect(s.blur).toBe(0)
    s.setBlur(50)
    expect(s.blur).toBe(30)
  })

  it('setSaturation clamps to [0, 200]', () => {
    const s = fresh()
    s.setSaturation(-10)
    expect(s.saturation).toBe(0)
    s.setSaturation(300)
    expect(s.saturation).toBe(200)
  })

  it('setDensity rejects invalid density', () => {
    const s = fresh()
    s.setDensity('ultra')
    expect(s.density).toBe('normal')
  })

  it('setDensity accepts compact', () => {
    const s = fresh()
    s.setDensity('compact')
    expect(s.density).toBe('compact')
  })

  it('setDensity accepts comfortable', () => {
    const s = fresh()
    s.setDensity('comfortable')
    expect(s.density).toBe('comfortable')
  })

  it('resolvedTheme returns dark for auto when OS prefers dark', () => {
    vi.stubGlobal('matchMedia', () => ({ matches: false }))
    const s = fresh()
    s.setTheme('auto')
    expect(s.resolvedTheme).toBe('dark')
  })

  it('resolvedTheme returns light for auto when OS prefers light', () => {
    vi.stubGlobal('matchMedia', () => ({ matches: true }))
    const s = fresh()
    s.setTheme('auto')
    expect(s.resolvedTheme).toBe('light')
  })

  it('reset restores all defaults', () => {
    const s = fresh()
    s.setBrightness(130)
    s.setContrast(80)
    s.setOpacity(90)
    s.setBlur(10)
    s.setSaturation(150)
    s.setDensity('compact')
    s.reset()
    expect(s.brightness).toBe(100)
    expect(s.contrast).toBe(100)
    expect(s.opacity).toBe(100)
    expect(s.blur).toBe(0)
    expect(s.saturation).toBe(100)
    expect(s.density).toBe('normal')
    expect(s.theme).toBe('dark')
  })

  it('applyToDocument sets data-theme and data-density on documentElement', () => {
    const s = fresh()
    s.applyToDocument()
    const root = document.documentElement
    expect(root.getAttribute('data-theme')).toBe('dark')
    expect(root.getAttribute('data-density')).toBe('normal')
  })

  it('applyToDocument sets CSS custom properties', () => {
    const s = fresh()
    s.applyToDocument()
    const root = document.documentElement
    expect(root.style.getPropertyValue('--ui-opacity')).toBe('1')
    expect(root.style.getPropertyValue('--ui-blur')).toBe('0px')
    expect(root.style.getPropertyValue('--ui-font-scale')).toBe('1')
    expect(root.style.getPropertyValue('--ui-pad-scale')).toBe('1')
    expect(root.style.getPropertyValue('--ui-gap-scale')).toBe('1')
  })

  it('applyToDocument applies filter when brightness !== 100', () => {
    const s = fresh()
    s.setBrightness(120)
    const root = document.documentElement
    const filter = root.style.getPropertyValue('--ui-filter')
    expect(filter).toContain('brightness')
  })

  // --- §C5 font-size zoom ---

  it('defaults fontSize to 13px and applies --ui-font-base', () => {
    const s = fresh()
    expect(s.fontSize).toBe(13)
    expect(document.documentElement.style.getPropertyValue('--ui-font-base')).toBe('13px')
  })

  it('setFontSize clamps to the configured range', () => {
    const s = fresh()
    s.setFontSize(50)
    expect(s.fontSize).toBe(17) // clamped to max
    s.setFontSize(-5)
    expect(s.fontSize).toBe(11) // clamped to min
    s.setFontSize(14)
    expect(s.fontSize).toBe(14)
  })

  it('setFontSize updates --ui-font-base on the root', () => {
    const s = fresh()
    s.setFontSize(16)
    expect(document.documentElement.style.getPropertyValue('--ui-font-base')).toBe('16px')
  })

  it('setFontSize persists across reload', () => {
    const s = fresh()
    s.setFontSize(15)
    const s2 = fresh()
    expect(s2.fontSize).toBe(15)
  })

  it('reset() restores fontSize to the default', () => {
    const s = fresh()
    s.setFontSize(16)
    s.reset()
    expect(s.fontSize).toBe(13)
  })

  it('exposes the fontSize range in ranges', () => {
    const s = fresh()
    expect(s.ranges.fontSize).toEqual([11, 17])
  })
})
