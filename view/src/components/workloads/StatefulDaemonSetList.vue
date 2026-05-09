<script setup>
import { ref, computed, onMounted } from 'vue'
import { callGo, cachedCallGo, invalidateCache, DEFAULT_TTL } from '../../composables/useBridge'

const props = defineProps({
  type: { type: String, default: 'statefulsets' }
})

const resourceType = computed(() => props.type || 'statefulsets')
const resourceLabel = computed(() => {
  const map = { statefulsets: 'StatefulSet', daemonsets: 'DaemonSet' }
  return map[resourceType.value] || 'StatefulSet'
})
const resourceLabelPlural = computed(() => resourceLabel.value + 's')
const subtitle = computed(() => resourceType.value === 'daemonsets'
  ? 'Ensures pod copies run on every (or selected) node'
  : 'Stateful workload with persistent identity and storage')

const workloads = ref([])
const wsDetail = ref(null)
const expandedWs = ref(null)
const loading = ref(false)
const error = ref(null)
const notification = ref(null)

// Manifest popup state
const manifestPopup = ref(false)
const editingManifest = ref(false)
const manifestContent = ref('')
const manifestKind = ref('')
const manifestName = ref('')
const manifestNamespace = ref('')
const manifestLoading = ref(false)
const manifestApplying = ref(false)

async function fetchData() {
  loading.value = true
  error.value = null
  try {
    await cachedCallGo('ListResources', [resourceType.value, ''], DEFAULT_TTL)
      .then(r => { window.__tmp_res = r; return r })
      .catch(e => { throw e })
    // Re-read from cache to avoid stale reference issues
    const r = window.__tmp_res
    if (r && r.items && r.items.length > 0) {
      workloads.value = r.items.map(item => {
        const readyParts = (item.fields?.ready || '0/0').split('/')
        return {
          name: item.name,
          namespace: item.namespace,
          status: item.status || 'Running',
          statusColor: item.statusColor || 'green',
          desired: parseInt(item.fields?.desired || readyParts[1] || '0'),
          current: parseInt(item.fields?.current || readyParts[0] || '0'),
          ready: parseInt(readyParts[0] || '0'),
          nodeSelector: item.fields?.node_selector || null,
          age: item.age || '—'
        }
      })
    } else {
      workloads.value = []
    }
  } catch (e) {
    console.error('[SDSList] fetch failed:', e)
    error.value = e?.message || String(e)
    workloads.value = []
  } finally {
    loading.value = false
  }
}

onMounted(fetchData)

// Expose for CenterPanel re-init
function refresh() { fetchData() }
defineExpose({ refresh })

async function toggleExpand(wsName) {
  if (expandedWs.value === wsName) {
    expandedWs.value = null
    wsDetail.value = null
  } else {
    expandedWs.value = wsName
    const ws = workloads.value.find(w => w.name === wsName)
    if (ws) {
      try {
        const d = await cachedCallGo('GetResourceDetail', [resourceType.value, ws.namespace, wsName], DEFAULT_TTL)
        wsDetail.value = d
      } catch (e) {
        console.error('[SDSList] detail:', e)
        wsDetail.value = null
      }
    }
  }
}

async function openManifest(ws) {
  manifestLoading.value = true
  manifestPopup.value = true
  manifestKind.value = resourceLabel.value
  manifestName.value = ws.name
  manifestNamespace.value = ws.namespace
  manifestContent.value = ''
  editingManifest.value = false
  try {
    const yaml = await callGo('GetResourceYaml', resourceType.value, ws.namespace, ws.name)
    manifestContent.value = yaml
  } catch (e) {
    manifestContent.value = `# Error fetching manifest: ${e.message || e}`
  } finally {
    manifestLoading.value = false
  }
}

function closeManifest() {
  manifestPopup.value = false
  editingManifest.value = false
  manifestContent.value = ''
}

function toggleEditManifest() {
  editingManifest.value = !editingManifest.value
}

async function applyManifest() {
  if (!manifestContent.value.trim()) return
  manifestApplying.value = true
  try {
    const result = await callGo('ApplyYaml', manifestContent.value)
    invalidateCache('ListResources', resourceType.value, '_all')
    notification.value = `✓ ${result}`
    setTimeout(() => { notification.value = null }, 5000)
    closeManifest()
    await fetchData()
  } catch (e) {
    notification.value = `✗ Apply failed: ${e.message || e}`
    setTimeout(() => { notification.value = null }, 8000)
  } finally {
    manifestApplying.value = false
  }
}

async function deleteResource(ws) {
  if (!confirm(`Delete ${resourceLabel.value} "${ws.name}" in namespace "${ws.namespace}"?`)) return
  try {
    await callGo('DeleteResource', resourceType.value, ws.namespace, ws.name)
    invalidateCache('ListResources', resourceType.value, '_all')
    notification.value = `🗑 Deleted ${ws.name}`
    setTimeout(() => { notification.value = null }, 5000)
    await fetchData()
  } catch (e) {
    notification.value = `✗ Delete failed: ${e.message || e}`
    setTimeout(() => { notification.value = null }, 8000)
  }
}
</script>

<template>
  <div class="ws-view">
    <div class="header">
      <div class="title">{{ resourceLabelPlural }}</div>
      <div class="subtitle">{{ subtitle }}</div>
    </div>

    <!-- Agent notification -->
    <div v-if="notification" class="agent-notification">
      <div class="notif-icon">
        <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M12 2v20M17 5H9.5a3.5 3.5 0 0 0 0 7h5a3.5 3.5 0 0 1 0 7H6"></path></svg>
      </div>
      <div class="notif-text">{{ notification }}</div>
    </div>

    <!-- State boxes -->
    <div v-if="loading && workloads.length === 0" class="state-box">
      <div class="spinner"></div>
      <span>Loading {{ resourceLabelPlural.toLowerCase() }}…</span>
    </div>
    <div v-else-if="error && workloads.length === 0" class="state-box error">
      <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="10"/><line x1="15" y1="9" x2="9" y2="15"/><line x1="9" y1="9" x2="15" y2="15"/></svg>
      <span>{{ error }}</span>
    </div>
    <div v-else-if="!loading && workloads.length === 0" class="state-box">
      <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M13 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V9z"/><polyline points="13 2 13 9 20 9"/></svg>
      <span>No {{ resourceLabelPlural.toLowerCase() }} found</span>
    </div>

    <!-- Resource list table -->
    <div v-if="workloads.length > 0" class="ws-list">
      <div class="ws-header-row">
        <div class="col-name">Name</div>
        <div class="col-ns">Namespace</div>
        <div class="col-num">Desired</div>
        <div class="col-num">Current</div>
        <div class="col-num">Ready</div>
        <div class="col-health">Health</div>
        <div class="col-actions">Actions</div>
      </div>

      <div v-for="w in workloads" :key="w.name" class="ws-row-container" :class="{'ai-active-pulse': w.isApplying}">
        <div class="ws-row" @click="toggleExpand(w.name)">
          <div class="col-name">
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" :style="{ color: type === 'daemonsets' ? '#fb923c' : '#a78bfa', marginRight: '8px' }">
              <polygon points="12 2 2 7 12 12 22 7 12 2"></polygon>
              <polyline points="2 17 12 22 22 17"></polyline>
              <polyline points="2 12 12 17 22 12"></polyline>
            </svg>
            {{ w.name }}
          </div>
          <div class="col-ns font-mono">{{ w.namespace }}</div>
          <div class="col-num font-mono">{{ w.desired }}</div>
          <div class="col-num font-mono">{{ w.current }}</div>
          <div class="col-num font-mono">{{ w.ready }}</div>

          <div class="col-health">
            <div class="health-bar-container">
              <div class="health-bar-fill" :style="{ width: (w.desired > 0 ? (w.ready / w.desired * 100) : 0) + '%' }" :class="{'degraded': w.ready < w.desired}"></div>
            </div>
          </div>

          <div class="col-actions" @click.stop>
            <button class="action-btn" @click="openManifest(w)" title="View/Edit YAML">⚙️ Config</button>
            <button class="action-btn delete" @click="deleteResource(w)" title="Delete">🗑</button>
            <svg class="chevron" :class="{ open: expandedWs === w.name }" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <polyline points="6 9 12 15 18 9"></polyline>
            </svg>
          </div>
        </div>

        <!-- Expanded Detail -->
        <div class="ws-expanded" v-if="expandedWs === w.name">
          <div v-if="detailLoading" style="color:#8b8f96; font-size:13px; padding:12px;">Loading…</div>
          <div v-else-if="wsDetail" class="expanded-grid">
            <div class="expanded-card">
              <h4 class="card-title">Properties</h4>
              <div class="props-grid">
                <div class="prop-row" v-for="prop in wsDetail.properties" :key="prop.key">
                  <span class="prop-label">{{ prop.key }}</span>
                  <span class="prop-value font-mono">{{ prop.value }}</span>
                </div>
              </div>
            </div>

            <div class="expanded-card" v-if="wsDetail.conditions && wsDetail.conditions.length">
              <h4 class="card-title">Conditions</h4>
              <div class="conditions-list">
                <div class="condition-row" v-for="c in wsDetail.conditions" :key="c.type">
                  <span class="cond-type font-mono">{{ c.type }}</span>
                  <span class="cond-status" :class="c.status === 'True' ? 'ok' : 'fail'">{{ c.status }}</span>
                  <span class="cond-reason font-mono" v-if="c.reason">{{ c.reason }}</span>
                </div>
              </div>
            </div>

            <div class="expanded-card" v-if="wsDetail.labels && Object.keys(wsDetail.labels).length">
              <h4 class="card-title">Labels</h4>
              <div class="labels-grid">
                <span class="label-chip font-mono" v-for="(v, k) in wsDetail.labels" :key="k">{{ k }}={{ v }}</span>
              </div>
            </div>

            <div class="expanded-card" v-if="wsDetail.events && wsDetail.events.length">
              <h4 class="card-title">Recent Events</h4>
              <div class="events-mini">
                <div class="event-mini-row" v-for="(ev, i) in wsDetail.events" :key="i" :class="ev.type?.toLowerCase()">
                  <span class="ev-type">{{ ev.type }}</span>
                  <span class="ev-reason font-mono">{{ ev.reason }}</span>
                  <span class="ev-msg">{{ ev.message }}</span>
                  <span class="ev-age font-mono">{{ ev.age }}</span>
                </div>
              </div>
            </div>
          </div>
          <div v-else class="no-detail">No detail data available for this resource.</div>
        </div>
      </div>
    </div>

    <!-- Manifest Popup -->
    <div v-if="manifestPopup" class="popup-overlay" @click.self="closeManifest">
      <div class="popup-panel manifest-popup" @click.stop>
        <div class="popup-header">
          <div class="popup-title">
            <span class="popup-kind">{{ manifestKind }}</span>
            <span class="popup-name font-mono">{{ manifestName }}</span>
            <span class="popup-ns font-mono" v-if="manifestNamespace">{{ manifestNamespace }}</span>
          </div>
          <div class="popup-actions">
            <button v-if="!editingManifest" class="action-btn" @click="toggleEditManifest">✏️ Edit</button>
            <button v-if="editingManifest" class="action-btn primary" @click="toggleEditManifest">📖 View</button>
            <button class="action-btn primary" :disabled="manifestApplying || !manifestContent.trim()" @click="applyManifest">
              {{ manifestApplying ? '⏳ Applying…' : '🚀 Redeploy' }}
            </button>
            <button class="action-btn close" @click="closeManifest">✕</button>
          </div>
        </div>
        <div class="manifest-body">
          <div v-if="manifestLoading" class="manifest-loading">
            <div class="spinner"></div>
            <span>Loading manifest…</span>
          </div>
          <textarea v-else-if="editingManifest"
            class="manifest-editor font-mono"
            v-model="manifestContent"
            spellcheck="false"
          ></textarea>
          <pre v-else class="manifest-viewer font-mono">{{ manifestContent }}</pre>
        </div>
        <div class="popup-footer">
          <span class="hint">Double-click the editor area to toggle fullscreen. Changes are live after clicking Redeploy.</span>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.ws-view {
  padding: 24px;
  display: flex;
  flex-direction: column;
  gap: 16px;
  overflow-y: auto;
  flex: 1;
  min-height: 0;
}
.header .title { font-size: 20px; font-weight: 500; color: #fff; margin-bottom: 4px; text-transform: capitalize; }
.header .subtitle { font-size: 13px; color: #8b8f96; }

/* ── State Box ── */
.state-box {
  display: flex; align-items: center; gap: 12px;
  padding: 24px; border-radius: 8px;
  background: #1e2023; border: 1px solid rgba(255,255,255,0.08);
  font-size: 13px; color: #8b8f96;
}
.state-box.error { border-color: rgba(240,84,84,0.3); color: #f05454; }
.spinner { width: 18px; height: 18px; border: 2px solid rgba(167,139,250,0.3); border-top-color: #a78bfa; border-radius: 50%; animation: spin 0.7s linear infinite; flex-shrink: 0; }
@keyframes spin { to { transform: rotate(360deg); } }

/* ── Table ── */
.ws-list { background: #1e2023; border: 1px solid rgba(255, 255, 255, 0.08); border-radius: 8px; overflow: hidden; }
.ws-header-row {
  display: grid;
  grid-template-columns: 2fr 1.2fr 70px 70px 70px 1fr 160px;
  gap: 12px;
  padding: 12px 16px;
  background: rgba(255, 255, 255, 0.03);
  border-bottom: 1px solid rgba(255, 255, 255, 0.08);
  font-size: 11px;
  font-weight: 600;
  color: #8b8f96;
  text-transform: uppercase;
  letter-spacing: 0.05em;
}
.ws-row-container { border-bottom: 1px solid rgba(255, 255, 255, 0.04); transition: all 0.3s ease; }
.ws-row-container:last-child { border-bottom: none; }
.ws-row {
  display: grid;
  grid-template-columns: 2fr 1.2fr 70px 70px 70px 1fr 160px;
  gap: 12px;
  padding: 14px 16px;
  font-size: 13px;
  color: #e8eaec;
  align-items: center;
  cursor: pointer;
  transition: background 0.2s;
}
.ws-row:hover { background: rgba(255, 255, 255, 0.02); }

.col-name { display: flex; align-items: center; font-weight: 500; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.col-num { text-align: center; font-variant-numeric: tabular-nums; }
.font-mono { font-family: var(--mono); color: #b0b4ba; font-size: 12px; }

.health-bar-container { width: 100%; height: 6px; background: rgba(255, 255, 255, 0.06); border-radius: 3px; overflow: hidden; }
.health-bar-fill { height: 100%; background: #3ecf8e; transition: width 0.3s ease; border-radius: 3px; }
.health-bar-fill.degraded { background: #f5a623; }

.col-actions {
  display: flex;
  align-items: center;
  gap: 6px;
  justify-content: flex-end;
}
.action-btn {
  background: rgba(255,255,255,0.06);
  border: 1px solid rgba(255,255,255,0.1);
  color: #b0b4ba;
  padding: 4px 10px;
  border-radius: 4px;
  font-size: 11px;
  cursor: pointer;
  transition: all 0.2s;
  white-space: nowrap;
}
.action-btn:hover { background: rgba(255,255,255,0.12); color: #e8eaec; }
.action-btn.primary { background: rgba(167,139,250,0.15); border-color: rgba(167,139,250,0.3); color: #a78bfa; }
.action-btn.primary:hover { background: rgba(167,139,250,0.25); }
.action-btn.primary:disabled { opacity: 0.4; cursor: not-allowed; }
.action-btn.delete:hover { background: rgba(240,84,84,0.15); border-color: rgba(240,84,84,0.3); color: #f05454; }
.action-btn.close { background: transparent; border: none; color: #6b7078; font-size: 16px; padding: 4px 8px; }
.action-btn.close:hover { color: #e8eaec; }

.chevron { transition: transform 0.2s ease; color: #6b7078; flex-shrink: 0; }
.chevron.open { transform: rotate(180deg); }

/* ── Expanded ── */
.ws-expanded { padding: 16px; background: #141517; border-top: 1px dashed rgba(255,255,255,0.08); }
.expanded-grid { display: flex; flex-direction: column; gap: 12px; }
.expanded-card { background: #1e2023; border: 1px solid rgba(255,255,255,0.05); border-radius: 6px; padding: 16px; }
.card-title { font-size: 13px; font-weight: 600; color: #fff; margin: 0 0 12px 0; }
.no-detail { color: #6b7078; font-size: 13px; padding: 12px; }
.props-grid { display: flex; flex-direction: column; gap: 4px; }
.prop-row { display: flex; justify-content: space-between; font-size: 12px; padding: 4px 0; border-bottom: 1px solid rgba(255,255,255,0.03); }
.prop-label { color: #8b8f96; flex-shrink: 0; }
.prop-value { color: #e8eaec; max-width: 60%; text-align: right; word-break: break-all; }
.conditions-list { display: flex; flex-direction: column; gap: 4px; }
.condition-row { display: flex; align-items: center; gap: 12px; font-size: 12px; }
.cond-type { color: #e8eaec; min-width: 120px; }
.cond-status { font-size: 11px; padding: 2px 6px; border-radius: 4px; font-weight: 600; }
.cond-status.ok { background: rgba(62,207,142,0.15); color: #3ecf8e; }
.cond-status.fail { background: rgba(240,84,84,0.15); color: #f05454; }
.cond-reason { color: #8b8f96; }
.events-mini { display: flex; flex-direction: column; gap: 4px; max-height: 200px; overflow-y: auto; }
.event-mini-row { display: grid; grid-template-columns: 60px 120px 1fr 50px; gap: 8px; font-size: 11px; padding: 4px 0; border-bottom: 1px solid rgba(255,255,255,0.03); align-items: center; }
.event-mini-row.warning { color: #f5a623; }
.event-mini-row.normal { color: #b0b4ba; }
.ev-type { font-weight: 600; }
.ev-reason { color: #a78bfa; }
.ev-msg { color: #8b8f96; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.ev-age { color: #6b7078; text-align: right; }
.labels-grid { display: flex; flex-wrap: wrap; gap: 4px; }
.label-chip { padding: 2px 8px; border-radius: 4px; font-size: 10.5px; background: rgba(255,255,255,0.05); border: 1px solid rgba(255,255,255,0.08); color: #b0b4ba; white-space: nowrap; max-width: 100%; overflow: hidden; text-overflow: ellipsis; }

/* ── Pulse animation ── */
@keyframes pulse-glow {
  0% { box-shadow: inset 0 0 0px rgba(167, 139, 250, 0); background: transparent; }
  50% { box-shadow: inset 0 0 10px rgba(167, 139, 250, 0.4); background: rgba(167, 139, 250, 0.05); }
  100% { box-shadow: inset 0 0 0px rgba(167, 139, 250, 0); background: transparent; }
}
.ai-active-pulse { animation: pulse-glow 2s infinite; border-left: 3px solid #a78bfa; }

/* ── Agent notification ── */
.agent-notification { display: flex; align-items: center; gap: 12px; background: rgba(167, 139, 250, 0.15); border: 1px solid rgba(167, 139, 250, 0.3); padding: 12px 16px; border-radius: 6px; color: #e8eaec; font-size: 13px; animation: slide-down 0.3s ease-out; }
.notif-icon { color: #a78bfa; display: flex; }
@keyframes slide-down { from { opacity: 0; transform: translateY(-10px); } to { opacity: 1; transform: translateY(0); } }

/* ── Manifest Popup ── */
.popup-overlay {
  position: fixed; top: 0; left: 0; right: 0; bottom: 0;
  background: rgba(0,0,0,0.65); z-index: 1000;
  display: flex; align-items: center; justify-content: center;
  backdrop-filter: blur(4px);
  animation: fade-in 0.15s ease-out;
}
@keyframes fade-in { from { opacity: 0; } to { opacity: 1; } }

.popup-panel {
  background: #1a1b1e; border: 1px solid rgba(255,255,255,0.1);
  border-radius: 12px; max-width: 800px; width: 90%;
  max-height: 85vh; display: flex; flex-direction: column;
  box-shadow: 0 20px 60px rgba(0,0,0,0.5);
}
.popup-header {
  display: flex; align-items: center; justify-content: space-between;
  padding: 16px 20px; border-bottom: 1px solid rgba(255,255,255,0.08);
  flex-shrink: 0;
}
.popup-title { display: flex; align-items: center; gap: 8px; font-size: 14px; }
.popup-kind { color: #a78bfa; font-weight: 600; }
.popup-name { color: #e8eaec; }
.popup-ns { color: #6b7078; }
.popup-actions { display: flex; align-items: center; gap: 8px; }
.manifest-loading { display: flex; align-items: center; gap: 12px; padding: 32px; color: #8b8f96; font-size: 13px; justify-content: center; }
.manifest-body { flex: 1; overflow: auto; min-height: 300px; }
.manifest-viewer {
  padding: 16px 20px; margin: 0; font-size: 12px; line-height: 1.6;
  color: #c9d1d9; white-space: pre; tab-size: 2;
  overflow: auto; max-height: 55vh;
}
.manifest-editor {
  width: 100%; min-height: 300px; height: 55vh;
  padding: 16px 20px; font-size: 12px; line-height: 1.6;
  background: #121314; color: #c9d1d9; border: none; outline: none;
  resize: vertical; tab-size: 2;
  font-family: var(--mono);
}
.manifest-editor:focus { background: #0d0e0f; }
.popup-footer {
  padding: 10px 20px; border-top: 1px solid rgba(255,255,255,0.06);
  flex-shrink: 0;
}
.hint { font-size: 11px; color: #6b7078; }
</style>
