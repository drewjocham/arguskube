<script setup>
import { ref, inject } from 'vue'

const isAllowed = inject('isAllowed')

const runbooks = ref([
  { id: 'rb-1', name: 'OOMKilled Response', trigger: 'OOMKilled alert', steps: 4, lastRun: '2h ago', status: 'ready' },
  { id: 'rb-2', name: 'CrashLoop Triage', trigger: 'CrashLoopBackOff', steps: 5, lastRun: '6h ago', status: 'ready' },
  { id: 'rb-3', name: 'Node Pressure Remediation', trigger: 'DiskPressure / MemoryPressure', steps: 6, lastRun: '1d ago', status: 'ready' },
  { id: 'rb-4', name: 'Deploy Rollback', trigger: 'Manual / Error rate spike', steps: 3, lastRun: 'Never', status: 'draft' },
  { id: 'rb-5', name: 'Certificate Renewal', trigger: 'cert-manager warning', steps: 4, lastRun: '14d ago', status: 'ready' },
])

const selectedRunbook = ref(null)
</script>

<template>
  <div class="runbooks-view">
    <div class="view-header">
      <div class="view-title">Runbooks</div>
      <div class="view-sub">Automated response playbooks for common incidents</div>
    </div>

    <div class="runbooks-grid">
      <div
        v-for="rb in runbooks"
        :key="rb.id"
        class="runbook-card"
        :class="{ selected: selectedRunbook?.id === rb.id, locked: !isAllowed('custom_runbooks') && rb.status === 'draft' }"
        @click="selectedRunbook = rb"
      >
        <div class="rb-status" :class="'rb-' + rb.status"></div>
        <div class="rb-body">
          <div class="rb-name">{{ rb.name }}</div>
          <div class="rb-trigger">Trigger: {{ rb.trigger }}</div>
          <div class="rb-meta">
            <span>{{ rb.steps }} steps</span>
            <span class="rb-dot">·</span>
            <span>Last: {{ rb.lastRun }}</span>
          </div>
        </div>
        <div v-if="rb.status === 'draft'" class="rb-badge">DRAFT</div>
        <div v-if="!isAllowed('custom_runbooks') && rb.status === 'draft'" class="rb-pro-badge">PRO</div>
      </div>
    </div>

    <div v-if="runbooks.length === 0" class="empty-state">
      No runbooks configured yet
    </div>
  </div>
</template>

<style scoped>
.runbooks-view { display: flex; flex-direction: column; gap: 12px; }

.view-header { margin-bottom: 4px; }
.view-title { font-size: 14px; font-weight: 500; color: var(--text); margin-bottom: 2px; }
.view-sub { font-size: 12px; color: var(--text3); }

.runbooks-grid { display: flex; flex-direction: column; gap: 6px; }

.runbook-card {
  display: flex; gap: 10px; padding: 12px 14px; cursor: pointer;
  background: var(--bg3); border: 1px solid var(--border); border-radius: var(--r);
  transition: all 0.12s; align-items: flex-start; position: relative;
}
.runbook-card:hover { background: var(--bg4); border-color: var(--border2); }
.runbook-card.selected { background: rgba(79,142,247,0.08); border-color: rgba(79,142,247,0.3); }
.runbook-card.locked { opacity: 0.5; }

.rb-status { width: 3px; border-radius: 2px; align-self: stretch; flex-shrink: 0; }
.rb-ready { background: var(--green); }
.rb-draft { background: var(--text3); }

.rb-body { flex: 1; min-width: 0; }
.rb-name { font-size: 13px; font-weight: 500; color: var(--text); margin-bottom: 3px; }
.rb-trigger { font-size: 11.5px; color: var(--text2); margin-bottom: 4px; }
.rb-meta { font-size: 10.5px; color: var(--text3); display: flex; gap: 3px; }
.rb-dot { opacity: 0.5; }

.rb-badge {
  font-size: 9px; font-weight: 600; font-family: var(--mono);
  padding: 1px 5px; border-radius: 3px;
  background: var(--bg5); color: var(--text3); letter-spacing: 0.05em;
}
.rb-pro-badge {
  position: absolute; top: 4px; right: 4px;
  font-size: 8px; font-weight: 600; font-family: var(--mono);
  padding: 1px 5px; border-radius: 3px;
  background: rgba(167,139,250,0.2); color: var(--purple); letter-spacing: 0.05em;
}

.empty-state { text-align: center; padding: 40px; color: var(--text3); font-size: 13px; }
</style>
