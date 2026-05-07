<script setup>
import { ref, computed, onMounted, nextTick, watch } from 'vue'
import { useResources } from '../../composables/useResources'
import { callGo } from '../../composables/useBridge'

const props = defineProps({
  type: { type: String, default: 'configmaps' }
})

const { result, detail, loading, detailLoading, listResources, getResourceDetail } = useResources()

const resourceKind = ref(props.type || 'configmaps')

const configmaps = ref([])
const cmDetail = ref(null)
const expandedCm = ref(null)
const notification = ref(null)
const viewRef = ref(null)

// AI schooling state
const schoolLoading = ref(false)
const schoolingResource = ref(null)
const schoolResult = ref(null)

// Dynamic title/subtitle based on type
const title = computed(() => {
  return props.type === 'secrets' ? 'Secrets' : 'Config Maps'
})

const subtitle = computed(() => {
  return props.type === 'secrets'
    ? 'Sensitive data stored as key-value pairs (values obfuscated)'
    : 'Non-confidential data stored in key-value pairs'
})

async function fetchData(kind) {
  await listResources(kind, '')
  if (result.value && result.value.items && result.value.items.length > 0) {
    configmaps.value = result.value.items.map(item => ({
      name: item.name,
      namespace: item.namespace,
      data: item.fields?.data || '0',
      age: item.age || '—'
    }))
  } else {
    configmaps.value = []
  }
}

onMounted(async () => {
  await fetchData(resourceKind.value)
})

// Watch for type prop changes and re-fetch data, reset expanded state
watch(() => props.type, async (newType) => {
  resourceKind.value = newType || 'configmaps'
  expandedCm.value = null
  cmDetail.value = null
  schoolResult.value = null
  schoolingResource.value = null
  await fetchData(resourceKind.value)
})

async function toggleExpand(cmName) {
  if (expandedCm.value === cmName) {
    expandedCm.value = null
    cmDetail.value = null
    schoolResult.value = null
    schoolingResource.value = null
    return
  }

  expandedCm.value = cmName
  schoolResult.value = null
  schoolingResource.value = null
  const cm = configmaps.value.find(c => c.name === cmName)
  if (cm) {
    await getResourceDetail(resourceKind.value, cm.namespace, cmName)
    if (detail.value) {
      cmDetail.value = detail.value
    }
  }
  await nextTick()
  scrollExpandedIntoView()
}

async function schoolMe(cmName) {
  if (schoolLoading.value) return
  schoolingResource.value = cmName
  schoolResult.value = null
  schoolLoading.value = true
  try {
    const cm = configmaps.value.find(c => c.name === cmName)
    if (!cm) return
    // Build a brief context string and send to AI backend
    const kind = props.type === 'secrets' ? 'Secret' : 'ConfigMap'
    const labelContext = cmDetail.value?.labels
      ? Object.entries(cmDetail.value.labels).map(([k, v]) => `${k}=${v}`).join(', ')
      : 'none'
    const dataKeys = cmDetail.value?.data
      ? Object.keys(cmDetail.value.data).join(', ')
      : 'none'
    const context = `${kind} "${cmName}" in namespace "${cm.namespace}"\nLabels: ${labelContext}\nKeys: ${dataKeys}`
    const response = await callGo('SendChatMessage', '', `Teach me about this ${kind}: ${context}`)
    schoolResult.value = response || `I can explain ${kind} "${cmName}". It lives in the ${cm.namespace} namespace with ${cm.data} data fields.`
  } catch (e) {
    schoolResult.value = `Could not reach AI agent: ${e?.message || 'unknown error'}`
  } finally {
    schoolLoading.value = false
  }
}

function scrollExpandedIntoView() {
  const el = viewRef.value?.querySelector('.cm-expanded')
  if (el && typeof el.scrollIntoView === 'function') {
    el.scrollIntoView({ block: 'nearest', behavior: 'smooth' })
  }
}
</script>

<template>
  <div class="cm-view" ref="viewRef">
    <div class="header">
      <div class="title">{{ title }}</div>
      <div class="subtitle">{{ subtitle }}</div>
    </div>
    
    <div v-if="notification" class="agent-notification">
      <div class="notif-icon">
        <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M12 2v20M17 5H9.5a3.5 3.5 0 0 0 0 7h5a3.5 3.5 0 0 1 0 7H6"></path></svg>
      </div>
      <div class="notif-text">{{ notification }}</div>
    </div>

    <div class="scroll-area">
    <div class="cm-list">
      <div class="cm-header-row">
        <div class="col-name">Name</div>
        <div class="col-ns">Namespace</div>
        <div class="col-data">Data Keys</div>
        <div class="col-age">Age</div>
      </div>

      <div v-for="cm in configmaps" :key="cm.namespace + '/' + cm.name" class="cm-row-container" :class="{'ai-active-pulse': cm.isApplying || schoolingResource === cm.name}">
        <div class="cm-row" @click="toggleExpand(cm.name)">
          <div class="col-name">
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" style="color: #6ba3f9; margin-right: 8px;"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"></path><polyline points="14 2 14 8 20 8"></polyline><line x1="16" y1="13" x2="8" y2="13"></line><line x1="16" y1="17" x2="8" y2="17"></line><polyline points="10 9 9 9 8 9"></polyline></svg>
            {{ cm.name }}
          </div>
          <div class="col-ns font-mono">{{ cm.namespace }}</div>
          <div class="col-data font-mono">{{ cm.data }} keys</div>
          <div class="col-age font-mono" style="display:flex; justify-content:space-between; align-items:center;">
            {{ cm.age }}
            <svg class="chevron" :class="{ open: expandedCm === cm.name }" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <polyline points="6 9 12 15 18 9"></polyline>
            </svg>
          </div>
        </div>

        <!-- Expanded View -->
        <div class="cm-expanded" v-if="expandedCm === cm.name">
          <div v-if="detailLoading" style="color:#8b8f96; font-size:13px; padding:12px;">Loading…</div>
          <div v-else-if="cmDetail" class="expanded-grid">
            <div class="expanded-card">
              <div class="card-title-row">
                <h4 class="card-title">Data Keys</h4>
                <button class="school-btn" @click.stop="schoolMe(cm.name)" :disabled="schoolLoading && schoolingResource === cm.name" title="Ask the AI agent to school you about this resource">
                  <svg v-if="schoolLoading && schoolingResource === cm.name" class="spin" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="10" stroke-dasharray="31.4 31.4" stroke-dashoffset="0"/></svg>
                  <svg v-else width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M12 6.5V2a10 10 0 0 1 10 10h-4.5M6.5 12H2A10 10 0 0 1 12 2v4.5M12 17.5V22A10 10 0 0 1 2 12h4.5M17.5 12H22A10 10 0 0 1 12 22v-4.5"/></svg>
                  School me
                </button>
              </div>
              <div v-if="schoolingResource === cm.name && schoolResult" class="school-result">
                <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" class="school-result-icon"><path d="M12 2L2 7l10 5 10-5-10-5zM2 17l10 5 10-5M2 12l10 5 10-5"/></svg>
                <span>{{ schoolResult }}</span>
              </div>
              <div v-if="cmDetail.data && Object.keys(cmDetail.data).length" class="cm-data-grid">
                <div class="cm-data-row" v-for="(v, k) in cmDetail.data" :key="cm.name + '-' + k">
                  <span class="cm-data-key font-mono">{{ k }}</span>
                  <pre class="cm-data-val font-mono">{{ props.type === 'secrets' ? '•••••••• (obfuscated)' : (v.length > 200 ? v.substring(0, 200) + '…' : v) }}</pre>
                </div>
              </div>
              <div v-else class="props-grid">
                <div class="prop-row" v-for="prop in cmDetail.properties" :key="prop.key">
                  <span class="prop-label">{{ prop.key }}</span>
                  <span class="prop-value font-mono">{{ prop.value }}</span>
                </div>
              </div>
            </div>

            <div class="expanded-card" v-if="cmDetail.labels && Object.keys(cmDetail.labels).length">
              <h4 class="card-title">Labels</h4>
              <div class="labels-grid">
                <span class="label-chip" v-for="(v, k) in cmDetail.labels" :key="cm.name + '-label-' + k">{{ k }}={{ v }}</span>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
    </div>
  </div>
</template>

<style scoped>
.cm-view { padding: 24px; display: flex; flex-direction: column; gap: 24px; min-height: 0; flex: 1; box-sizing: border-box; }
.scroll-area { flex: 1; overflow-y: auto; min-height: 0; }
.header .title { font-size: 20px; font-weight: 500; color: #fff; margin-bottom: 4px; }
.header .subtitle { font-size: 13px; color: #8b8f96; }

.cm-list { background: #1e2023; border: 1px solid rgba(255, 255, 255, 0.08); border-radius: 8px; overflow: hidden; }

.cm-header-row {
  display: grid;
  grid-template-columns: 2fr 1fr 100px 100px;
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

.cm-row-container {
  border-bottom: 1px solid rgba(255, 255, 255, 0.04);
  transition: all 0.3s ease;
}
.cm-row-container:last-child { border-bottom: none; }

.cm-expanded { padding: 16px; background: #141517; border-top: 1px dashed rgba(255,255,255,0.08); }

.cm-row {
  display: grid;
  grid-template-columns: 2fr 1fr 100px 100px;
  gap: 16px;
  padding: 14px 16px;
  font-size: 13px;
  color: #e8eaec;
  align-items: center;
  cursor: pointer;
  transition: background 0.2s;
}
.cm-row:hover { background: rgba(255, 255, 255, 0.02); }

.col-name { display: flex; align-items: center; font-weight: 500; }
.font-mono { font-family: var(--mono); color: #b0b4ba; font-size: 12px; }

.chevron { transition: transform 0.2s ease; color: #6b7078; }
.chevron.open { transform: rotate(180deg); }

/* Pulse Animation */
@keyframes pulse-glow {
  0% { box-shadow: inset 0 0 0px rgba(167, 139, 250, 0); background: transparent; }
  50% { box-shadow: inset 0 0 10px rgba(167, 139, 250, 0.4); background: rgba(167, 139, 250, 0.05); }
  100% { box-shadow: inset 0 0 0px rgba(167, 139, 250, 0); background: transparent; }
}
.ai-active-pulse {
  animation: pulse-glow 2s infinite;
  border-left: 3px solid #a78bfa;
}

/* Expanded Area — height is fine, handled by parent scroll */
.expanded-grid {
  display: flex;
  flex-direction: column;
}
.expanded-card {
  background: #1e2023;
  border: 1px solid rgba(255, 255, 255, 0.05);
  border-radius: 6px;
  padding: 16px;
  display: flex;
  flex-direction: column;
}
.card-title { font-size: 13px; font-weight: 600; color: #fff; margin: 0; }
.card-title-row { display: flex; justify-content: space-between; align-items: center; margin-bottom: 12px; }

/* School button */
.school-btn {
  display: inline-flex; align-items: center; gap: 5px;
  background: rgba(167, 139, 250, 0.12); border: 1px solid rgba(167, 139, 250, 0.25);
  color: #a78bfa; font-size: 11px; font-weight: 500;
  padding: 4px 10px; border-radius: 6px; cursor: pointer;
  transition: all 0.15s; white-space: nowrap;
}
.school-btn:hover { background: rgba(167, 139, 250, 0.22); border-color: rgba(167, 139, 250, 0.45); }
.school-btn:disabled { opacity: 0.5; cursor: not-allowed; }

/* Spinner animation */
@keyframes spin { from { transform: rotate(0deg); } to { transform: rotate(360deg); } }
.spin { animation: spin 1s linear infinite; }

/* School result */
.school-result {
  display: flex; align-items: flex-start; gap: 8px;
  background: rgba(167, 139, 250, 0.08); border: 1px solid rgba(167, 139, 250, 0.15);
  border-radius: 6px; padding: 10px 12px; margin-bottom: 12px;
  font-size: 12px; color: #e8eaec; line-height: 1.5;
}
.school-result-icon { color: #a78bfa; flex-shrink: 0; margin-top: 2px; }

/* Data grid */
.cm-data-grid { display: flex; flex-direction: column; gap: 8px; }
.cm-data-row { display: flex; flex-direction: column; gap: 4px; padding: 8px; background: #0d0d0d; border-radius: 4px; border: 1px solid rgba(255,255,255,0.05); }
.cm-data-key { color: #3794ff; font-weight: 600; font-size: 12px; }
.cm-data-val { color: #b0b4ba; font-size: 11px; margin: 0; white-space: pre-wrap; word-break: break-all; max-height: 120px; overflow-y: auto; }

/* Properties */
.props-grid { display: flex; flex-direction: column; gap: 4px; }
.prop-row { display: flex; justify-content: space-between; font-size: 12px; padding: 4px 0; border-bottom: 1px solid rgba(255,255,255,0.03); }
.prop-label { color: #8b8f96; }
.prop-value { color: #e8eaec; max-width: 60%; text-align: right; word-break: break-all; }

/* Labels */
.labels-grid { display: flex; flex-wrap: wrap; gap: 6px; }
.label-chip { background: rgba(55, 148, 255, 0.1); color: #3794ff; font-size: 11px; padding: 3px 8px; border-radius: 4px; font-family: var(--mono); }

/* Agent Notification */
.agent-notification { display: flex; align-items: center; gap: 12px; background: rgba(167, 139, 250, 0.15); border: 1px solid rgba(167, 139, 250, 0.3); padding: 12px 16px; border-radius: 6px; margin-bottom: 16px; color: #e8eaec; font-size: 13px; animation: slide-down 0.3s ease-out; }
.notif-icon { color: #a78bfa; display: flex; }
@keyframes slide-down { from { opacity: 0; transform: translateY(-10px); } to { opacity: 1; transform: translateY(0); } }
</style>
