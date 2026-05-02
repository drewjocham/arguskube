<script setup>
import { ref, computed } from 'vue'
import { useToast } from '../../composables/useToast'

const props = defineProps({
  report: { type: Object, default: null },
  loading: { type: Boolean, default: false },
  error: { type: String, default: null },
})

const emit = defineEmits(['run-scan'])
const selectedFinding = ref(null)
const filterSev = ref('all') // 'all', 'error', 'warning', 'info', 'ok'
const searchQuery = ref('')
const { addToast } = useToast()

const showScheduler = ref(false)
const scheduleFreq = ref('none')
const collapsedSections = ref({})

function saveSchedule() {
  showScheduler.value = false
  if (scheduleFreq.value !== 'none') {
    addToast(`Audit scan scheduled: ${scheduleFreq.value}`)
  } else {
    addToast('Audit scan schedule disabled.')
  }
}

const filteredFindings = computed(() => {
  if (!props.report?.findings) return []
  let findings = props.report.findings

  if (filterSev.value !== 'all') {
    findings = findings.filter(f => f.severity === filterSev.value)
  }

  if (searchQuery.value) {
    const q = searchQuery.value.toLowerCase()
    findings = findings.filter(f =>
      f.name.toLowerCase().includes(q) ||
      f.resource.toLowerCase().includes(q) ||
      f.message.toLowerCase().includes(q) ||
      f.namespace.toLowerCase().includes(q)
    )
  }

  // Sort: errors first, then warnings, info, ok.
  return findings.sort((a, b) => b.sevLevel - a.sevLevel)
})

const sevCounts = computed(() => {
  if (!props.report) return { error: 0, warning: 0, info: 0, ok: 0 }
  return {
    error: props.report.totalError,
    warning: props.report.totalWarn,
    info: props.report.totalInfo,
    ok: props.report.totalOk,
  }
})

function gradeColor(grade) {
  if (!grade) return 'var(--text3)'
  switch (grade[0]) {
    case 'A': return 'var(--green)'
    case 'B': return 'var(--green2)'
    case 'C': return 'var(--amber)'
    case 'D': return 'var(--amber2)'
    default: return 'var(--red)'
  }
}

function sevColor(sev) {
  switch (sev) {
    case 'error': return 'var(--red)'
    case 'warning': return 'var(--amber)'
    case 'info': return 'var(--accent)'
    default: return 'var(--green)'
  }
}

function sevBg(sev) {
  switch (sev) {
    case 'error': return 'rgba(240,84,84,0.1)'
    case 'warning': return 'rgba(245,166,35,0.1)'
    case 'info': return 'rgba(79,142,247,0.08)'
    default: return 'rgba(62,207,142,0.06)'
  }
}

function toggleSection(key) {
  collapsedSections.value[key] = !collapsedSections.value[key]
}

function isSectionOpen(key) {
  return collapsedSections.value[key] !== true
}

function fixWithAgent(finding) {
  addToast(`The AI Agent has been prompted to resolve: "${finding.name}".\n\nIt will execute: ${finding.command || 'Auto-generated remediation script'}`, 6000)
}
</script>

<template>
  <div class="popeye-view">
    <!-- Header bar -->
    <div class="popeye-header">
      <div class="popeye-title">
        <div class="popeye-icon schedule-btn" @click="showScheduler = !showScheduler" title="Schedule Scans">
          <svg width="14" height="14" viewBox="0 0 14 14" fill="none">
            <circle cx="7" cy="7" r="5.5" stroke="currentColor" stroke-width="1.5"/>
            <path d="M7 4v3l2 1" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/>
          </svg>
          
          <div v-if="showScheduler" class="schedule-popover" @click.stop>
            <div class="sched-title">Schedule Scans</div>
            <select class="sched-select" v-model="scheduleFreq">
              <option value="none">Disabled</option>
              <option value="hourly">Every Hour</option>
              <option value="daily">Every Day</option>
              <option value="weekly">Every Week</option>
            </select>
            <button class="sched-save" @click="saveSchedule">Save</button>
          </div>
        </div>
        Cluster Audit
      </div>

      <button class="scan-btn" :class="{ scanning: loading }" @click="emit('run-scan')" :disabled="loading">
        <span v-if="loading" class="spinner"></span>
        {{ loading ? 'Scanning...' : 'Run Scan' }}
      </button>
    </div>

    <!-- Score card -->
    <div v-if="report" class="score-strip">
      <div class="score-grade" :style="{ color: gradeColor(report.grade) }">
        {{ report.grade }}
      </div>
      <div class="score-detail">
        <div class="score-number">Score: {{ report.score }}%</div>
        <div class="score-meta">
          {{ report.findings?.length || 0 }} findings · {{ report.scanTimeMs }}ms
        </div>
      </div>
      <div class="sev-pills">
        <div class="sev-pill" :class="{ active: filterSev === 'all' }" @click="filterSev = 'all'">
          All
        </div>
        <div class="sev-pill sev-err" :class="{ active: filterSev === 'error' }" @click="filterSev = 'error'">
          {{ sevCounts.error }} errors
        </div>
        <div class="sev-pill sev-warn" :class="{ active: filterSev === 'warning' }" @click="filterSev = 'warning'">
          {{ sevCounts.warning }} warnings
        </div>
        <div class="sev-pill sev-info" :class="{ active: filterSev === 'info' }" @click="filterSev = 'info'">
          {{ sevCounts.info }} info
        </div>
        <div class="sev-pill sev-ok" :class="{ active: filterSev === 'ok' }" @click="filterSev = 'ok'">
          {{ sevCounts.ok }} ok
        </div>
      </div>
    </div>

    <!-- Search -->
    <div v-if="report" class="search-bar">
      <input
        v-model="searchQuery"
        class="search-input"
        placeholder="Filter findings..."
        type="text"
      />
    </div>

    <!-- Error state -->
    <div v-if="error" class="popeye-error">
      {{ error }}
    </div>

    <!-- Empty state -->
    <div v-if="!report && !loading && !error" class="popeye-empty">
      <div class="empty-icon">
        <svg width="32" height="32" viewBox="0 0 32 32" fill="none">
          <circle cx="16" cy="16" r="12" stroke="var(--text3)" stroke-width="1.5" stroke-dasharray="3 3"/>
          <path d="M16 11v6M16 21h0" stroke="var(--text3)" stroke-width="2" stroke-linecap="round"/>
        </svg>
      </div>
      <div class="empty-text">Run an Argus Scan to audit your cluster</div>
      <div class="empty-sub">Checks resources, security, best practices</div>
    </div>

    <!-- Findings list + detail split -->
    <div v-if="report && !error" class="findings-split">
      <!-- Findings list -->
      <div class="findings-list">
        <div
          v-for="finding in filteredFindings"
          :key="finding.id"
          class="finding-item"
          :class="{ selected: selectedFinding?.id === finding.id }"
          @click="selectedFinding = finding"
        >
          <div class="finding-sev" :style="{ background: sevColor(finding.severity) }"></div>
          <div class="finding-body">
            <div class="finding-name">{{ finding.name }}</div>
            <div class="finding-meta">
              <span class="finding-resource">{{ finding.resource }}</span>
              <span v-if="finding.namespace" class="finding-ns">{{ finding.namespace }}</span>
            </div>
            <div class="finding-msg">{{ finding.message }}</div>
          </div>
        </div>

        <div v-if="filteredFindings.length === 0" class="no-findings">
          No findings match the current filter
        </div>
      </div>

      <!-- Finding detail -->
      <div class="finding-detail" v-if="selectedFinding">
        <div class="detail-header">
          <div class="detail-sev-badge" :style="{ background: sevBg(selectedFinding.severity), color: sevColor(selectedFinding.severity) }">
            {{ selectedFinding.severity.toUpperCase() }}
          </div>
          <div class="detail-title">{{ selectedFinding.name }}</div>
        </div>

        <div class="detail-meta-row">
          <span class="detail-chip">{{ selectedFinding.resource }}</span>
          <span v-if="selectedFinding.namespace" class="detail-chip">{{ selectedFinding.namespace }}</span>
        </div>

        <div class="detail-section">
          <div class="detail-label collapsible" @click="toggleSection('finding')">
            <svg class="section-chevron" :class="{ rotated: !isSectionOpen('finding') }" width="8" height="8" viewBox="0 0 8 8"><polyline points="2 2 4 6 6 2" fill="none" stroke="currentColor" stroke-width="1.2" stroke-linecap="round" stroke-linejoin="round"/></svg>
            Finding
          </div>
          <div v-show="isSectionOpen('finding')" class="detail-text">{{ selectedFinding.message }}</div>
        </div>

        <div class="detail-section">
          <div class="detail-label collapsible" @click="toggleSection('explanation')">
            <svg class="section-chevron" :class="{ rotated: !isSectionOpen('explanation') }" width="8" height="8" viewBox="0 0 8 8"><polyline points="2 2 4 6 6 2" fill="none" stroke="currentColor" stroke-width="1.2" stroke-linecap="round" stroke-linejoin="round"/></svg>
            What This Means
          </div>
          <div v-show="isSectionOpen('explanation')" class="detail-text">{{ selectedFinding.explanation }}</div>
        </div>

        <div class="detail-section">
          <div class="detail-label collapsible" @click="toggleSection('fix')">
            <svg class="section-chevron" :class="{ rotated: !isSectionOpen('fix') }" width="8" height="8" viewBox="0 0 8 8"><polyline points="2 2 4 6 6 2" fill="none" stroke="currentColor" stroke-width="1.2" stroke-linecap="round" stroke-linejoin="round"/></svg>
            How to Fix
          </div>
          <div v-show="isSectionOpen('fix')" class="detail-text">{{ selectedFinding.fix }}</div>
        </div>

        <div v-if="selectedFinding.command" class="detail-section">
          <div class="detail-label collapsible" @click="toggleSection('command')">
            <svg class="section-chevron" :class="{ rotated: !isSectionOpen('command') }" width="8" height="8" viewBox="0 0 8 8"><polyline points="2 2 4 6 6 2" fill="none" stroke="currentColor" stroke-width="1.2" stroke-linecap="round" stroke-linejoin="round"/></svg>
            Command
          </div>
          <div v-show="isSectionOpen('command')" class="detail-cmd">{{ selectedFinding.command }}</div>
        </div>
        
        <div class="detail-section" style="margin-top: 24px;">
          <button class="agent-fix-btn" @click="fixWithAgent(selectedFinding)">
            Ask Argus
          </button>
        </div>
      </div>

      <div v-else class="finding-detail empty-detail">
        Select a finding to see details
      </div>
    </div>
  </div>
</template>

<style scoped>
.popeye-view { display: flex; flex-direction: column; gap: 10px; height: 100%; }

.popeye-header {
  display: flex; align-items: center; justify-content: space-between;
}
.popeye-title {
  display: flex; align-items: center; gap: 7px;
  font-size: 11px; font-weight: 600; letter-spacing: 0.06em;
  text-transform: uppercase; color: var(--text3);
}
.popeye-icon { color: var(--accent2); display: flex; }

.schedule-btn {
  position: relative;
  cursor: pointer;
  transition: color 0.15s;
}
.schedule-btn:hover {
  color: var(--accent);
}
.schedule-popover {
  position: absolute;
  top: 24px; left: 0;
  background: var(--bg3);
  border: 1px solid var(--border);
  border-radius: 6px;
  padding: 12px;
  box-shadow: 0 4px 12px rgba(0,0,0,0.3);
  z-index: 50;
  width: 180px;
  display: flex;
  flex-direction: column;
  gap: 10px;
  cursor: default;
  text-transform: none;
  letter-spacing: normal;
}
.sched-title {
  font-size: 11px;
  font-weight: 600;
  color: var(--text);
  text-transform: uppercase;
  letter-spacing: 0.05em;
}
.sched-select {
  background: var(--bg2);
  border: 1px solid var(--border);
  color: var(--text);
  padding: 6px;
  border-radius: 4px;
  font-size: 12px;
  outline: none;
}
.sched-save {
  background: var(--accent);
  color: white;
  border: none;
  padding: 6px 12px;
  border-radius: 4px;
  font-size: 12px;
  font-weight: 500;
  cursor: pointer;
}
.sched-save:hover { background: var(--accent2); }

.scan-btn {
  padding: 5px 14px; border-radius: 6px; font-size: 12px; font-weight: 500;
  cursor: pointer; border: 1px solid rgba(79,142,247,0.35);
  background: rgba(79,142,247,0.12); color: var(--accent2);
  transition: all 0.15s; display: flex; align-items: center; gap: 6px;
  font-family: var(--font);
}
.scan-btn:hover:not(:disabled) { background: rgba(79,142,247,0.22); border-color: rgba(79,142,247,0.5); }
.scan-btn:disabled { opacity: 0.6; cursor: wait; }
.scan-btn .spinner {
  width: 12px; height: 12px; border: 2px solid transparent;
  border-top-color: var(--accent2); border-radius: 50%; animation: spin 0.8s linear infinite;
}

.score-strip {
  display: flex; align-items: center; gap: 12px; padding: 10px 14px;
  background: var(--bg3); border: 1px solid var(--border); border-radius: var(--r2);
}
.score-grade { font-size: 28px; font-weight: 600; font-family: var(--mono); line-height: 1; }
.score-detail { flex: 1; }
.score-number { font-size: 13px; font-weight: 500; color: var(--text); }
.score-meta { font-size: 11px; color: var(--text3); }

.sev-pills { display: flex; gap: 4px; margin-left: auto; }
.sev-pill {
  padding: 3px 8px; border-radius: 10px; font-size: 10.5px; font-weight: 500;
  font-family: var(--mono); cursor: pointer; border: 1px solid var(--border);
  background: var(--bg3); color: var(--text2); transition: all 0.1s;
}
.sev-pill:hover { background: var(--bg4); }
.sev-pill.active { border-color: var(--accent); color: var(--accent2); background: rgba(79,142,247,0.1); }
.sev-err.active { border-color: var(--red); color: var(--red2); background: rgba(240,84,84,0.1); }
.sev-warn.active { border-color: var(--amber); color: var(--amber2); background: rgba(245,166,35,0.1); }
.sev-info.active { border-color: var(--accent); color: var(--accent2); background: rgba(79,142,247,0.08); }
.sev-ok.active { border-color: var(--green); color: var(--green2); background: rgba(62,207,142,0.06); }

.search-bar { display: flex; }
.search-input {
  flex: 1; padding: 6px 10px; background: var(--bg3); border: 1px solid var(--border);
  border-radius: var(--r2); color: var(--text); font-family: var(--font); font-size: 12px;
  outline: none; transition: border-color 0.15s;
}
.search-input:focus { border-color: rgba(79,142,247,0.5); }
.search-input::placeholder { color: var(--text3); }

.popeye-error { color: var(--red2); font-size: 13px; padding: 20px; text-align: center; }
.popeye-empty {
  display: flex; flex-direction: column; align-items: center; justify-content: center;
  padding: 60px 20px; gap: 8px;
}
.empty-icon { opacity: 0.5; }
.empty-text { font-size: 14px; color: var(--text2); }
.empty-sub { font-size: 12px; color: var(--text3); }

.findings-split { display: flex; gap: 10px; flex: 1; min-height: 0; overflow: hidden; }

.findings-list {
  width: 50%; overflow-y: auto; display: flex; flex-direction: column; gap: 4px;
}

.finding-item {
  display: flex; gap: 10px; padding: 9px 12px; cursor: pointer;
  background: var(--bg3); border: 1px solid var(--border); border-radius: var(--r2);
  transition: all 0.12s; align-items: flex-start;
}
.finding-item:hover { background: var(--bg4); border-color: var(--border2); }
.finding-item.selected { background: rgba(79,142,247,0.08); border-color: rgba(79,142,247,0.3); }

.finding-sev { width: 3px; border-radius: 2px; align-self: stretch; flex-shrink: 0; }
.finding-body { flex: 1; min-width: 0; }
.finding-name { font-size: 12.5px; font-weight: 500; color: var(--text); white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.finding-meta { display: flex; gap: 5px; margin: 2px 0 4px; }
.finding-resource {
  font-size: 10px; font-family: var(--mono); color: var(--text3);
  padding: 1px 5px; background: var(--bg5); border-radius: 3px;
}
.finding-ns {
  font-size: 10px; font-family: var(--mono); color: var(--accent2);
  padding: 1px 5px; background: rgba(79,142,247,0.08); border-radius: 3px;
}
.finding-msg { font-size: 11.5px; color: var(--text2); line-height: 1.4; display: -webkit-box; -webkit-line-clamp: 2; -webkit-box-orient: vertical; overflow: hidden; }

.no-findings { text-align: center; padding: 30px; color: var(--text3); font-size: 12px; }

.finding-detail {
  width: 50%; overflow-y: auto; padding: 14px;
  background: var(--bg3); border: 1px solid var(--border); border-radius: var(--r2);
}
.finding-detail.empty-detail {
  display: flex; align-items: center; justify-content: center;
  color: var(--text3); font-size: 13px;
}

.detail-header { display: flex; align-items: center; gap: 10px; margin-bottom: 12px; }
.detail-sev-badge {
  padding: 2px 8px; border-radius: 4px; font-size: 10px; font-weight: 600;
  font-family: var(--mono); letter-spacing: 0.05em;
}
.detail-title { font-size: 14px; font-weight: 500; color: var(--text); }

.detail-meta-row { display: flex; gap: 5px; margin-bottom: 14px; }
.detail-chip {
  font-size: 10.5px; font-family: var(--mono); color: var(--text2);
  padding: 2px 7px; background: var(--bg5); border-radius: 4px;
}

.detail-section { margin-bottom: 14px; }
.detail-label {
  font-size: 10px; font-weight: 600; letter-spacing: 0.07em; color: var(--text3);
  text-transform: uppercase; margin-bottom: 5px;
}
.detail-label.collapsible {
  cursor: pointer; display: flex; align-items: center; gap: 5px;
  padding: 3px 0; border-radius: 3px; transition: color 0.15s;
  user-select: none;
}
.detail-label.collapsible:hover { color: var(--text2); }
.section-chevron { flex-shrink: 0; transition: transform 0.15s ease; }
.section-chevron.rotated { transform: rotate(-90deg); }
.detail-text { font-size: 12.5px; color: var(--text2); line-height: 1.65; }

.detail-cmd {
  font-family: var(--mono); font-size: 11px; color: var(--accent2);
  padding: 8px 10px; background: rgba(79,142,247,0.06); border: 1px solid rgba(79,142,247,0.15);
  border-radius: 6px; word-break: break-all; line-height: 1.5; cursor: pointer;
}
.detail-cmd:hover { background: rgba(79,142,247,0.12); }

.agent-fix-btn {
  display: inline-flex; align-items: center; justify-content: center;
  padding: 8px 16px; border-radius: 6px; border: none; cursor: pointer;
  background: linear-gradient(135deg, #a78bfa 0%, #8b5cf6 100%);
  color: white; font-weight: 600; font-size: 13px; font-family: var(--font);
  transition: all 0.2s ease; box-shadow: 0 2px 4px rgba(139, 92, 246, 0.2);
}
.agent-fix-btn:hover {
  transform: translateY(-1px);
  box-shadow: 0 4px 8px rgba(139, 92, 246, 0.4);
}
.agent-fix-btn:active {
  transform: translateY(1px);
}
</style>
