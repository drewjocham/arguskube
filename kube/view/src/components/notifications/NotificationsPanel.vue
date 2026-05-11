<script setup>
import { storeToRefs } from 'pinia'
import { onMounted, onUnmounted, computed, ref } from 'vue'
import { useNotificationsStore } from '../../stores/notifications'
import { useSpotCheck } from '../../composables/useSpotCheck'

const store = useNotificationsStore()
const { panelOpen, sortedItems, settings, items } = storeToRefs(store)
const { runOne: runSpotCheckOne } = useSpotCheck()

const expanded = ref(new Set())
const showSettings = ref(false)
const draftMax = ref(settings.value.maxItems)

function close() { store.closePanel() }

function onDocClick(e) {
  if (!panelOpen.value) return
  if (e.target.closest('.notif-panel') || e.target.closest('.tb-bell')) return
  close()
}
onMounted(() => document.addEventListener('mousedown', onDocClick))
onUnmounted(() => document.removeEventListener('mousedown', onDocClick))

function onEscape(e) {
  if (e.key === 'Escape' && panelOpen.value) close()
}
onMounted(() => document.addEventListener('keydown', onEscape))
onUnmounted(() => document.removeEventListener('keydown', onEscape))

function toggleExpand(id) {
  store.markRead(id)
  if (expanded.value.has(id)) expanded.value.delete(id)
  else expanded.value.add(id)
  // Force reactivity on Set.
  expanded.value = new Set(expanded.value)
}

function isExpanded(id) { return expanded.value.has(id) }

function dismiss(id) {
  store.remove(id)
}

function clearAll() {
  store.clearAll()
}

function rerun(n) {
  const payload = n.rerunPayload || {}
  // Spot-check rerun: ask the backend to re-fire the same probe.
  // The fresh result lands as a NEW notification via the existing
  // event channel, so we don't add a duplicate row here ourselves.
  if (payload.type === 'spot-check' && payload.name) {
    runSpotCheckOne(payload.name)
    return
  }
  // Fallback (other rerun kinds, e.g. NetworkPolicy review): just
  // emit a stub notification so the user sees their click landed.
  // A future iteration can route these to dedicated backend rerun
  // handlers, the same way spot-check does.
  store.add({
    kind: n.kind,
    title: `Rerunning: ${n.title}`,
    body: n.body,
    rerunnable: n.rerunnable,
    rerunPayload: n.rerunPayload,
    meta: { ...n.meta, rerunOf: n.id },
  })
}

function applyMaxItems() {
  store.setMaxItems(draftMax.value)
  showSettings.value = false
}

function fmtTime(ts) {
  if (!ts) return '—'
  try {
    const d = new Date(ts)
    return d.toLocaleString('en-GB', { hour: '2-digit', minute: '2-digit', day: '2-digit', month: '2-digit' })
  } catch (_) { return ts }
}

function kindIcon(kind) {
  switch (kind) {
    case 'spot-check': return '◉'
    case 'scan': return '🔍'
    case 'alert': return '●'
    case 'warn': return '⚠'
    case 'error': return '✕'
    default: return '·'
  }
}

const totalShown = computed(() => sortedItems.value.length)
const cap = computed(() => settings.value.maxItems)
</script>

<template>
  <Teleport to="body">
    <div v-if="panelOpen" class="notif-panel">
      <div class="notif-header">
        <div class="notif-title">
          Notifications
          <span class="notif-count">{{ totalShown }} / {{ cap }}</span>
        </div>
        <div class="notif-header-actions">
          <button class="notif-link" @click="store.markAllRead()" v-if="items.length">Mark all read</button>
          <button class="notif-link" @click="showSettings = !showSettings" :title="'Configure cap'">Settings</button>
          <button class="notif-link danger" @click="clearAll" v-if="items.length">Clear all</button>
          <button class="notif-close" @click="close" title="Close (Esc)">×</button>
        </div>
      </div>

      <div v-if="showSettings" class="notif-settings">
        <label class="notif-setting-row">
          <span>Max retained notifications</span>
          <input type="number" min="1" max="5000" v-model.number="draftMax" />
          <button class="notif-link" @click="applyMaxItems">Apply</button>
        </label>
        <div class="notif-setting-hint">
          When the cap is reached, oldest notifications are evicted first. Hard limit: 5000.
        </div>
      </div>

      <div v-if="sortedItems.length === 0" class="notif-empty">
        No notifications yet. Argus will post here when it spot-checks the cluster, finishes a scan, or surfaces a finding.
      </div>

      <div v-else class="notif-list">
        <div
          v-for="n in sortedItems"
          :key="n.id"
          class="notif-row"
          :class="[`kind-${n.kind}`, { unread: !n.read }]"
        >
          <div class="notif-row-head" @click="toggleExpand(n.id)">
            <span class="notif-icon">{{ kindIcon(n.kind) }}</span>
            <div class="notif-text">
              <div class="notif-row-title">{{ n.title }}</div>
              <div class="notif-row-time">{{ fmtTime(n.createdAt) }}</div>
            </div>
            <div class="notif-row-actions" @click.stop>
              <button v-if="n.rerunnable" class="notif-row-btn" @click="rerun(n)" title="Run again">⟳</button>
              <button class="notif-row-btn danger" @click="dismiss(n.id)" title="Dismiss">×</button>
            </div>
          </div>
          <div v-if="isExpanded(n.id) && n.body" class="notif-row-body">{{ n.body }}</div>
        </div>
      </div>
    </div>
  </Teleport>
</template>

<style>
/* Unscoped — the panel is teleported to <body> and would otherwise lose
   the data-v-xxxx scope attribute Vue uses for scoped styles. */
.notif-panel {
  position: fixed;
  top: 50px;
  right: 12px;
  width: 380px;
  max-height: calc(100vh - 80px);
  background: #16171a;
  border: 1px solid rgba(255, 255, 255, 0.1);
  border-radius: 10px;
  box-shadow: 0 16px 40px rgba(0, 0, 0, 0.5);
  z-index: 1300;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  animation: notif-pop 0.15s ease-out;
}
@keyframes notif-pop {
  from { opacity: 0; transform: translateY(-4px); }
  to { opacity: 1; transform: translateY(0); }
}

.notif-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 10px 14px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.06);
  background: #1a1b1e;
  gap: 8px;
  flex-shrink: 0;
}
.notif-title {
  display: inline-flex; align-items: center; gap: 8px;
  font-size: 13px; font-weight: 600; color: #e8eaec;
}
.notif-count {
  font-family: var(--mono, ui-monospace);
  font-size: 10.5px;
  color: #6b7078;
  padding: 1px 6px;
  background: rgba(255, 255, 255, 0.04);
  border-radius: 8px;
}
.notif-header-actions { display: flex; align-items: center; gap: 4px; }
.notif-link {
  background: none; border: none;
  color: #b0b4ba; font-size: 11px;
  padding: 3px 6px; border-radius: 3px; cursor: pointer;
  transition: all 0.1s;
}
.notif-link:hover { background: rgba(255, 255, 255, 0.06); color: #fff; }
.notif-link.danger { color: #f7c1c1; }
.notif-link.danger:hover { background: rgba(240, 84, 84, 0.15); color: #ff7575; }
.notif-close {
  background: none; border: none; color: #8b8f96; cursor: pointer;
  font-size: 16px; line-height: 1; padding: 2px 6px; border-radius: 3px;
}
.notif-close:hover { background: rgba(255, 255, 255, 0.06); color: #fff; }

.notif-settings {
  padding: 8px 14px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.06);
  background: rgba(255, 255, 255, 0.02);
  display: flex; flex-direction: column; gap: 4px;
}
.notif-setting-row {
  display: flex; align-items: center; gap: 8px;
  font-size: 12px; color: #b0b4ba;
}
.notif-setting-row input {
  width: 90px; padding: 3px 6px;
  background: #0f1012; border: 1px solid rgba(255, 255, 255, 0.08);
  border-radius: 3px; color: #e8eaec; font-size: 11.5px;
}
.notif-setting-hint { font-size: 10.5px; color: #6b7078; }

.notif-empty { padding: 20px; text-align: center; color: #8b8f96; font-size: 12.5px; line-height: 1.5; }

.notif-list { overflow-y: auto; flex: 1; min-height: 0; }
.notif-row {
  border-bottom: 1px solid rgba(255, 255, 255, 0.04);
  padding: 0;
}
.notif-row.unread { background: rgba(79, 142, 247, 0.04); }
.notif-row-head {
  display: flex; gap: 10px; padding: 10px 14px;
  cursor: pointer; align-items: center;
}
.notif-row-head:hover { background: rgba(255, 255, 255, 0.02); }
.notif-icon {
  flex-shrink: 0;
  width: 18px; text-align: center; color: #8b8f96;
  font-family: var(--mono, ui-monospace);
}
.notif-row.kind-spot-check .notif-icon { color: #a78bfa; }
.notif-row.kind-scan .notif-icon       { color: #4f8ef7; }
.notif-row.kind-alert .notif-icon      { color: #f05454; }
.notif-row.kind-warn .notif-icon       { color: #f5a623; }
.notif-row.kind-error .notif-icon      { color: #f05454; }

.notif-text { flex: 1; min-width: 0; }
.notif-row-title {
  font-size: 12.5px; color: #e8eaec; font-weight: 500;
  white-space: nowrap; overflow: hidden; text-overflow: ellipsis;
}
.notif-row-time { font-size: 10.5px; color: #6b7078; margin-top: 2px; font-family: var(--mono, ui-monospace); }

.notif-row-actions { display: flex; gap: 2px; }
.notif-row-btn {
  background: none; border: none; color: #6b7078;
  width: 22px; height: 22px;
  border-radius: 3px; cursor: pointer; font-size: 13px;
}
.notif-row-btn:hover { background: rgba(255, 255, 255, 0.07); color: #e8eaec; }
.notif-row-btn.danger:hover { color: #ff7575; background: rgba(240, 84, 84, 0.12); }

.notif-row-body {
  padding: 0 14px 12px 42px;
  font-size: 12px; line-height: 1.5;
  color: #b0b4ba;
  white-space: pre-wrap;
  word-break: break-word;
}
</style>
