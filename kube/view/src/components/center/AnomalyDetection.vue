<script setup>
import { ref, computed, onMounted, watch } from 'vue'
import { callGo, useAnomaly } from '../../composables/useWails'
import { useBackgroundTasks } from '../../composables/useBackgroundTasks'

const viewMode = ref('dashboard') // 'dashboard' or 'management'
const {
  anomalies: agentAnomalies,
  settings: backendSettings,
  rules: backendRules,
  jobs: backendJobs,
  loading: agentLoading,
  error: agentError,
  connectAgent,
  getSettings,
  saveSettings,
  getRules,
  saveRule,
  toggleRule: backendToggleRule,
  deleteRule: backendDeleteRule,
  getJobs,
} = useAnomaly()

// Live namespaces drive the Target Scope select instead of a hard-coded list.
const namespaces = ref([])
async function loadNamespaces() {
  try {
    const res = await callGo('ListAllNamespaces')
    namespaces.value = Array.isArray(res) ? res : []
  } catch (e) {
    console.warn('[anomaly] failed to load namespaces:', e)
    namespaces.value = []
  }
}

// Inspect drawer state — shows the raw anomaly + metadata for a row.
const inspectTarget = ref(null)
function openInspect(an) { inspectTarget.value = an }
function closeInspect() { inspectTarget.value = null }

// New-rule modal state.
const showRuleModal = ref(false)
const ruleSaving = ref(false)
const ruleForm = ref({
  id: '',
  name: '',
  severity: 'warning',
  condition: '',
  enabled: true,
})
function openNewRule() {
  ruleForm.value = { id: '', name: '', severity: 'warning', condition: '', enabled: true }
  showRuleModal.value = true
}
async function submitNewRule() {
  const { name } = ruleForm.value
  if (!name.trim()) return
  ruleSaving.value = true
  try {
    await saveRule({
      id: 'rule-' + Date.now().toString(36),
      name: ruleForm.value.name.trim(),
      severity: ruleForm.value.severity,
      condition: ruleForm.value.condition.trim(),
      enabled: ruleForm.value.enabled,
    })
    showRuleModal.value = false
  } catch (e) {
    alert('Failed to save rule: ' + (e?.message || e))
  } finally {
    ruleSaving.value = false
  }
}
async function onDeleteRule(rule) {
  if (!confirm(`Delete rule "${rule.name}"?`)) return
  try {
    await backendDeleteRule(rule.id)
  } catch (e) {
    alert('Delete failed: ' + (e?.message || e))
  }
}

// Acknowledge / Resolve route the anomaly through the existing Incident
// store: an acknowledged anomaly becomes an open incident, a resolved one
// is created with status=resolved. The incident becomes the durable record.
const incidentForAlert = ref({}) // alertId → incident.id
async function acknowledgeAlert(alert) {
  try {
    const incident = await callGo(
      'CreateIncident',
      alert.title,                     // title
      String(alert.sev || 'warning').toLowerCase(),
      'anomaly',                       // type
      `Acknowledged from anomaly dashboard at ${new Date().toLocaleString()}`,
      backendSettings.value?.targetScope || 'all',
    )
    if (incident?.id) incidentForAlert.value[alert.id] = incident.id
  } catch (e) {
    alert('Acknowledge failed: ' + (e?.message || e))
  }
}
async function resolveAlert(alert) {
  // If we acknowledged it earlier, transition the incident to resolved.
  // Otherwise create a fresh resolved incident so the action still leaves
  // a paper trail.
  const existing = incidentForAlert.value[alert.id]
  try {
    if (existing) {
      await callGo('UpdateIncident', existing, 'resolved', `Resolved at ${new Date().toLocaleString()}`)
    } else {
      const incident = await callGo(
        'CreateIncident',
        alert.title,
        String(alert.sev || 'warning').toLowerCase(),
        'anomaly',
        `Resolved without acknowledge at ${new Date().toLocaleString()}`,
        backendSettings.value?.targetScope || 'all',
      )
      if (incident?.id) incidentForAlert.value[alert.id] = incident.id
    }
  } catch (e) {
    alert('Resolve failed: ' + (e?.message || e))
  }
}

// Persist anomaly data across navigation.
const { getTask, startTask, completeTask, failTask } = useBackgroundTasks()
const ANOMALY_KEY = 'anomaly-detection'

// Local UI state — synced from backend on mount.
const sensitivitySlider = ref(30)
const baselineWindow = ref(7)
const metricType = ref('cpu')
const algorithm = ref('smart')
const sensitivitySelect = ref('high')
const threshold = ref(85)
const targetNamespace = ref('all')
const settingsSaved = ref(false)

// Sync local refs from backend settings when they arrive.
watch(backendSettings, (s) => {
  if (!s) return
  sensitivitySlider.value = s.sensitivity
  baselineWindow.value = s.baselineWindow
  metricType.value = s.metricType
  algorithm.value = s.algorithm
  threshold.value = s.threshold
  targetNamespace.value = s.targetScope
  // Derive sensitivity select from numeric slider.
  if (s.sensitivity < 33) sensitivitySelect.value = 'low'
  else if (s.sensitivity < 66) sensitivitySelect.value = 'medium'
  else sensitivitySelect.value = 'high'
})

onMounted(async () => {
  // Restore previously persisted data so the user sees results instantly.
  const stored = getTask(ANOMALY_KEY)
  if (stored?.status === 'completed') {
    // Restore to agentAnomalies if it's still empty after initial bind.
    // The useAnomaly composable will overwrite with fresh data once it resolves.
  }
  // Load persisted settings, rules, and live anomalies in parallel.
  await Promise.all([
    getSettings(),
    getRules(),
    getJobs(),
    connectAgent('all'),
    loadNamespaces(),
  ])
  // Persist fresh results for next navigation.
  if (agentAnomalies.value && agentAnomalies.value.length > 0) {
    completeTask(ANOMALY_KEY, agentAnomalies.value)
  }
})

// Keep persisted data in sync with live updates.
watch(agentAnomalies, (val) => {
  if (val && val.length > 0) {
    completeTask(ANOMALY_KEY, val)
  }
}, { deep: true })

// Apply settings → persist to backend.
async function applySettings() {
  const s = {
    sensitivity: sensitivitySlider.value,
    baselineWindow: baselineWindow.value,
    metricType: metricType.value,
    algorithm: algorithm.value,
    threshold: threshold.value,
    targetScope: targetNamespace.value,
  }
  await saveSettings(s)
  settingsSaved.value = true
  setTimeout(() => { settingsSaved.value = false }, 2000)
}

// Sync sensitivity select → numeric slider.
watch(sensitivitySelect, (val) => {
  if (val === 'low') sensitivitySlider.value = 15
  else if (val === 'medium') sensitivitySlider.value = 50
  else sensitivitySlider.value = 85
})

// ── Dynamic chart paths for the main dashboard SVG ─────────────
const maxChartPoints = 20

function buildAnomalyChartPath() {
  const anomalies = agentAnomalies.value
  if (!anomalies || anomalies.length === 0) {
    return 'M0 75 L500 75' // flat baseline
  }
  const recent = anomalies.slice(-maxChartPoints)
  const count = recent.length
  if (count < 2) return 'M0 75 L500 75'
  const points = recent.map((a, i) => {
    const x = (i / (count - 1)) * 500
    // score maps to y: 0 → bottom(140), 100 → top(10)
    const y = Math.min(140, Math.max(10, 150 - (a.score / 100) * 140))
    return `${x},${y}`
  })
  return 'M' + points.join(' L')
}

function buildAnomalyBandPath() {
  const base = buildAnomalyChartPath()
  if (base === 'M0 75 L500 75') {
    return 'M0 75 L500 75 L500 105 L0 105 Z'
  }
  const coords = base.replace('M', '').split(' L').map(p => {
    const [x, y] = p.split(',').map(Number)
    return `${x},${Math.min(y + 28, 145)}`
  })
  return base + ' L' + coords.reverse().join(' L') + ' Z'
}

const anomalyChartPath = computed(buildAnomalyChartPath)
const anomalyBandPath = computed(buildAnomalyBandPath)

// ── Chart x-axis labels ─────────────────────────────────────────
const chartLabels = computed(() => {
  const anomalies = agentAnomalies.value
  if (!anomalies || anomalies.length === 0) {
    return ['19:00','18:00','17:00','12:00','13:00','14:00','15:00','16:00']
  }
  const recent = anomalies.slice(-maxChartPoints)
  const step = Math.max(1, Math.floor(recent.length / 8))
  const labels = []
  for (let i = 0; i < recent.length && labels.length < 8; i += step) {
    labels.push(recent[i].timestamp?.slice(11, 16) || '--:--')
  }
  while (labels.length < 8) labels.push('--:--')
  return labels
})

// ── Anomaly pin (only shows when a high-severity spike exists) ──
const anomalyPin = computed(() => {
  const anomalies = agentAnomalies.value
  if (!anomalies) return null
  const spike = anomalies.find(a => a.score > 90)
  if (!spike) return null
  return {
    label: '⚠️ ANOMALY DETECTED',
    time: spike.timestamp ? spike.timestamp.slice(11, 16) + ' ' + spike.timestamp.slice(0, 10) : '',
  }
})

const recentAlerts = computed(() => {
  if (!agentAnomalies.value || agentAnomalies.value.length === 0) return []
  return agentAnomalies.value.map((a, i) => ({
    id: i,
    time: a.timestamp,
    sev: a.score > 90 ? 'Critical' : (a.score > 80 ? 'High' : 'Medium'),
    title: a.rule + ' - ' + a.target
  }))
})

const barChartData = computed(() => {
  // Build from real anomaly data bucketed by score ranges, or fallback to static.
  if (!agentAnomalies.value || agentAnomalies.value.length === 0) {
    return [
      { val: 0, color: 'var(--text3)' },
    ]
  }
  // Group anomalies into 8 time buckets.
  const anoms = agentAnomalies.value
  const bucketCount = Math.min(8, anoms.length)
  const bucketSize = Math.ceil(anoms.length / bucketCount)
  const buckets = []
  for (let i = 0; i < bucketCount; i++) {
    const slice = anoms.slice(i * bucketSize, (i + 1) * bucketSize)
    const avg = slice.reduce((s, a) => s + a.score, 0) / slice.length
    buckets.push({
      val: avg,
      color: avg > 80 ? 'var(--red)' : avg > 50 ? 'var(--amber)' : 'var(--accent)',
    })
  }
  return buckets
})

const detectedAnomalies = computed(() => {
  if (!agentAnomalies.value || agentAnomalies.value.length === 0) return []
  return agentAnomalies.value.map((a, i) => ({
    id: 'an-' + i,
    time: a.timestamp,
    rule: a.rule,
    target: a.target,
    conf: a.score.toFixed(1) + '%'
  }))
})

async function onToggleRule(rule) {
  await backendToggleRule(rule.id)
}

// Compute hour-of-day distribution of recorded anomalies. Returns an array of
// 24 numbers in [0, 1] (relative density). Used for the Seasonality card.
const seasonalityBuckets = computed(() => {
  const out = new Array(24).fill(0)
  const list = agentAnomalies.value || []
  if (!list.length) return out
  for (const a of list) {
    if (!a.timestamp) continue
    const h = parseInt(String(a.timestamp).slice(11, 13), 10)
    if (Number.isFinite(h) && h >= 0 && h < 24) out[h]++
  }
  const max = Math.max(1, ...out)
  return out.map(v => v / max)
})

const seasonalityActualPath = computed(() => {
  const b = seasonalityBuckets.value
  return b.map((v, i) => `${(i / 23) * 200},${100 - v * 80}`).join(' ')
})
const seasonalityExpectedPath = computed(() => {
  // Simple smoothing: each bucket = avg of itself + the two neighbours.
  const b = seasonalityBuckets.value
  const points = b.map((_, i) => {
    const prev = b[(i + 23) % 24], cur = b[i], next = b[(i + 1) % 24]
    const v = (prev + cur + next) / 3
    return `${(i / 23) * 200},${100 - v * 80}`
  })
  return points.join(' ') + ' L200,100 L0,100 Z'
})

// Acknowledged-alerts indicator for the recent-alerts list.
function isAcknowledged(alertId) { return Boolean(incidentForAlert.value[alertId]) }
</script>

<template>
  <div class="anomaly-container">
    
    <!-- View Switcher Tabs -->
    <div class="view-tabs">
      <div class="tab" :class="{ active: viewMode === 'dashboard' }" @click="viewMode = 'dashboard'">
        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" style="margin-right: 6px; vertical-align: middle;">
          <rect x="3" y="3" width="18" height="18" rx="2" ry="2"></rect>
          <line x1="3" y1="9" x2="21" y2="9"></line>
          <line x1="9" y1="21" x2="9" y2="9"></line>
        </svg>
        Dashboard
      </div>
      <div class="tab" :class="{ active: viewMode === 'management' }" @click="viewMode = 'management'">
        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" style="margin-right: 6px; vertical-align: middle;">
          <line x1="4" y1="21" x2="4" y2="14"></line>
          <line x1="4" y1="10" x2="4" y2="3"></line>
          <line x1="12" y1="21" x2="12" y2="12"></line>
          <line x1="12" y1="8" x2="12" y2="3"></line>
          <line x1="20" y1="21" x2="20" y2="16"></line>
          <line x1="20" y1="12" x2="20" y2="3"></line>
          <line x1="1" y1="14" x2="7" y2="14"></line>
          <line x1="9" y1="8" x2="15" y2="8"></line>
          <line x1="17" y1="16" x2="23" y2="16"></line>
        </svg>
        Rules & Configuration
      </div>
    </div>

    <!-- VIEW: Dashboard -->
    <div v-if="viewMode === 'dashboard'" class="anomaly-dashboard">
      <!-- Main Column -->
      <div class="main-col">
        <!-- Top Chart -->
        <div class="dashboard-card main-chart-card">
          <div class="card-header">
            <div class="card-title">System Anomaly Score</div>
            <div class="card-actions">
              <span class="live-badge"><span class="dot"></span> Real Time</span>
            </div>
          </div>
          <div class="chart-container">
            <!-- Y-Axis Labels -->
            <div class="y-axis">
              <span>100</span>
              <span>90</span>
              <span>60</span>
              <span>30</span>
              <span>0</span>
            </div>
            <!-- SVG Chart -->
            <div class="svg-wrapper">
              <svg viewBox="0 0 500 150" preserveAspectRatio="none" class="chart-svg">
                <!-- Grid lines -->
                <line x1="0" y1="0" x2="500" y2="0" class="grid-line" />
                <line x1="0" y1="37.5" x2="500" y2="37.5" class="grid-line" />
                <line x1="0" y1="75" x2="500" y2="75" class="grid-line" />
                <line x1="0" y1="112.5" x2="500" y2="112.5" class="grid-line" />
                <line x1="0" y1="150" x2="500" y2="150" class="grid-line" />
                
                <!-- Expected Band (dynamic from real data) -->
                <path :d="anomalyBandPath" class="band-fill" />
                
                <!-- Actual Line (dynamic from real data) -->
                <path :d="anomalyChartPath" class="actual-line" />
              </svg>

              <!-- Tooltip Pin — only when high-severity anomaly exists -->
              <div v-if="anomalyPin" class="anomaly-pin">
                <div class="pin-box">{{ anomalyPin.label }} — {{ anomalyPin.time }}</div>
                <div class="pin-arrow"></div>
              </div>

              <!-- X-Axis Labels -->
              <div class="x-axis">
                <span v-for="(lbl, i) in chartLabels" :key="i">{{ lbl }}</span>
              </div>
            </div>
          </div>
        </div>

        <!-- Bottom Row -->
        <div class="bottom-row">
          <!-- Parameters -->
          <div class="dashboard-card param-card">
            <div class="card-header"><div class="card-title">Anomaly Parameters</div></div>
            <div class="card-body">
              <div class="mini-chart-title">{{ backendRules.length > 0 ? backendRules.length + ' Detection Rules' : 'Detection Rules' }}</div>
              <div class="mini-chart-svg">
                <svg viewBox="0 0 200 50" preserveAspectRatio="none">
                  <path d="M0 40 Q20 30 40 40 T80 20 T120 40 L140 40 L160 10 L180 30 L200 30 L200 50 L0 50 Z" class="mini-band" />
                  <path d="M0 40 Q20 30 40 40 T80 20 T120 40 L140 40 L160 10 L180 30 L200 30" class="mini-line" />
                </svg>
              </div>
              
              <div class="slider-group">
                <label>Sensitivity (Low-High)</label>
                <div class="slider-row">
                  <span>Low</span>
                  <input type="range" min="0" max="100" v-model="sensitivitySlider" class="custom-slider" />
                  <span>High</span>
                </div>
              </div>
              <div class="slider-group">
                <label>Baseline Window ({{ baselineWindow }} days)</label>
                <div class="slider-row">
                  <input type="range" min="1" max="30" v-model="baselineWindow" class="custom-slider" />
                </div>
              </div>

              <div class="dropdown-row">
                <div class="dd-group">
                  <label>Metric Type</label>
                  <select v-model="metricType" class="custom-select">
                    <option value="cpu">CPU Utilization</option>
                    <option value="mem">Memory Usage</option>
                  </select>
                </div>
                <div class="dd-group">
                  <label>Algorithm</label>
                  <select v-model="algorithm" class="custom-select">
                    <option value="smart">Smart Baseline</option>
                    <option value="fixed">Fixed Threshold</option>
                  </select>
                </div>
              </div>
            </div>
          </div>

          <!-- Seasonality (derived from anomaly history, hour-of-day buckets) -->
          <div class="dashboard-card seasonal-card">
            <div class="card-header">
              <div class="card-title">Seasonality (by hour, UTC)</div>
              <div v-if="backendJobs.length" class="card-actions">
                <span class="job-badge">{{ backendJobs.length }} jobs</span>
              </div>
            </div>
            <div class="card-body">
              <div v-if="!agentAnomalies.length" class="empty-state" style="margin: auto;">
                Waiting for anomaly history…
              </div>
              <template v-else>
                <div class="legend">
                  <span class="l-item"><span class="l-dot expected"></span> Smoothed</span>
                  <span class="l-item"><span class="l-dot actual"></span> Hourly density</span>
                </div>
                <div class="season-chart">
                  <svg viewBox="0 0 200 100" preserveAspectRatio="none">
                    <polygon :points="seasonalityExpectedPath" class="season-expected" />
                    <polyline :points="seasonalityActualPath" class="season-actual" />
                  </svg>
                </div>
                <div class="season-x-axis">
                  <span>00:00</span><span>06:00</span><span>12:00</span><span>18:00</span><span>23:00</span>
                </div>
              </template>
            </div>
          </div>
        </div>
      </div>

      <!-- Right Column -->
      <div class="side-col">
        <div class="dashboard-card alert-mgmt-card">
          <div class="card-header"><div class="card-title">Alert Management</div></div>
          <div class="card-body">
            <div class="sub-title">Active Alerts</div>
            
            <template v-if="recentAlerts.length > 0">
              <div class="active-alert-card">
                <div class="aa-header">
                  <span class="aa-icon">🚩</span>
                  <span class="aa-id">#A{{ recentAlerts[0].id }}</span>
                  <span class="aa-sev">{{ recentAlerts[0].sev.toUpperCase() }} SEVERITY</span>
                </div>
                <div class="aa-title">{{ recentAlerts[0].title }}</div>
                <div class="aa-status">STATUS: <span class="status-open">OPEN</span></div>
                <div class="aa-actions">
                  <button class="btn-ack" @click="acknowledgeAlert(recentAlerts[0])">
                    {{ incidentForAlert[recentAlerts[0].id] ? 'ACKNOWLEDGED' : 'ACKNOWLEDGE' }}
                  </button>
                  <button class="btn-res" @click="resolveAlert(recentAlerts[0])">RESOLVE</button>
                </div>
              </div>
            </template>
            <div v-else class="empty-state">No active anomaly alerts</div>

            <div class="sub-title mt-16">RECENT ALERTS (Last 24h)</div>
            <div class="timeline">
              <div class="tl-item" v-for="al in recentAlerts" :key="al.id">
                <div class="tl-dot" :class="al.sev === 'Medium' ? 'dot-amber' : 'dot-blue'"></div>
                <div class="tl-time">
                  {{ al.time?.slice(11, 16) || '--:--' }}
                  <br/>
                  <span class="tl-sub">{{ al.time?.slice(0, 10) || '' }}</span>
                </div>
                <div class="tl-sev" :class="al.sev === 'Medium' ? 'sev-amber' : 'sev-blue'">{{ al.sev }}</div>
                <div class="tl-desc">{{ al.title }}</div>
              </div>
            </div>
          </div>
        </div>

        <div class="dashboard-card alerts-time-card">
          <div class="card-header"><div class="card-title">Alerts Over Time</div></div>
          <div class="card-body">
            <div class="bar-chart-container">
              <div class="y-axis-bars">
                <span>80</span><span>60</span><span>40</span><span>20</span><span>0</span>
              </div>
              <div class="bars">
                <div v-for="(b, i) in barChartData" :key="i" class="bar" :style="{ height: b.val + '%', background: b.color }"></div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- Inspect drawer — shows the raw anomaly record for the selected row -->
    <div v-if="inspectTarget" class="anomaly-modal-backdrop" @click.self="closeInspect">
      <div class="anomaly-modal">
        <div class="modal-header">
          <div class="modal-title">Anomaly detail</div>
          <button class="modal-close" @click="closeInspect">×</button>
        </div>
        <div class="modal-body">
          <div class="kv"><span class="k">Time</span><span class="v mono">{{ inspectTarget.time || '—' }}</span></div>
          <div class="kv"><span class="k">Rule</span><span class="v">{{ inspectTarget.rule || '—' }}</span></div>
          <div class="kv"><span class="k">Target</span><span class="v mono">{{ inspectTarget.target || '—' }}</span></div>
          <div class="kv"><span class="k">Confidence</span><span class="v">{{ inspectTarget.conf || '—' }}</span></div>
          <pre class="raw-json">{{ JSON.stringify(inspectTarget, null, 2) }}</pre>
        </div>
        <div class="modal-footer">
          <button class="view-btn" @click="closeInspect">Close</button>
        </div>
      </div>
    </div>

    <!-- New-rule modal -->
    <div v-if="showRuleModal" class="anomaly-modal-backdrop" @click.self="showRuleModal = false">
      <div class="anomaly-modal">
        <div class="modal-header">
          <div class="modal-title">New detection rule</div>
          <button class="modal-close" @click="showRuleModal = false">×</button>
        </div>
        <div class="modal-body form-body">
          <label class="rule-field">
            <span>Name</span>
            <input v-model="ruleForm.name" placeholder="e.g. CPU spike on api pods" class="ctrl-input" />
          </label>
          <label class="rule-field">
            <span>Severity</span>
            <select v-model="ruleForm.severity" class="ctrl-input">
              <option value="info">Info</option>
              <option value="warning">Warning</option>
              <option value="critical">Critical</option>
            </select>
          </label>
          <label class="rule-field">
            <span>Condition (free-form)</span>
            <input v-model="ruleForm.condition" placeholder="e.g. cpu_pct &gt; 90 for 5m" class="ctrl-input mono" />
          </label>
          <label class="rule-field-inline">
            <input type="checkbox" v-model="ruleForm.enabled" /> Enabled on save
          </label>
        </div>
        <div class="modal-footer">
          <button class="view-btn" @click="showRuleModal = false" :disabled="ruleSaving">Cancel</button>
          <button class="apply-btn" @click="submitNewRule" :disabled="ruleSaving || !ruleForm.name.trim()">
            {{ ruleSaving ? 'Saving…' : 'Save rule' }}
          </button>
        </div>
      </div>
    </div>

    <!-- VIEW: Rules & Configuration (Old Layout) -->
    <div v-if="viewMode === 'management'" class="anomaly-view">
      <!-- Top Header & Config -->
      <div class="config-section">
        <div class="config-header">
          <div class="header-title">
            <div class="pulse-dot"></div>
            AI Anomaly Detection
          </div>
          <div class="header-sub">Configure ML parameters and manage detection rules across your cluster.</div>
        </div>
        
        <div class="config-controls">
          <div class="control-group">
            <label>Sensitivity</label>
            <select v-model="sensitivitySelect" class="ctrl-input">
              <option value="low">Low (Fewer False Positives)</option>
              <option value="medium">Medium</option>
              <option value="high">High (Maximum Detection)</option>
            </select>
          </div>
          
          <div class="control-group">
            <label>Confidence Threshold: {{ threshold }}%</label>
            <input type="range" min="50" max="99" v-model="threshold" class="ctrl-slider" />
          </div>
          
          <div class="control-group">
            <label>Target Scope</label>
            <select v-model="targetNamespace" class="ctrl-input">
              <option value="all">All Namespaces</option>
              <option v-for="ns in namespaces" :key="ns" :value="ns">{{ ns }}</option>
            </select>
          </div>

          <button class="apply-btn" @click="applySettings">{{ settingsSaved ? '✓ Saved' : 'Apply Settings' }}</button>
        </div>
      </div>

      <div class="main-content">
        <!-- Left: Rules -->
        <div class="rules-panel">
          <div class="panel-header">
            Detection Rules
            <button class="add-rule-btn" @click="openNewRule">+ New Rule</button>
          </div>
          <div class="rules-list">
            <div v-if="!backendRules.length" class="empty-state" style="margin: 12px 0;">
              No rules yet. Click <strong>+ New Rule</strong> to add one.
            </div>
            <div v-for="rule in backendRules" :key="rule.id" class="rule-card" :class="{ disabled: !rule.enabled }">
              <div class="rule-info">
                <div class="rule-name">{{ rule.name }}</div>
                <div class="rule-sev" :class="'sev-' + rule.severity">{{ rule.severity }}</div>
                <div v-if="rule.condition" class="rule-cond mono">{{ rule.condition }}</div>
              </div>
              <div class="rule-actions">
                <div class="toggle-switch" :class="{ active: rule.enabled }" @click="onToggleRule(rule)" title="Toggle">
                  <div class="toggle-knob"></div>
                </div>
                <button class="rule-del" @click="onDeleteRule(rule)" title="Delete">×</button>
              </div>
            </div>
          </div>
        </div>

        <!-- Right: Detected Anomalies -->
        <div class="detected-panel">
          <div class="panel-header">
            Detected Anomalies
          </div>
          <div class="table-container">
            <table class="anomaly-table">
              <thead>
                <tr>
                  <th>Time</th>
                  <th>Triggered Rule</th>
                  <th>Target Resource</th>
                  <th>Confidence</th>
                  <th>Action</th>
                </tr>
              </thead>
              <tbody>
                <tr v-if="agentLoading">
                  <td colspan="5" class="dim" style="text-align:center;padding:24px;">Loading anomaly data...</td>
                </tr>
                <tr v-else-if="detectedAnomalies.length === 0">
                  <td colspan="5" class="dim" style="text-align:center;padding:24px;">No anomalies detected</td>
                </tr>
                <tr v-for="an in detectedAnomalies" :key="an.id">
                  <td class="dim">{{ an.time }}</td>
                  <td>{{ an.rule }}</td>
                  <td class="mono target-col">{{ an.target }}</td>
                  <td><span class="conf-badge">{{ an.conf }}</span></td>
                  <td><button class="view-btn" @click="openInspect(an)">Inspect</button></td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>
      </div>
    </div>

  </div>
</template>

<style scoped>
.anomaly-container {
  display: flex;
  flex-direction: column;
  height: 100%;
  overflow: hidden;
  background: var(--bg);
}

/* Tabs Switcher */
.view-tabs {
  display: flex;
  background: var(--bg2);
  border-bottom: 1px solid var(--border);
  padding: 0 16px;
  gap: 16px;
  flex-shrink: 0;
}
.tab {
  padding: 12px 0;
  font-size: 12.5px;
  font-weight: 500;
  color: var(--text2);
  cursor: pointer;
  border-bottom: 2px solid transparent;
  transition: all 0.15s;
}
.tab:hover { color: var(--text); }
.tab.active { color: var(--accent2); border-bottom-color: var(--accent); }


/* ---- DASHBOARD VIEW STYLES ---- */
.anomaly-dashboard {
  flex: 1;
  display: flex;
  gap: 16px;
  padding: 16px;
  overflow-y: auto;
}
.main-col { flex: 1; display: flex; flex-direction: column; gap: 16px; min-width: 0; }
.side-col { width: 320px; display: flex; flex-direction: column; gap: 16px; flex-shrink: 0; }

.dashboard-card {
  background: var(--bg2); border: 1px solid var(--border); border-radius: var(--r);
  display: flex; flex-direction: column;
}
.card-header {
  padding: 14px 16px; border-bottom: 1px solid var(--border);
  display: flex; justify-content: space-between; align-items: center;
}
.card-title { font-size: 14px; font-weight: 600; color: var(--text); }
.card-body { padding: 16px; flex: 1; display: flex; flex-direction: column; }

/* Top Chart */
.main-chart-card { flex: 1; min-height: 280px; }
.live-badge {
  font-size: 11px; font-weight: 600; color: var(--green2);
  background: rgba(62,207,142,0.1); padding: 4px 10px; border-radius: 4px; border: 1px solid rgba(62,207,142,0.2);
  display: flex; align-items: center; gap: 6px;
}
.live-badge .dot { width: 6px; height: 6px; border-radius: 50%; background: var(--green2); animation: pulse 2s infinite; }

.chart-container { display: flex; flex: 1; padding: 16px; gap: 12px; }
.y-axis { display: flex; flex-direction: column; justify-content: space-between; color: var(--text3); font-size: 11px; font-family: var(--mono); padding-bottom: 24px; }
.svg-wrapper { flex: 1; position: relative; display: flex; flex-direction: column; }
.chart-svg { width: 100%; flex: 1; overflow: visible; }

.grid-line { stroke: var(--border); stroke-width: 1; }
.band-fill { fill: rgba(79,142,247,0.1); stroke: rgba(79,142,247,0.3); stroke-width: 1; }
.actual-line { fill: none; stroke: var(--accent); stroke-width: 2; }
.spike-fill { fill: rgba(240,84,84,0.15); }
.spike-line { fill: none; stroke: var(--red); stroke-width: 2; }

.anomaly-pin { position: absolute; left: 54%; top: 10px; transform: translateX(-50%); display: flex; flex-direction: column; align-items: center; }
.pin-box { background: var(--red); color: white; padding: 4px 10px; border-radius: 4px; font-size: 10.5px; font-weight: 600; letter-spacing: 0.05em; }
.pin-arrow { width: 0; height: 0; border-left: 6px solid transparent; border-right: 6px solid transparent; border-top: 6px solid var(--red); margin-top: -1px; }

.x-axis { display: flex; justify-content: space-between; color: var(--text3); font-size: 11px; font-family: var(--mono); margin-top: 8px; }

/* Bottom Row */
.bottom-row { display: flex; gap: 16px; height: 280px; }
.param-card, .seasonal-card { flex: 1; }

.mini-chart-title { font-size: 12px; font-weight: 600; color: var(--text); margin-bottom: 8px; }
.mini-chart-svg { height: 50px; margin-bottom: 16px; }
.mini-band { fill: rgba(79,142,247,0.1); stroke: rgba(79,142,247,0.3); stroke-width: 1; }
.mini-line { fill: none; stroke: var(--accent); stroke-width: 1.5; }
.mini-spike-fill { fill: rgba(240,84,84,0.15); }
.mini-spike-line { fill: none; stroke: var(--red); stroke-width: 1.5; }

.slider-group { margin-bottom: 12px; }
.slider-group label { display: block; font-size: 11px; color: var(--text2); margin-bottom: 4px; font-weight: 500; }
.slider-row { display: flex; align-items: center; gap: 8px; font-size: 10px; color: var(--text3); }
.custom-slider { flex: 1; accent-color: var(--accent); height: 4px; border-radius: 2px; }

.dropdown-row { display: flex; gap: 12px; margin-top: auto; }
.dd-group { flex: 1; display: flex; flex-direction: column; gap: 4px; }
.dd-group label { font-size: 11px; color: var(--text2); font-weight: 500; }
.custom-select { background: var(--bg3); border: 1px solid var(--border); color: var(--text); padding: 6px; border-radius: 4px; font-size: 11px; outline: none; }

/* Seasonality */
.legend { display: flex; gap: 16px; justify-content: center; margin-bottom: 12px; }
.l-item { font-size: 11px; color: var(--text2); display: flex; align-items: center; gap: 6px; }
.l-dot { width: 8px; height: 8px; border-radius: 50%; }
.l-dot.expected { background: rgba(255,255,255,0.2); }
.l-dot.actual { background: var(--accent); }

.season-chart { height: 120px; margin-bottom: 16px; flex: 1; }
.season-expected { fill: rgba(255,255,255,0.05); stroke: rgba(255,255,255,0.1); stroke-width: 1; }
.season-actual { fill: none; stroke: var(--accent); stroke-width: 1.5; }

/* Alert Management */
.alert-mgmt-card { flex: 1; }
.sub-title { font-size: 12px; font-weight: 600; color: var(--text); margin-bottom: 10px; }
.mt-16 { margin-top: 24px; text-transform: uppercase; font-size: 11px; letter-spacing: 0.05em; color: var(--text3); }

.active-alert-card { background: rgba(240,84,84,0.08); border: 1px solid rgba(240,84,84,0.2); border-radius: 6px; padding: 12px; }
.aa-header { display: flex; align-items: center; gap: 8px; margin-bottom: 8px; }
.aa-id { font-family: var(--mono); font-size: 11px; color: var(--text); font-weight: 600; }
.aa-sev { background: rgba(240,84,84,0.15); color: var(--red2); padding: 2px 6px; border-radius: 4px; font-size: 10px; font-weight: 600; }
.aa-title { font-size: 13px; font-weight: 500; color: var(--text); margin-bottom: 4px; }
.aa-status { font-size: 11px; color: var(--text2); margin-bottom: 12px; }
.status-open { color: var(--red2); font-weight: 600; }

.aa-actions { display: flex; gap: 8px; }
.aa-actions button { flex: 1; padding: 6px; border-radius: 4px; font-size: 11px; font-weight: 600; cursor: pointer; transition: all 0.15s; }
.btn-ack { background: transparent; border: 1px solid var(--accent); color: var(--accent2); }
.btn-ack:hover { background: rgba(79,142,247,0.1); }
.btn-res { background: var(--accent); border: 1px solid var(--accent); color: white; }
.btn-res:hover { background: var(--accent2); }

/* Timeline */
.timeline { display: flex; flex-direction: column; gap: 0; position: relative; }
.timeline::before { content: ''; position: absolute; left: 4px; top: 8px; bottom: 8px; width: 1px; background: var(--border); }
.tl-item { display: flex; gap: 12px; align-items: flex-start; padding: 8px 0; position: relative; }
.tl-dot { width: 9px; height: 9px; border-radius: 50%; background: var(--bg2); border: 2px solid var(--text3); position: relative; z-index: 2; margin-top: 3px; }
.dot-amber { border-color: var(--amber); }
.dot-blue { border-color: var(--accent); }
.tl-time { font-size: 11px; color: var(--text2); font-family: var(--mono); width: 45px; flex-shrink: 0; }
.tl-sub { color: var(--text3); font-size: 10px; }
.tl-sev { padding: 2px 6px; border-radius: 4px; font-size: 10px; font-weight: 600; flex-shrink: 0; margin-top: 1px; }
.sev-amber { background: rgba(245,166,35,0.15); color: var(--amber2); }
.sev-blue { background: rgba(79,142,247,0.15); color: var(--accent2); }
.tl-desc { font-size: 12px; color: var(--text); flex: 1; padding-top: 2px; }

/* Alerts Over Time */
.alerts-time-card { height: 220px; }
.bar-chart-container { display: flex; gap: 12px; height: 100%; padding-top: 8px; }
.y-axis-bars { display: flex; flex-direction: column; justify-content: space-between; font-size: 10.5px; color: var(--text3); font-family: var(--mono); padding-bottom: 20px; }
.bars { flex: 1; display: flex; align-items: flex-end; justify-content: space-between; border-bottom: 1px solid var(--border); padding-bottom: 0; height: calc(100% - 20px); }
.bar { width: 10%; border-radius: 2px 2px 0 0; transition: height 0.3s; }


/* ---- MANAGEMENT VIEW STYLES ---- */
.anomaly-view {
  flex: 1; display: flex; flex-direction: column;
}

.config-section { padding: 16px 20px; background: var(--bg2); border-bottom: 1px solid var(--border); }
.config-header { margin-bottom: 16px; }
.header-title { font-size: 14px; font-weight: 500; color: var(--text); display: flex; align-items: center; gap: 8px; }
.pulse-dot { width: 8px; height: 8px; border-radius: 50%; background: var(--purple); box-shadow: 0 0 8px var(--purple); animation: pulse 2s infinite; }
.header-sub { font-size: 12px; color: var(--text3); margin-top: 4px; margin-left: 16px; }

.config-controls { display: flex; gap: 20px; align-items: flex-end; background: var(--bg3); padding: 12px 16px; border-radius: 8px; border: 1px solid var(--border); }
.control-group { display: flex; flex-direction: column; gap: 6px; flex: 1; }
.control-group label { font-size: 11px; font-weight: 600; color: var(--text2); text-transform: uppercase; letter-spacing: 0.05em; }
.ctrl-input { background: var(--bg); border: 1px solid var(--border); color: var(--text); padding: 6px 10px; border-radius: 4px; font-size: 12px; outline: none; }
.ctrl-input:focus { border-color: var(--accent); }
.ctrl-slider { width: 100%; accent-color: var(--purple); margin-top: 4px; }
.apply-btn { background: rgba(167,139,250,0.15); border: 1px solid rgba(167,139,250,0.3); color: var(--purple); padding: 7px 16px; border-radius: 4px; font-size: 12px; font-weight: 500; cursor: pointer; transition: all 0.2s; height: 31px; }
.apply-btn:hover { background: rgba(167,139,250,0.25); }

.main-content { flex: 1; display: flex; overflow: hidden; }

.rules-panel { width: 320px; background: var(--bg2); border-right: 1px solid var(--border); display: flex; flex-direction: column; }
.panel-header { padding: 12px 16px; font-size: 13px; font-weight: 500; color: var(--text); border-bottom: 1px solid var(--border); display: flex; justify-content: space-between; align-items: center; background: var(--bg3); }
.add-rule-btn { background: transparent; border: 1px dashed var(--border2); color: var(--text2); border-radius: 4px; padding: 3px 8px; font-size: 10px; cursor: pointer; }
.add-rule-btn:hover { color: var(--accent2); border-color: var(--accent); }

.rules-list { flex: 1; overflow-y: auto; padding: 12px; display: flex; flex-direction: column; gap: 8px; }
.rule-card { background: var(--bg3); border: 1px solid var(--border); border-radius: 6px; padding: 12px; display: flex; justify-content: space-between; align-items: center; transition: all 0.2s; }
.rule-card.disabled { opacity: 0.5; }
.rule-name { font-size: 12.5px; color: var(--text); font-weight: 500; margin-bottom: 4px; }
.rule-sev { font-size: 10px; font-weight: 600; text-transform: uppercase; letter-spacing: 0.05em; }
.sev-critical { color: var(--red2); }
.sev-warning { color: var(--amber2); }

.toggle-switch { width: 36px; height: 20px; background: var(--bg5); border-radius: 10px; position: relative; cursor: pointer; transition: background 0.2s; }
.toggle-switch.active { background: var(--green); }
.toggle-knob { width: 16px; height: 16px; background: white; border-radius: 50%; position: absolute; top: 2px; left: 2px; transition: transform 0.2s; box-shadow: 0 1px 3px rgba(0,0,0,0.3); }
.toggle-switch.active .toggle-knob { transform: translateX(16px); }

.detected-panel { flex: 1; display: flex; flex-direction: column; background: var(--bg); }
.table-container { flex: 1; overflow-y: auto; padding: 16px; }
.anomaly-table { width: 100%; border-collapse: collapse; font-size: 12.5px; }
.anomaly-table th { text-align: left; padding: 8px 12px; color: var(--text3); font-weight: 500; border-bottom: 1px solid var(--border); }
.anomaly-table td { padding: 12px; border-bottom: 1px solid var(--border); color: var(--text); }
.conf-badge { background: rgba(167,139,250,0.15); color: var(--purple); padding: 2px 6px; border-radius: 4px; font-size: 11px; font-weight: 600; }
.view-btn { background: var(--bg3); border: 1px solid var(--border); color: var(--text2); padding: 4px 10px; border-radius: 4px; font-size: 11px; cursor: pointer; transition: all 0.15s; }
.view-btn:hover { background: var(--bg4); color: var(--text); }

/* Empty state */
.empty-state {
  font-size: 12px;
  color: var(--text3);
  text-align: center;
  padding: 24px 12px;
  background: var(--bg3);
  border-radius: 6px;
  border: 1px dashed var(--border);
}

/* Saved flash */
.apply-btn.saved {
  background: rgba(62,207,142,0.15);
  border-color: rgba(62,207,142,0.3);
  color: var(--green2);
}

/* Rule card extras */
.rule-info { display: flex; flex-direction: column; gap: 3px; min-width: 0; }
.rule-cond { font-size: 11px; color: var(--text3); margin-top: 2px; word-break: break-all; }
.rule-actions { display: flex; align-items: center; gap: 8px; }
.rule-del {
  background: transparent; border: 1px solid var(--border); color: var(--text3);
  width: 22px; height: 22px; border-radius: 4px; cursor: pointer; font-size: 14px;
  line-height: 1; padding: 0;
}
.rule-del:hover { color: var(--red2); border-color: var(--red); }

/* Job badge */
.job-badge {
  background: rgba(167,139,250,0.15); color: var(--purple);
  padding: 2px 8px; border-radius: 10px; font-size: 10.5px; font-weight: 600;
}

/* Seasonality x-axis */
.season-x-axis {
  display: flex; justify-content: space-between;
  font-size: 10.5px; color: var(--text3); font-family: var(--mono);
}

/* Modal */
.anomaly-modal-backdrop {
  position: fixed; inset: 0; z-index: 100;
  background: rgba(0,0,0,0.5);
  display: flex; align-items: center; justify-content: center;
  padding: 24px;
}
.anomaly-modal {
  background: var(--bg2); border: 1px solid var(--border); border-radius: 8px;
  width: min(560px, 100%); max-height: 88vh;
  display: flex; flex-direction: column;
  box-shadow: 0 16px 40px rgba(0,0,0,0.45);
}
.modal-header {
  display: flex; justify-content: space-between; align-items: center;
  padding: 12px 16px; border-bottom: 1px solid var(--border);
}
.modal-title { font-size: 13.5px; font-weight: 600; color: var(--text); }
.modal-close {
  background: transparent; border: 0; color: var(--text2);
  width: 24px; height: 24px; cursor: pointer; font-size: 18px;
}
.modal-close:hover { color: var(--text); }
.modal-body { padding: 14px 18px; overflow: auto; }
.modal-footer {
  display: flex; gap: 8px; justify-content: flex-end;
  padding: 10px 16px; border-top: 1px solid var(--border);
}
.kv { display: flex; gap: 12px; font-size: 12.5px; padding: 4px 0; }
.kv .k { width: 110px; color: var(--text3); }
.kv .v { color: var(--text); }
.kv .v.mono { font-family: var(--mono); }
.raw-json {
  margin-top: 10px; padding: 10px; background: var(--bg);
  border: 1px solid var(--border); border-radius: 4px;
  color: var(--text2); font-family: var(--mono); font-size: 11px;
  overflow: auto; max-height: 320px; line-height: 1.45;
}
.form-body { display: flex; flex-direction: column; gap: 12px; }
.rule-field { display: flex; flex-direction: column; gap: 4px; font-size: 12px; color: var(--text2); }
.rule-field-inline { display: flex; align-items: center; gap: 6px; font-size: 12.5px; color: var(--text2); }
</style>
