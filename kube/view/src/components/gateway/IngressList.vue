<script setup>
import { ref, onMounted } from 'vue'
import { useResources } from '../../composables/useWails'
import { callGo } from '../../composables/useBridge'
import ArgusRecommendations from '../common/ArgusRecommendations.vue'
import CreateIngressPopup from './CreateIngressPopup.vue'

// Dedicated Ingress list view. Mirrors NetworkPolicyList: dense table
// of resources, click-to-expand for details, "+ New Ingress" button
// opening a scaffold-driven create popup, and the shared
// ArgusRecommendations panel for data-grounded suggestions
// (missing TLS, broken backends, duplicate routes, no class).

const { result, detail, loading, error, detailLoading, listResources, getResourceDetail } = useResources()

const ingresses = ref([])
const itemDetail = ref(null)
const expandedItem = ref(null)
const notification = ref(null)

function mapItems() {
  if (result.value && result.value.items && result.value.items.length > 0) {
    ingresses.value = result.value.items.map(item => ({
      name: item.name,
      namespace: item.namespace,
      class: item.fields?.class || '—',
      hosts: item.fields?.hosts || '—',
      loadBalancer: item.fields?.load_balancer || '—',
      age: item.age || '—',
    }))
  } else {
    ingresses.value = []
  }
}

async function refresh(force = false) {
  await listResources('ingresses', '', force)
  mapItems()
  refreshRecommendations()
}

// ── Argus inline recommendations ──────────────────────────────────
// Pulls data-grounded ingress suggestions (missing TLS, broken
// backends, duplicate host+path routes, missing ingressClassName)
// from the new RecommendIngresses App method. Refreshed alongside
// the list AND after a successful create/apply so a fixed rec
// disappears next render.
const recommendations = ref([])
const recsLoading = ref(false)
const recsError = ref('')

async function refreshRecommendations() {
  recsLoading.value = true
  recsError.value = ''
  try {
    const out = await callGo('RecommendIngresses', '')
    recommendations.value = Array.isArray(out) ? out : []
  } catch (e) {
    recsError.value = e?.message || String(e)
    recommendations.value = []
  } finally {
    recsLoading.value = false
  }
}

// ── Create-ingress popup ──────────────────────────────────────────
const createOpen = ref(false)
const createInitialYaml = ref('')

function openCreate(initialYaml = '') {
  createInitialYaml.value = initialYaml || ''
  createOpen.value = true
}

function onCreated() {
  createOpen.value = false
  notification.value = 'Ingress applied. Refreshing list…'
  setTimeout(() => { notification.value = null }, 4000)
  refresh(true)
}

function onApplyRecommendationFix(rec) {
  // Some ingress recs ship a YAML fragment (the `tls:` patch) rather
  // than a full manifest — still useful to drop into the editor so
  // the user can paste it into their existing resource.
  openCreate(rec?.suggestedYAML || '')
}

onMounted(() => refresh())

async function toggleExpand(itemName) {
  if (expandedItem.value === itemName) {
    expandedItem.value = null
    itemDetail.value = null
  } else {
    expandedItem.value = itemName
    const item = ingresses.value.find(i => i.name === itemName)
    if (item) {
      await getResourceDetail('ingresses', item.namespace, itemName)
      if (detail.value) {
        itemDetail.value = detail.value
      }
    }
  }
}
</script>

<template>
  <div class="ing-view">
    <div class="header">
      <div class="header-row">
        <div>
          <div class="title">Ingresses</div>
          <div class="subtitle">HTTP/HTTPS routing into the cluster — recommendations cover TLS, broken backends, and route conflicts.</div>
        </div>
        <div class="ing-header-actions">
          <button
            class="create-btn"
            @click="openCreate()"
            title="Author an Ingress from a scaffold and apply it"
            data-testid="ingress-create-btn"
          >
            + New Ingress
          </button>
          <button class="refresh-btn" @click="refresh(true)" :disabled="loading">{{ loading ? 'Loading…' : '↻ Refresh' }}</button>
        </div>
      </div>
    </div>

    <div v-if="notification" class="agent-notification">
      <div class="notif-icon">
        <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M12 2v20M17 5H9.5a3.5 3.5 0 0 0 0 7h5a3.5 3.5 0 0 1 0 7H6"></path></svg>
      </div>
      <div class="notif-text">{{ notification }}</div>
    </div>

    <ArgusRecommendations
      :recommendations="recommendations"
      :loading="recsLoading"
      title="Argus recommendations — Ingress coverage"
      class="ing-recs"
      @refresh="refreshRecommendations"
      @apply-fix="onApplyRecommendationFix"
    />

    <div class="ing-scroll-area">
      <div v-if="loading && !ingresses.length" class="state-box">Loading…</div>
      <div v-else-if="error" class="state-box state-error">{{ error }}</div>
      <div v-else-if="!ingresses.length" class="state-box">No ingresses found in this cluster.</div>

      <div v-else class="ing-list">
        <div class="ing-header-row ing-grid">
          <div class="col-name">Name</div>
          <div class="col-ns">Namespace</div>
          <div class="col-class">Class</div>
          <div class="col-hosts">Hosts</div>
          <div class="col-lb">Load Balancer</div>
          <div class="col-age">Age</div>
        </div>

        <div v-for="i in ingresses" :key="i.namespace + '/' + i.name" class="ing-row-container">
          <div class="ing-row ing-grid" @click="toggleExpand(i.name)">
            <div class="col-name">
              <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" style="color: #38bdf8; margin-right: 8px;">
                <path d="M2 12h6l3-9 4 18 3-9h4"></path>
              </svg>
              {{ i.name }}
            </div>
            <div class="col-ns font-mono">{{ i.namespace }}</div>
            <div class="col-class font-mono tag">{{ i.class }}</div>
            <div class="col-hosts font-mono">{{ i.hosts }}</div>
            <div class="col-lb font-mono">{{ i.loadBalancer }}</div>
            <div class="col-age font-mono" style="display:flex; justify-content:space-between; align-items:center;">
              {{ i.age }}
              <svg class="chevron" :class="{ open: expandedItem === i.name }" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <polyline points="6 9 12 15 18 9"></polyline>
              </svg>
            </div>
          </div>

          <div class="ing-expanded" v-if="expandedItem === i.name">
            <div v-if="detailLoading" style="color:#8b8f96; font-size:13px; padding:12px;">Loading…</div>
            <div v-else-if="itemDetail" class="expanded-grid">
              <div class="expanded-card">
                <h4 class="card-title">Properties</h4>
                <div class="props-grid">
                  <div class="prop-row" v-for="prop in itemDetail.properties" :key="prop.key">
                    <span class="prop-label">{{ prop.key }}</span>
                    <span class="prop-value font-mono">{{ prop.value }}</span>
                  </div>
                </div>
              </div>

              <div class="expanded-card" v-if="itemDetail.labels && Object.keys(itemDetail.labels).length">
                <h4 class="card-title">Labels</h4>
                <div class="labels-grid">
                  <span class="label-chip" v-for="(v, k) in itemDetail.labels" :key="k">{{ k }}={{ v }}</span>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>

    <CreateIngressPopup
      :show="createOpen"
      :initial-yaml="createInitialYaml"
      @close="createOpen = false"
      @applied="onCreated"
    />
  </div>
</template>

<style scoped>
.ing-view { padding: 24px; display: flex; flex-direction: column; gap: 24px; min-height: 0; flex: 1; box-sizing: border-box; }
.ing-scroll-area { flex: 1; overflow-y: auto; min-height: 0; }
.header .title { font-size: 20px; font-weight: 500; color: #fff; margin-bottom: 4px; }
.header .subtitle { font-size: 13px; color: #8b8f96; }
.header-row { display: flex; justify-content: space-between; align-items: flex-start; }
.refresh-btn { background: rgba(255,255,255,0.06); border: 1px solid rgba(255,255,255,0.1); color: #b0b4ba; padding: 6px 12px; border-radius: 6px; font-size: 12px; cursor: pointer; transition: all 0.15s; }
.refresh-btn:hover { background: rgba(255,255,255,0.1); color: #fff; }
.refresh-btn:disabled { opacity: 0.5; cursor: not-allowed; }
.create-btn { background: #6d4ade; border: 1px solid #6d4ade; color: #fff; padding: 6px 14px; border-radius: 6px; font-size: 12px; cursor: pointer; font-weight: 500; }
.create-btn:hover { background: #5a3bc7; border-color: #5a3bc7; }
.ing-recs { margin: 0 0 8px; }

.ing-header-actions { display: flex; align-items: center; gap: 8px; }

.state-box { padding: 40px; text-align: center; color: #8b8f96; font-size: 13px; background: #1e2023; border: 1px solid rgba(255,255,255,0.08); border-radius: 8px; }
.state-error { color: #f05454; }

.ing-list { background: #1e2023; border: 1px solid rgba(255, 255, 255, 0.08); border-radius: 8px; overflow: hidden; }
.ing-header-row {
  display: grid;
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
.ing-row-container { border-bottom: 1px solid rgba(255, 255, 255, 0.04); transition: all 0.3s ease; }
.ing-row-container:last-child { border-bottom: none; }
.ing-row {
  display: grid;
  gap: 16px;
  padding: 14px 16px;
  font-size: 13px;
  color: #e8eaec;
  align-items: center;
  cursor: pointer;
  transition: background 0.2s;
}
.ing-grid { grid-template-columns: 2fr 1fr 1fr 2fr 1.5fr 80px; }
.ing-row:hover { background: rgba(255, 255, 255, 0.02); }

.chevron { transition: transform 0.2s ease; color: #6b7078; }
.chevron.open { transform: rotate(180deg); }

.ing-expanded {
  padding: 16px;
  background: #141517;
  border-top: 1px dashed rgba(255,255,255,0.08);
}
.expanded-grid { display: flex; flex-direction: column; gap: 12px; }
.expanded-card { background: #1e2023; border: 1px solid rgba(255, 255, 255, 0.05); border-radius: 6px; padding: 16px; display: flex; flex-direction: column; }
.card-title { font-size: 13px; font-weight: 600; color: #fff; margin: 0 0 12px 0; }

.props-grid { display: flex; flex-direction: column; gap: 4px; }
.prop-row { display: flex; justify-content: space-between; font-size: 12px; padding: 4px 0; border-bottom: 1px solid rgba(255,255,255,0.03); }
.prop-label { color: #8b8f96; }
.prop-value { color: #e8eaec; max-width: 60%; text-align: right; word-break: break-all; }

.labels-grid { display: flex; flex-wrap: wrap; gap: 6px; }
.label-chip { background: rgba(55, 148, 255, 0.1); color: #3794ff; font-size: 11px; padding: 3px 8px; border-radius: 4px; font-family: var(--mono); }

.agent-notification { display: flex; align-items: center; gap: 12px; background: rgba(167, 139, 250, 0.15); border: 1px solid rgba(167, 139, 250, 0.3); padding: 12px 16px; border-radius: 6px; margin-bottom: 16px; color: #e8eaec; font-size: 13px; animation: slide-down 0.3s ease-out; }
.notif-icon { color: #a78bfa; display: flex; }
@keyframes slide-down { from { opacity: 0; transform: translateY(-10px); } to { opacity: 1; transform: translateY(0); } }

.col-name { display: flex; align-items: center; font-weight: 500; }
.font-mono { font-family: var(--mono); color: #b0b4ba; font-size: 12px; }
.tag { background: rgba(255,255,255,0.05); padding: 4px 6px; border-radius: 4px; display: inline-block; border: 1px solid rgba(255,255,255,0.05); }
</style>
