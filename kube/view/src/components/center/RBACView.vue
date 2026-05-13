<script setup>
import { ref, onMounted } from 'vue'
import { callGo } from '../../composables/useBridge'
import Select from '../common/Select.vue'

const namespace = ref('default')
const namespaces = ref([])
const permissions = ref([])
const impersonateUser = ref('')
const impersonateGroup = ref('')
const loading = ref(false)
const error = ref(null)

onMounted(async () => {
  try {
    const nss = await callGo('ListNamespaces')
    if (nss) namespaces.value = nss
  } catch {}
})

async function checkPermissions() {
  loading.value = true
  error.value = null
  try {
    permissions.value = await callGo('GetCommonPermissions', namespace.value)
  } catch (e) {
    error.value = e?.message || String(e)
  }
  loading.value = false
}

async function checkVerb(resource, verb) {
  loading.value = true
  try {
    const result = await callGo('CheckCanI', namespace.value, verb, resource, '')
    if (result) {
      const existing = permissions.value.findIndex(p => p.resource === resource && p.verb === verb)
      if (existing >= 0) {
        permissions.value[existing] = result
      } else {
        permissions.value.push(result)
      }
    }
  } catch {}
  loading.value = false
}

const resourceColors = {
  pods: '#3794ff', deployments: '#a78bfa', services: '#f5a623',
  configmaps: '#3ecf8e', secrets: '#f05454', ingresses: '#38bdf8', events: '#8b8f96',
}

function resourceColor(resource) {
  return resourceColors[resource] || '#8b8f96'
}
</script>

<template>
  <div class="rbac-view">
    <div class="header">
      <div class="title">RBAC & Permissions</div>
      <div class="subtitle">View effective permissions and impersonate users</div>
    </div>

    <div class="controls">
      <Select v-model="namespace" :options="namespaces" size="sm" aria-label="Namespace" />
      <button class="btn-primary" @click="checkPermissions" :disabled="loading">
        {{ loading ? 'Checking…' : 'Check Permissions' }}
      </button>
    </div>

    <div v-if="error" class="error-banner">{{ error }}</div>

    <div v-if="permissions.length" class="matrix-card">
      <div class="matrix-title">Permission Matrix — {{ namespace }}</div>
      <div class="matrix">
        <div class="matrix-header">
          <div class="mh-resource">Resource</div>
          <div class="mh-verb" v-for="verb in ['get','list','watch']" :key="verb">{{ verb }}</div>
        </div>
        <div v-for="resource in ['pods','deployments','services','configmaps','secrets','ingresses','events']" :key="resource" class="matrix-row">
          <div class="mr-resource">
            <span class="resource-dot" :style="{ background: resourceColor(resource) }"></span>
            {{ resource }}
          </div>
          <div class="mr-verb" v-for="verb in ['get','list','watch']" :key="verb">
            <div
              class="perm-badge"
              :class="(permissions.find(p => p.resource === resource && p.verb === verb)?.allowed) ? 'allowed' : 'denied'"
              @click="checkVerb(resource, verb)"
              :title="(permissions.find(p => p.resource === resource && p.verb === verb)?.reason) || ''"
            >
              {{ permissions.find(p => p.resource === resource && p.verb === verb)?.allowed ? '✓' : '✗' }}
            </div>
          </div>
        </div>
      </div>
    </div>

    <div class="impersonate-card">
      <div class="card-title">Impersonate User / ServiceAccount</div>
      <div class="imp-row">
        <input v-model="impersonateUser" class="form-input" placeholder="system:serviceaccount:kube-system:default" />
        <input v-model="impersonateGroup" class="form-input short" placeholder="group (optional)" />
        <button class="btn-primary" :disabled="!impersonateUser" @click="callGo('ImpersonateUser', impersonateUser, impersonateGroup ? [impersonateGroup] : [])">
          View As
        </button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.rbac-view { padding: 24px; display: flex; flex-direction: column; gap: 16px; overflow-y: auto; flex: 1; min-height: 0; }
.header .title { font-size: 20px; font-weight: 500; color: #fff; margin-bottom: 4px; }
.header .subtitle { font-size: 13px; color: #8b8f96; }
.controls { display: flex; gap: 8px; align-items: center; }
.btn-primary { padding: 6px 16px; font-size: 12px; background: rgba(167,139,250,0.15); border: 1px solid rgba(167,139,250,0.3); color: #a78bfa; border-radius: 5px; cursor: pointer; }
.btn-primary:disabled { opacity: 0.4; cursor: not-allowed; }
.error-banner { padding: 8px 12px; background: rgba(240,84,84,0.12); border: 1px solid rgba(240,84,84,0.25); border-radius: 6px; color: #f05454; font-size: 12px; }

.card-title { font-size: 14px; font-weight: 600; color: #e8eaec; margin-bottom: 12px; }
.matrix-card { background: #1e2023; border: 1px solid rgba(255,255,255,0.08); border-radius: 8px; padding: 16px; }
.matrix-title { font-size: 13px; font-weight: 600; color: #8b8f96; margin-bottom: 12px; }
.matrix { display: flex; flex-direction: column; gap: 2px; }
.matrix-header, .matrix-row { display: grid; grid-template-columns: 140px repeat(3, 60px); gap: 4px; align-items: center; padding: 4px 0; }
.matrix-header { font-size: 10.5px; text-transform: uppercase; letter-spacing: 0.04em; color: #6b7078; border-bottom: 1px solid rgba(255,255,255,0.06); padding-bottom: 8px; }
.mh-verb { text-align: center; }
.mr-resource { font-size: 12px; color: #e8eaec; display: flex; align-items: center; gap: 6px; }
.resource-dot { width: 6px; height: 6px; border-radius: 50%; }
.mr-verb { display: flex; justify-content: center; }
.perm-badge { width: 24px; height: 24px; display: flex; align-items: center; justify-content: center; border-radius: 4px; font-size: 11px; cursor: pointer; transition: all 0.1s; }
.perm-badge.allowed { background: rgba(62,207,142,0.15); color: #3ecf8e; }
.perm-badge.denied { background: rgba(240,84,84,0.1); color: #f05454; }

.impersonate-card { background: #1e2023; border: 1px solid rgba(255,255,255,0.08); border-radius: 8px; padding: 16px; }
.imp-row { display: flex; gap: 8px; align-items: center; }
.form-input { flex: 1; padding: 7px 10px; font-size: 12.5px; background: #141517; border: 1px solid rgba(255,255,255,0.1); border-radius: 5px; color: #e8eaec; outline: none; font-family: inherit; }
.form-input.short { max-width: 200px; }
.font-mono { font-family: var(--mono); }
</style>
