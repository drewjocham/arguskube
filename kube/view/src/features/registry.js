// Feature registry — the contract between the shell and feature modules.
//
// A feature lives in its own folder under src/features/<id>/ and exports a
// manifest from ./manifest.js. The registry collects manifests at boot via
// import.meta.glob, validates them, and exposes lookup helpers. The shell
// (App.vue, Sidebar.vue, CenterPanel.vue) consumes the registry — it
// never imports a feature's components directly.
//
// Migration path: a feature today is "ported" by moving its components
// into src/features/<id>/, writing a manifest.js, and deleting the
// hard-coded import + dispatch in the shell. Vite code-splits each
// feature's panel/tabs into its own chunk via the lazy `component`
// loaders, so startup only pays for the active feature.

import { defineAsyncComponent, markRaw, reactive } from 'vue'

/**
 * @typedef {() => Promise<{ default: import('vue').Component }>} LazyComponent
 *   A dynamic import expression: `() => import('./MyPanel.vue')`.
 *   Vite splits each into its own chunk.
 */

/**
 * @typedef {Object} FeatureTab
 * @property {string} id            Stable tab id (URL-safe, matches persisted prefs).
 * @property {string} label         Human-readable label shown in the SectionTabs bar.
 * @property {LazyComponent} component  Lazy loader for this tab's panel.
 * @property {boolean} [pro]        Pro-tier gated; rendered with a badge.
 */

/**
 * @typedef {Object} FeatureSection
 * @property {string} id            Section id (also keys the sidebar slot).
 * @property {string} label         Sidebar label.
 * @property {string} icon          SVG path data for the sidebar icon.
 * @property {FeatureTab[]} [tabs]  Tabs rendered inside this section. Optional —
 *                                  a section with no tabs renders its single
 *                                  panel directly.
 * @property {LazyComponent} [panel] When tabs is omitted, the section renders this.
 * @property {string} [defaultTab]  Tab to open on first visit. Defaults to tabs[0].
 */

/**
 * @typedef {Object} FeaturePanel
 *   A floating / overlay panel mounted globally by the shell (e.g. the
 *   terminal drawer). Not part of section routing.
 * @property {string} id            Panel id (used by `<FeaturePanel id="…">`).
 * @property {LazyComponent} component
 */

/**
 * @typedef {Object} FeatureTool
 *   A capability one feature exposes for others to call. Lets features
 *   collaborate without importing each other.
 * @property {string} id
 * @property {(...args: any[]) => any} invoke
 */

/**
 * @typedef {Object} FeatureManifest
 * @property {string} id                       Globally unique feature id.
 * @property {FeatureSection} [section]        Sidebar + center-panel routing.
 * @property {FeaturePanel[]} [panels]         Globally mountable panels.
 * @property {FeatureTool[]} [tools]           Inter-feature capabilities.
 * @property {string[]} [requires]             Other feature ids this depends on.
 */

// Reactive so Sidebar/CenterPanel re-render when (future) hot-loaded
// features register at runtime. For boot-time registration this is just
// a convenience — the registration happens before mount.
const _features = reactive(/** @type {Record<string, FeatureManifest>} */ ({}))
const _panels = reactive(/** @type {Record<string, FeaturePanel>} */ ({}))
const _tools = reactive(/** @type {Record<string, FeatureTool>} */ ({}))

/**
 * @param {FeatureManifest} manifest
 */
export function registerFeature(manifest) {
  if (!manifest || typeof manifest.id !== 'string') {
    throw new Error('[features] manifest missing id')
  }
  if (_features[manifest.id]) {
    throw new Error(`[features] duplicate feature id: ${manifest.id}`)
  }
  _features[manifest.id] = markRaw(manifest)
  for (const p of manifest.panels || []) {
    if (_panels[p.id]) throw new Error(`[features] duplicate panel id: ${p.id}`)
    _panels[p.id] = markRaw(p)
  }
  for (const t of manifest.tools || []) {
    if (_tools[t.id]) throw new Error(`[features] duplicate tool id: ${t.id}`)
    _tools[t.id] = markRaw(t)
  }
}

/** @returns {FeatureManifest[]} */
export function listFeatures() {
  return Object.values(_features)
}

/** @returns {FeatureSection[]} */
export function listSections() {
  return listFeatures()
    .map((f) => f.section)
    .filter(Boolean)
}

/**
 * @param {string} id
 * @returns {FeatureSection | undefined}
 */
export function getSection(id) {
  return listFeatures().find((f) => f.section?.id === id)?.section
}

/**
 * Resolve a lazy component into a real async component the shell can render.
 * Memoised — the same loader yields the same async wrapper, so Vue treats it
 * as one component across re-renders (preserves state).
 * @param {LazyComponent} loader
 */
const _asyncCache = new WeakMap()
export function resolveLazy(loader) {
  if (!loader) return null
  let resolved = _asyncCache.get(loader)
  if (!resolved) {
    resolved = markRaw(defineAsyncComponent(loader))
    _asyncCache.set(loader, resolved)
  }
  return resolved
}

/**
 * @param {string} panelId
 * @returns {import('vue').Component | null}
 */
export function resolvePanel(panelId) {
  const p = _panels[panelId]
  return p ? resolveLazy(p.component) : null
}

/**
 * @param {string} toolId
 * @returns {((...args: any[]) => any) | null}
 */
export function invokeTool(toolId, ...args) {
  const t = _tools[toolId]
  if (!t) throw new Error(`[features] unknown tool: ${toolId}`)
  return t.invoke(...args)
}
