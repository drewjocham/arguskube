<script setup>
import { ref, onMounted, computed } from 'vue'
import { useSetup } from '../../composables/useWails'

const { tools, loading, actionLoading, checkTools, installPopeye, deployAgent, undeployAgent } = useSetup()

const agentNamespace = ref('kubewatcher')
const lastResult = ref(null)

onMounted(() => {
  checkTools()
})

function toolIcon(name) {
  const icons = {
    kubectl: '⎈',
    docker: '🐳',
    helm: '⛵',
    popeye: '🔍',
    'kubewatcher-agent': '🤖',
  }
  return icons[name] || '🔧'
}

function toolLabel(name) {
  const labels = {
    kubectl: 'kubectl',
    docker: 'Docker',
    helm: 'Helm',
    popeye: 'Popeye (Cluster Audit)',
    'kubewatcher-agent': 'KubeWatcher Agent (Anomaly Detector)',
  }
  return labels[name] || name
}

function toolDescription(name) {
  const descriptions = {
    kubectl: 'Kubernetes CLI — required for cluster communication',
    docker: 'Container runtime — used to run Popeye if binary not installed',
    helm: 'Package manager — optional, used for Helm-based deployments',
    popeye: 'Cluster linter that scans your workloads for best-practice violations',
    'kubewatcher-agent': 'In-cluster DaemonSet that provides real-time anomaly detection, topology mapping, and streaming metrics',
  }
  return descriptions[name] || ''
}

function isInstallable(name) {
  return name === 'popeye' || name === 'kubewatcher-agent'
}

async function handleInstall(name) {
  if (name === 'popeye') {
    lastResult.value = await installPopeye()
  } else if (name === 'kubewatcher-agent') {
    lastResult.value = await deployAgent(agentNamespace.value)
  }
}

async function handleUninstall(name) {
  if (name === 'kubewatcher-agent') {
    lastResult.value = await undeployAgent(agentNamespace.value)
  }
}

const readyCount = computed(() => tools.value.filter(t => t.installed).length)
const totalCount = computed(() => tools.value.length)
</script>

<template>
  <div class="setup-view">
    <div class="setup-header">
      <div class="setup-title">
        <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
          <circle cx="12" cy="12" r="3"></circle>
          <path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1 0 2.83 2 2 0 0 1-2.83 0l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-2 2 2 2 0 0 1-2-2v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83 0 2 2 0 0 1 0-2.83l.06-.06A1.65 1.65 0 0 0 4.68 15a1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1-2-2 2 2 0 0 1 2-2h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 0-2.83 2 2 0 0 1 2.83 0l.06.06A1.65 1.65 0 0 0 9 4.68a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 2-2 2 2 0 0 1 2 2v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 0 2 2 0 0 1 0 2.83l-.06.06A1.65 1.65 0 0 0 19.4 9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 2 2 2 2 0 0 1-2 2h-.09a1.65 1.65 0 0 0-1.51 1z"></path>
        </svg>
        Setup & Configuration
      </div>
      <div class="setup-subtitle">One-click setup for KubeWatcher tools and services</div>
    </div>

    <!-- Status overview -->
    <div class="status-bar">
      <div class="status-text">
        <span class="ready-count">{{ readyCount }}/{{ totalCount }}</span> tools ready
      </div>
      <button class="refresh-btn" @click="checkTools" :disabled="loading">
        <svg :class="{ spinning: loading }" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
          <polyline points="23 4 23 10 17 10"></polyline>
          <polyline points="1 20 1 14 7 14"></polyline>
          <path d="M3.51 9a9 9 0 0 1 14.85-3.36L23 10M1 14l4.64 4.36A9 9 0 0 0 20.49 15"></path>
        </svg>
        Refresh
      </button>
    </div>

    <!-- Result toast -->
    <div v-if="lastResult" class="result-toast" :class="lastResult.success ? 'success' : 'error'">
      <div class="toast-msg">{{ lastResult.message }}</div>
      <div v-if="lastResult.output" class="toast-output">{{ lastResult.output }}</div>
      <button class="toast-close" @click="lastResult = null">×</button>
    </div>

    <!-- Tool cards -->
    <div class="tool-grid">
      <div v-for="tool in tools" :key="tool.name" class="tool-card" :class="{ installed: tool.installed }">
        <div class="tool-main">
          <div class="tool-icon-wrap">
            <span class="tool-emoji">{{ toolIcon(tool.name) }}</span>
            <div class="tool-status-dot" :class="tool.installed ? 'ok' : 'missing'"></div>
          </div>
          <div class="tool-body">
            <div class="tool-name">{{ toolLabel(tool.name) }}</div>
            <div class="tool-desc">{{ toolDescription(tool.name) }}</div>
            <div class="tool-meta">
              <span v-if="tool.installed" class="meta-badge installed-badge">
                Installed
                <template v-if="tool.version"> · {{ tool.version }}</template>
                <template v-if="tool.via"> ({{ tool.via }})</template>
              </span>
              <span v-else class="meta-badge missing-badge">Not installed</span>
            </div>
          </div>
          <div class="tool-actions">
            <template v-if="isInstallable(tool.name)">
              <button
                v-if="!tool.installed"
                class="install-btn"
                :disabled="actionLoading === tool.name"
                @click="handleInstall(tool.name)"
              >
                <span v-if="actionLoading === tool.name" class="btn-spinner"></span>
                {{ actionLoading === tool.name ? 'Installing...' : 'Install' }}
              </button>
              <button
                v-else-if="tool.name === 'kubewatcher-agent'"
                class="remove-btn"
                :disabled="actionLoading === tool.name"
                @click="handleUninstall(tool.name)"
              >
                Remove
              </button>
            </template>
          </div>
        </div>

        <!-- Agent namespace config -->
        <div v-if="tool.name === 'kubewatcher-agent' && !tool.installed" class="agent-config">
          <label class="config-label">Deploy namespace:</label>
          <input type="text" class="config-input" v-model="agentNamespace" placeholder="kubewatcher" />
        </div>

        <!-- Popeye install info -->
        <div v-if="tool.name === 'popeye' && !tool.installed" class="install-info">
          Will try <code>go install</code> first, then fall back to <code>docker pull quay.io/derailed/popeye</code>
        </div>
      </div>
    </div>

    <!-- What gets deployed -->
    <div class="info-section">
      <div class="info-title">What the agent deploys</div>
      <div class="info-body">
        The KubeWatcher Agent is deployed as a DaemonSet with read-only RBAC permissions.
        It provides real-time anomaly detection, service topology mapping, and streaming pod/node metrics.
        Resources created: ServiceAccount, ClusterRole, ClusterRoleBinding, DaemonSet, Service.
        The agent runs as non-root with a read-only filesystem and dropped capabilities.
      </div>
    </div>
  </div>
</template>

<style scoped>
.setup-view {
  padding: 24px;
  display: flex;
  flex-direction: column;
  gap: 20px;
  overflow-y: auto;
  height: 100%;
}

.setup-header { }
.setup-title {
  display: flex;
  align-items: center;
  gap: 10px;
  font-size: 20px;
  font-weight: 500;
  color: #fff;
  margin-bottom: 6px;
}
.setup-title svg { color: #a78bfa; }
.setup-subtitle { font-size: 13px; color: #8b8f96; }

/* Status bar */
.status-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 10px 16px;
  background: #1e2023;
  border: 1px solid rgba(255,255,255,0.08);
  border-radius: 8px;
}
.status-text { font-size: 13px; color: #b0b4ba; }
.ready-count { font-weight: 600; color: #3ecf8e; font-family: 'SF Mono', Consolas, monospace; }

.refresh-btn {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 5px 12px;
  background: rgba(255,255,255,0.05);
  border: 1px solid rgba(255,255,255,0.1);
  border-radius: 6px;
  color: #b0b4ba;
  font-size: 12px;
  cursor: pointer;
  font-family: inherit;
  transition: all 0.15s;
}
.refresh-btn:hover { background: rgba(255,255,255,0.1); color: #fff; }
.refresh-btn:disabled { opacity: 0.5; cursor: wait; }

@keyframes spin { to { transform: rotate(360deg); } }
.spinning { animation: spin 1s linear infinite; }

/* Result toast */
.result-toast {
  padding: 12px 16px;
  border-radius: 8px;
  font-size: 13px;
  position: relative;
  animation: slide-down 0.2s ease-out;
}
.result-toast.success {
  background: rgba(62, 207, 142, 0.1);
  border: 1px solid rgba(62, 207, 142, 0.3);
  color: #3ecf8e;
}
.result-toast.error {
  background: rgba(240, 84, 84, 0.1);
  border: 1px solid rgba(240, 84, 84, 0.3);
  color: #f05454;
}
.toast-msg { font-weight: 500; }
.toast-output {
  margin-top: 6px;
  font-family: 'SF Mono', Consolas, monospace;
  font-size: 11px;
  color: #8b8f96;
  max-height: 80px;
  overflow-y: auto;
  white-space: pre-wrap;
}
.toast-close {
  position: absolute;
  top: 8px;
  right: 10px;
  background: none;
  border: none;
  color: inherit;
  cursor: pointer;
  font-size: 16px;
  opacity: 0.6;
}
.toast-close:hover { opacity: 1; }
@keyframes slide-down { from { opacity: 0; transform: translateY(-8px); } to { opacity: 1; transform: translateY(0); } }

/* Tool cards */
.tool-grid { display: flex; flex-direction: column; gap: 8px; }

.tool-card {
  background: #1e2023;
  border: 1px solid rgba(255,255,255,0.08);
  border-radius: 8px;
  padding: 16px;
  transition: border-color 0.15s;
}
.tool-card.installed { border-left: 3px solid #3ecf8e; }
.tool-card:not(.installed) { border-left: 3px solid rgba(255,255,255,0.1); }

.tool-main {
  display: flex;
  align-items: flex-start;
  gap: 14px;
}

.tool-icon-wrap { position: relative; }
.tool-emoji { font-size: 24px; }
.tool-status-dot {
  position: absolute;
  bottom: -2px;
  right: -2px;
  width: 10px;
  height: 10px;
  border-radius: 50%;
  border: 2px solid #1e2023;
}
.tool-status-dot.ok { background: #3ecf8e; }
.tool-status-dot.missing { background: #6b7078; }

.tool-body { flex: 1; min-width: 0; }
.tool-name { font-size: 14px; font-weight: 500; color: #e8eaec; margin-bottom: 4px; }
.tool-desc { font-size: 12px; color: #8b8f96; line-height: 1.5; margin-bottom: 8px; }
.tool-meta { }

.meta-badge {
  display: inline-block;
  font-size: 11px;
  padding: 2px 8px;
  border-radius: 4px;
  font-weight: 600;
}
.installed-badge { background: rgba(62,207,142,0.12); color: #3ecf8e; }
.missing-badge { background: rgba(255,255,255,0.05); color: #6b7078; }

.tool-actions { flex-shrink: 0; }

.install-btn {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 7px 18px;
  background: linear-gradient(135deg, #3ecf8e 0%, #22a06b 100%);
  border: none;
  border-radius: 6px;
  color: #fff;
  font-weight: 600;
  font-size: 12px;
  cursor: pointer;
  font-family: inherit;
  transition: all 0.15s;
}
.install-btn:hover { transform: translateY(-1px); box-shadow: 0 4px 12px rgba(62,207,142,0.3); }
.install-btn:disabled { opacity: 0.6; cursor: wait; transform: none; box-shadow: none; }

.remove-btn {
  padding: 7px 18px;
  background: rgba(240,84,84,0.1);
  border: 1px solid rgba(240,84,84,0.3);
  border-radius: 6px;
  color: #f05454;
  font-weight: 500;
  font-size: 12px;
  cursor: pointer;
  font-family: inherit;
  transition: all 0.15s;
}
.remove-btn:hover { background: rgba(240,84,84,0.2); }
.remove-btn:disabled { opacity: 0.5; cursor: wait; }

.btn-spinner {
  width: 12px;
  height: 12px;
  border: 2px solid rgba(255,255,255,0.3);
  border-top-color: #fff;
  border-radius: 50%;
  animation: spin 0.8s linear infinite;
  display: inline-block;
}

/* Agent config */
.agent-config {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-top: 12px;
  padding-top: 12px;
  border-top: 1px solid rgba(255,255,255,0.05);
}
.config-label { font-size: 12px; color: #8b8f96; white-space: nowrap; }
.config-input {
  width: 180px;
  padding: 5px 10px;
  background: rgba(0,0,0,0.2);
  border: 1px solid rgba(255,255,255,0.1);
  border-radius: 4px;
  color: #e8eaec;
  font-family: 'SF Mono', Consolas, monospace;
  font-size: 12px;
  outline: none;
}
.config-input:focus { border-color: rgba(167,139,250,0.5); }

/* Install info */
.install-info {
  margin-top: 10px;
  font-size: 11px;
  color: #6b7078;
  padding-top: 10px;
  border-top: 1px solid rgba(255,255,255,0.04);
}
.install-info code {
  background: rgba(255,255,255,0.06);
  padding: 1px 5px;
  border-radius: 3px;
  font-family: 'SF Mono', Consolas, monospace;
  color: #a78bfa;
}

/* Info section */
.info-section {
  background: rgba(167,139,250,0.05);
  border: 1px solid rgba(167,139,250,0.15);
  border-radius: 8px;
  padding: 16px;
}
.info-title {
  font-size: 12px;
  font-weight: 600;
  color: #a78bfa;
  margin-bottom: 8px;
}
.info-body {
  font-size: 12px;
  color: #8b8f96;
  line-height: 1.6;
}
</style>
