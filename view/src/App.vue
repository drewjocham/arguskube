<script setup>
import { ref, provide } from 'vue'
import { useClusterInfo, useMetrics, useAlerts, useDiagnostics, useFeatures } from './composables/useWails'
import { useWailsEvent, Events } from './composables/useEvents'
import Titlebar from './components/titlebar/Titlebar.vue'
import Sidebar from './components/sidebar/Sidebar.vue'
import CenterPanel from './components/center/CenterPanel.vue'
import DiagnosticsPanel from './components/diagnostics/DiagnosticsPanel.vue'
import TerminalView from './components/terminal/TerminalView.vue'

const { info: clusterInfo } = useClusterInfo()
const { metrics } = useMetrics()
const { alerts } = useAlerts()
const { bundle, loading: diagLoading, error: diagError, diagnose } = useDiagnostics()
const { tier, isAllowed } = useFeatures()

const logLines = ref([])
const selectedAlert = ref(null)
const activeNav = ref('alerts')
const terminalOpen = ref(false)
const terminalHeight = ref(220)

// Real-time event listeners from Go backend.
useWailsEvent(Events.ALERT_UPDATE, (data) => {
  if (data) alerts.value = data
})

useWailsEvent(Events.METRICS_UPDATE, (data) => {
  if (data) metrics.value = data
})

useWailsEvent(Events.LOG_LINE, (data) => {
  if (data) {
    logLines.value.push(data)
    if (logLines.value.length > 200) {
      logLines.value = logLines.value.slice(-200)
    }
  }
})

function onAlertSelect(alert) {
  selectedAlert.value = alert
  if (isAllowed('ai_diagnostics')) {
    diagnose(alert.id)
  }
}

function toggleTerminal() {
  terminalOpen.value = !terminalOpen.value
}

// Terminal resize drag.
let dragging = false
let startY = 0
let startHeight = 0

function onDragStart(e) {
  dragging = true
  startY = e.clientY
  startHeight = terminalHeight.value
  document.addEventListener('mousemove', onDragMove)
  document.addEventListener('mouseup', onDragEnd)
  e.preventDefault()
}

function onDragMove(e) {
  if (!dragging) return
  const delta = startY - e.clientY
  terminalHeight.value = Math.max(100, Math.min(500, startHeight + delta))
}

function onDragEnd() {
  dragging = false
  document.removeEventListener('mousemove', onDragMove)
  document.removeEventListener('mouseup', onDragEnd)
}

provide('tier', tier)
provide('isAllowed', isAllowed)
</script>

<template>
  <Titlebar :clusterInfo="clusterInfo" @toggle-terminal="toggleTerminal" :terminalOpen="terminalOpen" />
  <div class="main">
    <Sidebar
      :clusterInfo="clusterInfo"
      :alerts="alerts"
      :activeNav="activeNav"
      @update:activeNav="activeNav = $event"
    />
    <div class="center-area">
      <div class="center-content">
        <CenterPanel
          :metrics="metrics"
          :alerts="alerts"
          :selectedAlert="selectedAlert"
          :logLines="logLines"
          :activeNav="activeNav"
          @select-alert="onAlertSelect"
        />
        <DiagnosticsPanel
          :selectedAlert="selectedAlert"
          :bundle="bundle"
          :loading="diagLoading"
          :error="diagError"
          @diagnose="onAlertSelect"
        />
      </div>

      <!-- Terminal panel -->
      <template v-if="terminalOpen">
        <div class="terminal-divider" @mousedown="onDragStart">
          <div class="divider-handle"></div>
        </div>
        <div class="terminal-panel" :style="{ height: terminalHeight + 'px' }">
          <div class="terminal-header">
            <div class="terminal-tabs">
              <div class="terminal-tab active">Terminal</div>
            </div>
            <button class="terminal-close" @click="toggleTerminal">
              <svg width="10" height="10" viewBox="0 0 10 10">
                <path d="M2 2l6 6M8 2l-6 6" stroke="currentColor" stroke-width="1.3" stroke-linecap="round"/>
              </svg>
            </button>
          </div>
          <TerminalView :visible="terminalOpen" />
        </div>
      </template>
    </div>
  </div>
</template>

<style scoped>
.main {
  flex: 1;
  display: flex;
  overflow: hidden;
}

.center-area {
  flex: 1;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.center-content {
  flex: 1;
  display: flex;
  overflow: hidden;
}

/* Terminal panel */
.terminal-divider {
  height: 4px;
  background: var(--border);
  cursor: row-resize;
  flex-shrink: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: background 0.15s;
}
.terminal-divider:hover { background: var(--accent); }
.divider-handle {
  width: 30px;
  height: 2px;
  border-radius: 1px;
  background: var(--text3);
  opacity: 0.4;
}

.terminal-panel {
  display: flex;
  flex-direction: column;
  background: var(--bg);
  flex-shrink: 0;
  overflow: hidden;
}

.terminal-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  height: 30px;
  background: var(--bg2);
  border-top: 1px solid var(--border);
  border-bottom: 1px solid var(--border);
  padding: 0 10px;
  flex-shrink: 0;
}

.terminal-tabs { display: flex; gap: 0; }

.terminal-tab {
  font-size: 11px;
  font-weight: 500;
  color: var(--text2);
  padding: 4px 10px;
  cursor: pointer;
}
.terminal-tab.active { color: var(--text); }

.terminal-close {
  background: none;
  border: none;
  color: var(--text3);
  cursor: pointer;
  padding: 3px;
  border-radius: 3px;
  display: flex;
  transition: all 0.15s;
}
.terminal-close:hover { background: var(--bg4); color: var(--text); }
</style>
