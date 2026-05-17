export interface SavedFilterEntry {
  id: string
  name: string
  query: string
  filters: Array<{ field: string; value: string }>
  limit: number | null
  createdAt: string
  updatedAt: string
}

export interface ProfileSnapshot {
  appearance: Record<string, unknown>
  navVisibility: { visible: Record<string, boolean> }
  sectionTabs: { tabs: Record<string, string> }
  uiPrefs: { rightPanelWidth: number }
  savedFilters: SavedFilterEntry[]
}

export interface ProfileVariant {
  id: string
  parentId: string
  name: string
  description: string
  version: string
  snapshot: ProfileSnapshot
}

export interface ProfileGroup {
  id: string
  name: string
  description: string
  variants: ProfileVariant[]
}

export interface FlattenedVariant extends ProfileVariant {
  groupName: string
  groupId: string
}
