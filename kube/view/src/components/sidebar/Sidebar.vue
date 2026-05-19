<script setup>
import { ref, computed, inject, onMounted, onUnmounted } from 'vue'
import { useContexts } from '../../composables/useWails'
import { callGo } from '../../composables/useBridge'
import { useNavSearchStore } from '../../stores/navSearch'
import { useSectionTabsStore } from '../../stores/sectionTabs'
import { useNavVisibilityStore } from '../../stores/navVisibility'
import { useAppearanceStore } from '../../stores/appearance'
import { useAppNavStore } from '../../stores/appNav'
import { SECTIONS, SECTION_ORDER } from '../../lib/sectionTabs'
import ContextMenu from '../shared/ContextMenu.vue'

// Sidebar — section-level navigation. Each row is one of the 9 SECTIONS
// from lib/sectionTabs.js. Sub-items (Pods, Deployments, …) live inside
// CenterPanel as tabs; the sidebar no longer renders them. The user
// clicks a section header to switch to it; CenterPanel reads the
// section's remembered tab from useSectionTabsStore.
//
// What survived from the previous (37-row) layout:
//   - Cluster context selector + dropdown
//   - Sidebar collapse to icon-only mode + teleported popover
//   - Argus Context card at the bottom
//   - navSearch hook (now filtering at the tab-label level)
//
// What's new:
//   - navVisibility filter — optional sections (Storage, Knowledge, …)
//     are hidden by default. Re-enable from Settings → Navigation.

const navSearch = useNavSearchStore()
const sectionTabs = useSectionTabsStore()
const navVisibility = useNavVisibilityStore()
const appearance = useAppearanceStore()
const appNav = useAppNavStore()

const props = defineProps({
  clusterInfo: { type: Object, default: null },
  alerts: { type: Array, default: () => [] },
  // activeNav is now a section id ('monitoring', 'workloads', …). The
  // legacy contract (tab ids like 'pods') is still tolerated upstream
  // — App.vue maps inbound tab requests onto (section, tab) — but
  // by the time the prop lands here it's always a section id.
  activeNav: { type: String, default: 'monitoring' },
})

const emit = defineEmits(['update:activeNav', 'context-switched'])

const isAllowed = inject('isAllowed')

// --- Blast radius / production detection
const blastRadius = ref(null)
async function checkBlastRadius() {
  try {
    blastRadius.value = await callGo('GetBlastRadiusInfo')
  } catch {}
}
onMounted(checkBlastRadius)

// --- Cluster context selector (unchanged from the previous Sidebar)
const { contexts, loading: ctxLoading, switching, listContexts, switchContext } = useContexts()
const ctxDropdownOpen = ref(false)

onMounted(() => { listContexts() })

async function onSwitchContext(name) {
  await switchContext(name)
  ctxDropdownOpen.value = false
  emit('context-switched', name)
}

function toggleCtxDropdown() {
  ctxDropdownOpen.value = !ctxDropdownOpen.value
  if (ctxDropdownOpen.value) listContexts()
}

function onDocClick(e) {
  if (ctxDropdownOpen.value && !e.target.closest('.cluster-area')) {
    ctxDropdownOpen.value = false
  }
}
onMounted(() => document.addEventListener('click', onDocClick))
onUnmounted(() => document.removeEventListener('click', onDocClick))

// --- Sidebar collapse + popover (collapsed-mode flyout from each icon)
const sidebarCollapsed = ref(false)
const popoverSection = ref(null)
const popoverTop = ref(0)
const popoverLeft = ref(0)

function toggleSidebar() {
  sidebarCollapsed.value = !sidebarCollapsed.value
  popoverSection.value = null
}

function openPopover(sectionId, event) {
  if (!sidebarCollapsed.value) return
  if (popoverSection.value === sectionId) {
    popoverSection.value = null
    return
  }
  popoverSection.value = sectionId
  const rect = event.currentTarget.getBoundingClientRect()
  const sidebar = event.currentTarget.closest('.sidebar')
  const sidebarRect = sidebar.getBoundingClientRect()
  popoverTop.value = rect.top
  popoverLeft.value = sidebarRect.right + 4
}

function closePopover(e) {
  if (popoverSection.value && !e.target.closest('.sidebar-popover') && !e.target.closest('.icon-item')) {
    popoverSection.value = null
  }
}
onMounted(() => document.addEventListener('click', closePopover))
onUnmounted(() => document.removeEventListener('click', closePopover))

function popoverTabs() {
  if (!popoverSection.value) return []
  return SECTIONS[popoverSection.value]?.tabs || []
}
function popoverLabel() {
  if (!popoverSection.value) return ''
  return SECTIONS[popoverSection.value]?.label || ''
}

// --- Section list, filtered by visibility AND search.
// Order is canonical SECTION_ORDER → user can't reshuffle, which matches
// the previous fixed-order navTree. Visibility removes the optional
// sections the user (or the first-launch defaults) hasn't enabled.
const visibleSections = computed(() =>
  SECTION_ORDER
    .filter((id) => navVisibility.isVisible(id))
    .map((id) => SECTIONS[id])
)

// When the user types in the titlebar search, we keep a section
// visible iff any of its tabs match. Clicking such a section navigates
// to it AND jumps to the first matching tab, so the search result is
// honored end-to-end.
const filteredSections = computed(() => {
  if (!navSearch.active) {
    return visibleSections.value.map((s) => ({ ...s, matchedTabs: null }))
  }
  const out = []
  for (const sec of visibleSections.value) {
    const matchedTabs = sec.tabs.filter((t) => navSearch.matches(t.label))
    if (matchedTabs.length) out.push({ ...sec, matchedTabs })
  }
  return out
})

// --- Alert badge surfaces on the Monitoring row.
const criticalCount = computed(() =>
  props.alerts.filter((a) => a.severity === 'critical').length
)
const warningCount = computed(() =>
  props.alerts.filter((a) => a.severity === 'warning').length
)

function sectionBadge(sectionId) {
  if (sectionId !== 'monitoring') return null
  if (criticalCount.value > 0) return { tone: 'red', n: criticalCount.value }
  if (warningCount.value > 0) return { tone: 'amber', n: warningCount.value }
  return null
}

// --- Click handlers
function onSectionClick(section) {
  // When the user is searching, jump to the first tab that actually
  // matched. Otherwise, leave the section's remembered tab in place.
  if (navSearch.active) {
    const matched = section.matchedTabs?.[0]
    if (matched) sectionTabs.setTab(section.id, matched.id)
  }
  emit('update:activeNav', section.id)
}

function onTabHit(sectionId, tabId) {
  // Click on a search-result tab row: switch to its section + tab.
  sectionTabs.setTab(sectionId, tabId)
  emit('update:activeNav', sectionId)
}

function onPopoverTabClick(sectionId, tabId) {
  sectionTabs.setTab(sectionId, tabId)
  emit('update:activeNav', sectionId)
  popoverSection.value = null
}

// --- §C3 Right-click quick-toggle menu on section headers
const ctxMenu = ref(null)

function openSectionMenu(event, sectionId) {
  const items = []
  // Hide is only meaningful for optional sections — hiding a core
  // one would corner the user; the Settings panel handles edge cases.
  const sec = navVisibility.sections.find((s) => s.id === sectionId)
  if (sec && !sec.core) {
    items.push({ id: 'hide', label: `Hide ${sec.label}` })
  }
  items.push({ id: 'show-all', label: 'Show all sections' })
  items.push({ id: 'open-settings', label: 'Open navigation settings' })
  ctxMenu.value = { x: event.clientX, y: event.clientY, sectionId, items }
}

function onMenuSelect(id) {
  const sectionId = ctxMenu.value?.sectionId
  if (id === 'hide' && sectionId) {
    navVisibility.hide(sectionId)
  } else if (id === 'show-all') {
    for (const s of navVisibility.sections) {
      if (!navVisibility.isVisible(s.id)) navVisibility.show(s.id)
    }
  } else if (id === 'open-settings') {
    // Reveal Admin so the user can find the Settings panel even if it
    // wasn't visible before. Then jump to admin/settings.
    navVisibility.show('admin')
    sectionTabs.setTab('admin', 'settings')
    appNav.requestNav({ navId: 'settings' })
    emit('update:activeNav', 'admin')
  }
}

function closeMenu() { ctxMenu.value = null }
</script>

<template>
  <div class="sidebar" :class="{ 'sidebar-collapsed': sidebarCollapsed }">
    <!-- Collapse toggle -->
    <div class="collapse-toggle" @click="toggleSidebar" :title="sidebarCollapsed ? 'Expand sidebar' : 'Collapse sidebar'">
      <svg :class="{ flipped: sidebarCollapsed }" width="12" height="12" viewBox="0 0 12 12">
        <polyline points="8 2 4 6 8 10" fill="none" stroke="currentColor" stroke-width="1.4" stroke-linecap="round" stroke-linejoin="round"/>
      </svg>
    </div>

    <!-- Cluster selector -->
    <div class="cluster-area" v-if="!sidebarCollapsed">
      <div class="cluster-selector" @click.stop="toggleCtxDropdown" :class="{ open: ctxDropdownOpen }">
        <div class="cluster-icon">
          <svg width="14" height="14" viewBox="0 0 14 14" fill="none">
            <circle cx="7" cy="7" r="5" stroke="white" stroke-width="1.5"/>
            <circle cx="7" cy="7" r="2" fill="white"/>
          </svg>
        </div>
        <div class="cluster-info">
          <div class="cluster-name">
            {{ clusterInfo?.name || '—' }}
            <span v-if="blastRadius?.isProd" class="prod-badge">PROD</span>
          </div>
          <div class="cluster-sub">{{ clusterInfo?.nodeCount || 0 }} nodes · {{ clusterInfo?.k8sVersion || '—' }}</div>
        </div>
        <svg class="chevron-down" :class="{ flipped: ctxDropdownOpen }" width="10" height="10" viewBox="0 0 10 10">
          <path d="M3 4l2 2.5L7 4" stroke="currentColor" stroke-width="1.2" fill="none" stroke-linecap="round"/>
        </svg>
      </div>

      <!-- Context dropdown -->
      <div v-if="ctxDropdownOpen" class="ctx-dropdown">
        <div class="ctx-dropdown-header">Kubernetes Contexts</div>
        <div v-if="ctxLoading" class="ctx-loading">Loading…</div>
        <div v-else class="ctx-list">
          <div
            v-for="ctx in contexts"
            :key="ctx.name"
            class="ctx-item"
            :class="{ active: ctx.active, switching: switching }"
            @click.stop="onSwitchContext(ctx.name)"
          >
            <div class="ctx-dot" :class="{ active: ctx.active }"></div>
            <div class="ctx-details">
              <div class="ctx-name">{{ ctx.name }}</div>
              <div class="ctx-cluster">{{ ctx.cluster }}</div>
            </div>
            <div v-if="ctx.active" class="ctx-badge">active</div>
          </div>
        </div>
      </div>
    </div>

    <!-- Collapsed: cluster icon only -->
    <div class="cluster-area-mini" v-if="sidebarCollapsed" @click.stop="toggleCtxDropdown" title="Switch context">
      <div class="cluster-icon">
        <svg width="14" height="14" viewBox="0 0 14 14" fill="none">
          <circle cx="7" cy="7" r="5" stroke="white" stroke-width="1.5"/>
          <circle cx="7" cy="7" r="2" fill="white"/>
        </svg>
      </div>
    </div>

    <!-- EXPANDED: Section list -->
    <div class="nav-scroll" v-if="!sidebarCollapsed">
      <div
        v-if="navSearch.active && filteredSections.length === 0"
        class="nav-empty"
      >
        No menu items match <span class="nav-empty-q">"{{ navSearch.query }}"</span>.
      </div>

      <template v-for="section in filteredSections" :key="section.id">
        <div
          class="section-row"
          :class="{ active: activeNav === section.id }"
          :data-testid="`sidebar-section-${section.id}`"
          @click="onSectionClick(section)"
          @contextmenu.prevent="openSectionMenu($event, section.id)"
        >
          <svg class="section-icon" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">
            <path :d="section.icon" />
          </svg>
          <span class="section-label">{{ section.label }}</span>
          <span
            v-if="sectionBadge(section.id)"
            class="badge"
            :class="`badge-${sectionBadge(section.id).tone}`"
          >{{ sectionBadge(section.id).n }}</span>
        </div>

        <!-- Search hits are surfaced as a flat list of tab rows under
             the section. Clicking jumps straight to that tab. -->
        <div
          v-if="navSearch.active && section.matchedTabs"
          class="section-hits"
        >
          <div
            v-for="tab in section.matchedTabs"
            :key="`${section.id}.${tab.id}`"
            class="tab-hit"
            :class="{ active: activeNav === section.id && sectionTabs.activeTab(section.id) === tab.id, 'pro-locked': tab.pro && !isAllowed(tab.id) }"
            :data-testid="`sidebar-hit-${section.id}-${tab.id}`"
            @click.stop="onTabHit(section.id, tab.id)"
          >
            <span class="tab-hit-label">
              <template v-for="(seg, i) in navSearch.highlight(tab.label)" :key="i">
                <mark v-if="seg.hit" class="nav-label-hit">{{ seg.text }}</mark>
                <template v-else>{{ seg.text }}</template>
              </template>
            </span>
            <span v-if="tab.pro" class="pro-badge">PRO</span>
          </div>
        </div>
      </template>
    </div>

    <!-- COLLAPSED: Icon-only navigation -->
    <div class="nav-scroll icon-nav" v-if="sidebarCollapsed">
      <div
        v-for="section in visibleSections"
        :key="section.id"
        class="icon-item"
        :class="{ active: activeNav === section.id, 'popover-open': popoverSection === section.id }"
        :title="section.label"
        @click.stop="openPopover(section.id, $event)"
      >
        <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">
          <path :d="section.icon" />
        </svg>
      </div>
    </div>

    <!-- Popover for collapsed section — lists tabs so the user can jump
         to a specific tab without first expanding the sidebar. -->
    <Teleport to="body">
      <div
        v-if="popoverSection"
        class="sidebar-popover"
        :style="{ top: popoverTop + 'px', left: popoverLeft + 'px' }"
        @click.stop
      >
        <div class="popover-header">{{ popoverLabel() }}</div>
        <div
          v-for="tab in popoverTabs()"
          :key="tab.id"
          class="popover-item"
          :class="{
            active: activeNav === popoverSection && sectionTabs.activeTab(popoverSection) === tab.id,
            'pro-locked': tab.pro && !isAllowed(tab.id),
          }"
          @click="onPopoverTabClick(popoverSection, tab.id)"
        >
          {{ tab.label }}
          <span v-if="tab.pro && !isAllowed(tab.id)" class="pro-badge">PRO</span>
        </div>
      </div>
    </Teleport>

    <!-- AI Context card -->
    <div class="ai-context-card" v-if="!sidebarCollapsed">
      <div class="ai-context-header">
        <div class="ai-dot"></div>
        Argus Context
      </div>
      <div class="ai-context-body">
        {{ alerts.length }} active alerts · 12h window
      </div>
      <div class="ai-context-action" v-if="isAllowed('runbook_automation')">
        Attach runbook →
      </div>
      <div class="ai-context-action pro-label" v-else>
        PRO: Attach runbook
      </div>
    </div>

    <!-- §C4 Density + theme quick-picks in the sidebar footer.
         One-click access so users don't have to open Settings just to
         tighten spacing or flip the theme. The theme selector exposes
         the three modes the appearance store accepts: light, dark,
         auto (follow OS). Active state reflects the *stored* value,
         not the resolved value, so 'auto' shows as its own state
         instead of silently masquerading as light or dark. -->
    <div
      class="sidebar-footer"
      v-if="!sidebarCollapsed"
      data-testid="sidebar-footer"
    >
      <div class="theme-selector" :title="`Theme: ${appearance.theme}`">
        <button
          v-for="t in ['light', 'dark', 'auto']"
          :key="t"
          type="button"
          class="theme-btn"
          :class="{ active: appearance.theme === t }"
          :title="`${t.charAt(0).toUpperCase() + t.slice(1)} theme`"
          :aria-pressed="appearance.theme === t"
          :data-testid="`theme-${t}`"
          @click="appearance.setTheme(t)"
        >
          <!-- Sun -->
          <svg v-if="t === 'light'" width="14" height="14" viewBox="0 0 24 24" fill="none"
               stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"
               aria-hidden="true">
            <circle cx="12" cy="12" r="4" />
            <path d="M12 2v2M12 20v2M4.93 4.93l1.41 1.41M17.66 17.66l1.41 1.41M2 12h2M20 12h2M4.93 19.07l1.41-1.41M17.66 6.34l1.41-1.41" />
          </svg>
          <!-- Moon -->
          <svg v-else-if="t === 'dark'" width="14" height="14" viewBox="0 0 24 24" fill="none"
               stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"
               aria-hidden="true">
            <path d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79z" />
          </svg>
          <!-- Auto (half-filled circle) -->
          <svg v-else width="14" height="14" viewBox="0 0 24 24" fill="none"
               stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"
               aria-hidden="true">
            <circle cx="12" cy="12" r="9" />
            <path d="M12 3v18" />
            <path d="M12 3a9 9 0 0 1 0 18z" fill="currentColor" stroke="none" />
          </svg>
        </button>
      </div>

      <div class="density-selector" :title="`UI density: ${appearance.density}`">
        <button
          v-for="d in ['compact', 'normal', 'comfortable']"
          :key="d"
          type="button"
          class="density-btn"
          :class="{ active: appearance.density === d }"
          :title="`${d.charAt(0).toUpperCase() + d.slice(1)} density`"
          :data-testid="`density-${d}`"
          @click="appearance.setDensity(d)"
        >
          <span v-if="d === 'compact'">⇕</span>
          <span v-else-if="d === 'normal'">⊞</span>
          <span v-else>⊟</span>
        </button>
      </div>
    </div>

    <!-- §C3 Right-click context menu on section headers -->
    <ContextMenu
      v-if="ctxMenu"
      :x="ctxMenu.x"
      :y="ctxMenu.y"
      :items="ctxMenu.items"
      test-id="sidebar-section-menu"
      @select="onMenuSelect"
      @close="closeMenu"
    />
  </div>
</template>

<style scoped>
.sidebar {
  width: 230px;
  background: var(--bg2);
  border-right: 1px solid var(--border);
  display: flex;
  flex-direction: column;
  flex-shrink: 0;
  overflow: hidden;
  position: relative;
  transition: width 0.2s ease;
}
.sidebar-collapsed { width: 48px; }

.collapse-toggle {
  position: absolute;
  top: 8px;
  right: -10px;
  width: 20px;
  height: 20px;
  border-radius: 50%;
  background: var(--bg3);
  border: 1px solid var(--border);
  color: var(--text2);
  display: flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  z-index: 10;
  transition: all 0.15s;
}
.collapse-toggle:hover { background: var(--bg4); color: var(--text); }
.collapse-toggle svg { transition: transform 0.2s ease; }
.collapse-toggle svg.flipped { transform: rotate(180deg); }

.cluster-area { padding: 10px 10px 6px; }

.cluster-area-mini {
  padding: 10px 0 6px;
  display: flex;
  justify-content: center;
  cursor: pointer;
}

.cluster-selector {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 10px;
  border-radius: 6px;
  background: var(--bg3);
  border: 1px solid var(--border);
  cursor: pointer;
  transition: all 0.15s;
}
.cluster-selector:hover { background: var(--bg4); }
.cluster-selector.open { border-color: var(--accent2); }
.cluster-icon {
  width: 22px; height: 22px;
  background: var(--accent2);
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}
.cluster-info { flex: 1; min-width: 0; }
.cluster-name {
  font-size: 12px;
  font-weight: 600;
  color: var(--text);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.cluster-sub { font-size: 10px; color: var(--text3); }
.prod-badge { font-size: 8px; font-weight: 700; padding: 1px 4px; border-radius: 3px; background: rgba(240,84,84,0.2); color: #f05454; margin-left: 4px; letter-spacing: 0.05em; vertical-align: middle; }
.chevron-down { color: var(--text3); transition: transform 0.15s; flex-shrink: 0; }
.chevron-down.flipped { transform: rotate(180deg); }

.ctx-dropdown {
  margin-top: 4px;
  background: var(--bg3);
  border: 1px solid var(--border);
  border-radius: 6px;
  overflow: hidden;
}
.ctx-dropdown-header {
  padding: 6px 10px;
  font-size: 10px;
  text-transform: uppercase;
  letter-spacing: 0.04em;
  color: var(--text3);
  background: var(--bg2);
  border-bottom: 1px solid var(--border);
}
.ctx-loading { padding: 10px; font-size: 12px; color: var(--text3); }
.ctx-list { max-height: 240px; overflow-y: auto; }
.ctx-item {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 6px 10px;
  cursor: pointer;
  transition: background 0.15s;
}
.ctx-item:hover { background: var(--bg4); }
.ctx-item.active { background: rgba(79, 142, 247, 0.1); }
.ctx-dot {
  width: 6px; height: 6px;
  border-radius: 50%;
  background: var(--text3);
  flex-shrink: 0;
}
.ctx-dot.active { background: var(--accent2); }
.ctx-details { flex: 1; min-width: 0; }
.ctx-name {
  font-size: 12px;
  color: var(--text);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.ctx-cluster {
  font-size: 10px;
  color: var(--text3);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.ctx-badge {
  font-size: 9px;
  text-transform: uppercase;
  letter-spacing: 0.04em;
  color: var(--accent2);
}

.nav-scroll {
  flex: 1;
  overflow-y: auto;
  padding: 6px;
}
.nav-scroll.icon-nav { padding: 6px 0; }

.section-row {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 7px 10px;
  border-radius: 5px;
  cursor: pointer;
  color: var(--text2);
  font-size: 12.5px;
  font-weight: 500;
  margin-bottom: 1px;
  transition: background 0.1s, color 0.1s;
}
.section-row:hover {
  background: var(--bg3);
  color: var(--text);
}
.section-row.active {
  background: rgba(79, 142, 247, 0.12);
  color: var(--accent2);
}
.section-icon { flex-shrink: 0; }
.section-label { flex: 1; min-width: 0; }
.badge {
  font-size: 10px;
  font-weight: 600;
  padding: 1px 6px;
  border-radius: 9px;
  flex-shrink: 0;
}
.badge-red { background: rgba(208, 90, 90, 0.18); color: var(--red, #d05a5a); }
.badge-amber { background: rgba(212, 162, 86, 0.18); color: var(--amber, #d4a256); }

.section-hits {
  display: flex;
  flex-direction: column;
  gap: 1px;
  margin: 2px 0 6px 22px;
}
.tab-hit {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 4px 8px;
  border-radius: 4px;
  font-size: 11.5px;
  color: var(--text2);
  cursor: pointer;
  transition: background 0.1s, color 0.1s;
}
.tab-hit:hover {
  background: var(--bg3);
  color: var(--text);
}
.tab-hit.active {
  background: rgba(79, 142, 247, 0.1);
  color: var(--accent2);
}
.tab-hit.pro-locked { opacity: 0.6; cursor: default; }
.tab-hit-label { white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }

.nav-label-hit {
  background: rgba(212, 162, 86, 0.28);
  color: var(--text);
  padding: 0;
  border-radius: 2px;
}
.nav-empty {
  padding: 12px 10px;
  font-size: 12px;
  color: var(--text3);
}
.nav-empty-q { color: var(--text2); font-weight: 500; }

.icon-item {
  width: 100%;
  height: 40px;
  display: flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  color: var(--text2);
  transition: background 0.1s, color 0.1s;
  position: relative;
}
.icon-item:hover {
  background: var(--bg3);
  color: var(--text);
}
.icon-item.active {
  background: rgba(79, 142, 247, 0.12);
  color: var(--accent2);
}
.icon-item.active::before {
  content: '';
  position: absolute;
  left: 0;
  top: 8px;
  bottom: 8px;
  width: 2px;
  background: var(--accent2);
}
.icon-item.popover-open { background: var(--bg3); color: var(--text); }

.sidebar-popover {
  position: fixed;
  z-index: 100;
  background: var(--bg2);
  border: 1px solid var(--border);
  border-radius: 6px;
  padding: 4px;
  min-width: 180px;
  box-shadow: 0 4px 20px rgba(0, 0, 0, 0.5);
}
.popover-header {
  padding: 6px 10px;
  font-size: 11px;
  text-transform: uppercase;
  letter-spacing: 0.04em;
  color: var(--text3);
  border-bottom: 1px solid var(--border);
  margin-bottom: 4px;
}
.popover-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 6px 10px;
  font-size: 12px;
  color: var(--text2);
  border-radius: 4px;
  cursor: pointer;
  transition: background 0.1s, color 0.1s;
}
.popover-item:hover {
  background: var(--bg3);
  color: var(--text);
}
.popover-item.active {
  background: rgba(79, 142, 247, 0.12);
  color: var(--accent2);
}
.popover-item.pro-locked { opacity: 0.6; cursor: default; }

.pro-badge {
  font-size: 9px;
  font-weight: 700;
  letter-spacing: 0.04em;
  padding: 1px 5px;
  border-radius: 3px;
  background: rgba(208, 156, 88, 0.16);
  color: #d09c58;
}

.ai-context-card {
  margin: 8px 10px 12px;
  padding: 10px;
  background: var(--bg3);
  border: 1px solid var(--border);
  border-radius: 6px;
}
.ai-context-header {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 11px;
  font-weight: 600;
  color: var(--text);
  margin-bottom: 4px;
}
.ai-dot {
  width: 6px; height: 6px;
  border-radius: 50%;
  background: var(--accent2);
  box-shadow: 0 0 6px var(--accent2);
}
.ai-context-body {
  font-size: 11px;
  color: var(--text3);
  margin-bottom: 6px;
}
.ai-context-action {
  font-size: 11px;
  color: var(--accent2);
  cursor: pointer;
}
.ai-context-action.pro-label { color: var(--amber, #d4a256); cursor: default; }

/* §C4 — quick-picks pinned to the bottom of the sidebar.
   Two pill-groups (theme + density) sit side by side. Wraps cleanly
   on a narrow sidebar — each group is its own self-contained pill. */
.sidebar-footer {
  padding: 6px 10px 10px;
  border-top: 1px solid var(--border, #2a2a2a);
  display: flex;
  flex-wrap: wrap;
  justify-content: center;
  gap: 6px;
}
.theme-selector,
.density-selector {
  display: flex;
  gap: 2px;
  padding: 2px;
  background: var(--bg3, #222);
  border: 1px solid var(--border, #2a2a2a);
  border-radius: 6px;
}
.theme-btn,
.density-btn {
  background: none;
  border: none;
  color: var(--text3, #5a5a5a);
  cursor: pointer;
  width: 26px;
  height: 22px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  font-size: 13px;
  border-radius: 4px;
  transition: background 0.1s, color 0.1s;
  font-family: inherit;
  padding: 0;
}
.theme-btn:hover,
.density-btn:hover { background: var(--bg4, #2a2a2a); color: var(--text, #e5e5e5); }
.theme-btn.active,
.density-btn.active {
  background: rgba(79, 142, 247, 0.16);
  color: var(--accent2, #4a9eff);
}
.theme-btn svg {
  display: block;
}
</style>
