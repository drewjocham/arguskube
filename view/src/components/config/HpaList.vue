<script setup>
import { ref, onMounted } from 'vue'
import { useResources, useVPARecommendations } from '../../composables/useWails'

const { result, detail, loading, detailLoading, listResources, getResourceDetail } = useResources()
const { vpas, loading: vpasLoading, error: vpasError, fetchVPAs } = useVPARecommendations()


const hpas = ref([])
const hpaDetail = ref(null)
const expandedHpa = ref(null)
const notification = ref(null)

onMounted(async () => {
  await listResources('hpa', '')
  if (result.value && result.value.items && result.value.items.length > 0) {
    hpas.value = result.value.items.map(item => ({
      name: item.name,
      namespace: item.namespace,
      reference: item.fields?.reference || '—',
      targets: item.fields?.targets || '—',
      minPods: parseInt(item.fields?.min_pods || '0'),
      maxPods: parseInt(item.fields?.max_pods || '0'),
      replicas: parseInt(item.fields?.replicas || '0'),
      age: item.age || '—'
    }))
  } else {
    hpas.value = []
  }

  // Also fetch VPA recommendations.
  fetchVPAs('')
})

function isOverTarget(targetStr) {
  const parts = targetStr.split(' / ').map(s => parseInt(s.replace('%','')))
  if (parts.length < 2 || isNaN(parts[0]) || isNaN(parts[1])) return false
  return parts[0] >= parts[1]
}

async function toggleExpand(hpaName) {
  if (expandedHpa.value === hpaName) {
    expandedHpa.value = null
    hpaDetail.value = null
  } else {
    expandedHpa.value = hpaName
    const hpa = hpas.value.find(h => h.name === hpaName)
    if (hpa) {
      await getResourceDetail('hpa', hpa.namespace, hpaName)
      if (detail.value) {
        hpaDetail.value = detail.value
      }
    }
  }
}
</script>

<template>
  <div class="hpa-view">
    <div class="header">
      <div class="title">Autoscaling</div>
      <div class="subtitle">HPA horizontal scaling and VPA vertical resource recommendations</div>
    </div>
    
    <div v-if="notification" class="agent-notification">
      <div class="notif-icon">
        <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M12 2v20M17 5H9.5a3.5 3.5 0 0 0 0 7h5a3.5 3.5 0 0 1 0 7H6"></path></svg>
      </div>
      <div class="notif-text">{{ notification }}</div>
    </div>

    <div class="hpa-list">
      <div class="hpa-header-row">
        <div class="col-name">Name</div>
        <div class="col-ns">Namespace</div>
        <div class="col-ref">Reference</div>
        <div class="col-tar">Targets</div>
        <div class="col-min">MinPods</div>
        <div class="col-max">MaxPods</div>
        <div class="col-rep">Replicas</div>
        <div class="col-age">Age</div>
      </div>

      <div v-for="h in hpas" :key="h.name" class="hpa-row-container" :class="{'ai-active-pulse': h.isApplying}">
        <div class="hpa-row" @click="toggleExpand(h.name)">
          <div class="col-name">
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" style="color: #60a5fa; margin-right: 8px;"><polyline points="22 12 18 12 15 21 9 3 6 12 2 12"></polyline></svg>
            {{ h.name }}
          </div>
          <div class="col-ns font-mono">{{ h.namespace }}</div>
          <div class="col-ref font-mono">{{ h.reference }}</div>
          
          <div class="col-tar">
            <span class="target-badge" :class="{'alert': isOverTarget(h.targets)}">{{ h.targets }}</span>
          </div>
          
          <div class="col-min font-mono">{{ h.minPods }}</div>
          <div class="col-max font-mono">{{ h.maxPods }}</div>
          <div class="col-rep font-mono" :class="{'maxed': h.replicas === h.maxPods}">{{ h.replicas }}</div>
          <div class="col-age font-mono" style="display:flex; justify-content:space-between; align-items:center;">
            {{ h.age }}
            <svg class="chevron" :class="{ open: expandedHpa === h.name }" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <polyline points="6 9 12 15 18 9"></polyline>
            </svg>
          </div>
        </div>

        <!-- Expanded View -->
        <div class="hpa-expanded" v-if="expandedHpa === h.name">
          <div v-if="detailLoading" style="color:#8b8f96; font-size:13px; padding:12px;">Loading…</div>
          <div v-else-if="hpaDetail" class="expanded-grid">
            <div class="expanded-card">
              <h4 class="card-title">Properties</h4>
              <div class="props-grid">
                <div class="prop-row" v-for="prop in hpaDetail.properties" :key="prop.key">
                  <span class="prop-label">{{ prop.key }}</span>
                  <span class="prop-value font-mono">{{ prop.value }}</span>
                </div>
              </div>
            </div>

            <div class="expanded-card" v-if="hpaDetail.conditions && hpaDetail.conditions.length">
              <h4 class="card-title">Conditions</h4>
              <div class="conditions-list">
                <div class="condition-row" v-for="c in hpaDetail.conditions" :key="c.type">
                  <span class="cond-type font-mono">{{ c.type }}</span>
                  <span class="cond-status" :class="c.status === 'True' ? 'ok' : 'fail'">{{ c.status }}</span>
                  <span class="cond-reason font-mono" v-if="c.reason">{{ c.reason }}</span>
                </div>
              </div>
            </div>

            <div class="expanded-card" v-if="hpaDetail.events && hpaDetail.events.length">
              <h4 class="card-title">Recent Events</h4>
              <div class="events-mini">
                <div class="event-mini-row" v-for="(ev, i) in hpaDetail.events" :key="i" :class="ev.type?.toLowerCase()">
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
    </div>

    <!-- VPA Recommendations Section -->
    <div class="vpa-section">
      <div class="section-header">
        <div class="section-title">
          <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" style="color: #a78bfa;"><path d="M12 20V10"></path><path d="M18 20V4"></path><path d="M6 20v-4"></path></svg>
          VPA Recommendations
        </div>
        <div class="section-subtitle">Vertical Pod Autoscaler resource tuning from autoscaling.k8s.io</div>
      </div>

      <div v-if="vpasLoading" class="vpa-loading">Loading VPA data…</div>
      <div v-else-if="vpasError" class="vpa-empty">
        <span style="color:#f5a623;">{{ vpasError }}</span>
        <span class="vpa-hint">VPA CRDs may not be installed. Run <code>kubectl get crd verticalpodautoscalers.autoscaling.k8s.io</code> to check.</span>
      </div>
      <div v-else-if="vpas.length === 0" class="vpa-empty">
        No VPA objects found. Install the VPA controller and create VerticalPodAutoscaler resources to see recommendations.
      </div>
      <div v-else class="vpa-cards">
        <div v-for="vpa in vpas" :key="vpa.name + vpa.namespace" class="vpa-card">
          <div class="vpa-card-header">
            <div class="vpa-name">{{ vpa.name }}</div>
            <div class="vpa-ns font-mono">{{ vpa.namespace }}</div>
          </div>
          <div class="vpa-target">
            <span class="vpa-target-label">Target:</span>
            <span class="vpa-target-ref font-mono">{{ vpa.targetRef }}</span>
            <span class="vpa-mode-badge" :class="vpa.updateMode?.toLowerCase()">{{ vpa.updateMode || 'Off' }}</span>
          </div>
          <div v-if="vpa.containers && vpa.containers.length" class="vpa-containers">
            <div v-for="c in vpa.containers" :key="c.containerName" class="vpa-container">
              <div class="vpa-container-name font-mono">{{ c.containerName }}</div>
              <div class="vpa-recs-grid">
                <div class="vpa-rec-col">
                  <div class="vpa-rec-header">Lower Bound</div>
                  <div class="vpa-rec-val">{{ c.lowerCpu || '—' }} CPU</div>
                  <div class="vpa-rec-val">{{ c.lowerMemory || '—' }} mem</div>
                </div>
                <div class="vpa-rec-col target">
                  <div class="vpa-rec-header">Target</div>
                  <div class="vpa-rec-val highlight">{{ c.targetCpu || '—' }} CPU</div>
                  <div class="vpa-rec-val highlight">{{ c.targetMemory || '—' }} mem</div>
                </div>
                <div class="vpa-rec-col">
                  <div class="vpa-rec-header">Upper Bound</div>
                  <div class="vpa-rec-val">{{ c.upperCpu || '—' }} CPU</div>
                  <div class="vpa-rec-val">{{ c.upperMemory || '—' }} mem</div>
                </div>
              </div>
            </div>
          </div>
          <div v-else class="vpa-no-recs">No recommendations computed yet</div>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.hpa-view { padding: 24px; display: flex; flex-direction: column; gap: 24px; overflow-y: auto; height: 100%; }
.header .title { font-size: 20px; font-weight: 500; color: #fff; margin-bottom: 4px; }
.header .subtitle { font-size: 13px; color: #8b8f96; }

.hpa-list { background: #1e2023; border: 1px solid rgba(255, 255, 255, 0.08); border-radius: 8px; overflow: hidden; }

.hpa-header-row {
  display: grid;
  grid-template-columns: 2fr 1fr 2fr 120px 80px 80px 80px 80px;
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

.hpa-row-container {
  border-bottom: 1px solid rgba(255, 255, 255, 0.04);
  transition: all 0.3s ease;
}
.hpa-row-container:last-child { border-bottom: none; }

.hpa-row {
  display: grid;
  grid-template-columns: 2fr 1fr 2fr 120px 80px 80px 80px 80px;
  gap: 16px;
  padding: 14px 16px;
  font-size: 13px;
  color: #e8eaec;
  align-items: center;
  cursor: pointer;
  transition: background 0.2s;
}
.hpa-row:hover { background: rgba(255, 255, 255, 0.02); }

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

/* Expanded Area */
.hpa-expanded {
  padding: 16px;
  background: #141517;
  border-top: 1px dashed rgba(255,255,255,0.08);
}
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
.card-title { font-size: 13px; font-weight: 600; color: #fff; margin: 0 0 12px 0; }

/* Properties */
.props-grid { display: flex; flex-direction: column; gap: 4px; }
.prop-row { display: flex; justify-content: space-between; font-size: 12px; padding: 4px 0; border-bottom: 1px solid rgba(255,255,255,0.03); }
.prop-label { color: #8b8f96; }
.prop-value { color: #e8eaec; max-width: 60%; text-align: right; word-break: break-all; }

/* Conditions */
.conditions-list { display: flex; flex-direction: column; gap: 4px; }
.condition-row { display: flex; align-items: center; gap: 12px; font-size: 12px; }
.cond-type { color: #e8eaec; min-width: 120px; }
.cond-status { font-size: 11px; padding: 2px 6px; border-radius: 4px; font-weight: 600; }
.cond-status.ok { background: rgba(62, 207, 142, 0.15); color: #3ecf8e; }
.cond-status.fail { background: rgba(240, 84, 84, 0.15); color: #f05454; }
.cond-reason { color: #8b8f96; }

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
.agent-notification { display: flex; align-items: center; gap: 12px; background: rgba(167, 139, 250, 0.15); border: 1px solid rgba(167, 139, 250, 0.3); padding: 12px 16px; border-radius: 6px; margin-bottom: 16px; color: #e8eaec; font-size: 13px; animation: slide-down 0.3s ease-out; }
.notif-icon { color: #a78bfa; display: flex; }
@keyframes slide-down { from { opacity: 0; transform: translateY(-10px); } to { opacity: 1; transform: translateY(0); } }

.col-name { display: flex; align-items: center; font-weight: 500; }
.font-mono { font-family: 'SF Mono', Consolas, monospace; color: #b0b4ba; font-size: 12px; }

.target-badge { font-size: 11px; padding: 2px 6px; border-radius: 4px; font-weight: 600; background: rgba(255,255,255,0.05); color: #ccc; font-family: 'SF Mono', Consolas, monospace; border: 1px solid rgba(255,255,255,0.05); }
.target-badge.alert { background: rgba(240, 84, 84, 0.15); color: #f05454; border-color: rgba(240, 84, 84, 0.3); }

.maxed { color: #f05454; font-weight: 600; }

/* VPA Section */
.vpa-section { display: flex; flex-direction: column; gap: 16px; }
.section-header { display: flex; flex-direction: column; gap: 4px; }
.section-title { display: flex; align-items: center; gap: 8px; font-size: 16px; font-weight: 500; color: #fff; }
.section-subtitle { font-size: 13px; color: #8b8f96; }

.vpa-loading { color: #8b8f96; font-size: 13px; padding: 16px; text-align: center; }
.vpa-empty { color: #6b7078; font-size: 13px; text-align: center; padding: 24px; display: flex; flex-direction: column; gap: 8px; align-items: center; }
.vpa-hint { font-size: 11px; color: #4b5058; }
.vpa-hint code { background: rgba(255,255,255,0.06); padding: 2px 6px; border-radius: 3px; font-family: 'SF Mono', Consolas, monospace; color: #8b8f96; }

.vpa-cards { display: grid; grid-template-columns: repeat(auto-fill, minmax(380px, 1fr)); gap: 16px; }

.vpa-card {
  background: #1e2023; border: 1px solid rgba(255,255,255,0.08); border-radius: 8px;
  padding: 16px; display: flex; flex-direction: column; gap: 12px;
  border-left: 3px solid #a78bfa;
}

.vpa-card-header { display: flex; justify-content: space-between; align-items: center; }
.vpa-name { font-size: 14px; font-weight: 600; color: #e8eaec; }
.vpa-ns { font-size: 11px; padding: 2px 6px; background: rgba(255,255,255,0.05); border-radius: 4px; color: #b0b4ba; }

.vpa-target { display: flex; align-items: center; gap: 8px; font-size: 12px; }
.vpa-target-label { color: #6b7078; }
.vpa-target-ref { color: #a78bfa; }
.vpa-mode-badge {
  font-size: 10px; padding: 2px 6px; border-radius: 3px; font-weight: 600;
  background: rgba(255,255,255,0.05); color: #8b8f96; text-transform: uppercase; letter-spacing: 0.03em;
}
.vpa-mode-badge.auto { background: rgba(62, 207, 142, 0.15); color: #3ecf8e; }
.vpa-mode-badge.recreate { background: rgba(245, 166, 35, 0.15); color: #f5a623; }
.vpa-mode-badge.initial { background: rgba(55, 148, 255, 0.15); color: #3794ff; }

.vpa-containers { display: flex; flex-direction: column; gap: 12px; }

.vpa-container {
  background: rgba(0,0,0,0.2); border: 1px solid rgba(255,255,255,0.04); border-radius: 6px; padding: 12px;
}
.vpa-container-name { font-size: 12px; color: #3794ff; margin-bottom: 8px; }

.vpa-recs-grid { display: grid; grid-template-columns: 1fr 1fr 1fr; gap: 8px; }
.vpa-rec-col { display: flex; flex-direction: column; gap: 4px; text-align: center; }
.vpa-rec-col.target { background: rgba(167, 139, 250, 0.08); border-radius: 4px; padding: 4px; }
.vpa-rec-header { font-size: 10px; color: #6b7078; text-transform: uppercase; letter-spacing: 0.04em; font-weight: 600; }
.vpa-rec-val { font-size: 11px; color: #b0b4ba; font-family: 'SF Mono', Consolas, monospace; }
.vpa-rec-val.highlight { color: #a78bfa; font-weight: 600; }

.vpa-no-recs { font-size: 12px; color: #6b7078; text-align: center; padding: 8px; font-style: italic; }
</style>
