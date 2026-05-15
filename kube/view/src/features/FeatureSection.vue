<script setup>
// FeatureSection — registry-driven section renderer. Given a sectionId,
// looks up the section in the feature registry, renders the SectionTabs
// bar from manifest.tabs, and renders the active tab's lazy component
// via <component :is>. The shell never names a feature.
//
// The active tab is read from useSectionTabsStore so it persists across
// reloads, identical to the hard-coded sections.
import { computed } from 'vue'
import { storeToRefs } from 'pinia'
import SectionTabs from '../components/shared/SectionTabs.vue'
import { useSectionTabsStore } from '../stores/sectionTabs'
import { getSection, resolveLazy } from './registry'

const props = defineProps({
  sectionId: { type: String, required: true },
})

const section = computed(() => getSection(props.sectionId))

const sectionTabsStore = useSectionTabsStore()
const { tabs: sectionTabValues } = storeToRefs(sectionTabsStore)

const activeTabId = computed(() => {
  const sec = section.value
  if (!sec) return ''
  const stored = sectionTabValues.value[props.sectionId]
  if (stored && sec.tabs?.some((t) => t.id === stored)) return stored
  return sec.defaultTab || sec.tabs?.[0]?.id || ''
})

const activeTab = computed(() =>
  section.value?.tabs?.find((t) => t.id === activeTabId.value) || null,
)

const ActiveComponent = computed(() => {
  if (activeTab.value?.component) return resolveLazy(activeTab.value.component)
  if (section.value?.panel) return resolveLazy(section.value.panel)
  return null
})

function setTab(id) {
  sectionTabsStore.setTab(props.sectionId, id)
}
</script>

<template>
  <template v-if="section">
    <SectionTabs
      v-if="section.tabs?.length"
      :tabs="section.tabs"
      :active-tab="activeTabId"
      @update:active-tab="setTab"
    />
    <div class="feature-content">
      <component :is="ActiveComponent" v-if="ActiveComponent" />
      <div v-else class="feature-empty">No content for {{ sectionId }}</div>
    </div>
  </template>
  <div v-else class="feature-missing">Unknown section: {{ sectionId }}</div>
</template>

<style scoped>
.feature-content { flex: 1; min-height: 0; display: flex; flex-direction: column; overflow: hidden; }
.feature-empty,
.feature-missing {
  padding: 12px;
  font-size: 12px;
  color: var(--text3);
}
</style>
