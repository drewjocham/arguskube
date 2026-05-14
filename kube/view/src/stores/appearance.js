// Appearance store — themes + tactile knobs for tuning the visual feel
// of the entire app (brightness, contrast, opacity, density). Values are
// pushed onto :root as CSS custom properties so any component using the
// theme tokens picks them up automatically.
//
// Three concepts kept separate on purpose:
//
//   theme       — picks a base palette (dark / light / auto-from-OS)
//   sliders     — fine-tune the feel within that palette
//   density     — controls macro spacing without touching the palette

import { defineStore } from 'pinia'
import { computed, ref, watch } from 'vue'

const STORAGE_KEY = 'argus.appearance.v1'

const DEFAULTS = {
  // theme: dark | light | auto
  theme: 'dark',
  brightness: 100,   // 50 = darker, 150 = brighter
  contrast: 100,     // 50 = washed out, 150 = punchy
  opacity: 100,      // 60 = translucent window, 100 = solid
  blur: 0,           // 0–30, only meaningful when opacity < 100
  density: 'normal', // compact | normal | comfortable
  saturation: 100,   // 0 = grayscale, 200 = vivid — the "shiny / dull" knob
  fontSize: 13,      // base UI font size in px (CSS :root font-size driver)
}

const RANGES = {
  brightness: [50, 150],
  contrast:   [60, 140],
  opacity:    [60, 100],
  blur:       [0, 30],
  saturation: [0, 200],
  fontSize:   [11, 17],
}

const DENSITIES = {
  compact:     { fontScale: 0.92, padScale: 0.85, gap: 0.85 },
  normal:      { fontScale: 1.00, padScale: 1.00, gap: 1.00 },
  comfortable: { fontScale: 1.06, padScale: 1.18, gap: 1.18 },
}

function clamp(n, [lo, hi]) {
  const v = Number(n)
  if (!Number.isFinite(v)) return lo
  return Math.max(lo, Math.min(hi, v))
}

function loadFromStorage() {
  try {
    const raw = localStorage.getItem(STORAGE_KEY)
    if (!raw) return null
    return JSON.parse(raw)
  } catch {
    return null
  }
}

function saveToStorage(state) {
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(state))
  } catch {
    // best-effort
  }
}

// resolveTheme picks dark/light, honoring the OS preference when "auto".
function resolveTheme(mode) {
  if (mode === 'light' || mode === 'dark') return mode
  if (typeof window === 'undefined' || !window.matchMedia) return 'dark'
  return window.matchMedia('(prefers-color-scheme: light)').matches ? 'light' : 'dark'
}

export const useAppearanceStore = defineStore('appearance', () => {
  const persisted = loadFromStorage() || {}
  const theme = ref(persisted.theme || DEFAULTS.theme)
  const brightness = ref(clamp(persisted.brightness ?? DEFAULTS.brightness, RANGES.brightness))
  const contrast = ref(clamp(persisted.contrast ?? DEFAULTS.contrast, RANGES.contrast))
  const opacity = ref(clamp(persisted.opacity ?? DEFAULTS.opacity, RANGES.opacity))
  const blur = ref(clamp(persisted.blur ?? DEFAULTS.blur, RANGES.blur))
  const saturation = ref(clamp(persisted.saturation ?? DEFAULTS.saturation, RANGES.saturation))
  const density = ref(DENSITIES[persisted.density] ? persisted.density : DEFAULTS.density)
  const fontSize = ref(clamp(persisted.fontSize ?? DEFAULTS.fontSize, RANGES.fontSize))

  const resolvedTheme = computed(() => resolveTheme(theme.value))

  function applyToDocument() {
    if (typeof document === 'undefined') return
    const root = document.documentElement
    root.setAttribute('data-theme', resolvedTheme.value)
    root.setAttribute('data-density', density.value)

    const isLight = resolvedTheme.value === 'light'
    root.classList.toggle('theme-light', isLight)
    root.classList.toggle('theme-dark', !isLight)

    // Theme switching is currently palette-based: the
    // `:root[data-theme="light"]` block in theme.css swaps every
    // CSS custom property to a light tone, and components that read
    // `var(--bg)` etc. follow along. Components that still hardcode
    // dark hex literals will NOT switch — that requires a per-
    // component migration.
    //
    // Earlier iterations tried a global `filter: invert(1) hue-rotate(180deg)`
    // on body / html / #app to brute-force a light look without
    // touching any component. It didn't render reliably in the
    // Wails WKWebView build on macOS — the chip would activate but
    // no visual change followed. Removed to avoid the false promise.
    //
    // Brightness / contrast / saturation knobs still apply via the
    // existing #app rule (`#app { filter: var(--ui-filter, none) }`).

    // Clear any filter inline styles left over from previous builds
    // so they don't accumulate.
    root.style.filter = ''
    if (document.body) document.body.style.filter = ''
    const appEl = document.getElementById('app')
    if (appEl) appEl.style.filter = ''

    // Theme inversion is now CSS-driven via
    // `:root[data-theme="light"] body { filter: invert(...) }` —
    // toggling data-theme above is enough. The user-tunable knobs
    // (brightness/contrast/saturation) apply to #app (a child of
    // body) so they STACK with body's filter and don't override it.
    const filters = []
    if (brightness.value !== 100) filters.push(`brightness(${brightness.value}%)`)
    if (contrast.value !== 100) filters.push(`contrast(${contrast.value}%)`)
    if (saturation.value !== 100) filters.push(`saturate(${saturation.value}%)`)
    root.style.setProperty('--ui-filter', filters.length ? filters.join(' ') : 'none')
    root.style.setProperty('--ui-opacity', String(opacity.value / 100))
    root.style.setProperty('--ui-blur', `${blur.value}px`)

    // Density tuning: scale typography + spacing tokens. Components that
    // use --ui-pad-scale / --ui-gap-scale opt into responsive spacing;
    // legacy fixed-px components keep working unchanged.
    const d = DENSITIES[density.value] || DENSITIES.normal
    root.style.setProperty('--ui-font-scale', String(d.fontScale))
    root.style.setProperty('--ui-pad-scale', String(d.padScale))
    root.style.setProperty('--ui-gap-scale', String(d.gap))

    // Global UI base font size. Components that use rem units pick this
    // up automatically; the rest (px-based) stay anchored at the
    // designer's chosen size. Range 11-17px keeps the layout from
    // breaking at the extremes.
    root.style.setProperty('--ui-font-base', `${fontSize.value}px`)
  }

  function persist() {
    saveToStorage({
      theme: theme.value,
      brightness: brightness.value,
      contrast: contrast.value,
      opacity: opacity.value,
      blur: blur.value,
      saturation: saturation.value,
      density: density.value,
      fontSize: fontSize.value,
    })
  }

  // One watcher fires applyToDocument + persist on any change. Cheaper
  // and less fragile than wiring individual setters.
  watch(
    [theme, brightness, contrast, opacity, blur, saturation, density, fontSize],
    () => {
      applyToDocument()
      persist()
    },
    { flush: 'sync' },
  )

  // OS-level dark/light flip should propagate when theme === 'auto'.
  if (typeof window !== 'undefined' && window.matchMedia) {
    const mq = window.matchMedia('(prefers-color-scheme: light)')
    const handler = () => {
      if (theme.value === 'auto') applyToDocument()
    }
    if (mq.addEventListener) mq.addEventListener('change', handler)
    else if (mq.addListener) mq.addListener(handler) // older webview
  }

  // Each setter calls applyToDocument() and persist() directly, in
  // addition to mutating the ref so the watcher can also pick it up.
  // The direct call eliminates any dependency on Vue's reactivity
  // timing — clicking the Light button runs the DOM mutation
  // synchronously inside the same task as the click handler.
  function setTheme(t) {
    if (t !== 'dark' && t !== 'light' && t !== 'auto') return
    theme.value = t
    applyToDocument()
    persist()
  }
  function setBrightness(n) { brightness.value = clamp(n, RANGES.brightness); applyToDocument(); persist() }
  function setContrast(n)   { contrast.value   = clamp(n, RANGES.contrast);   applyToDocument(); persist() }
  function setOpacity(n)    { opacity.value    = clamp(n, RANGES.opacity);    applyToDocument(); persist() }
  function setBlur(n)       { blur.value       = clamp(n, RANGES.blur);       applyToDocument(); persist() }
  function setSaturation(n) { saturation.value = clamp(n, RANGES.saturation); applyToDocument(); persist() }
  function setDensity(d)    { if (!DENSITIES[d]) return; density.value = d;   applyToDocument(); persist() }
  function setFontSize(n)   { fontSize.value   = clamp(n, RANGES.fontSize);   applyToDocument(); persist() }

  function reset() {
    theme.value = DEFAULTS.theme
    brightness.value = DEFAULTS.brightness
    contrast.value = DEFAULTS.contrast
    opacity.value = DEFAULTS.opacity
    blur.value = DEFAULTS.blur
    saturation.value = DEFAULTS.saturation
    density.value = DEFAULTS.density
    fontSize.value = DEFAULTS.fontSize
  }

  // Always apply on store creation so a reload restores the look before
  // any component mounts.
  applyToDocument()

  return {
    theme,
    brightness,
    contrast,
    opacity,
    blur,
    saturation,
    density,
    fontSize,
    resolvedTheme,
    ranges: RANGES,
    densities: Object.keys(DENSITIES),
    setTheme,
    setBrightness,
    setContrast,
    setOpacity,
    setBlur,
    setSaturation,
    setDensity,
    setFontSize,
    reset,
    applyToDocument,
  }
})
