<script setup>
import { ref } from 'vue'

const timeRange = ref('Last 1 hour')

// Mock chart points
const generateWave = (points, amp, freq, offset) => {
  const res = []
  for (let i = 0; i < points; i++) {
    res.push(50 + Math.sin(i * freq + offset) * amp + (Math.random() * 10 - 5))
  }
  return res
}

function pointsToPath(data, width, height) {
  if (!data.length) return ''
  const step = width / (data.length - 1)
  const d = data.map((val, i) => {
    // Map 0-100 to height-0
    const y = height - (val / 100 * height)
    const x = i * step
    return `${i === 0 ? 'M' : 'L'} ${x} ${y}`
  })
  return d.join(' ')
}

function pointsToArea(data, width, height) {
  const line = pointsToPath(data, width, height)
  return `${line} L ${width} ${height} L 0 ${height} Z`
}

const panels = ref([
  {
    id: 1, type: 'area', title: 'CPU Utilization', val: '42%',
    query: 'sum(rate(container_cpu_usage_seconds_total[5m])) by (pod)',
    color: 'var(--accent)', bg: 'rgba(79, 142, 247, 0.15)',
    data: generateWave(100, 20, 0.1, 0), editing: false, span: 1
  },
  {
    id: 2, type: 'area', title: 'Memory Usage', val: '8.4 GB',
    query: 'sum(container_memory_working_set_bytes) by (pod)',
    color: 'var(--purple)', bg: 'rgba(167, 139, 250, 0.15)',
    data: generateWave(100, 10, 0.05, 2), editing: false, span: 1
  },
  {
    id: 3, type: 'network', title: 'Network I/O',
    query: 'rate(container_network_receive_bytes_total[5m])',
    rxData: generateWave(100, 30, 0.2, 1), txData: generateWave(100, 15, 0.2, 1.5),
    editing: false, span: 2
  },
  {
    id: 4, type: 'stat', title: 'Disk IOPS',
    query: 'rate(container_fs_reads_total[5m])',
    reads: '4,281', writes: '1,092',
    editing: false, span: 1
  },
  {
    id: 5, type: 'gauge', title: 'Active Pods',
    query: 'count(kube_pod_info)',
    val: '124', limit: '200',
    editing: false, span: 1
  }
])

function addPanel() {
  panels.value.push({
    id: Date.now(), type: 'area', title: 'New Custom Metric', val: '0',
    query: 'rate(http_requests_total[5m])',
    color: 'var(--teal)', bg: 'rgba(45, 212, 191, 0.15)',
    data: generateWave(100, 5, 0.1, 0), editing: true, span: 1
  })
}

function savePanel(p) {
  p.editing = false
}

const expandedPanel = ref(null)

function toggleExpand(p) {
  if (expandedPanel.value === p.id) {
    expandedPanel.value = null
  } else {
    expandedPanel.value = p.id
  }
}
</script>

<template>
  <div class="metrics-dashboard">
    <!-- Toolbar -->
    <div class="dashboard-toolbar">
      <div class="db-title">
        <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" style="margin-right:8px; vertical-align:text-bottom;">
          <rect x="3" y="3" width="7" height="7"></rect>
          <rect x="14" y="3" width="7" height="7"></rect>
          <rect x="14" y="14" width="7" height="7"></rect>
          <rect x="3" y="14" width="7" height="7"></rect>
        </svg>
        Cluster Overview
      </div>
      <div class="db-controls">
        <button class="db-btn outline" @click="addPanel">
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <line x1="12" y1="5" x2="12" y2="19"></line><line x1="5" y1="12" x2="19" y2="12"></line>
          </svg>
          Add Panel
        </button>
        <div class="db-select-wrapper ml-1">
          <select class="db-select">
            <option>All Namespaces</option>
            <option>kube-system</option>
            <option>default</option>
          </select>
        </div>
        <div class="db-select-wrapper">
          <select v-model="timeRange" class="db-select">
            <option>Last 15 minutes</option>
            <option>Last 1 hour</option>
            <option>Last 24 hours</option>
          </select>
        </div>
        <button class="db-btn primary">
          <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <path d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79z"></path>
          </svg>
        </button>
      </div>
    </div>

    <!-- Grid -->
    <div class="panels-grid" :class="{ 'has-expanded': expandedPanel !== null }">
      <div v-for="p in panels" :key="p.id" 
           class="panel" 
           :class="[(p.span === 2 ? 'span-2' : ''), (expandedPanel === p.id ? 'expanded' : '')]"
           v-show="expandedPanel === null || expandedPanel === p.id">
        
        <div class="panel-header">
          <div style="display:flex; align-items:center; gap:8px;">
            <span class="panel-title">{{ p.title }}</span>
            <button class="icon-btn edit-icon" @click="p.editing = !p.editing" title="Edit PromQL Query">
              <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7"></path><path d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z"></path></svg>
            </button>
            <button class="icon-btn expand-icon" @click="toggleExpand(p)" :title="expandedPanel === p.id ? 'Collapse' : 'Expand'">
              <svg v-if="expandedPanel !== p.id" width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="15 3 21 3 21 9"></polyline><polyline points="9 21 3 21 3 15"></polyline><line x1="21" y1="3" x2="14" y2="10"></line><line x1="3" y1="21" x2="10" y2="14"></line></svg>
              <svg v-else width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="4 14 10 14 10 20"></polyline><polyline points="20 10 14 10 14 4"></polyline><line x1="14" y1="10" x2="21" y2="3"></line><line x1="3" y1="21" x2="10" y2="14"></line></svg>
            </button>
          </div>
          <span class="panel-val" v-if="p.val">{{ p.val }}</span>
          <div class="panel-legend" v-else-if="p.type === 'network'">
            <span class="leg-item"><span class="leg-dot rx"></span> RX: 120 Mbps</span>
            <span class="leg-item"><span class="leg-dot tx"></span> TX: 45 Mbps</span>
          </div>
        </div>

        <!-- Edit Overlay -->
        <div class="panel-edit" v-if="p.editing">
          <div class="edit-group">
            <label>Panel Title</label>
            <input type="text" v-model="p.title" class="edit-input" />
          </div>
          <div class="edit-group">
            <label>PromQL Query A</label>
            <textarea v-model="p.query" class="edit-input font-mono" rows="3"></textarea>
          </div>
          <div class="edit-actions">
            <button class="save-btn" @click="savePanel(p)">Apply Changes</button>
          </div>
        </div>

        <!-- Render visual based on type -->
        <div class="panel-body" v-else>
          
          <template v-if="p.type === 'area'">
            <svg viewBox="0 0 400 100" preserveAspectRatio="none" class="panel-svg">
              <path :d="pointsToArea(p.data, 400, 100)" :fill="p.bg" />
              <path :d="pointsToPath(p.data, 400, 100)" fill="none" :stroke="p.color" stroke-width="1.5" />
            </svg>
          </template>

          <template v-else-if="p.type === 'network'">
            <svg viewBox="0 0 800 150" preserveAspectRatio="none" class="panel-svg">
              <line x1="0" y1="30" x2="800" y2="30" stroke="var(--border)" stroke-width="1" stroke-dasharray="4" />
              <line x1="0" y1="75" x2="800" y2="75" stroke="var(--border)" stroke-width="1" stroke-dasharray="4" />
              <line x1="0" y1="120" x2="800" y2="120" stroke="var(--border)" stroke-width="1" stroke-dasharray="4" />
              <path :d="pointsToArea(p.rxData, 800, 150)" fill="rgba(45, 212, 191, 0.15)" />
              <path :d="pointsToPath(p.rxData, 800, 150)" fill="none" stroke="var(--teal)" stroke-width="1.5" />
              <path :d="pointsToArea(p.txData, 800, 150)" fill="rgba(245, 166, 35, 0.15)" />
              <path :d="pointsToPath(p.txData, 800, 150)" fill="none" stroke="var(--amber)" stroke-width="1.5" />
            </svg>
          </template>

          <template v-else-if="p.type === 'stat'">
            <div class="stat-big">
              <div class="stat-num">{{ p.reads }}</div>
              <div class="stat-sub">Reads/sec</div>
            </div>
            <div class="stat-big mt">
              <div class="stat-num">{{ p.writes }}</div>
              <div class="stat-sub">Writes/sec</div>
            </div>
          </template>

          <template v-else-if="p.type === 'gauge'">
            <div class="flex-center" style="flex:1; display:flex;">
              <div class="gauge">
                <svg viewBox="0 0 100 50">
                  <path d="M 10 50 A 40 40 0 0 1 90 50" fill="none" stroke="var(--bg4)" stroke-width="12" stroke-linecap="round" />
                  <path d="M 10 50 A 40 40 0 0 1 70 15" fill="none" stroke="var(--green)" stroke-width="12" stroke-linecap="round" />
                </svg>
                <div class="gauge-val">{{ p.val }}</div>
                <div class="gauge-lbl">of {{ p.limit }} Limit</div>
              </div>
            </div>
          </template>

        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.metrics-dashboard {
  display: flex;
  flex-direction: column;
  height: 100%;
  background: #111217; /* Grafana-like dark bg */
  color: #c8c9ca;
}

.dashboard-toolbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 12px 16px;
  background: var(--bg2);
  border-bottom: 1px solid var(--border);
}

.db-title {
  font-size: 15px;
  font-weight: 500;
  color: var(--text);
}

.db-controls {
  display: flex;
  gap: 8px;
}

.db-select {
  background: var(--bg3);
  border: 1px solid var(--border);
  color: var(--text);
  padding: 6px 28px 6px 12px;
  border-radius: 4px;
  font-size: 12px;
  outline: none;
  appearance: none;
}
.db-select-wrapper {
  position: relative;
}
.db-select-wrapper::after {
  content: '▼';
  position: absolute;
  right: 10px;
  top: 50%;
  transform: translateY(-50%);
  font-size: 8px;
  color: var(--text3);
  pointer-events: none;
}

.db-btn {
  background: var(--bg3);
  border: 1px solid var(--border);
  color: var(--text);
  min-width: 30px;
  padding: 0 10px;
  border-radius: 4px;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  font-size: 11.5px;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.15s;
}
.db-btn:hover { background: var(--bg4); }
.db-btn.primary { background: rgba(79,142,247,0.15); border-color: rgba(79,142,247,0.3); color: var(--accent); padding: 0 8px; }
.db-btn.outline { background: transparent; border-color: var(--border2); color: var(--text2); }
.db-btn.outline:hover { color: var(--text); border-color: var(--text3); }

.ml-1 { margin-left: 8px; }

/* Grid layout */
.panels-grid {
  flex: 1;
  padding: 12px;
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  grid-auto-rows: 240px;
  gap: 12px;
  overflow-y: auto;
}
.panels-grid.has-expanded {
  display: flex;
  overflow: hidden;
}

.panel {
  background: #181b1f; /* Grafana panel bg */
  border: 1px solid #2c3235;
  border-radius: 3px;
  display: flex;
  flex-direction: column;
  position: relative;
}
.panel.span-2 {
  grid-column: span 2;
}

.panel.expanded {
  flex: 1;
  border-color: var(--accent);
}

.panel::before {
  content: '';
  position: absolute;
  top: -1px; left: -1px; right: -1px; height: 2px;
  background: transparent;
  transition: background 0.2s;
  border-radius: 3px 3px 0 0;
}
.panel:hover::before {
  background: var(--accent);
}

.panel-header {
  padding: 8px 12px;
  display: flex;
  justify-content: space-between;
  align-items: center;
}
.panel-title {
  font-size: 12.5px;
  font-weight: 500;
  color: var(--text);
}
.panel-val {
  font-size: 18px;
  font-weight: 600;
  color: var(--text);
  font-family: var(--mono);
}

.icon-btn {
  background: transparent;
  border: none;
  color: var(--text3);
  cursor: pointer;
  padding: 4px;
  border-radius: 4px;
  display: flex; align-items: center; justify-content: center;
  transition: all 0.15s;
  opacity: 0;
}
.panel:hover .edit-icon, .panel:hover .expand-icon {
  opacity: 1;
}
.icon-btn:hover {
  background: rgba(255,255,255,0.1);
  color: var(--text);
}

/* Edit Overlay */
.panel-edit {
  position: absolute;
  top: 36px; left: 0; right: 0; bottom: 0;
  background: rgba(24, 27, 31, 0.95);
  backdrop-filter: blur(4px);
  z-index: 10;
  padding: 16px;
  display: flex;
  flex-direction: column;
  gap: 12px;
}
.edit-group { display: flex; flex-direction: column; gap: 6px; }
.edit-group label { font-size: 11px; color: var(--accent); font-weight: 600; text-transform: uppercase; letter-spacing: 0.05em; }
.edit-input { background: #0b0c10; border: 1px solid #2c3235; padding: 8px; border-radius: 4px; color: var(--text); font-size: 12px; resize: none; outline: none; }
.edit-input:focus { border-color: var(--accent); }
.font-mono { font-family: var(--mono); color: #a5d6ff; }
.edit-actions { margin-top: auto; display: flex; justify-content: flex-end; }
.save-btn { background: var(--accent); border: 1px solid var(--accent); color: white; padding: 6px 12px; border-radius: 4px; font-size: 12px; font-weight: 500; cursor: pointer; }
.save-btn:hover { background: var(--accent2); }


.panel-legend {
  display: flex;
  gap: 12px;
  font-size: 11px;
}
.leg-item { display: flex; align-items: center; gap: 4px; }
.leg-dot { width: 8px; height: 2px; }
.leg-dot.rx { background: var(--teal); }
.leg-dot.tx { background: var(--amber); }

.panel-body {
  flex: 1;
  position: relative;
  overflow: hidden;
  display: flex;
  flex-direction: column;
}
.panel-svg {
  width: 100%;
  height: 100%;
  position: absolute;
  bottom: 0;
  left: 0;
}

/* Stat Blocks */
.stat-big {
  padding: 16px;
  flex: 1;
  display: flex;
  flex-direction: column;
  justify-content: center;
  align-items: center;
}
.stat-big.mt {
  border-top: 1px solid #2c3235;
}
.stat-num {
  font-size: 32px;
  font-weight: 600;
  color: var(--text);
  font-family: var(--mono);
}
.stat-sub {
  font-size: 12px;
  color: var(--text3);
  margin-top: 4px;
}

/* Gauge */
.gauge {
  position: relative;
  width: 180px;
  text-align: center;
}
.gauge-val {
  position: absolute;
  bottom: 15px;
  width: 100%;
  font-size: 28px;
  font-weight: 600;
  color: var(--green);
  font-family: var(--mono);
}
.gauge-lbl {
  position: absolute;
  bottom: -5px;
  width: 100%;
  font-size: 11px;
  color: var(--text3);
}
</style>
