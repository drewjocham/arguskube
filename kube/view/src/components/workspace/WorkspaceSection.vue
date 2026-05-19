<script setup>
import { computed } from 'vue'
import { storeToRefs } from 'pinia'
import { useSectionTabsStore } from '../../stores/sectionTabs'
import { SECTIONS } from '../../lib/sectionTabs'
import ConnectionPanel from './ConnectionPanel.vue'
import SlackPanel from './SlackPanel.vue'
import DocsPanel from './DocsPanel.vue'
import SheetsPanel from './SheetsPanel.vue'
import TasksPanel from './TasksPanel.vue'
import GChatPanel from './GChatPanel.vue'
import SlackEventsPanel from './SlackEventsPanel.vue'
import CalendarPanel from './CalendarPanel.vue'
import ICloudPanel from './ICloudPanel.vue'
import MicrosoftPanel from './MicrosoftPanel.vue'
import CustomPanel from './CustomPanel.vue'

const sectionTabsStore = useSectionTabsStore()
const { tabs: sectionTabValues } = storeToRefs(sectionTabsStore)

const availableTabs = computed(() => SECTIONS.workspace?.tabs || [])
const active = computed(() => sectionTabValues.value.workspace || availableTabs.value[0]?.id || 'connections')

function setTab(id) { sectionTabsStore.setTab('workspace', id) }
</script>

<template>
  <div class="ws-section">
    <nav class="tabs">
      <button v-for="t in availableTabs" :key="t.id" class="tab"
        :class="{ active: t.id === active }" @click="setTab(t.id)">{{ t.label }}</button>
    </nav>
    <ConnectionPanel v-if="active === 'connections'" />
    <SlackPanel v-else-if="active === 'slack'" @switch-tab="setTab" />
    <DocsPanel v-else-if="active === 'gdocs'" @switch-tab="setTab" />
    <SheetsPanel v-else-if="active === 'gsheets'" @switch-tab="setTab" />
    <TasksPanel v-else-if="active === 'gtasks'" @switch-tab="setTab" />
    <GChatPanel v-else-if="active === 'gchat'" @switch-tab="setTab" />
    <CalendarPanel v-else-if="active === 'gcal'" @switch-tab="setTab" />
    <MicrosoftPanel v-else-if="active === 'microsoft'" @switch-tab="setTab" />
    <ICloudPanel v-else-if="active === 'icloud'" @switch-tab="setTab" />
    <CustomPanel v-else-if="active === 'custom'" @switch-tab="setTab" />
    <SlackEventsPanel v-else-if="active === 'slack-events'" />
  </div>
</template>

<style scoped>
.ws-section { flex: 1; min-height: 0; display: flex; flex-direction: column; overflow: hidden; }
.tabs { display: flex; align-items: center; height: 38px; border-bottom: 1px solid var(--border); background: var(--bg2); padding: 0 16px; gap: 2px; flex-shrink: 0; }
.tab { padding: 5px 12px; font-size: 12.5px; font-weight: 400; color: var(--text2); cursor: pointer; border-radius: 6px; background: transparent; border: 0; transition: all 0.1s; }
.tab:hover { background: var(--bg3); color: var(--text); }
.tab.active { background: rgba(79,142,247,0.12); color: var(--accent2); font-weight: 500; }
</style>
