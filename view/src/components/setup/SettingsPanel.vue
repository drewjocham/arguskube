<script setup>
import { ref, onMounted, reactive } from 'vue'
import { storeToRefs } from 'pinia'
import { callGo, useContexts } from '../../composables/useWails'
import { useAppearanceStore } from '../../stores/appearance'
import { useNotificationsStore } from '../../stores/notifications'
import { useSpotCheck } from '../../composables/useSpotCheck'

// Agent profile — what Argus is allowed to do without asking. Defaults
// are conservative (he documents but never mutates). Backend sanitizes
// on save, so we only need rough validation here.
const agentProfile = reactive({
  autoInvestigate: true,
  autoDocument: true,
  canAck: false,
  canSilence: false,
  canAdjustParams: false,
  silenceWindow: 3600000000000, // ns — 1h
  fatigueThreshold: 5,
})
const silenceWindowMin = ref(60) // exposed in minutes for the UI
const agentProfileSaving = ref(false)
const agentProfileMsg = ref('')

async function loadAgentProfile() {
  try {
    const p = await callGo('GetAgentProfile')
    if (p && typeof p === 'object') {
      Object.assign(agentProfile, p)
      silenceWindowMin.value = Math.max(1, Math.round((p.silenceWindow || 3600000000000) / 60000000000))
    }
  } catch (e) {
    console.warn('[settings] GetAgentProfile failed:', e)
  }
}

async function saveAgentProfile() {
  agentProfileSaving.value = true
  agentProfileMsg.value = ''
  try {
    // Convert UI minutes back to nanoseconds for the Go duration field.
    const payload = {
      ...agentProfile,
      silenceWindow: Math.max(1, Math.min(24 * 60, silenceWindowMin.value)) * 60 * 1e9,
    }
    await callGo('SetAgentProfile', payload)
    Object.assign(agentProfile, payload)
    agentProfileMsg.value = 'Saved.'
  } catch (e) {
    agentProfileMsg.value = 'Error: ' + (e?.message || e)
  } finally {
    agentProfileSaving.value = false
    setTimeout(() => { agentProfileMsg.value = '' }, 4000)
  }
}

// Appearance — entirely client-side: the values live in localStorage
// and apply to <html> via the appearance store. No round-trip through
// the Go backend, so the UI updates as the user drags.
const appearance = useAppearanceStore()
// storeToRefs only handles reactive refs; plain objects exposed by the
// setup-style store (ranges, densities) come back undefined if you
// destructure them through it. Pull those off the store directly.
const { theme: appTheme, brightness, contrast, opacity, blur, saturation, density } = storeToRefs(appearance)
const ranges = appearance.ranges

// Notifications: cap setting + manual spot-check trigger.
const notifStore = useNotificationsStore()
const { settings: notifSettings } = storeToRefs(notifStore)
const draftNotifMax = ref(notifSettings.value.maxItems)
function applyNotifMax() {
  notifStore.setMaxItems(draftNotifMax.value)
  draftNotifMax.value = notifStore.settings.maxItems // reflect clamping
}

const { runAll: runSpotChecksNow } = useSpotCheck()
const spotChecksRunning = ref(false)
async function triggerSpotChecks() {
  spotChecksRunning.value = true
  try {
    await runSpotChecksNow()
  } finally {
    // The backend kicks off async; clear the flag after a short
    // grace so the button doesn't stay disabled forever if no events
    // come back.
    setTimeout(() => { spotChecksRunning.value = false }, 1500)
  }
}

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
  loadAgentProfile()
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
      <!-- Appearance — theme + visual feel. All client-side; persisted
           in localStorage and applied via CSS custom properties. -->
      <div class="section">
        <div class="section-title">Appearance</div>

        <div class="field">
          <label class="field-label">Theme</label>
          <div class="theme-row">
            <button
              type="button"
              class="theme-chip"
              :class="{ active: appTheme === 'dark' }"
              @click="appearance.setTheme('dark')"
            >
              <span class="theme-swatch" data-swatch="dark"></span>
              Dark
            </button>
            <button
              type="button"
              class="theme-chip"
              :class="{ active: appTheme === 'light' }"
              @click="appearance.setTheme('light')"
            >
              <span class="theme-swatch" data-swatch="light"></span>
              Light
            </button>
            <button
              type="button"
              class="theme-chip"
              :class="{ active: appTheme === 'auto' }"
              @click="appearance.setTheme('auto')"
            >
              <span class="theme-swatch" data-swatch="auto"></span>
              Auto (OS)
            </button>
          </div>
          <div class="field-hint">
            Auto follows your operating system's light/dark setting.
            <strong>Light is experimental</strong> — the palette swaps for
            components that use the design tokens, but a number of legacy
            components still hardcode dark colors and won't change. A full
            migration is on the roadmap.
          </div>
        </div>

        <div class="field">
          <div class="slider-head">
            <label class="field-label">Brightness</label>
            <span class="slider-value">{{ brightness }}%</span>
          </div>
          <input
            type="range"
            class="slider"
            :min="ranges.brightness[0]"
            :max="ranges.brightness[1]"
            :value="brightness"
            @input="appearance.setBrightness(Number($event.target.value))"
          />
          <div class="field-hint">Dim or brighten the entire UI without changing the theme.</div>
        </div>

        <div class="field">
          <div class="slider-head">
            <label class="field-label">Contrast</label>
            <span class="slider-value">{{ contrast }}%</span>
          </div>
          <input
            type="range"
            class="slider"
            :min="ranges.contrast[0]"
            :max="ranges.contrast[1]"
            :value="contrast"
            @input="appearance.setContrast(Number($event.target.value))"
          />
          <div class="field-hint">Push contrast up for sharper text, down for softer surfaces.</div>
        </div>

        <div class="field">
          <div class="slider-head">
            <label class="field-label">Saturation <span class="hint-inline">— shiny vs. dull</span></label>
            <span class="slider-value">{{ saturation }}%</span>
          </div>
          <input
            type="range"
            class="slider"
            :min="ranges.saturation[0]"
            :max="ranges.saturation[1]"
            :value="saturation"
            @input="appearance.setSaturation(Number($event.target.value))"
          />
          <div class="field-hint">0% = grayscale, 100% = default, 200% = vivid.</div>
        </div>

        <div class="field">
          <div class="slider-head">
            <label class="field-label">Window Opacity</label>
            <span class="slider-value">{{ opacity }}%</span>
          </div>
          <input
            type="range"
            class="slider"
            :min="ranges.opacity[0]"
            :max="ranges.opacity[1]"
            :value="opacity"
            @input="appearance.setOpacity(Number($event.target.value))"
          />
          <div class="field-hint">Useful when the desktop window is translucent (Wails).</div>
        </div>

        <div class="field">
          <div class="slider-head">
            <label class="field-label">Window Blur Radius</label>
            <span class="slider-value">{{ blur }}px</span>
          </div>
          <input
            type="range"
            class="slider"
            :min="ranges.blur[0]"
            :max="ranges.blur[1]"
            :value="blur"
            @input="appearance.setBlur(Number($event.target.value))"
          />
          <div class="field-hint">Softens whatever shows behind a translucent window.</div>
        </div>

        <div class="field">
          <label class="field-label">UI Density</label>
          <div class="theme-row">
            <button
              v-for="d in ['compact', 'normal', 'comfortable']"
              :key="d"
              type="button"
              class="theme-chip"
              :class="{ active: density === d }"
              @click="appearance.setDensity(d)"
            >{{ d.charAt(0).toUpperCase() + d.slice(1) }}</button>
          </div>
          <div class="field-hint">Scales typography and padding across the app.</div>
        </div>

        <div class="field">
          <button class="test-btn" @click="appearance.reset()">Reset to defaults</button>
        </div>
      </div>

      <!-- Notifications & Spot-checks -->
      <div class="section">
        <div class="section-title">Notifications</div>

        <div class="field">
          <label class="field-label">Max notifications kept</label>
          <div class="field-row">
            <input
              v-model.number="draftNotifMax"
              type="number"
              min="1"
              max="5000"
              step="50"
              class="field-input"
              style="max-width: 140px;"
            />
            <button class="test-btn" @click="applyNotifMax">Apply</button>
          </div>
          <div class="field-hint">
            Argus keeps the last <strong>{{ notifSettings.maxItems }}</strong> spot-check
            findings, scan summaries, and alerts in the bell panel. Older
            entries roll off automatically. Cap is between 1 and 5000.
          </div>
        </div>

        <div class="field">
          <label class="field-label">Spot-checks</label>
          <div class="field-row">
            <button
              class="test-btn"
              :disabled="spotChecksRunning"
              @click="triggerSpotChecks"
            >{{ spotChecksRunning ? 'Running…' : 'Run spot-checks now' }}</button>
          </div>
          <div class="field-hint">
            Runs every registered probe (nodes, cluster metrics, decision-log
            freshness) and posts findings to the notifications panel. Argus
            also runs these every 30 minutes automatically.
          </div>
        </div>
      </div>

      <!-- Agent Profile — what Argus is allowed to do without asking. -->
      <div class="section">
        <div class="section-title">Agent Profile</div>
        <div class="section-hint">
          Argus always documents what he sees. The toggles below decide what,
          if anything, he can <em>change</em> on your behalf. Defaults are
          read-only — he never mutates alerts without permission.
        </div>

        <div class="field">
          <label class="toggle-label">
            <input type="checkbox" class="toggle-checkbox" v-model="agentProfile.autoInvestigate" />
            <span class="toggle-text">
              <strong>Auto-investigate new alerts.</strong>
              When a new alert fires, Argus runs a diagnosis and saves the findings.
            </span>
          </label>
        </div>

        <div class="field">
          <label class="toggle-label">
            <input type="checkbox" class="toggle-checkbox" v-model="agentProfile.autoDocument" />
            <span class="toggle-text">
              <strong>Auto-document.</strong>
              Even with the AI off, log every fire / silence / dismissal so the
              fatigue detector has data.
            </span>
          </label>
        </div>

        <div class="field">
          <label class="toggle-label">
            <input type="checkbox" class="toggle-checkbox" v-model="agentProfile.canAck" />
            <span class="toggle-text">
              <strong>Allow agent to ack alerts.</strong>
              Off by default. With this on, Argus may acknowledge alerts after
              investigation; he still records the rationale.
            </span>
          </label>
        </div>

        <div class="field">
          <label class="toggle-label">
            <input type="checkbox" class="toggle-checkbox" v-model="agentProfile.canSilence" />
            <span class="toggle-text">
              <strong>Allow agent to silence noisy alerts.</strong>
              Off by default. With this on, Argus may silence a signature he
              identifies as repeated noise. The fatigue detector recommends
              enabling this if alerts keep getting dismissed.
            </span>
          </label>
        </div>

        <div class="field">
          <label class="toggle-label">
            <input type="checkbox" class="toggle-checkbox" v-model="agentProfile.canAdjustParams" />
            <span class="toggle-text">
              <strong>Allow agent to adjust alert parameters.</strong>
              Off by default. With this on, Argus may apply (not just suggest)
              threshold tweaks for chronically noisy alerts.
            </span>
          </label>
        </div>

        <div class="field">
          <label class="field-label">Default silence window (minutes)</label>
          <input
            type="number"
            class="field-input"
            min="1"
            max="1440"
            step="5"
            v-model.number="silenceWindowMin"
            style="max-width: 140px;"
          />
          <div class="field-hint">
            How long a silenced alert stays suppressed. Capped at 24 hours.
          </div>
        </div>

        <div class="field">
          <label class="field-label">Fatigue threshold</label>
          <input
            type="number"
            class="field-input"
            min="1"
            max="100"
            step="1"
            v-model.number="agentProfile.fatigueThreshold"
            style="max-width: 140px;"
          />
          <div class="field-hint">
            After this many silences/dismissals of the same alert signature,
            Argus fires the "alerts losing value" warning. Default 5.
          </div>
        </div>

        <div class="field">
          <div class="field-row">
            <button class="test-btn" :disabled="agentProfileSaving" @click="saveAgentProfile">
              {{ agentProfileSaving ? 'Saving…' : 'Apply agent profile' }}
            </button>
            <span v-if="agentProfileMsg" class="test-result" :class="{ ok: !agentProfileMsg.startsWith('Error'), fail: agentProfileMsg.startsWith('Error') }">
              {{ agentProfileMsg }}
            </span>
          </div>
        </div>
      </div>

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

/* --- Appearance section ------------------------------------------------ */

.theme-row { display: flex; flex-wrap: wrap; gap: 8px; margin-bottom: 4px; }
.theme-chip {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  padding: 8px 12px;
  border-radius: 8px;
  border: 1px solid var(--border);
  background: var(--bg);
  color: var(--text2);
  font-size: 12.5px;
  font-weight: 500;
  cursor: pointer;
  transition: border-color 0.15s, background 0.15s, color 0.15s;
}
.theme-chip:hover { background: var(--bg4); color: var(--text); }
.theme-chip.active {
  border-color: var(--accent);
  background: rgba(79,142,247,0.12);
  color: var(--text);
}

.theme-swatch {
  display: inline-block;
  width: 18px;
  height: 18px;
  border-radius: 50%;
  border: 1px solid var(--border2);
}
.theme-swatch[data-swatch="dark"]  { background: linear-gradient(135deg, #1a1c1e 50%, #2f3236 50%); }
.theme-swatch[data-swatch="light"] { background: linear-gradient(135deg, #f7f8fa 50%, #e2e6eb 50%); }
.theme-swatch[data-swatch="auto"]  { background: linear-gradient(135deg, #1a1c1e 50%, #f7f8fa 50%); }

.slider-head {
  display: flex;
  justify-content: space-between;
  align-items: baseline;
  margin-bottom: 6px;
}
.slider-value {
  font-family: var(--mono);
  font-size: 11.5px;
  color: var(--text2);
  font-variant-numeric: tabular-nums;
}
.hint-inline { color: var(--text3); font-weight: 400; font-size: 11px; }

.slider {
  -webkit-appearance: none;
  appearance: none;
  width: 100%;
  height: 4px;
  border-radius: 999px;
  background: var(--bg4);
  outline: none;
  cursor: pointer;
}
.slider::-webkit-slider-thumb {
  -webkit-appearance: none;
  appearance: none;
  width: 16px;
  height: 16px;
  border-radius: 50%;
  background: var(--accent);
  border: 2px solid var(--bg2);
  box-shadow: 0 1px 4px rgba(0,0,0,0.3);
  cursor: grab;
  transition: transform 0.1s;
}
.slider::-webkit-slider-thumb:active { cursor: grabbing; transform: scale(1.1); }
.slider::-moz-range-thumb {
  width: 16px;
  height: 16px;
  border-radius: 50%;
  background: var(--accent);
  border: 2px solid var(--bg2);
  box-shadow: 0 1px 4px rgba(0,0,0,0.3);
  cursor: grab;
}
</style>
