import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { useAppearanceStore } from './appearance'
import { useNavVisibilityStore } from './navVisibility'
import { useSectionTabsStore } from './sectionTabs'
import { useUIPrefsStore } from './uiPrefs'
import { useSavedFiltersStore } from './savedFilters'
import type { ProfileGroup, ProfileVariant, FlattenedVariant, ProfileSnapshot, SavedFilterEntry } from '../types/profiles'
import {
  loadFromBackend,
  loadActiveFromBackend,
  pushGroup,
  pushVariant,
  pushDeleteGroup,
  pushDeleteVariant,
  pushActive,
  type BackendGroup,
  type BackendVariant,
} from './profilesSync'

const STORAGE_KEY = 'argus.profiles.v1'

// getSessionTokenSync mirrors the workspace store's reader — Wails
// mode ignores the arg, SaaS mode passes "" and the bridge layers
// the Authorization header in from localStorage. Kept inline here so
// the store stays self-contained and the test fakes don't have to
// stand up the auth store.
function getSessionTokenSync(): string {
  try {
    if (typeof localStorage === 'undefined') return ''
    const raw = localStorage.getItem('argus.auth.session')
    if (!raw) return ''
    const parsed = JSON.parse(raw)
    return parsed?.token || ''
  } catch {
    return ''
  }
}

// mapBackendGroup turns a wire-format BackendGroup into the local
// ProfileGroup shape. Wire shape carries timestamps + an unknown-
// typed snapshot; the local shape is strictly typed.
function mapBackendGroup(bg: BackendGroup): ProfileGroup {
  return {
    id: bg.id,
    name: bg.name,
    description: bg.description,
    variants: (bg.variants || []).map(mapBackendVariant),
  }
}

function mapBackendVariant(bv: BackendVariant): ProfileVariant {
  return {
    id: bv.id,
    parentId: bv.parentId,
    name: bv.name,
    description: bv.description,
    version: bv.version,
    snapshot: normalizeSnapshot(bv.snapshot),
  }
}

// normalizeSnapshot guards against missing fields so the apply path
// doesn't have to special-case undefineds (the original
// localStorage-only version trusted the persisted shape).
function normalizeSnapshot(raw: unknown): ProfileSnapshot {
  const s = (raw && typeof raw === 'object' ? raw : {}) as Partial<ProfileSnapshot>
  return {
    appearance: s.appearance && typeof s.appearance === 'object' ? s.appearance : {},
    navVisibility: { visible: s.navVisibility?.visible || {} },
    sectionTabs: { tabs: s.sectionTabs?.tabs || {} },
    uiPrefs: { rightPanelWidth: s.uiPrefs?.rightPanelWidth ?? 340 },
    savedFilters: Array.isArray(s.savedFilters) ? s.savedFilters : [],
  }
}

function newId(): string {
  if (typeof crypto !== 'undefined' && crypto.randomUUID) return crypto.randomUUID()
  return `p-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`
}

interface PersistedState {
  groups: ProfileGroup[]
  activeGroupId: string
  activeVariantId: string
}

function loadFromStorage(): PersistedState | null {
  try {
    const raw = localStorage.getItem(STORAGE_KEY)
    if (!raw) return null
    return JSON.parse(raw) as PersistedState
  } catch {
    return null
  }
}

function saveToStorage(state: PersistedState): void {
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(state))
  } catch {
    // best-effort
  }
}

function captureAppearance(): Record<string, unknown> {
  const store = useAppearanceStore()
  return {
    theme: store.theme,
    brightness: store.brightness,
    contrast: store.contrast,
    opacity: store.opacity,
    blur: store.blur,
    density: store.density,
    saturation: store.saturation,
    fontSize: store.fontSize,
  }
}

function captureNavVisibility(): { visible: Record<string, boolean> } {
  const store = useNavVisibilityStore()
  return { visible: { ...store.visible } }
}

function captureSectionTabs(): { tabs: Record<string, string> } {
  const store = useSectionTabsStore()
  return { tabs: { ...store.tabs } }
}

function captureUIPrefs(): { rightPanelWidth: number } {
  const store = useUIPrefsStore()
  return { rightPanelWidth: store.rightPanelWidth }
}

function captureSavedFilters(): SavedFilterEntry[] {
  const store = useSavedFiltersStore()
  return JSON.parse(JSON.stringify(store.entries))
}

function applyAppearance(data: Record<string, unknown>): void {
  const store = useAppearanceStore()
  if (typeof data.theme === 'string') store.setTheme(data.theme)
  if (typeof data.brightness === 'number') store.setBrightness(data.brightness)
  if (typeof data.contrast === 'number') store.setContrast(data.contrast)
  if (typeof data.opacity === 'number') store.setOpacity(data.opacity)
  if (typeof data.blur === 'number') store.setBlur(data.blur)
  if (typeof data.density === 'string') store.setDensity(data.density)
  if (typeof data.saturation === 'number') store.setSaturation(data.saturation)
  if (typeof data.fontSize === 'number') store.setFontSize(data.fontSize)
}

function applyNavVisibility(data: { visible: Record<string, boolean> }): void {
  const store = useNavVisibilityStore()
  for (const [id, visible] of Object.entries(data.visible || {})) {
    if (visible) store.show(id)
    else store.hide(id)
  }
}

function applySectionTabs(data: { tabs: Record<string, string> }): void {
  const store = useSectionTabsStore()
  for (const [sectionId, tabId] of Object.entries(data.tabs || {})) {
    store.setTab(sectionId, tabId)
  }
}

function applyUIPrefs(data: { rightPanelWidth: number }): void {
  const store = useUIPrefsStore()
  if (typeof data.rightPanelWidth === 'number') {
    store.setRightPanelWidth(data.rightPanelWidth)
  }
}

function applySavedFilters(entries: SavedFilterEntry[]): void {
  const store = useSavedFiltersStore()
  store.clear()
  for (const entry of entries) {
    store.save(entry.name, {
      query: entry.query,
      filters: entry.filters,
      limit: entry.limit,
    })
  }
}

export const useProfilesStore = defineStore('profiles', () => {
  const persisted = loadFromStorage()
  const groups = ref<ProfileGroup[]>(persisted?.groups || [])
  const activeGroupId = ref(persisted?.activeGroupId || '')
  const activeVariantId = ref(persisted?.activeVariantId || '')
  const hydrated = ref(false)
  const backendAvailable = ref(false)

  function persist(): void {
    saveToStorage({
      groups: groups.value,
      activeGroupId: activeGroupId.value,
      activeVariantId: activeVariantId.value,
    })
  }

  // hydrate replaces the in-memory state from the backend if it's
  // reachable. Idempotent — call once on app boot. Failures leave
  // the localStorage-loaded state untouched (offline / web mode
  // without auth keep working).
  //
  // Returns true when the backend served the data, false when the
  // localStorage fallback is what's in memory. Tests assert on this.
  async function hydrate(): Promise<boolean> {
    if (hydrated.value) return backendAvailable.value
    hydrated.value = true

    const remote = await loadFromBackend(getSessionTokenSync)
    if (remote === null) {
      // Backend not reachable — keep whatever loadFromStorage gave us.
      return false
    }
    backendAvailable.value = true
    groups.value = remote.map(mapBackendGroup)

    const active = await loadActiveFromBackend(getSessionTokenSync)
    if (active) {
      activeGroupId.value = active.groupId
      activeVariantId.value = active.variantId
    }
    persist()
    return true
  }

  const activeGroup = computed<ProfileGroup | null>(() =>
    groups.value.find(g => g.id === activeGroupId.value) || null
  )

  const activeVariant = computed<ProfileVariant | null>(() => {
    const g = activeGroup.value
    if (!g) return null
    return g.variants.find(v => v.id === activeVariantId.value) || null
  })

  const allVariants = computed<FlattenedVariant[]>(() => {
    const out: FlattenedVariant[] = []
    for (const g of groups.value) {
      for (const v of g.variants) {
        out.push({ ...v, groupName: g.name, groupId: g.id })
      }
    }
    return out
  })

  // Per-target debounce timers. Rapid mutations (drag a slider while
  // a variant is open; rename a group letter-by-letter) coalesce to
  // one backend write 300ms after the last edit. Each timer is keyed
  // by the row it targets so two parallel edits don't trample each
  // other.
  //
  // Production motivation: at 50k users, frontend bursts that fire
  // 60 writes/sec per user would multiply into noticeable DB write
  // contention even with WAL. The debounce keeps the write rate at
  // most ~3/sec per user per row, which the existing SQLite handle
  // (single-writer) absorbs comfortably.
  const SYNC_DEBOUNCE_MS = 300
  const groupSyncTimers: Record<string, ReturnType<typeof setTimeout>> = {}
  const variantSyncTimers: Record<string, ReturnType<typeof setTimeout>> = {}

  function debounceSyncGroup(g: ProfileGroup): void {
    if (!backendAvailable.value) return
    const existing = groupSyncTimers[g.id]
    if (existing) clearTimeout(existing)
    groupSyncTimers[g.id] = setTimeout(() => {
      delete groupSyncTimers[g.id]
      void pushGroup(getSessionTokenSync, g)
    }, SYNC_DEBOUNCE_MS)
  }

  function debounceSyncVariant(groupId: string, v: ProfileVariant): void {
    if (!backendAvailable.value) return
    const key = `${groupId}:${v.id}`
    const existing = variantSyncTimers[key]
    if (existing) clearTimeout(existing)
    variantSyncTimers[key] = setTimeout(() => {
      delete variantSyncTimers[key]
      void pushVariant(getSessionTokenSync, groupId, v)
    }, SYNC_DEBOUNCE_MS)
  }

  // syncGroup / syncVariant are the existing names callers use. We
  // keep them as the public hooks and have them dispatch to the
  // debounced versions — callers stay simple, behaviour stays fast.
  function syncGroup(g: ProfileGroup): void {
    debounceSyncGroup(g)
  }

  function syncVariant(groupId: string, v: ProfileVariant): void {
    debounceSyncVariant(groupId, v)
  }

  function syncDeleteGroup(id: string): void {
    if (!backendAvailable.value) return
    // Cancel any pending upsert for this group so we don't write a
    // gone row right after deleting it. Same for every variant
    // pending under it — variant timers are keyed "${groupId}:".
    const pending = groupSyncTimers[id]
    if (pending) {
      clearTimeout(pending)
      delete groupSyncTimers[id]
    }
    for (const key of Object.keys(variantSyncTimers)) {
      if (key.startsWith(`${id}:`)) {
        clearTimeout(variantSyncTimers[key])
        delete variantSyncTimers[key]
      }
    }
    void pushDeleteGroup(getSessionTokenSync, id)
  }

  function syncDeleteVariant(groupId: string, variantId: string): void {
    if (!backendAvailable.value) return
    const key = `${groupId}:${variantId}`
    const pending = variantSyncTimers[key]
    if (pending) {
      clearTimeout(pending)
      delete variantSyncTimers[key]
    }
    void pushDeleteVariant(getSessionTokenSync, groupId, variantId)
  }

  function syncActive(groupId: string, variantId: string): void {
    if (!backendAvailable.value) return
    void pushActive(getSessionTokenSync, groupId, variantId)
  }

  function createGroup(name: string, description = ''): ProfileGroup {
    const group: ProfileGroup = {
      id: newId(),
      name: name.trim() || 'New Profile',
      description: description.trim(),
      variants: [],
    }
    groups.value.push(group)
    if (groups.value.length === 1) {
      activeGroupId.value = group.id
      syncActive(group.id, '')
    }
    persist()
    syncGroup(group)
    return group
  }

  function updateGroup(id: string, patch: Partial<Pick<ProfileGroup, 'name' | 'description'>>): void {
    const g = groups.value.find(x => x.id === id)
    if (!g) return
    if (patch.name != null) g.name = patch.name.trim() || g.name
    if (patch.description != null) g.description = patch.description.trim()
    persist()
    syncGroup(g)
  }

  function deleteGroup(id: string): void {
    const idx = groups.value.findIndex(g => g.id === id)
    if (idx < 0) return
    groups.value.splice(idx, 1)
    if (activeGroupId.value === id) {
      activeGroupId.value = groups.value[0]?.id || ''
      if (!activeGroupId.value) activeVariantId.value = ''
      syncActive(activeGroupId.value, activeVariantId.value)
    }
    persist()
    syncDeleteGroup(id)
  }

  function createVariant(groupId: string, name: string, description = '', version = '1.0'): ProfileVariant | null {
    const g = groups.value.find(x => x.id === groupId)
    if (!g) return null
    const variant: ProfileVariant = {
      id: newId(),
      parentId: groupId,
      name: name.trim() || 'New Variant',
      description: description.trim(),
      version: version.trim() || '1.0',
      snapshot: {
        appearance: {},
        navVisibility: { visible: {} },
        sectionTabs: { tabs: {} },
        uiPrefs: { rightPanelWidth: 340 },
        savedFilters: [],
      },
    }
    g.variants.push(variant)
    if (!activeVariantId.value) {
      activeVariantId.value = variant.id
      activeGroupId.value = groupId
      syncActive(groupId, variant.id)
    }
    persist()
    syncVariant(groupId, variant)
    return variant
  }

  function updateVariant(groupId: string, variantId: string, patch: Partial<Pick<ProfileVariant, 'name' | 'description' | 'version'>>): void {
    const g = groups.value.find(x => x.id === groupId)
    if (!g) return
    const v = g.variants.find(x => x.id === variantId)
    if (!v) return
    if (patch.name != null) v.name = patch.name.trim() || v.name
    if (patch.description != null) v.description = patch.description.trim()
    if (patch.version != null) v.version = patch.version.trim()
    persist()
    syncVariant(groupId, v)
  }

  function deleteVariant(groupId: string, variantId: string): void {
    const g = groups.value.find(x => x.id === groupId)
    if (!g) return
    const idx = g.variants.findIndex(v => v.id === variantId)
    if (idx < 0) return
    g.variants.splice(idx, 1)
    if (activeVariantId.value === variantId) {
      activeVariantId.value = g.variants[0]?.id || ''
      if (!activeVariantId.value) {
        const other = groups.value.find(x => x.id !== groupId)
        if (other?.variants[0]) {
          activeGroupId.value = other.id
          activeVariantId.value = other.variants[0].id
        } else {
          activeGroupId.value = groups.value[0]?.id || ''
        }
      }
      syncActive(activeGroupId.value, activeVariantId.value)
    }
    persist()
    syncDeleteVariant(groupId, variantId)
  }

  function captureToVariant(groupId: string, variantId: string): void {
    const g = groups.value.find(x => x.id === groupId)
    if (!g) return
    const v = g.variants.find(x => x.id === variantId)
    if (!v) return
    v.snapshot = {
      appearance: captureAppearance(),
      navVisibility: captureNavVisibility(),
      sectionTabs: captureSectionTabs(),
      uiPrefs: captureUIPrefs(),
      savedFilters: captureSavedFilters(),
    }
    persist()
    syncVariant(groupId, v)
  }

  function applyVariant(variantId: string): void {
    const target = allVariants.value.find(v => v.id === variantId)
    if (!target) return
    const s = target.snapshot
    if (!s) return

    if (s.appearance && Object.keys(s.appearance).length > 0) {
      applyAppearance(s.appearance)
    }
    if (s.navVisibility) {
      applyNavVisibility(s.navVisibility)
    }
    if (s.sectionTabs) {
      applySectionTabs(s.sectionTabs)
    }
    if (s.uiPrefs) {
      applyUIPrefs(s.uiPrefs)
    }
    if (s.savedFilters && s.savedFilters.length > 0) {
      applySavedFilters(s.savedFilters)
    }

    activeVariantId.value = target.id
    activeGroupId.value = target.groupId || target.parentId
    persist()
    syncActive(activeGroupId.value, activeVariantId.value)
  }

  function setActive(groupId: string, variantId: string): void {
    activeGroupId.value = groupId
    activeVariantId.value = variantId
    persist()
    syncActive(groupId, variantId)
  }

  function duplicateVariant(groupId: string, variantId: string): ProfileVariant | null {
    const g = groups.value.find(x => x.id === groupId)
    if (!g) return null
    const v = g.variants.find(x => x.id === variantId)
    if (!v) return null
    const copy: ProfileVariant = {
      ...JSON.parse(JSON.stringify(v)),
      id: newId(),
      name: v.name + ' (copy)',
    }
    g.variants.push(copy)
    persist()
    syncVariant(groupId, copy)
    return copy
  }

  return {
    groups,
    activeGroupId,
    activeVariantId,
    hydrated,
    backendAvailable,
    activeGroup,
    activeVariant,
    allVariants,
    hydrate,
    createGroup,
    updateGroup,
    deleteGroup,
    createVariant,
    updateVariant,
    deleteVariant,
    captureToVariant,
    applyVariant,
    setActive,
    duplicateVariant,
  }
})
