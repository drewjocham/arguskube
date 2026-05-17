import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { useAppearanceStore } from './appearance'
import { useNavVisibilityStore } from './navVisibility'
import { useSectionTabsStore } from './sectionTabs'
import { useUIPrefsStore } from './uiPrefs'
import { useSavedFiltersStore } from './savedFilters'
import type { ProfileGroup, ProfileVariant, FlattenedVariant, ProfileSnapshot, SavedFilterEntry } from '../types/profiles'

const STORAGE_KEY = 'argus.profiles.v1'

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

  function persist(): void {
    saveToStorage({
      groups: groups.value,
      activeGroupId: activeGroupId.value,
      activeVariantId: activeVariantId.value,
    })
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
    }
    persist()
    return group
  }

  function updateGroup(id: string, patch: Partial<Pick<ProfileGroup, 'name' | 'description'>>): void {
    const g = groups.value.find(x => x.id === id)
    if (!g) return
    if (patch.name != null) g.name = patch.name.trim() || g.name
    if (patch.description != null) g.description = patch.description.trim()
    persist()
  }

  function deleteGroup(id: string): void {
    const idx = groups.value.findIndex(g => g.id === id)
    if (idx < 0) return
    groups.value.splice(idx, 1)
    if (activeGroupId.value === id) {
      activeGroupId.value = groups.value[0]?.id || ''
      if (!activeGroupId.value) activeVariantId.value = ''
    }
    persist()
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
    }
    persist()
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
    }
    persist()
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
  }

  function setActive(groupId: string, variantId: string): void {
    activeGroupId.value = groupId
    activeVariantId.value = variantId
    persist()
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
    return copy
  }

  return {
    groups,
    activeGroupId,
    activeVariantId,
    activeGroup,
    activeVariant,
    allVariants,
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
