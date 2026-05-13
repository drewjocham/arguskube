<script setup>
import { computed } from 'vue'

// SectionTabs — the tab bar rendered at the top of each section in
// CenterPanel. Stateless: every input comes from props, the only
// output is the `update:active-tab` event. Reuses the same .tabs / .tab
// CSS the legacy alerts sub-tab already uses, so the visual language
// is identical — only the data drives a different row of pills.
//
// Slot:
//   #actions — section-level toolbar buttons rendered after the spacer
//              (e.g. "Diagnose All", "Run Scan"). Keeps section-specific
//              affordances on the same row as the tabs without each
//              section having to redefine the bar styling.

const props = defineProps({
  // Array of { id: string, label: string, pro?: boolean }. The
  // structure matches lib/sectionTabs.js so callers can pass
  // SECTIONS[sectionId].tabs directly.
  tabs: { type: Array, required: true },
  // Currently-selected tab id. Compared by string equality.
  activeTab: { type: String, default: '' },
  // Optional per-tab badge counts: { tabId: number }. A non-zero
  // count renders an amber dot + the number after the label.
  badgeCounts: { type: Object, default: () => ({}) },
  // Optional severity for the badge ('warn' | 'error'). Both share the
  // same dot affordance from the existing tab-meta styling; we tint
  // the dot only.
  badgeSeverity: { type: String, default: 'warn' },
})

const emit = defineEmits(['update:active-tab'])

const tabsList = computed(() => Array.isArray(props.tabs) ? props.tabs : [])

function onTabClick(id) {
  if (!id || id === props.activeTab) return
  emit('update:active-tab', id)
}

function onKeyNav(e, id) {
  if (e.key === 'Enter' || e.key === ' ') {
    e.preventDefault()
    onTabClick(id)
  }
}

function badgeFor(tabId) {
  const n = Number(props.badgeCounts?.[tabId])
  if (!Number.isFinite(n) || n <= 0) return 0
  return n
}

function dotColor() {
  return props.badgeSeverity === 'error' ? 'var(--red)' : 'var(--amber)'
}
</script>

<template>
  <div class="tabs" role="tablist" data-testid="section-tabs">
    <button
      v-for="tab in tabsList"
      :key="tab.id"
      type="button"
      role="tab"
      :aria-selected="tab.id === activeTab"
      :class="['tab', { active: tab.id === activeTab }]"
      :data-testid="`section-tab-${tab.id}`"
      :data-active="tab.id === activeTab"
      @click="onTabClick(tab.id)"
      @keydown="onKeyNav($event, tab.id)"
    >
      <span
        v-if="badgeFor(tab.id) > 0"
        class="tab-dot"
        :style="{ background: dotColor() }"
        aria-hidden="true"
      ></span>{{ tab.label }}<span
        v-if="tab.pro"
        class="tab-pro"
        aria-label="Pro feature"
      >PRO</span><span
        v-if="badgeFor(tab.id) > 0"
        class="tab-count"
        :aria-label="`${badgeFor(tab.id)} items`"
      >{{ badgeFor(tab.id) }}</span>
    </button>
    <div class="tab-spacer"></div>
    <slot name="actions" />
  </div>
</template>

<style scoped>
/* Mirrors the existing .tabs / .tab styles in CenterPanel.vue so a
   visual diff across sections is zero. Defined locally so the
   component is self-contained — CenterPanel can drop its inline
   copy in the same turn that introduces SectionTabs. */
.tabs {
  display: flex; align-items: center; height: 38px;
  border-bottom: 1px solid var(--border); background: var(--bg2);
  padding: 0 16px; gap: 2px; flex-shrink: 0;
}
.tab {
  padding: 5px 12px; font-size: 12.5px; font-weight: 400; color: var(--text2);
  cursor: pointer; border-radius: 6px; transition: all 0.1s; white-space: nowrap;
  background: none; border: none; font-family: inherit;
  display: inline-flex; align-items: center; gap: 6px;
}
.tab:hover { background: var(--bg3); color: var(--text); }
.tab:focus-visible {
  outline: 1px solid var(--accent2, #4a9eff);
  outline-offset: -1px;
}
.tab.active {
  background: rgba(79, 142, 247, 0.12);
  color: var(--accent2);
  font-weight: 500;
}

.tab-dot {
  display: inline-block; width: 5px; height: 5px;
  border-radius: 50%;
}
.tab-count {
  font-size: 10.5px;
  color: var(--text3);
  margin-left: 2px;
}
.tab-pro {
  font-size: 9px; font-weight: 700; letter-spacing: 0.04em;
  padding: 1px 5px; border-radius: 3px;
  background: rgba(208, 156, 88, 0.16); color: #d09c58;
  margin-left: 4px;
}
.tab-spacer { flex: 1; }
</style>
