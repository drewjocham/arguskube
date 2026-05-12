<script setup>
import { ref, onMounted, onUnmounted, nextTick } from 'vue'
import { useResources, usePodExec, useServicePods } from '../../composables/useWails'
import { useWailsEvent } from '../../composables/useEvents'

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
  showPodPicker.value = true
  await fetchServicePods(svc.namespace, svc.name)

  // If only one pod, go straight to shell.
  if (backingPods.value.length === 1) {
    showPodPicker.value = false
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
useWailsEvent('exec:output', (data) => {
  if (shellTerm && data) {
    shellTerm.write(data)
  }
})

// Listen for exec session end.
useWailsEvent('exec:exit', () => {
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

          <!-- Pod picker (when service has multiple backing pods) -->
          <div v-else-if="showPodPicker" class="pod-picker">
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

          <!-- Shell tab -->
          <div v-else-if="activeTab === 'shell'" class="shell-section">
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
</template>

<style scoped>
.svc-view { padding: 24px; display: flex; flex-direction: column; gap: 24px; overflow-y: auto; height: 100%; }
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
</style>
