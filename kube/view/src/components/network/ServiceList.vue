<script setup>
import { ref, onMounted, onUnmounted, nextTick } from 'vue'
import { useResources, usePodExec, useServicePods } from '../../composables/useWails'
import { bus } from '../../lib/bus'
import { callGo } from '../../composables/useBridge'

const props = defineProps({
  type: { type: String, default: 'services' }
})

const { result, detail, loading, error, detailLoading, listResources, getResourceDetail } = useResources()
const { connected: execConnected, error: execError, startExec, sendInput: sendExecInput, resizeExec, closeExec } = usePodExec()
const { pods: backingPods, loading: podsLoading, error: podsError, fetchServicePods } = useServicePods()

const resourceKind = props.type || 'services'

const services = ref([])
const svcDetail = ref(null)
const expandedSvc = ref(null)
const notification = ref(null)
const activeTab = ref('details')
const readinessResult = ref(null)
const readinessLoading = ref(false)

async function analyzeReadiness(svc) {
  readinessLoading.value = true
  try {
    const result = await callGo('AnalyzeEndpointReadiness', svc.namespace, svc.name)
    readinessResult.value = result
  } catch {
    readinessResult.value = null
  }
  readinessLoading.value = false
}

// Shell state.
const shellTermRef = ref(null)
const showPodPicker = ref(false)
const shellService = ref(null)
let shellTerm = null
let shellFitAddon = null

function mapItems() {
  if (result.value && result.value.items && result.value.items.length > 0) {
    services.value = result.value.items.map(item => ({
      name: item.name,
      namespace: item.namespace,
      status: item.status,
      type: item.fields?.type || 'ClusterIP',
      clusterIP: item.fields?.cluster_ip || '—',
      externalIP: item.fields?.external_ip || '<none>',
      ports: item.fields?.ports || '—',
      age: item.age || '—'
    }))
  } else {
    services.value = []
  }
}

async function refresh(force = false) {
  await listResources(resourceKind, '', force)
  mapItems()
}

onMounted(() => refresh())

async function toggleExpand(svcName) {
  if (expandedSvc.value === svcName) {
    expandedSvc.value = null
    svcDetail.value = null
    destroyShellTerminal()
    activeTab.value = 'details'
  } else {
    destroyShellTerminal()
    activeTab.value = 'details'
    expandedSvc.value = svcName
    const svc = services.value.find(s => s.name === svcName)
    if (svc) {
      await getResourceDetail(resourceKind, svc.namespace, svcName)
      if (detail.value) {
        svcDetail.value = detail.value
      }
    }
  }
}

// Shell — resolve service to pods, let user pick, then exec into it.
async function openShellForService(svc) {
  expandedSvc.value = svc.name
  shellService.value = svc
  // Flip the tab IMMEDIATELY so the Details pane unmounts and the
  // pod-picker / terminal pane has a chance to render. Without this
  // the v-if chain short-circuited on details and the picker was
  // never visible — users with multi-pod services saw no UI change
  // after clicking Shell.
  activeTab.value = 'shell'
  showPodPicker.value = true
  await fetchServicePods(svc.namespace, svc.name)

  // If only one pod, go straight to shell.
  if (backingPods.value.length === 1) {
    await openShellIntoPod(backingPods.value[0])
  }
}

async function openShellIntoPod(pod) {
  showPodPicker.value = false
  activeTab.value = 'shell'
  await nextTick()
  await initShellTerminal(pod)
}

async function initShellTerminal(pod) {
  const { Terminal } = await import('xterm')
  const { FitAddon } = await import('xterm-addon-fit')

  if (shellTerm) {
    shellTerm.dispose()
    shellTerm = null
  }

  const term = new Terminal({
    cursorBlink: true,
    fontSize: 13,
    fontFamily: "'Cascadia Mono', 'Cascadia Code', 'SF Mono', Consolas, monospace",
    theme: {
      background: '#1a1c1e',
      foreground: '#e8eaec',
      cursor: '#4f8ef7',
      selectionBackground: 'rgba(79,142,247,0.3)',
    },
    allowTransparency: true,
  })

  const fitAddon = new FitAddon()
  term.loadAddon(fitAddon)

  if (shellTermRef.value) {
    term.open(shellTermRef.value)
    fitAddon.fit()
  }

  shellTerm = term
  shellFitAddon = fitAddon

  term.onData((data) => {
    sendExecInput(data)
  })

  const rows = term.rows
  const cols = term.cols
  await startExec(pod.namespace, pod.name, pod.container || '', rows, cols)

  term.onResize(({ rows, cols }) => {
    resizeExec(rows, cols)
  })
}

function destroyShellTerminal() {
  if (shellTerm) {
    shellTerm.dispose()
    shellTerm = null
    shellFitAddon = null
  }
  closeExec()
  showPodPicker.value = false
  shellService.value = null
}

// Listen for exec output.
bus.useWailsEvent('exec:output', (data) => {
  if (shellTerm && data) {
    shellTerm.write(data)
  }
})

// Listen for exec session end.
bus.useWailsEvent('exec:exit', () => {
  if (shellTerm) {
    shellTerm.write('\r\n\x1b[33m[Session ended]\x1b[0m\r\n')
  }
})

onUnmounted(() => {
  destroyShellTerminal()
})
</script>

<template>
  <div class="svc-view">
    <div class="header">
      <div class="header-row">
        <div>
          <div class="title">Services</div>
          <div class="subtitle">Network abstractions exposing applications running on Pods</div>
        </div>
        <button class="refresh-btn" @click="refresh(true)" :disabled="loading">{{ loading ? 'Loading…' : '↻ Refresh' }}</button>
      </div>
    </div>
    
    <div v-if="notification" class="agent-notification">
      <div class="notif-icon">
        <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M12 2v20M17 5H9.5a3.5 3.5 0 0 0 0 7h5a3.5 3.5 0 0 1 0 7H6"></path></svg>
      </div>
      <div class="notif-text">{{ notification }}</div>
    </div>

    <div class="svc-scroll-area">
    <div v-if="loading && !services.length" class="state-box">Loading services…</div>
    <div v-else-if="error" class="state-box state-error">{{ error }}</div>
    <div v-else-if="!services.length" class="state-box">No services found in this cluster.</div>

    <div v-else class="svc-list">
      <div class="svc-header-row">
        <div class="col-name">Name</div>
        <div class="col-ns">Namespace</div>
        <div class="col-type">Type</div>
        <div class="col-ip">Cluster IP</div>
        <div class="col-ext">External IP</div>
        <div class="col-ports">Ports</div>
      </div>

      <div v-for="s in services" :key="s.name" class="svc-row-container">
        <div class="svc-row" @click="toggleExpand(s.name)">
          <div class="col-name">
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" style="color: #f5a623; margin-right: 8px;"><path d="M10 13a5 5 0 0 0 7.54.54l3-3a5 5 0 0 0-7.07-7.07l-1.72 1.71"></path><path d="M14 11a5 5 0 0 0-7.54-.54l-3 3a5 5 0 0 0 7.07 7.07l1.71-1.71"></path></svg>
            {{ s.name }}
          </div>
          <div class="col-ns font-mono">{{ s.namespace }}</div>
          <div class="col-type">
            <span class="type-badge" :class="s.type.toLowerCase()">{{ s.type }}</span>
          </div>
          <div class="col-ip font-mono">{{ s.clusterIP }}</div>
          <div class="col-ext font-mono">{{ s.externalIP }}</div>
          <div class="col-ports font-mono" style="display:flex; justify-content:space-between; align-items:center;">
            {{ s.ports }}
            <div class="row-actions">
              <button class="row-shell-btn" @click.stop="openShellForService(s)" title="Shell into backing pod">
                <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="4 17 10 11 4 5"></polyline><line x1="12" y1="19" x2="20" y2="19"></line></svg>
              </button>
              <svg class="chevron" :class="{ open: expandedSvc === s.name }" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <polyline points="6 9 12 15 18 9"></polyline>
              </svg>
            </div>
          </div>
        </div>

        <!-- Expanded Service Details -->
        <div class="svc-expanded" v-if="expandedSvc === s.name">
          <!-- Tabs -->
          <div class="tab-bar">
            <button class="tab-btn" :class="{ active: activeTab === 'details' }" @click="activeTab = 'details'">Details</button>
            <button class="tab-btn" :class="{ active: activeTab === 'readiness' }" @click="activeTab = 'readiness'; analyzeReadiness(s)">
              <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="10"/><line x1="12" y1="8" x2="12" y2="12"/><line x1="12" y1="16" x2="12.01" y2="16"/></svg>
              Readiness
            </button>
            <button class="tab-btn shell-tab" :class="{ active: activeTab === 'shell' }" @click="openShellForService(s)">
              <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="4 17 10 11 4 5"></polyline><line x1="12" y1="19" x2="20" y2="19"></line></svg>
              Shell
            </button>
          </div>

          <!-- Details tab -->
          <div v-if="activeTab === 'details'">
            <div v-if="detailLoading" style="color:#8b8f96; font-size:13px; padding:12px;">Loading…</div>
            <div v-else-if="svcDetail" class="expanded-grid">
              <div class="expanded-card">
                <h4 class="card-title">Service Properties</h4>
                <div class="props-grid">
                  <div class="prop-row" v-for="prop in svcDetail.properties" :key="prop.key">
                    <span class="prop-label">{{ prop.key }}</span>
                    <span class="prop-value font-mono">{{ prop.value }}</span>
                  </div>
                </div>
              </div>
              <div class="expanded-card">
                <div v-if="svcDetail.labels && Object.keys(svcDetail.labels).length" style="margin-bottom:16px;">
                  <h4 class="card-title">Labels</h4>
                  <div class="labels-grid">
                    <span class="label-chip" v-for="(v, k) in svcDetail.labels" :key="k">{{ k }}={{ v }}</span>
                  </div>
                </div>
                <div v-if="svcDetail.events && svcDetail.events.length">
                  <h4 class="card-title">Recent Events</h4>
                  <div class="events-mini">
                    <div class="event-mini-row" v-for="(ev, i) in svcDetail.events" :key="i" :class="ev.type?.toLowerCase()">
                      <span class="ev-type">{{ ev.type }}</span>
                      <span class="ev-reason font-mono">{{ ev.reason }}</span>
                      <span class="ev-msg">{{ ev.message }}</span>
                      <span class="ev-age font-mono">{{ ev.age }}</span>
                    </div>
                  </div>
                </div>
              </div>
            </div>
          </div>

          <!-- Readiness tab -->
          <div v-else-if="activeTab === 'readiness'">
            <div v-if="readinessLoading" style="color:#8b8f96; font-size:13px; padding:12px;">Analyzing readiness…</div>
            <div v-else-if="!readinessResult" style="color:#8b8f96; font-size:13px; padding:12px;">No readiness data available</div>
            <div v-else class="expanded-grid">
              <div v-if="!readinessResult.healthy" class="readiness-warning-banner">
                <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="#f05454" stroke-width="2"><path d="M10.29 3.86L1.82 18a2 2 0 001.71 3h16.94a2 2 0 001.71-3L13.71 3.86a2 2 0 00-3.42 0z"/><line x1="12" y1="9" x2="12" y2="13"/><line x1="12" y1="17" x2="12.01" y2="17"/></svg>
                <div class="warning-body">
                  <div class="warning-title">Endpoint Mismatch Detected</div>
                  <div class="warning-detail">
                    Expected <strong>{{ readinessResult.expectedEndpoints }}</strong> endpoints,
                    found <strong>{{ readinessResult.actualEndpoints }}</strong> —
                    <strong>{{ readinessResult.missingCount }}</strong> missing
                  </div>
                </div>
              </div>
              <div v-else class="readiness-ok-banner">
                <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="#3ecf8e" stroke-width="2"><path d="M22 11.08V12a10 10 0 11-5.93-9.14"/><polyline points="22 4 12 14.01 9 11.01"/></svg>
                <span>All endpoints healthy ({{ readinessResult.actualEndpoints }}/{{ readinessResult.expectedEndpoints }})</span>
              </div>

              <div v-if="readinessResult.failingPods && readinessResult.failingPods.length" class="expanded-card failing-section">
                <h4 class="card-title" style="color:#f05454">Failing Pods ({{ readinessResult.failingPods.length }})</h4>
                <div v-for="fp in readinessResult.failingPods" :key="fp.name" class="failing-pod-card">
                  <div class="fp-header">
                    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="#f05454" stroke-width="2"><circle cx="12" cy="12" r="10"/><line x1="15" y1="9" x2="9" y2="15"/><line x1="9" y1="9" x2="15" y2="15"/></svg>
                    <span class="fp-name font-mono">{{ fp.name }}</span>
                    <span class="fp-reason">{{ fp.reason }}</span>
                    <span v-if="fp.exitCode" class="fp-exit font-mono">exit {{ fp.exitCode }}</span>
                  </div>
                  <div v-if="fp.logs" class="fp-logs font-mono">
                    <div class="fp-logs-header">Crash logs:</div>
                    <pre class="fp-log-content">{{ fp.logs }}</pre>
                  </div>
                </div>
              </div>

              <div class="expanded-card" v-if="readinessResult.timeline && readinessResult.timeline.length">
                <h4 class="card-title">Endpoint State Timeline</h4>
                <div class="timeline">
                  <div v-for="(ev, i) in readinessResult.timeline" :key="i" class="timeline-event" :class="ev.type">
                    <div class="tl-dot" :class="ev.type"></div>
                    <div class="tl-content">
                      <span class="tl-ip font-mono">{{ ev.ip }}</span>
                      <span class="tl-pod font-mono" v-if="ev.podName">{{ ev.podName }}</span>
                      <span class="tl-status" :class="ev.type">{{ ev.type }}</span>
                      <span class="tl-reason" v-if="ev.reason">{{ ev.reason }}</span>
                    </div>
                  </div>
                </div>
              </div>
            </div>
          </div>

          <!-- Pod picker (when service has multiple backing pods).
               Lives under the Shell tab — gated on showPodPicker so the
               terminal pane below can take over once a pod is chosen. -->
          <div v-else-if="activeTab === 'shell' && showPodPicker" class="pod-picker">
            <div v-if="podsLoading" class="picker-loading">Resolving backing pods…</div>
            <div v-else-if="podsError" class="picker-error">{{ podsError }}</div>
            <div v-else-if="!backingPods.length" class="picker-empty">No backing pods found for this service.</div>
            <div v-else class="picker-list">
              <div class="picker-title">Select a pod to shell into:</div>
              <button
                v-for="pod in backingPods"
                :key="pod.name"
                class="picker-pod"
                @click="openShellIntoPod(pod)"
              >
                <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="4 17 10 11 4 5"></polyline><line x1="12" y1="19" x2="20" y2="19"></line></svg>
                <span class="picker-pod-name font-mono">{{ pod.name }}</span>
                <span class="picker-pod-status" :class="pod.status.toLowerCase()">{{ pod.status }}</span>
              </button>
            </div>
          </div>

          <!-- Shell terminal — only after a pod has been selected
               (showPodPicker flipped back to false). -->
          <div v-else-if="activeTab === 'shell' && !showPodPicker" class="shell-section">
            <div class="shell-toolbar">
              <div class="shell-info">
                <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="4 17 10 11 4 5"></polyline><line x1="12" y1="19" x2="20" y2="19"></line></svg>
                <span class="font-mono">{{ s.namespace }}/{{ s.name }}</span>
                <span class="shell-status" :class="{ connected: execConnected }">{{ execConnected ? 'Connected' : 'Connecting…' }}</span>
              </div>
              <button class="shell-close-btn" @click="destroyShellTerminal(); activeTab = 'details'" title="Close shell">
                <svg width="14" height="14" viewBox="0 0 16 16" fill="currentColor"><path d="M7.116 8l-4.558 4.558.884.884L8 8.884l4.558 4.558.884-.884L8.884 8l4.558-4.558-.884-.884L8 7.116 3.442 2.558l-.884.884L7.116 8z"/></svg>
              </button>
            </div>
            <div v-if="execError" class="shell-error">{{ execError }}</div>
            <div class="shell-terminal" ref="shellTermRef"></div>
          </div>
        </div>
      </div>
    </div>
    </div>
  </div>
</template>

<style scoped>
.svc-view { padding: 24px; display: flex; flex-direction: column; gap: 24px; min-height: 0; flex: 1; box-sizing: border-box; }
.svc-scroll-area { flex: 1; overflow-y: auto; min-height: 0; }
.header .title { font-size: 20px; font-weight: 500; color: #fff; margin-bottom: 4px; }
.header .subtitle { font-size: 13px; color: #8b8f96; }
.header-row { display: flex; justify-content: space-between; align-items: flex-start; }
.refresh-btn { background: rgba(255,255,255,0.06); border: 1px solid rgba(255,255,255,0.1); color: #b0b4ba; padding: 6px 12px; border-radius: 6px; font-size: 12px; cursor: pointer; transition: all 0.15s; }
.refresh-btn:hover { background: rgba(255,255,255,0.1); color: #fff; }
.refresh-btn:disabled { opacity: 0.5; cursor: not-allowed; }
.state-box { padding: 40px; text-align: center; color: #8b8f96; font-size: 13px; background: #1e2023; border: 1px solid rgba(255,255,255,0.08); border-radius: 8px; }
.state-error { color: #f05454; }

.svc-list { background: #1e2023; border: 1px solid rgba(255, 255, 255, 0.08); border-radius: 8px; overflow: hidden; }

.svc-header-row {
  display: grid;
  grid-template-columns: 2fr 1fr 100px 1.5fr 1.5fr 1.5fr;
  gap: 16px;
  padding: 12px 16px;
  background: rgba(255, 255, 255, 0.03);
  border-bottom: 1px solid rgba(255, 255, 255, 0.08);
  font-size: 11px;
  font-weight: 600;
  color: #8b8f96;
  text-transform: uppercase;
  letter-spacing: 0.05em;
}

.svc-row-container {
  border-bottom: 1px solid rgba(255, 255, 255, 0.04);
}
.svc-row-container:last-child { border-bottom: none; }

.svc-row {
  display: grid;
  grid-template-columns: 2fr 1fr 100px 1.5fr 1.5fr 1.5fr;
  gap: 16px;
  padding: 14px 16px;
  font-size: 13px;
  color: #e8eaec;
  align-items: center;
  cursor: pointer;
  transition: background 0.2s;
}
.svc-row:hover { background: rgba(255, 255, 255, 0.03); }

.col-name { display: flex; align-items: center; font-weight: 500; }
.font-mono { font-family: var(--mono); color: #b0b4ba; font-size: 12px; }

.chevron { transition: transform 0.2s ease; color: #6b7078; }
.chevron.open { transform: rotate(180deg); }

.type-badge { font-size: 11px; padding: 2px 6px; border-radius: 4px; font-weight: 600; background: rgba(255,255,255,0.05); color: #ccc; }
.type-badge.loadbalancer { background: rgba(245, 166, 35, 0.15); color: #f5a623; }

/* Expanded Area */
.svc-expanded {
  padding: 16px;
  background: #141517;
  border-top: 1px dashed rgba(255,255,255,0.08);
}
.expanded-grid {
  display: flex;
  gap: 24px;
}
.expanded-card {
  background: #1e2023;
  border: 1px solid rgba(255, 255, 255, 0.05);
  border-radius: 6px;
  padding: 16px;
  flex: 1;
}
.card-title {
  font-size: 13px;
  font-weight: 600;
  color: #fff;
  margin: 0 0 16px 0;
}

/* Properties */
.props-grid { display: flex; flex-direction: column; gap: 4px; }
.prop-row { display: flex; justify-content: space-between; font-size: 12px; padding: 4px 0; border-bottom: 1px solid rgba(255,255,255,0.03); }
.prop-label { color: #8b8f96; }
.prop-value { color: #e8eaec; max-width: 60%; text-align: right; word-break: break-all; }

/* Labels */
.labels-grid { display: flex; flex-wrap: wrap; gap: 6px; }
.label-chip { background: rgba(55, 148, 255, 0.1); color: #3794ff; font-size: 11px; padding: 3px 8px; border-radius: 4px; font-family: var(--mono); }

/* Mini Events */
.events-mini { display: flex; flex-direction: column; gap: 4px; max-height: 200px; overflow-y: auto; }
.event-mini-row { display: grid; grid-template-columns: 60px 120px 1fr 50px; gap: 8px; font-size: 11px; padding: 4px 0; border-bottom: 1px solid rgba(255,255,255,0.03); align-items: center; }
.event-mini-row.warning { color: #f5a623; }
.event-mini-row.normal { color: #b0b4ba; }
.ev-type { font-weight: 600; }
.ev-reason { color: #a78bfa; }
.ev-msg { color: #8b8f96; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.ev-age { color: #6b7078; text-align: right; }

/* Agent Notification */
.agent-notification {
  display: flex;
  align-items: center;
  gap: 12px;
  background: rgba(167, 139, 250, 0.15);
  border: 1px solid rgba(167, 139, 250, 0.3);
  padding: 12px 16px;
  border-radius: 6px;
  margin-bottom: 16px;
  color: #e8eaec;
  font-size: 13px;
  animation: slide-down 0.3s ease-out;
}
.notif-icon { color: #a78bfa; display: flex; }
@keyframes slide-down {
  from { opacity: 0; transform: translateY(-10px); }
  to { opacity: 1; transform: translateY(0); }
}

/* Row actions */
.row-actions { display: flex; align-items: center; gap: 6px; }
.row-shell-btn {
  background: none; border: none; color: var(--text3, #6b7078); cursor: pointer;
  padding: 3px; border-radius: 4px; display: flex; align-items: center;
  opacity: 0; transition: all 0.15s cubic-bezier(0.16, 1, 0.3, 1);
}
.svc-row:hover .row-shell-btn { opacity: 1; }
.row-shell-btn:hover { color: #4f8ef7; background: rgba(79,142,247,0.12); }

/* Tab bar */
.tab-bar {
  display: flex; gap: 2px; padding: 8px 12px 0;
  border-bottom: 1px solid rgba(255,255,255,0.06);
}
.tab-btn {
  background: none; border: none; color: #8b8f96; font-size: 12px;
  padding: 6px 14px; cursor: pointer; border-bottom: 2px solid transparent;
  transition: all 0.15s; font-weight: 500;
}
.tab-btn:hover { color: #e8eaec; }
.tab-btn.active { color: #fff; border-bottom-color: #4f8ef7; }
.tab-btn.shell-tab { display: flex; align-items: center; gap: 5px; }

/* Pod picker */
.pod-picker { padding: 16px; }
.picker-loading, .picker-empty { color: #8b8f96; font-size: 13px; }
.picker-error { color: #f05454; font-size: 13px; }
.picker-title { font-size: 13px; color: #b0b4ba; margin-bottom: 10px; }
.picker-list { display: flex; flex-direction: column; gap: 4px; }
.picker-pod {
  display: flex; align-items: center; gap: 10px; padding: 8px 12px;
  background: rgba(255,255,255,0.03); border: 1px solid rgba(255,255,255,0.06);
  border-radius: 6px; cursor: pointer; color: #e8eaec; font-size: 13px;
  transition: all 0.15s;
}
.picker-pod:hover { background: rgba(79,142,247,0.08); border-color: rgba(79,142,247,0.2); }
.picker-pod svg { color: #4f8ef7; }
.picker-pod-name { flex: 1; }
.picker-pod-status { font-size: 11px; font-weight: 600; padding: 2px 6px; border-radius: 4px; }
.picker-pod-status.running { background: rgba(62,207,142,0.15); color: #3ecf8e; }
.picker-pod-status.pending { background: rgba(245,166,35,0.15); color: #f5a623; }
.picker-pod-status.failed { background: rgba(240,84,84,0.15); color: #f05454; }

/* Shell section */
.shell-section {
  display: flex; flex-direction: column; height: 350px;
}
.shell-toolbar {
  display: flex; align-items: center; justify-content: space-between;
  padding: 6px 12px; background: rgba(255,255,255,0.02);
  border-bottom: 1px solid rgba(255,255,255,0.06);
}
.shell-info {
  display: flex; align-items: center; gap: 8px;
  font-size: 12px; color: #b0b4ba;
}
.shell-status {
  font-size: 10px; padding: 1px 8px; border-radius: 10px;
  background: rgba(245,166,35,0.15); color: #f5a623; font-weight: 500;
}
.shell-status.connected { background: rgba(62,207,142,0.15); color: #3ecf8e; }
.shell-close-btn {
  background: none; border: none; color: #6b7078; cursor: pointer;
  padding: 4px; border-radius: 4px; display: flex; align-items: center;
  transition: all 0.15s;
}
.shell-close-btn:hover { color: rgba(232,17,35,0.9); background: rgba(232,17,35,0.1); }
.shell-error {
  padding: 8px 12px; font-size: 12px; color: #f05454;
  background: rgba(240,84,84,0.08); border-bottom: 1px solid rgba(240,84,84,0.15);
}
.shell-terminal {
  flex: 1; background: #1a1c1e; padding: 4px; overflow: hidden;
}
.shell-terminal :deep(.xterm) { height: 100%; }
.shell-terminal :deep(.xterm-viewport) { overflow-y: auto !important; }

/* Readiness Analyzer */
.readiness-warning-banner { display: flex; gap: 12px; padding: 14px 16px; background: rgba(240,84,84,0.1); border: 1px solid rgba(240,84,84,0.25); border-radius: 8px; margin-bottom: 12px; align-items: flex-start; }
.readiness-warning-banner .warning-body { flex: 1; }
.readiness-warning-banner .warning-title { font-size: 14px; font-weight: 600; color: #f05454; margin-bottom: 4px; }
.readiness-warning-banner .warning-detail { font-size: 13px; color: #e8eaec; }
.readiness-ok-banner { display: flex; align-items: center; gap: 10px; padding: 14px 16px; background: rgba(62,207,142,0.1); border: 1px solid rgba(62,207,142,0.25); border-radius: 8px; margin-bottom: 12px; font-size: 13px; color: #3ecf8e; }
.failing-section { border-color: rgba(240,84,84,0.2); }
.failing-pod-card { padding: 10px; background: rgba(240,84,84,0.04); border: 1px solid rgba(240,84,84,0.12); border-radius: 6px; margin-top: 6px; }
.fp-header { display: flex; align-items: center; gap: 8px; font-size: 12px; }
.fp-name { color: #e8eaec; flex: 1; }
.fp-reason { color: #f05454; font-weight: 500; }
.fp-exit { color: #8b8f96; }
.fp-logs { margin-top: 8px; }
.fp-logs-header { font-size: 11px; color: #8b8f96; margin-bottom: 4px; }
.fp-log-content { font-size: 11px; line-height: 1.4; color: #b0b4ba; background: #141517; padding: 8px; border-radius: 4px; max-height: 150px; overflow-y: auto; white-space: pre-wrap; word-break: break-all; margin: 0; }

.timeline { display: flex; flex-direction: column; gap: 0; }
.timeline-event { display: flex; gap: 10px; padding: 6px 0; border-left: 2px solid rgba(255,255,255,0.06); margin-left: 8px; padding-left: 16px; }
.timeline-event:last-child { border-left-color: transparent; }
.tl-dot { width: 8px; height: 8px; border-radius: 50%; flex-shrink: 0; margin-left: -20px; margin-top: 4px; }
.tl-dot.active { background: #3ecf8e; }
.tl-dot.not-ready { background: #f5a623; }
.tl-content { display: flex; gap: 8px; align-items: center; font-size: 12px; flex-wrap: wrap; }
.tl-ip { color: #a78bfa; }
.tl-pod { color: #8b8f96; }
.tl-status { font-size: 10px; padding: 1px 6px; border-radius: 3px; font-weight: 600; }
.tl-status.active { background: rgba(62,207,142,0.15); color: #3ecf8e; }
.tl-status.not-ready { background: rgba(245,166,35,0.15); color: #f5a623; }
.tl-reason { color: #8b8f96; font-size: 11px; }
</style>
