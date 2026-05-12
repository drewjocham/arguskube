<script setup>
import { ref, watch } from 'vue'
import { useResources } from '../../composables/useWails'

const props = defineProps({
  kind: { type: String, default: '' },
  namespace: { type: String, default: '' },
  name: { type: String, default: '' },
})

const emit = defineEmits(['close'])

const { detail, detailLoading, getResourceDetail } = useResources()
const activeTab = ref('properties')

watch([() => props.kind, () => props.namespace, () => props.name], () => {
  if (props.kind && props.name) {
    activeTab.value = 'properties'
    getResourceDetail(props.kind, props.namespace, props.name)
  }
}, { immediate: true })

const tabs = [
  { id: 'properties', label: 'Properties' },
  { id: 'labels', label: 'Labels' },
  { id: 'data', label: 'Data' },
  { id: 'conditions', label: 'Conditions' },
  { id: 'events', label: 'Events' },
]

function hasTab(id) {
  if (!detail.value) return false
  switch (id) {
    case 'properties': return true
    case 'labels': return detail.value.labels && Object.keys(detail.value.labels).length > 0
    case 'data': return detail.value.data && Object.keys(detail.value.data).length > 0
    case 'conditions': return detail.value.conditions && detail.value.conditions.length > 0
    case 'events': return detail.value.events && detail.value.events.length > 0
    default: return false
  }
}

function labelCount() {
  if (!detail.value) return 0
  return Object.keys(detail.value.labels || {}).length + Object.keys(detail.value.annotations || {}).length
}
</script>

<template>
  <div class="detail-panel" v-if="name">
    <!-- Header -->
    <div class="detail-header">
      <div class="detail-title">
        <span class="detail-kind">{{ detail?.kind || kind }}</span>
        <span class="detail-name">{{ name }}</span>
      </div>
      <button class="close-btn" @click="emit('close')">
        <svg width="12" height="12" viewBox="0 0 12 12">
          <path d="M3 3l6 6M9 3l-6 6" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/>
        </svg>
      </button>
    </div>

    <!-- Loading -->
    <div v-if="detailLoading" class="detail-loading">Loading…</div>

    <!-- Content -->
    <template v-else-if="detail">
      <!-- Tabs -->
      <div class="detail-tabs">
        <div
          v-for="tab in tabs.filter(t => hasTab(t.id))"
          :key="tab.id"
          class="detail-tab"
          :class="{ active: activeTab === tab.id }"
          @click="activeTab = tab.id"
        >{{ tab.label }}</div>
      </div>

      <div class="detail-content">
        <!-- Properties -->
        <div v-if="activeTab === 'properties'" class="prop-list">
          <div class="prop-meta">
            <div class="prop-row">
              <span class="prop-key">Created</span>
              <span class="prop-val">{{ detail.created }}</span>
            </div>
            <div class="prop-row" v-if="detail.namespace">
              <span class="prop-key">Namespace</span>
              <span class="prop-val ns-link">{{ detail.namespace }}</span>
            </div>
          </div>

          <div class="prop-row" v-for="prop in detail.properties" :key="prop.key">
            <span class="prop-key">{{ prop.key }}</span>
            <span class="prop-val">{{ prop.value }}</span>
          </div>
        </div>

        <!-- Labels + Annotations -->
        <div v-if="activeTab === 'labels'" class="prop-list">
          <div v-if="detail.labels && Object.keys(detail.labels).length > 0" class="label-section">
            <div class="label-section-title">Labels</div>
            <div class="label-grid">
              <div v-for="(v, k) in detail.labels" :key="k" class="label-chip">
                <span class="label-key">{{ k }}</span>
                <span class="label-sep">=</span>
                <span class="label-value">{{ v }}</span>
              </div>
            </div>
          </div>

          <div v-if="detail.annotations && Object.keys(detail.annotations).length > 0" class="label-section">
            <div class="label-section-title">Annotations</div>
            <div class="label-grid">
              <div v-for="(v, k) in detail.annotations" :key="k" class="label-chip annotation">
                <span class="label-key">{{ k }}</span>
                <span class="label-sep">=</span>
                <span class="label-value">{{ v }}</span>
              </div>
            </div>
          </div>
        </div>

        <!-- Data (ConfigMaps/Secrets) -->
        <div v-if="activeTab === 'data'" class="data-section">
          <div v-for="(v, k) in detail.data" :key="k" class="data-entry">
            <div class="data-filename">{{ k }}</div>
            <pre class="data-content">{{ v }}</pre>
          </div>
        </div>

        <!-- Conditions -->
        <div v-if="activeTab === 'conditions'" class="conditions-table">
          <table class="mini-table">
            <thead>
              <tr>
                <th>Type</th>
                <th>Status</th>
                <th>Reason</th>
                <th>Message</th>
                <th>Age</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="(cond, i) in detail.conditions" :key="i">
                <td class="mono">{{ cond.type }}</td>
                <td>
                  <span class="cond-status" :class="cond.status === 'True' ? 'cond-true' : 'cond-false'">
                    {{ cond.status }}
                  </span>
                </td>
                <td class="mono dim">{{ cond.reason }}</td>
                <td class="dim">{{ cond.message }}</td>
                <td class="mono dim">{{ cond.age }}</td>
              </tr>
            </tbody>
          </table>
        </div>

        <!-- Events -->
        <div v-if="activeTab === 'events'" class="events-section">
          <div v-if="detail.events.length === 0" class="empty-msg">No events found</div>
          <table v-else class="mini-table">
            <thead>
              <tr>
                <th>Type</th>
                <th>Reason</th>
                <th>Message</th>
                <th>Count</th>
                <th>Age</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="(ev, i) in detail.events" :key="i" :class="{ 'warning-row': ev.type === 'Warning' }">
                <td>
                  <span class="ev-type" :class="ev.type === 'Warning' ? 'ev-warn' : 'ev-normal'">{{ ev.type }}</span>
                </td>
                <td class="mono">{{ ev.reason }}</td>
                <td>{{ ev.message }}</td>
                <td class="mono center">{{ ev.count }}</td>
                <td class="mono dim">{{ ev.age }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>
    </template>
  </div>
</template>

<style scoped>
.detail-panel {
  width: 380px;
  background: var(--bg2);
  border-left: 1px solid var(--border);
  display: flex;
  flex-direction: column;
  overflow: hidden;
  flex-shrink: 0;
}

.detail-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 10px 14px;
  border-bottom: 1px solid var(--border);
  flex-shrink: 0;
}

.detail-title {
  display: flex;
  align-items: baseline;
  gap: 6px;
  min-width: 0;
}

.detail-kind {
  font-size: 10px;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  color: var(--text3);
  flex-shrink: 0;
}

.detail-name {
  font-size: 13px;
  font-weight: 500;
  color: var(--text);
  font-family: var(--mono);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.close-btn {
  background: none;
  border: none;
  color: var(--text3);
  cursor: pointer;
  padding: 4px;
  border-radius: 4px;
  transition: all 0.15s;
  display: flex;
}
.close-btn:hover { background: var(--bg4); color: var(--text); }

.detail-loading {
  padding: 40px;
  text-align: center;
  color: var(--text3);
}

/* Tabs */
.detail-tabs {
  display: flex;
  gap: 0;
  border-bottom: 1px solid var(--border);
  flex-shrink: 0;
  padding: 0 10px;
}

.detail-tab {
  padding: 7px 12px;
  font-size: 11px;
  font-weight: 500;
  color: var(--text3);
  cursor: pointer;
  border-bottom: 2px solid transparent;
  transition: all 0.15s;
}
.detail-tab:hover { color: var(--text2); }
.detail-tab.active { color: var(--accent2); border-bottom-color: var(--accent); }

.detail-content {
  flex: 1;
  overflow-y: auto;
  padding: 12px 14px;
}

/* Properties */
.prop-list {
  display: flex;
  flex-direction: column;
  gap: 0;
}

.prop-meta {
  margin-bottom: 8px;
  padding-bottom: 8px;
  border-bottom: 1px solid var(--border);
}

.prop-row {
  display: flex;
  justify-content: space-between;
  align-items: baseline;
  padding: 4px 0;
  gap: 12px;
}

.prop-key {
  font-size: 11px;
  color: var(--text3);
  flex-shrink: 0;
}

.prop-val {
  font-size: 11.5px;
  color: var(--text);
  font-family: var(--mono);
  text-align: right;
  word-break: break-all;
}

.ns-link { color: var(--accent2); }

/* Labels */
.label-section { margin-bottom: 14px; }
.label-section-title {
  font-size: 10px;
  font-weight: 600;
  color: var(--text3);
  text-transform: uppercase;
  letter-spacing: 0.05em;
  margin-bottom: 6px;
}

.label-grid {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
}

.label-chip {
  display: inline-flex;
  align-items: center;
  background: var(--bg4);
  border-radius: 4px;
  padding: 2px 6px;
  font-size: 10px;
  font-family: var(--mono);
  max-width: 100%;
  overflow: hidden;
}

.label-key { color: var(--accent2); }
.label-sep { color: var(--text3); margin: 0 2px; }
.label-value { color: var(--text2); overflow: hidden; text-overflow: ellipsis; }

.label-chip.annotation { background: rgba(167,139,250,0.08); }
.label-chip.annotation .label-key { color: var(--purple); }

/* Data */
.data-entry { margin-bottom: 12px; }
.data-filename {
  font-size: 11px;
  font-weight: 600;
  color: var(--text2);
  font-family: var(--mono);
  margin-bottom: 4px;
}

.data-content {
  background: var(--bg);
  border: 1px solid var(--border);
  border-radius: 6px;
  padding: 8px 10px;
  font-size: 11px;
  font-family: var(--mono);
  color: var(--text2);
  line-height: 1.5;
  overflow-x: auto;
  max-height: 200px;
  overflow-y: auto;
  white-space: pre-wrap;
  word-break: break-all;
}

/* Mini tables */
.mini-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 11px;
}

.mini-table th {
  background: var(--bg3);
  color: var(--text3);
  font-weight: 500;
  font-size: 10px;
  text-transform: uppercase;
  letter-spacing: 0.04em;
  padding: 5px 8px;
  text-align: left;
  border-bottom: 1px solid var(--border);
  cursor: default;
}

.mini-table td {
  padding: 4px 8px;
  border-bottom: 1px solid var(--border);
  color: var(--text2);
}

.mono { font-family: var(--mono); }
.dim { color: var(--text3); }
.center { text-align: center; }

.cond-status { font-family: var(--mono); font-weight: 500; }
.cond-true { color: var(--green); }
.cond-false { color: var(--red2); }

.ev-type { font-size: 10px; font-weight: 500; padding: 1px 5px; border-radius: 3px; }
.ev-normal { background: rgba(79,142,247,0.1); color: var(--accent2); }
.ev-warn { background: rgba(245,166,35,0.1); color: var(--amber2); }

.warning-row { background: rgba(245,166,35,0.03); }

.empty-msg { color: var(--text3); text-align: center; padding: 20px; }
</style>
