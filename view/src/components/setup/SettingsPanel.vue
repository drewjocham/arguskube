<script setup>
import { ref, onMounted } from 'vue'
import { callGo, useContexts } from '../../composables/useWails'

const settings = ref(null)
const loading = ref(true)
const saving = ref(false)
const saveMessage = ref('')

// Editable form state.
const form = ref({
  kubeconfigPath: '',
  currentContext: '',
  namespace: '',
  deepseekApiKey: '',
  anomstackUrl: '',
  mcpServersConfig: '',
  agentInstructions: '',
  prometheusUrl: '',
  argocdUrl: '',
  argocdToken: '',
  argocdInsecure: false,
  snykToken: '',
  trivyBinary: '',
  falcoUrl: '',
})

const argocdTesting = ref(false)
const argocdTestResult = ref('')

const { contexts, loading: ctxLoading, listContexts } = useContexts()

async function loadSettings() {
  loading.value = true
  try {
    const result = await callGo('GetSettings')
    if (result) {
      settings.value = result
      form.value = {
        kubeconfigPath: result.kubeconfigPath || '',
        currentContext: result.currentContext || '',
        namespace: result.namespace || '',
        deepseekApiKey: result.deepseekApiKey || '',
        anomstackUrl: result.anomstackUrl || '',
        mcpServersConfig: result.mcpServersConfig || '',
        agentInstructions: result.agentInstructions || '',
        prometheusUrl: result.prometheusUrl || '',
        argocdUrl: result.argocdUrl || '',
        argocdToken: result.argocdToken || '',
        argocdInsecure: result.argocdInsecure || false,
        snykToken: result.snykToken || '',
        trivyBinary: result.trivyBinary || '',
        falcoUrl: result.falcoUrl || '',
      }
    }
  } catch (e) {
    console.error('[settings] load failed:', e)
  } finally {
    loading.value = false
  }
}

async function saveSettings() {
  saving.value = true
  saveMessage.value = ''
  try {
    await callGo('UpdateSettings', {
      kubeconfigPath: form.value.kubeconfigPath,
      currentContext: form.value.currentContext,
      namespace: form.value.namespace,
      deepseekApiKey: form.value.deepseekApiKey,
      anomstackUrl: form.value.anomstackUrl,
      mcpServersConfig: form.value.mcpServersConfig,
      agentInstructions: form.value.agentInstructions,
      prometheusUrl: form.value.prometheusUrl,
      argocdUrl: form.value.argocdUrl,
      argocdToken: form.value.argocdToken,
      argocdInsecure: form.value.argocdInsecure,
      snykToken: form.value.snykToken,
      trivyBinary: form.value.trivyBinary,
      falcoUrl: form.value.falcoUrl,
    })
    saveMessage.value = 'Settings saved and applied immediately.'
    // Reload settings to get updated masked values.
    await loadSettings()
    // Reload contexts with new kubeconfig.
    await listContexts()
  } catch (e) {
    saveMessage.value = 'Error: ' + (e?.message || String(e))
  } finally {
    saving.value = false
    setTimeout(() => { saveMessage.value = '' }, 5000)
  }
}

async function testArgoCD() {
  argocdTesting.value = true
  argocdTestResult.value = ''
  try {
    await callGo('TestArgusCDConnection')
    argocdTestResult.value = 'Connected successfully'
  } catch (e) {
    argocdTestResult.value = 'Failed: ' + (e?.message || String(e))
  } finally {
    argocdTesting.value = false
    setTimeout(() => { argocdTestResult.value = '' }, 6000)
  }
}

onMounted(() => {
  loadSettings()
  listContexts()
})
</script>

<template>
  <div class="settings-view">
    <div class="header">
      <div class="header-text">
        <div class="title">Settings</div>
        <div class="subtitle">Runtime configuration — changes take effect immediately</div>
      </div>
    </div>

    <div class="scroll" v-if="!loading">
      <!-- Kubernetes Connection -->
      <div class="section">
        <div class="section-title">Kubernetes Connection</div>

        <div class="field">
          <label class="field-label">Kubeconfig Path</label>
          <input
            v-model="form.kubeconfigPath"
            type="text"
            class="field-input"
            placeholder="/Users/you/.kube/config (supports colon-separated multi-file)"
          />
          <div class="field-hint">
            Supports multi-file paths separated by <code>:</code> — e.g.
            <code>/path/local_config:/path/k3s-lab-config</code>.
            Leave empty to use the standard <code>KUBECONFIG</code> env var.
          </div>
        </div>

        <div class="field">
          <label class="field-label">Context</label>
          <div class="field-row">
            <select v-model="form.currentContext" class="field-select">
              <option value="">— auto (current context) —</option>
              <option v-for="ctx in contexts" :key="ctx.name" :value="ctx.name">
                {{ ctx.name }}{{ ctx.active ? ' (active)' : '' }}
              </option>
            </select>
            <button class="refresh-btn" @click="listContexts" :disabled="ctxLoading">
              {{ ctxLoading ? '…' : '↻' }}
            </button>
          </div>
        </div>

        <div class="field">
          <label class="field-label">Default Namespace</label>
          <input
            v-model="form.namespace"
            type="text"
            class="field-input"
            placeholder="(empty = all namespaces)"
          />
          <div class="field-hint">Filter to a single namespace, or leave empty to see all.</div>
        </div>
      </div>

      <!-- AI & Integrations -->
      <div class="section">
        <div class="section-title">AI & Integrations</div>

        <div class="field">
          <label class="field-label">DeepSeek API Key</label>
          <input
            v-model="form.deepseekApiKey"
            type="password"
            class="field-input mono"
            placeholder="sk-…"
          />
          <div class="field-hint">Used for AI diagnostics, auto-investigation, and the Argus AI chat. Saved to your local config file and applied immediately — no restart needed. <code>DEEPSEEK_API_KEY</code> is also honored as an env-var override.</div>
        </div>

        <div class="field">
          <label class="field-label">Anomstack URL</label>
          <input
            v-model="form.anomstackUrl"
            type="text"
            class="field-input mono"
            placeholder="http://localhost:8087"
          />
        </div>

        <div class="field">
          <label class="field-label">Agent Instructions</label>
          <textarea
            v-model="form.agentInstructions"
            class="field-input mono"
            rows="4"
            placeholder="Analyze the cluster health based on recent events and alerts."
          ></textarea>
          <div class="field-hint">Instructions for the hourly AI agent analysis loop.</div>
          <button class="test-btn" style="margin-top: 8px;" @click="callGo('TriggerAgentAnalysis')">Test Agent Analysis</button>
        </div>

        <div class="field">
          <label class="field-label">Prometheus URL</label>
          <input
            v-model="form.prometheusUrl"
            type="text"
            class="field-input mono"
            placeholder="http://prometheus:9090 (optional — enriches metrics)"
          />
        </div>

        <div class="field">
          <label class="field-label">MCP Servers Config</label>
          <textarea
            v-model="form.mcpServersConfig"
            class="field-input mono"
            rows="6"
            placeholder='{
  "mcpServers": {
    "my-server": {
      "command": "npx",
      "args": ["-y", "mcp-server"]
    }
  }
}'
          ></textarea>
          <div class="field-hint">JSON configuration for Model Context Protocol (MCP) servers. Passed to the AI assistant to extend its capabilities.</div>
        </div>
      </div>

      <!-- ArgusCD (Argo CD) -->
      <div class="section">
        <div class="section-title">ArgusCD — Argo CD Integration</div>

        <div class="field">
          <label class="field-label">Argo CD Server URL</label>
          <input
            v-model="form.argocdUrl"
            type="text"
            class="field-input mono"
            placeholder="https://argocd.example.com"
          />
          <div class="field-hint">
            The base URL of your Argo CD server API.
            Set <code>ARGOCD_URL</code> env var to persist across restarts.
          </div>
        </div>

        <div class="field">
          <label class="field-label">API Token</label>
          <input
            v-model="form.argocdToken"
            type="password"
            class="field-input mono"
            placeholder="eyJhbGciOi…"
          />
          <div class="field-hint">
            Argo CD API bearer token.
            Set <code>ARGOCD_TOKEN</code> env var to persist across restarts.
          </div>
        </div>

        <div class="field">
          <label class="field-label">TLS Verification</label>
          <label class="toggle-label">
            <input type="checkbox" v-model="form.argocdInsecure" class="toggle-checkbox" />
            <span class="toggle-text">Skip TLS certificate verification (insecure)</span>
          </label>
          <div class="field-hint">Enable for self-signed certs or in-cluster Argo CD with internal CA.</div>
        </div>

        <div class="field">
          <div class="field-row">
            <button class="test-btn" @click="testArgoCD" :disabled="argocdTesting || !form.argocdUrl">
              {{ argocdTesting ? 'Testing…' : 'Test Connection' }}
            </button>
            <span v-if="argocdTestResult" class="test-result" :class="{ ok: !argocdTestResult.startsWith('Failed'), fail: argocdTestResult.startsWith('Failed') }">
              {{ argocdTestResult }}
            </span>
          </div>
        </div>
      </div>

      <!-- Security Scanning Tools (all optional) -->
      <div class="section">
        <div class="section-title">Security Scanning Tools</div>
        <div class="section-hint">All tools are optional — configure the ones you use.</div>

        <div class="field">
          <label class="field-label">Snyk API Token</label>
          <input
            v-model="form.snykToken"
            type="password"
            class="field-input mono"
            placeholder="(optional)"
          />
          <div class="field-hint">
            Used for container image vulnerability scanning.
            Set <code>SNYK_TOKEN</code> env var to persist across restarts.
          </div>
        </div>

        <div class="field">
          <label class="field-label">Trivy Binary Path</label>
          <input
            v-model="form.trivyBinary"
            type="text"
            class="field-input mono"
            placeholder="trivy (default — uses $PATH)"
          />
          <div class="field-hint">
            Path to the <code>trivy</code> binary for filesystem and image scanning.
            Leave blank to use the default from <code>$PATH</code>.
            Set <code>KUBEWATCHER_TRIVY_BIN</code> env var to persist.
          </div>
        </div>

        <div class="field">
          <label class="field-label">Falco Endpoint</label>
          <input
            v-model="form.falcoUrl"
            type="text"
            class="field-input mono"
            placeholder="http://falco:8765 (optional)"
          />
          <div class="field-hint">
            Falco gRPC or HTTP endpoint for runtime threat detection events.
            Set <code>KUBEWATCHER_FALCO_URL</code> env var to persist.
          </div>
        </div>
      </div>

      <!-- Current State (read-only info) -->
      <div class="section" v-if="settings">
        <div class="section-title">Current State</div>
        <div class="info-grid">
          <div class="info-label">Tier</div>
          <div class="info-value">
            <span class="tier-badge" :class="settings.tier">{{ settings.tier }}</span>
          </div>
          <div class="info-label">MCP Servers</div>
          <div class="info-value">
            <span v-if="settings.mcpServersConfig" class="status-active">Configured</span>
            <span v-else class="text-muted">Not configured</span>
          </div>
          <div class="info-label">ArgusCD</div>
          <div class="info-value">
            <span v-if="settings.argocdUrl" class="mono">{{ settings.argocdUrl }}</span>
            <span v-else class="text-muted">Not configured</span>
          </div>
          <div class="info-label">Snyk</div>
          <div class="info-value">
            <span v-if="settings.snykToken" class="status-active">Configured</span>
            <span v-else class="text-muted">Not configured</span>
          </div>
          <div class="info-label">Trivy</div>
          <div class="info-value mono">{{ settings.trivyBinary || 'trivy' }}</div>
          <div class="info-label">Falco</div>
          <div class="info-value">
            <span v-if="settings.falcoUrl" class="mono">{{ settings.falcoUrl }}</span>
            <span v-else class="text-muted">Not configured</span>
          </div>
          <div class="info-label">Log Level</div>
          <div class="info-value mono">{{ settings.logLevel || 'info' }}</div>
        </div>
      </div>

      <!-- Save -->
      <div class="save-row">
        <button class="save-btn" @click="saveSettings" :disabled="saving">
          {{ saving ? 'Saving…' : 'Apply & Reconnect' }}
        </button>
        <div v-if="saveMessage" class="save-message" :class="{ error: saveMessage.startsWith('Error') }">
          {{ saveMessage }}
        </div>
      </div>
    </div>

    <div v-else class="loading-state">Loading settings…</div>
  </div>
</template>

<style scoped>
.settings-view { flex: 1; display: flex; flex-direction: column; overflow: hidden; }

.header {
  height: 50px; display: flex; align-items: center; padding: 0 20px;
  border-bottom: 1px solid var(--border); background: var(--bg2); flex-shrink: 0;
}
.title { font-size: 14px; font-weight: 600; color: var(--text); }
.subtitle { font-size: 11px; color: var(--text3); margin-top: 1px; }

.scroll { flex: 1; overflow-y: auto; padding: 20px; display: flex; flex-direction: column; gap: 24px; }

.section {
  background: var(--bg3); border: 1px solid var(--border); border-radius: 8px;
  padding: 16px 20px;
}
.section-title {
  font-size: 11px; font-weight: 600; text-transform: uppercase; letter-spacing: 0.06em;
  color: var(--text3); margin-bottom: 14px;
}

.field { margin-bottom: 14px; }
.field:last-child { margin-bottom: 0; }
.field-label { display: block; font-size: 12px; font-weight: 500; color: var(--text2); margin-bottom: 5px; }
.field-input {
  width: 100%; padding: 7px 10px; border-radius: 6px;
  border: 1px solid var(--border); background: var(--bg); color: var(--text);
  font-size: 12.5px; outline: none; transition: border-color 0.15s;
  box-sizing: border-box;
}
.field-input:focus { border-color: var(--accent); }
.field-input.mono { font-family: var(--mono); font-size: 12px; }
.field-input::placeholder { color: var(--text3); }

.field-select {
  flex: 1; padding: 7px 10px; border-radius: 6px;
  border: 1px solid var(--border); background: var(--bg); color: var(--text);
  font-size: 12.5px; outline: none; appearance: none;
  background-image: url("data:image/svg+xml;charset=utf-8,%3Csvg xmlns='http://www.w3.org/2000/svg' width='10' height='6'%3E%3Cpath d='M1 1l4 4 4-4' stroke='%235c6168' fill='none' stroke-width='1.3'/%3E%3C/svg%3E");
  background-repeat: no-repeat; background-position: right 10px center;
  padding-right: 28px;
}
.field-select:focus { border-color: var(--accent); }

.field-row { display: flex; gap: 6px; align-items: center; }

.refresh-btn {
  width: 32px; height: 32px; border-radius: 6px; border: 1px solid var(--border);
  background: var(--bg); color: var(--text2); cursor: pointer; font-size: 14px;
  display: flex; align-items: center; justify-content: center; transition: all 0.15s;
}
.refresh-btn:hover { background: var(--bg4); color: var(--text); }

.field-hint { font-size: 11px; color: var(--text3); margin-top: 4px; line-height: 1.4; }
.field-hint code {
  background: var(--bg); padding: 1px 4px; border-radius: 3px;
  font-family: var(--mono); font-size: 10.5px; color: var(--text2);
}

.info-grid { display: grid; grid-template-columns: 120px 1fr; gap: 8px 12px; align-items: center; }
.info-label { font-size: 12px; color: var(--text3); }
.info-value { font-size: 12.5px; color: var(--text); }
.info-value.mono { font-family: var(--mono); font-size: 12px; }

.tier-badge {
  display: inline-block; padding: 2px 8px; border-radius: 10px;
  font-size: 11px; font-weight: 600; text-transform: uppercase; letter-spacing: 0.04em;
}
.tier-badge.pro { background: rgba(79,142,247,0.15); color: var(--accent2); }
.tier-badge.free { background: var(--bg4); color: var(--text2); }

.toggle-label { display: flex; align-items: center; gap: 8px; cursor: pointer; }
.toggle-checkbox { accent-color: var(--accent); width: 14px; height: 14px; cursor: pointer; }
.toggle-text { font-size: 12.5px; color: var(--text2); }

.test-btn {
  padding: 6px 14px; border-radius: 6px; border: 1px solid var(--border);
  background: var(--bg); color: var(--text2); font-size: 12px; font-weight: 500;
  cursor: pointer; transition: all 0.15s;
}
.test-btn:hover { background: var(--bg4); color: var(--text); }
.test-btn:disabled { opacity: 0.5; cursor: not-allowed; }

.test-result { font-size: 12px; }
.test-result.ok { color: var(--green); }
.test-result.fail { color: var(--red); }

.save-row { display: flex; align-items: center; gap: 12px; padding-bottom: 20px; }
.save-btn {
  padding: 8px 20px; border-radius: 6px; border: none;
  background: var(--accent); color: white; font-size: 12.5px; font-weight: 500;
  cursor: pointer; transition: all 0.15s;
}
.save-btn:hover { background: var(--accent2); }
.save-btn:disabled { opacity: 0.5; cursor: not-allowed; }

.save-message { font-size: 12px; color: var(--green); }
.save-message.error { color: var(--red); }

.text-muted { color: var(--text3); font-style: italic; }
.status-active { color: var(--green); font-size: 12px; font-weight: 500; }

.section-hint { font-size: 11px; color: var(--text3); margin-top: -8px; margin-bottom: 12px; font-style: italic; }

.loading-state { flex: 1; display: flex; align-items: center; justify-content: center; color: var(--text3); font-size: 13px; }
</style>
