<script setup>
import { ref, computed, onMounted } from 'vue'
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
  llmBaseUrl: '',
  llmModel: '',
  anomstackUrl: '',
  prometheusUrl: '',
  argocdUrl: '',
  argocdToken: '',
  argocdInsecure: false,
  snykToken: '',
  trivyBinary: '',
  falcoUrl: '',
  // Pipelines / CI-CD
  pipelinesEnabled: false,
  pipelinesProvider: '',
  githubToken: '',
  githubOwner: '',
  githubRepo: '',
  githubWorkflow: '',
  gitlabUrl: '',
  gitlabToken: '',
  gitlabProjectId: '',
  gitlabRef: '',
  awsRegion: '',
  awsAccessKey: '',
  awsSecretKey: '',
  awsProject: '',
  gcpProject: '',
  gcpRegion: '',
  gcpCredentials: '',
  circleciToken: '',
  circleciProjectSlug: '',
  azureOrganization: '',
  azureProject: '',
  azurePipelineId: '',
  azureToken: '',
  azureBranch: '',
  notifyOnPrOpened: false,
  notifyOnPrUpdated: false,
  notifyOnPrCommented: false,
  notifyOnPrMerged: false,
  autoCodeReview: false,
  codeReviewDestination: 'local',
  gdriveFolderId: '',
  codeReviewS3Prefix: '',
  codeReviewEmailTo: '',
  confluenceUrl: '',
  confluenceEmail: '',
  confluenceToken: '',
  confluenceSpaceKey: '',
  confluenceParentPageId: '',
  notionToken: '',
  notionDatabaseId: '',
  evernoteToken: '',
  evernoteNotebookGuid: '',
  onenoteToken: '',
  onenoteSectionId: '',
  amplenoteApiKey: '',
  standardNotesUrl: '',
  standardNotesToken: '',
  obsidianVaultPath: '',
  joplinUrl: '',
  joplinToken: '',
  logseqGraphPath: '',
  bearToken: '',
})

const REVIEW_DESTINATIONS = [
  { id: 'local',          label: 'In-app only',    blurb: 'Reports stay in the Reports tab on this machine.' },
  { id: 'gdrive',         label: 'Google Drive',   blurb: 'Upload to a Drive folder you own. Requires Drive auth.' },
  { id: 's3',             label: 'S3',             blurb: 'Upload as Markdown into the S3 bucket configured for KubeWatcher.' },
  { id: 'email',          label: 'Email',          blurb: 'Send the review as a Markdown email to the recipients below.' },
  { id: 'confluence',     label: 'Confluence',     blurb: 'Create or update a Confluence page under a space you own.' },
  { id: 'notion',         label: 'Notion',         blurb: 'Create or update a page in a Notion database. Needs an integration token.' },
  { id: 'evernote',       label: 'Evernote',       blurb: 'Create a note in your Evernote account.' },
  { id: 'onenote',        label: 'Microsoft OneNote', blurb: 'Create a page in a OneNote section via Microsoft Graph.' },
  { id: 'amplenote',      label: 'Amplenote',      blurb: 'Create a note via the Amplenote REST API.' },
  { id: 'standard-notes', label: 'Standard Notes', blurb: 'Create a note on a Standard Notes server (cloud or self-hosted).' },
  { id: 'obsidian',       label: 'Obsidian',       blurb: 'Drop a Markdown file directly into a local Obsidian vault.' },
  { id: 'joplin',         label: 'Joplin',         blurb: 'Create a note via the Joplin Web Clipper API on localhost.' },
  { id: 'logseq',         label: 'Logseq',         blurb: 'Drop a Markdown page directly into a local Logseq graph.' },
  { id: 'bear',           label: 'Bear',           blurb: 'Create a Bear note via x-callback-url (macOS only).' },
]

// Catalog of supported providers. Order is the order they're shown in the UI.
const PIPELINE_PROVIDERS = [
  {
    id: 'github',
    name: 'GitHub Actions',
    blurb: 'Trigger workflow_dispatch on a user\'s repo. Best when your SaaS integrates with users\' GitHub repos.',
    auth: 'GitHub App (OAuth) or Personal Access Token',
  },
  {
    id: 'gitlab',
    name: 'GitLab CI/CD',
    blurb: 'POST to the Pipeline Triggers API. Best for self-hosted GitLab and unified source/CI workflows.',
    auth: 'Trigger Token, Project Access Token, or OAuth 2.0',
  },
  {
    id: 'aws-codebuild',
    name: 'AWS CodeBuild',
    blurb: 'StartBuild via the AWS SDK. Best for AWS-native SaaS with deep IAM integration.',
    auth: 'AWS IAM (access key + secret, or assumed role)',
  },
  {
    id: 'gcp-cloudbuild',
    name: 'Google Cloud Build',
    blurb: 'REST/gRPC builds for workloads inside GCP. Best for container builds in the Google ecosystem.',
    auth: 'GCP service account (JSON key)',
  },
  {
    id: 'circleci',
    name: 'CircleCI',
    blurb: 'POST to /api/v2/project/{slug}/pipeline. Fast, isolated SaaS-only job execution.',
    auth: 'API Token (header)',
  },
  {
    id: 'azure',
    name: 'Azure Pipelines',
    blurb: 'Trigger a run via the Azure DevOps Pipelines REST API. Best for Azure-native teams and Microsoft ecosystems.',
    auth: 'Personal Access Token (Basic auth) or service principal',
  },
]

const argocdTesting = ref(false)
const argocdTestResult = ref('')

const { contexts, loading: ctxLoading, listContexts } = useContexts()

// Billing & Usage state
const usage = ref(null)
const usageLoading = ref(false)
const usageError = ref('')
const billing = ref({ inputCostPer1M: 0, outputCostPer1M: 0, monthlyBudget: 0 })
const billingSaving = ref(false)
const billingSaveMsg = ref('')

function fmtNumber(n) {
  if (n === undefined || n === null) return '—'
  return n.toLocaleString()
}
function fmtCost(usd) {
  if (!Number.isFinite(usd) || usd === 0) return '—'
  return '$' + usd.toFixed(usd < 1 ? 4 : 2)
}
function fmtTimestamp(ts) {
  if (!ts) return '—'
  try { return new Date(ts).toLocaleString() } catch { return String(ts) }
}

const monthBudgetUsedPct = computed(() => {
  if (!usage.value || !billing.value.monthlyBudget) return 0
  const used = usage.value.month?.estCostUsd || 0
  return Math.min(100, Math.round((used / billing.value.monthlyBudget) * 100))
})

async function loadUsage() {
  usageLoading.value = true
  usageError.value = ''
  try {
    usage.value = await callGo('GetUsageSummary')
    if (usage.value) {
      // Mirror server-side billing values for the form so the rates input edits
      // the same numbers being displayed in the cost columns.
      billing.value = {
        inputCostPer1M: usage.value.rates?.InputPerMTokens ?? 0,
        outputCostPer1M: usage.value.rates?.OutputPerMTokens ?? 0,
        monthlyBudget: usage.value.monthlyBudget ?? 0,
      }
    }
  } catch (e) {
    usageError.value = e?.message || String(e)
  } finally {
    usageLoading.value = false
  }
}

async function saveBilling() {
  billingSaving.value = true
  billingSaveMsg.value = ''
  try {
    await callGo(
      'UpdateBillingRates',
      Number(billing.value.inputCostPer1M) || 0,
      Number(billing.value.outputCostPer1M) || 0,
      Number(billing.value.monthlyBudget) || 0,
    )
    billingSaveMsg.value = 'Saved'
    await loadUsage()
  } catch (e) {
    billingSaveMsg.value = 'Error: ' + (e?.message || String(e))
  } finally {
    billingSaving.value = false
    setTimeout(() => { billingSaveMsg.value = '' }, 4000)
  }
}

async function clearUsage() {
  if (!confirm('Erase all recorded LLM usage history? This cannot be undone.')) return
  try {
    await callGo('ClearUsageHistory')
    await loadUsage()
  } catch (e) {
    alert('Clear failed: ' + (e?.message || e))
  }
}

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
        llmBaseUrl: result.llmBaseUrl || '',
        llmModel: result.llmModel || '',
        anomstackUrl: result.anomstackUrl || '',
        prometheusUrl: result.prometheusUrl || '',
        argocdUrl: result.argocdUrl || '',
        argocdToken: result.argocdToken || '',
        argocdInsecure: result.argocdInsecure || false,
        snykToken: result.snykToken || '',
        trivyBinary: result.trivyBinary || '',
        falcoUrl: result.falcoUrl || '',
        pipelinesEnabled: result.pipelinesEnabled || false,
        pipelinesProvider: result.pipelinesProvider || '',
        githubToken: result.githubToken || '',
        githubOwner: result.githubOwner || '',
        githubRepo: result.githubRepo || '',
        githubWorkflow: result.githubWorkflow || '',
        gitlabUrl: result.gitlabUrl || '',
        gitlabToken: result.gitlabToken || '',
        gitlabProjectId: result.gitlabProjectId || '',
        gitlabRef: result.gitlabRef || '',
        awsRegion: result.awsRegion || '',
        awsAccessKey: result.awsAccessKey || '',
        awsSecretKey: result.awsSecretKey || '',
        awsProject: result.awsProject || '',
        gcpProject: result.gcpProject || '',
        gcpRegion: result.gcpRegion || '',
        gcpCredentials: result.gcpCredentials || '',
        circleciToken: result.circleciToken || '',
        circleciProjectSlug: result.circleciProjectSlug || '',
        azureOrganization: result.azureOrganization || '',
        azureProject: result.azureProject || '',
        azurePipelineId: result.azurePipelineId || '',
        azureToken: result.azureToken || '',
        azureBranch: result.azureBranch || '',
        notifyOnPrOpened: result.notifyOnPrOpened || false,
        notifyOnPrUpdated: result.notifyOnPrUpdated || false,
        notifyOnPrCommented: result.notifyOnPrCommented || false,
        notifyOnPrMerged: result.notifyOnPrMerged || false,
        autoCodeReview: result.autoCodeReview || false,
        codeReviewDestination: result.codeReviewDestination || 'local',
        gdriveFolderId: result.gdriveFolderId || '',
        codeReviewS3Prefix: result.codeReviewS3Prefix || '',
        codeReviewEmailTo: result.codeReviewEmailTo || '',
        confluenceUrl: result.confluenceUrl || '',
        confluenceEmail: result.confluenceEmail || '',
        confluenceToken: result.confluenceToken || '',
        confluenceSpaceKey: result.confluenceSpaceKey || '',
        confluenceParentPageId: result.confluenceParentPageId || '',
        notionToken: result.notionToken || '',
        notionDatabaseId: result.notionDatabaseId || '',
        evernoteToken: result.evernoteToken || '',
        evernoteNotebookGuid: result.evernoteNotebookGuid || '',
        onenoteToken: result.onenoteToken || '',
        onenoteSectionId: result.onenoteSectionId || '',
        amplenoteApiKey: result.amplenoteApiKey || '',
        standardNotesUrl: result.standardNotesUrl || '',
        standardNotesToken: result.standardNotesToken || '',
        obsidianVaultPath: result.obsidianVaultPath || '',
        joplinUrl: result.joplinUrl || '',
        joplinToken: result.joplinToken || '',
        logseqGraphPath: result.logseqGraphPath || '',
        bearToken: result.bearToken || '',
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
      llmBaseUrl: form.value.llmBaseUrl,
      llmModel: form.value.llmModel,
      anomstackUrl: form.value.anomstackUrl,
      prometheusUrl: form.value.prometheusUrl,
      argocdUrl: form.value.argocdUrl,
      argocdToken: form.value.argocdToken,
      argocdInsecure: form.value.argocdInsecure,
      snykToken: form.value.snykToken,
      trivyBinary: form.value.trivyBinary,
      falcoUrl: form.value.falcoUrl,
      pipelinesEnabled: form.value.pipelinesEnabled,
      pipelinesProvider: form.value.pipelinesProvider,
      githubToken: form.value.githubToken,
      githubOwner: form.value.githubOwner,
      githubRepo: form.value.githubRepo,
      githubWorkflow: form.value.githubWorkflow,
      gitlabUrl: form.value.gitlabUrl,
      gitlabToken: form.value.gitlabToken,
      gitlabProjectId: form.value.gitlabProjectId,
      gitlabRef: form.value.gitlabRef,
      awsRegion: form.value.awsRegion,
      awsAccessKey: form.value.awsAccessKey,
      awsSecretKey: form.value.awsSecretKey,
      awsProject: form.value.awsProject,
      gcpProject: form.value.gcpProject,
      gcpRegion: form.value.gcpRegion,
      gcpCredentials: form.value.gcpCredentials,
      circleciToken: form.value.circleciToken,
      circleciProjectSlug: form.value.circleciProjectSlug,
      azureOrganization: form.value.azureOrganization,
      azureProject: form.value.azureProject,
      azurePipelineId: form.value.azurePipelineId,
      azureToken: form.value.azureToken,
      azureBranch: form.value.azureBranch,
      notifyOnPrOpened: form.value.notifyOnPrOpened,
      notifyOnPrUpdated: form.value.notifyOnPrUpdated,
      notifyOnPrCommented: form.value.notifyOnPrCommented,
      notifyOnPrMerged: form.value.notifyOnPrMerged,
      autoCodeReview: form.value.autoCodeReview,
      codeReviewDestination: form.value.codeReviewDestination,
      gdriveFolderId: form.value.gdriveFolderId,
      codeReviewS3Prefix: form.value.codeReviewS3Prefix,
      codeReviewEmailTo: form.value.codeReviewEmailTo,
      confluenceUrl: form.value.confluenceUrl,
      confluenceEmail: form.value.confluenceEmail,
      confluenceToken: form.value.confluenceToken,
      confluenceSpaceKey: form.value.confluenceSpaceKey,
      confluenceParentPageId: form.value.confluenceParentPageId,
      notionToken: form.value.notionToken,
      notionDatabaseId: form.value.notionDatabaseId,
      evernoteToken: form.value.evernoteToken,
      evernoteNotebookGuid: form.value.evernoteNotebookGuid,
      onenoteToken: form.value.onenoteToken,
      onenoteSectionId: form.value.onenoteSectionId,
      amplenoteApiKey: form.value.amplenoteApiKey,
      standardNotesUrl: form.value.standardNotesUrl,
      standardNotesToken: form.value.standardNotesToken,
      obsidianVaultPath: form.value.obsidianVaultPath,
      joplinUrl: form.value.joplinUrl,
      joplinToken: form.value.joplinToken,
      logseqGraphPath: form.value.logseqGraphPath,
      bearToken: form.value.bearToken,
    })
    saveMessage.value = 'Settings saved. Cluster reconnected.'
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
  loadUsage()
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
          <label class="field-label">LLM API Key (Bearer)</label>
          <input
            v-model="form.deepseekApiKey"
            type="password"
            class="field-input mono"
            placeholder="sk-… or self-hosted vLLM bearer token"
          />
          <div class="field-hint">
            Sent as <code>Authorization: Bearer …</code> to the inference endpoint.
            For DeepSeek's hosted API leave the base URL below empty;
            for a self-hosted vLLM (vast.ai/GCP, see <code>infra/</code>), set this to
            the same token <code>llm_api_key</code> the IaC was deployed with.
            Set <code>DEEPSEEK_API_KEY</code> env var to persist.
          </div>
        </div>

        <div class="field">
          <label class="field-label">LLM Base URL</label>
          <input
            v-model="form.llmBaseUrl"
            type="text"
            class="field-input mono"
            placeholder="(empty = DeepSeek hosted) — http://1.2.3.4:8000/v1"
          />
          <div class="field-hint">
            Override to point at a self-hosted OpenAI-compatible server.
            Use the URL printed by <code>infra/vastai/llm_up.py</code> or the
            <code>endpoint</code> output from <code>terraform apply</code>.
            Append <code>/v1</code>. Set <code>KUBEWATCHER_LLM_BASE_URL</code> env var to persist.
          </div>
        </div>

        <div class="field">
          <label class="field-label">LLM Model</label>
          <input
            v-model="form.llmModel"
            type="text"
            class="field-input mono"
            placeholder="(empty = deepseek-chat) — meta-llama/Llama-3.1-8B-Instruct"
          />
          <div class="field-hint">
            Model id sent in the chat-completion request. Must match what your
            self-hosted vLLM was started with. Set <code>KUBEWATCHER_LLM_MODEL</code> env var to persist.
          </div>
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
          <label class="field-label">Prometheus URL</label>
          <input
            v-model="form.prometheusUrl"
            type="text"
            class="field-input mono"
            placeholder="http://prometheus:9090 (optional — enriches metrics)"
          />
        </div>
      </div>

      <!-- Billing & Usage -->
      <div class="section">
        <div class="section-title">Billing &amp; Usage</div>
        <div class="section-hint">
          LLM token totals are recorded per model on this machine. Set per-1M-token rates
          to estimate cost; the pay-as-you-go billing tier uses the same numbers.
        </div>

        <div v-if="usageLoading" class="usage-loading">Loading usage…</div>
        <div v-else-if="usageError" class="usage-error">Failed to load: {{ usageError }}</div>
        <div v-else-if="usage">
          <div class="usage-grid">
            <div class="usage-card">
              <div class="usage-label">Today</div>
              <div class="usage-value">{{ fmtNumber((usage.today?.in || 0) + (usage.today?.out || 0)) }}<span class="usage-unit">tokens</span></div>
              <div class="usage-sub">
                <span>{{ fmtNumber(usage.today?.calls) }} calls</span>
                <span class="dot">·</span>
                <span>{{ fmtCost(usage.today?.estCostUsd) }}</span>
              </div>
            </div>
            <div class="usage-card">
              <div class="usage-label">This month</div>
              <div class="usage-value">{{ fmtNumber((usage.month?.in || 0) + (usage.month?.out || 0)) }}<span class="usage-unit">tokens</span></div>
              <div class="usage-sub">
                <span>{{ fmtNumber(usage.month?.calls) }} calls</span>
                <span class="dot">·</span>
                <span>{{ fmtCost(usage.month?.estCostUsd) }}</span>
              </div>
            </div>
            <div class="usage-card">
              <div class="usage-label">Lifetime</div>
              <div class="usage-value">{{ fmtNumber((usage.lifetime?.in || 0) + (usage.lifetime?.out || 0)) }}<span class="usage-unit">tokens</span></div>
              <div class="usage-sub">
                <span>{{ fmtNumber(usage.lifetime?.calls) }} calls</span>
                <span class="dot">·</span>
                <span>{{ fmtCost(usage.lifetime?.estCostUsd) }}</span>
              </div>
            </div>
          </div>

          <div v-if="billing.monthlyBudget > 0" class="budget-row">
            <div class="budget-label">
              Monthly budget: <strong>{{ fmtCost(usage.month?.estCostUsd) }}</strong>
              of <strong>${{ Number(billing.monthlyBudget).toFixed(2) }}</strong>
              ({{ monthBudgetUsedPct }}%)
            </div>
            <div class="budget-bar">
              <div
                class="budget-bar-fill"
                :class="{ over: monthBudgetUsedPct >= 100, warn: monthBudgetUsedPct >= 80 }"
                :style="{ width: monthBudgetUsedPct + '%' }"
              ></div>
            </div>
          </div>

          <div v-if="usage.byModel?.length" class="usage-table-wrap">
            <table class="usage-table">
              <thead>
                <tr>
                  <th>Model</th>
                  <th class="num">Calls</th>
                  <th class="num">Input</th>
                  <th class="num">Output</th>
                  <th class="num">Est. cost</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="m in usage.byModel" :key="m.model">
                  <td class="mono">{{ m.model || '(unknown)' }}</td>
                  <td class="num">{{ fmtNumber(m.calls) }}</td>
                  <td class="num">{{ fmtNumber(m.in) }}</td>
                  <td class="num">{{ fmtNumber(m.out) }}</td>
                  <td class="num">{{ fmtCost(m.estCostUsd) }}</td>
                </tr>
              </tbody>
            </table>
          </div>
          <div v-else class="usage-empty">
            No LLM calls recorded yet. Run a diagnostic or PR review to populate this.
          </div>

          <div v-if="usage.firstRecordedAt" class="usage-since">
            Tracking since {{ fmtTimestamp(usage.firstRecordedAt) }}
          </div>
        </div>

        <div class="subsection">
          <div class="subsection-title">Cost rates &amp; budget</div>
          <div class="subsection-hint">
            Rates are dollars per 1,000,000 tokens. Default pricing for popular models:
            DeepSeek-Chat $0.27 / $1.10, Llama-3.1-8B (self-hosted) $0 / $0,
            GPT-4o $2.50 / $10.00.
          </div>
          <div class="field-row two">
            <div class="field">
              <label class="field-label">Input $/1M</label>
              <input v-model.number="billing.inputCostPer1M" type="number" step="0.01" min="0" class="field-input mono" placeholder="0" />
            </div>
            <div class="field">
              <label class="field-label">Output $/1M</label>
              <input v-model.number="billing.outputCostPer1M" type="number" step="0.01" min="0" class="field-input mono" placeholder="0" />
            </div>
          </div>
          <div class="field">
            <label class="field-label">Monthly budget ($)</label>
            <input v-model.number="billing.monthlyBudget" type="number" step="1" min="0" class="field-input mono" placeholder="0 — disabled" />
            <div class="field-hint">Soft cap. Inference is never blocked; the bar above just turns amber/red.</div>
          </div>
          <div class="field-row" style="gap: 8px;">
            <button class="action-btn primary" @click="saveBilling" :disabled="billingSaving">
              {{ billingSaving ? 'Saving…' : 'Save rates' }}
            </button>
            <button class="action-btn" @click="loadUsage" :disabled="usageLoading">Refresh</button>
            <button class="action-btn danger" @click="clearUsage">Reset usage</button>
            <span v-if="billingSaveMsg" class="save-msg" :class="{ error: billingSaveMsg.startsWith('Error') }">
              {{ billingSaveMsg }}
            </span>
          </div>
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

      <!-- Pipelines / CI-CD (all optional) -->
      <div class="section">
        <div class="section-title">Pipelines &amp; CI/CD</div>
        <div class="section-hint">
          Wire KubeWatcher into your build &amp; deploy pipelines. Pick the provider your code or
          infrastructure already lives in — only the fields for the selected provider are used.
        </div>

        <div class="field">
          <label class="toggle-label">
            <input type="checkbox" v-model="form.pipelinesEnabled" class="toggle-checkbox" />
            <span class="toggle-text">Enable pipeline integration</span>
          </label>
          <div class="field-hint">
            When off, all provider configuration below is preserved but inactive.
          </div>
        </div>

        <div class="provider-grid">
          <label
            v-for="p in PIPELINE_PROVIDERS"
            :key="p.id"
            class="provider-card"
            :class="{ active: form.pipelinesProvider === p.id }"
          >
            <input
              type="radio"
              name="pipeline-provider"
              :value="p.id"
              v-model="form.pipelinesProvider"
              class="provider-radio"
            />
            <div class="provider-body">
              <div class="provider-name">{{ p.name }}</div>
              <div class="provider-blurb">{{ p.blurb }}</div>
              <div class="provider-auth">Auth: {{ p.auth }}</div>
            </div>
          </label>
        </div>

        <!-- GitHub Actions -->
        <div v-if="form.pipelinesProvider === 'github'" class="provider-config">
          <div class="field">
            <label class="field-label">GitHub Token</label>
            <input v-model="form.githubToken" type="password" class="field-input mono" placeholder="ghp_… or installation token" />
            <div class="field-hint">PAT with <code>actions:write</code> or a GitHub App installation token.</div>
          </div>
          <div class="field-row two">
            <div class="field">
              <label class="field-label">Owner</label>
              <input v-model="form.githubOwner" type="text" class="field-input mono" placeholder="acme" />
            </div>
            <div class="field">
              <label class="field-label">Repository</label>
              <input v-model="form.githubRepo" type="text" class="field-input mono" placeholder="kube-watcher" />
            </div>
          </div>
          <div class="field">
            <label class="field-label">Workflow</label>
            <input v-model="form.githubWorkflow" type="text" class="field-input mono" placeholder="deploy.yml or workflow ID" />
            <div class="field-hint">Filename in <code>.github/workflows/</code> or numeric workflow ID — fired via <code>workflow_dispatch</code>.</div>
          </div>
        </div>

        <!-- GitLab CI/CD -->
        <div v-if="form.pipelinesProvider === 'gitlab'" class="provider-config">
          <div class="field">
            <label class="field-label">GitLab URL</label>
            <input v-model="form.gitlabUrl" type="text" class="field-input mono" placeholder="https://gitlab.com" />
            <div class="field-hint">Use your self-hosted instance URL or leave as <code>https://gitlab.com</code>.</div>
          </div>
          <div class="field">
            <label class="field-label">Trigger Token</label>
            <input v-model="form.gitlabToken" type="password" class="field-input mono" placeholder="glpat-… or trigger token" />
            <div class="field-hint">Pipeline trigger token, project access token, or PAT with <code>api</code> scope.</div>
          </div>
          <div class="field-row two">
            <div class="field">
              <label class="field-label">Project ID</label>
              <input v-model="form.gitlabProjectId" type="text" class="field-input mono" placeholder="12345 or group/project" />
            </div>
            <div class="field">
              <label class="field-label">Ref</label>
              <input v-model="form.gitlabRef" type="text" class="field-input mono" placeholder="main" />
            </div>
          </div>
        </div>

        <!-- AWS CodeBuild -->
        <div v-if="form.pipelinesProvider === 'aws-codebuild'" class="provider-config">
          <div class="field-row two">
            <div class="field">
              <label class="field-label">AWS Region</label>
              <input v-model="form.awsRegion" type="text" class="field-input mono" placeholder="us-east-1" />
            </div>
            <div class="field">
              <label class="field-label">CodeBuild Project</label>
              <input v-model="form.awsProject" type="text" class="field-input mono" placeholder="my-build-project" />
            </div>
          </div>
          <div class="field">
            <label class="field-label">Access Key ID</label>
            <input v-model="form.awsAccessKey" type="text" class="field-input mono" placeholder="AKIA…" />
            <div class="field-hint">Leave blank to use the ambient AWS credential chain (IAM role, env vars, profile).</div>
          </div>
          <div class="field">
            <label class="field-label">Secret Access Key</label>
            <input v-model="form.awsSecretKey" type="password" class="field-input mono" placeholder="(optional)" />
            <div class="field-hint">Required only if you provided an Access Key ID above. Needs <code>codebuild:StartBuild</code>.</div>
          </div>
        </div>

        <!-- GCP Cloud Build -->
        <div v-if="form.pipelinesProvider === 'gcp-cloudbuild'" class="provider-config">
          <div class="field-row two">
            <div class="field">
              <label class="field-label">GCP Project</label>
              <input v-model="form.gcpProject" type="text" class="field-input mono" placeholder="my-gcp-project" />
            </div>
            <div class="field">
              <label class="field-label">Region</label>
              <input v-model="form.gcpRegion" type="text" class="field-input mono" placeholder="global" />
            </div>
          </div>
          <div class="field">
            <label class="field-label">Service Account JSON Path</label>
            <input v-model="form.gcpCredentials" type="text" class="field-input mono" placeholder="/path/to/sa.json" />
            <div class="field-hint">
              Path to a service-account key file. Leave blank to use Application Default Credentials.
              Needs <code>cloudbuild.builds.create</code>.
            </div>
          </div>
        </div>

        <!-- CircleCI -->
        <div v-if="form.pipelinesProvider === 'circleci'" class="provider-config">
          <div class="field">
            <label class="field-label">API Token</label>
            <input v-model="form.circleciToken" type="password" class="field-input mono" placeholder="CCI_… personal API token" />
            <div class="field-hint">Sent in the <code>Circle-Token</code> header on API v2 calls.</div>
          </div>
          <div class="field">
            <label class="field-label">Project Slug</label>
            <input v-model="form.circleciProjectSlug" type="text" class="field-input mono" placeholder="github/acme/kube-watcher" />
            <div class="field-hint">Format <code>vcs/org/repo</code> — e.g. <code>github/acme/kube-watcher</code>.</div>
          </div>
        </div>

        <!-- Azure Pipelines -->
        <div v-if="form.pipelinesProvider === 'azure'" class="provider-config">
          <div class="field-row two">
            <div class="field">
              <label class="field-label">Organization</label>
              <input v-model="form.azureOrganization" type="text" class="field-input mono" placeholder="my-org" />
              <div class="field-hint">Used in <code>https://dev.azure.com/{org}/</code>.</div>
            </div>
            <div class="field">
              <label class="field-label">Project</label>
              <input v-model="form.azureProject" type="text" class="field-input mono" placeholder="my-project" />
            </div>
          </div>
          <div class="field-row two">
            <div class="field">
              <label class="field-label">Pipeline ID</label>
              <input v-model="form.azurePipelineId" type="text" class="field-input mono" placeholder="42" />
              <div class="field-hint">Numeric ID from the pipeline's URL.</div>
            </div>
            <div class="field">
              <label class="field-label">Branch Ref</label>
              <input v-model="form.azureBranch" type="text" class="field-input mono" placeholder="refs/heads/main" />
            </div>
          </div>
          <div class="field">
            <label class="field-label">Personal Access Token</label>
            <input v-model="form.azureToken" type="password" class="field-input mono" placeholder="(Build → Read &amp; execute scope)" />
            <div class="field-hint">
              PAT scoped to <code>Build (Read &amp; execute)</code>. Sent as HTTP Basic auth (empty username, PAT as password).
            </div>
          </div>
        </div>

        <div
          v-if="!form.pipelinesProvider"
          class="provider-empty"
        >
          Select a provider above to configure credentials.
        </div>

        <!-- PR notification toggles -->
        <div class="subsection">
          <div class="subsection-title">Notifications</div>
          <div class="subsection-hint">
            Notify me when a pull request is…
          </div>
          <div class="notify-grid">
            <label class="toggle-label">
              <input type="checkbox" v-model="form.notifyOnPrOpened" class="toggle-checkbox" />
              <span class="toggle-text">Opened</span>
            </label>
            <label class="toggle-label">
              <input type="checkbox" v-model="form.notifyOnPrUpdated" class="toggle-checkbox" />
              <span class="toggle-text">Updated <span class="toggle-sub">(new commits / pushed)</span></span>
            </label>
            <label class="toggle-label">
              <input type="checkbox" v-model="form.notifyOnPrCommented" class="toggle-checkbox" />
              <span class="toggle-text">Commented <span class="toggle-sub">(reviews &amp; comments)</span></span>
            </label>
            <label class="toggle-label">
              <input type="checkbox" v-model="form.notifyOnPrMerged" class="toggle-checkbox" />
              <span class="toggle-text">Merged <span class="toggle-sub">(or closed)</span></span>
            </label>
          </div>
        </div>

        <!-- Auto code review -->
        <div class="subsection">
          <div class="subsection-title">Auto Code Review</div>
          <div class="subsection-hint">
            Have the AI generate a review document for each PR. Reports are always saved
            in the Pipelines → Reports tab; pick where to additionally publish them.
          </div>

          <div class="field" style="margin-bottom: 12px;">
            <label class="toggle-label">
              <input type="checkbox" v-model="form.autoCodeReview" class="toggle-checkbox" />
              <span class="toggle-text">Run an AI code review on every PR</span>
            </label>
          </div>

          <div class="dest-grid">
            <label
              v-for="d in REVIEW_DESTINATIONS"
              :key="d.id"
              class="dest-card"
              :class="{ active: form.codeReviewDestination === d.id }"
            >
              <input
                type="radio"
                name="review-destination"
                :value="d.id"
                v-model="form.codeReviewDestination"
                class="provider-radio"
              />
              <div class="provider-body">
                <div class="provider-name">{{ d.label }}</div>
                <div class="provider-blurb">{{ d.blurb }}</div>
              </div>
            </label>
          </div>

          <div v-if="form.codeReviewDestination === 'gdrive'" class="dest-config">
            <div class="field">
              <label class="field-label">Drive Folder ID</label>
              <input
                v-model="form.gdriveFolderId"
                type="text"
                class="field-input mono"
                placeholder="1A2b3C4d5E6f7G8h9I…"
              />
              <div class="field-hint">
                The opaque ID after <code>/folders/</code> in the Drive URL. Drive OAuth must
                be authorized separately before the first upload will succeed.
              </div>
            </div>
          </div>

          <div v-if="form.codeReviewDestination === 's3'" class="dest-config">
            <div class="field">
              <label class="field-label">S3 Key Prefix</label>
              <input
                v-model="form.codeReviewS3Prefix"
                type="text"
                class="field-input mono"
                placeholder="code-reviews/"
              />
              <div class="field-hint">
                Uses the bucket from your S3 config above. Each report is uploaded as
                <code>&lt;prefix&gt;&lt;id&gt;.md</code>.
              </div>
            </div>
          </div>

          <div v-if="form.codeReviewDestination === 'email'" class="dest-config">
            <div class="field">
              <label class="field-label">Recipients</label>
              <input
                v-model="form.codeReviewEmailTo"
                type="text"
                class="field-input mono"
                placeholder="dev-leads@example.com, you@example.com"
              />
              <div class="field-hint">
                Comma-separated email addresses. The review is sent as a Markdown email body.
              </div>
            </div>
          </div>

          <div v-if="form.codeReviewDestination === 'confluence'" class="dest-config">
            <div class="field">
              <label class="field-label">Base URL</label>
              <input v-model="form.confluenceUrl" type="text" class="field-input mono" placeholder="https://acme.atlassian.net/wiki" />
            </div>
            <div class="field">
              <label class="field-label">Email</label>
              <input v-model="form.confluenceEmail" type="text" class="field-input mono" placeholder="you@acme.com" />
            </div>
            <div class="field">
              <label class="field-label">API Token</label>
              <input v-model="form.confluenceToken" type="password" class="field-input mono" placeholder="Atlassian API token" />
              <div class="field-hint">Generated at <code>id.atlassian.com/manage-profile/security/api-tokens</code>. Sent as Basic auth (email + token).</div>
            </div>
            <div class="field-row two">
              <div class="field">
                <label class="field-label">Space Key</label>
                <input v-model="form.confluenceSpaceKey" type="text" class="field-input mono" placeholder="ENG" />
              </div>
              <div class="field">
                <label class="field-label">Parent Page ID</label>
                <input v-model="form.confluenceParentPageId" type="text" class="field-input mono" placeholder="(optional)" />
              </div>
            </div>
          </div>

          <div v-if="form.codeReviewDestination === 'notion'" class="dest-config">
            <div class="field">
              <label class="field-label">Integration Token</label>
              <input v-model="form.notionToken" type="password" class="field-input mono" placeholder="ntn_… or secret_…" />
              <div class="field-hint">Create an internal integration at <code>notion.so/my-integrations</code> and share the target database with it.</div>
            </div>
            <div class="field">
              <label class="field-label">Database ID</label>
              <input v-model="form.notionDatabaseId" type="text" class="field-input mono" placeholder="32-char database id" />
              <div class="field-hint">The hex string in the database's URL after the workspace name.</div>
            </div>
          </div>

          <div v-if="form.codeReviewDestination === 'evernote'" class="dest-config">
            <div class="field">
              <label class="field-label">Developer Token</label>
              <input v-model="form.evernoteToken" type="password" class="field-input mono" placeholder="S=…" />
              <div class="field-hint">Request a token from Evernote. For production use, OAuth is recommended.</div>
            </div>
            <div class="field">
              <label class="field-label">Notebook GUID (optional)</label>
              <input v-model="form.evernoteNotebookGuid" type="text" class="field-input mono" placeholder="leave blank for default notebook" />
            </div>
          </div>

          <div v-if="form.codeReviewDestination === 'onenote'" class="dest-config">
            <div class="field">
              <label class="field-label">Microsoft Graph Access Token</label>
              <input v-model="form.onenoteToken" type="password" class="field-input mono" placeholder="OAuth access token (Notes.Create scope)" />
              <div class="field-hint">Acquire via the MS Graph OAuth flow. Tokens expire — refresh logic is required for ongoing use.</div>
            </div>
            <div class="field">
              <label class="field-label">Section ID</label>
              <input v-model="form.onenoteSectionId" type="text" class="field-input mono" placeholder="0-…!1" />
              <div class="field-hint">Visible in the Graph response from <code>/me/onenote/sections</code>.</div>
            </div>
          </div>

          <div v-if="form.codeReviewDestination === 'amplenote'" class="dest-config">
            <div class="field">
              <label class="field-label">API Key</label>
              <input v-model="form.amplenoteApiKey" type="password" class="field-input mono" placeholder="Amplenote API key" />
              <div class="field-hint">Generated under <code>Account → API Keys</code> in Amplenote.</div>
            </div>
          </div>

          <div v-if="form.codeReviewDestination === 'standard-notes'" class="dest-config">
            <div class="field">
              <label class="field-label">Server URL</label>
              <input v-model="form.standardNotesUrl" type="text" class="field-input mono" placeholder="https://api.standardnotes.com" />
              <div class="field-hint">Override only if you self-host.</div>
            </div>
            <div class="field">
              <label class="field-label">Session Token</label>
              <input v-model="form.standardNotesToken" type="password" class="field-input mono" placeholder="JWT session token" />
              <div class="field-hint">Obtained from a sign-in flow. Avoid pasting your password — fetch a token instead.</div>
            </div>
          </div>

          <div v-if="form.codeReviewDestination === 'obsidian'" class="dest-config">
            <div class="field">
              <label class="field-label">Vault Path</label>
              <input v-model="form.obsidianVaultPath" type="text" class="field-input mono" placeholder="/Users/you/Documents/Vault" />
              <div class="field-hint">Reports are written as <code>&lt;vault&gt;/Code Reviews/&lt;id&gt;.md</code>. Obsidian picks them up automatically.</div>
            </div>
          </div>

          <div v-if="form.codeReviewDestination === 'joplin'" class="dest-config">
            <div class="field">
              <label class="field-label">Web Clipper URL</label>
              <input v-model="form.joplinUrl" type="text" class="field-input mono" placeholder="http://127.0.0.1:41184" />
              <div class="field-hint">The local Joplin Web Clipper API. Enable it under Joplin → Tools → Options → Web Clipper.</div>
            </div>
            <div class="field">
              <label class="field-label">Token</label>
              <input v-model="form.joplinToken" type="password" class="field-input mono" placeholder="auth token" />
            </div>
          </div>

          <div v-if="form.codeReviewDestination === 'logseq'" class="dest-config">
            <div class="field">
              <label class="field-label">Graph Path</label>
              <input v-model="form.logseqGraphPath" type="text" class="field-input mono" placeholder="/Users/you/Logseq" />
              <div class="field-hint">Reports are written as <code>&lt;graph&gt;/pages/code-review-&lt;id&gt;.md</code>.</div>
            </div>
          </div>

          <div v-if="form.codeReviewDestination === 'bear'" class="dest-config">
            <div class="field">
              <label class="field-label">Bear API Token</label>
              <input v-model="form.bearToken" type="password" class="field-input mono" placeholder="generated under Bear → Help → Advanced → API token" />
              <div class="field-hint">Bear is macOS-only and uses an <code>x-callback-url</code> scheme to create notes.</div>
            </div>
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
          <div class="info-label">Pipelines</div>
          <div class="info-value">
            <span v-if="settings.pipelinesEnabled && settings.pipelinesProvider" class="status-active">
              {{ PIPELINE_PROVIDERS.find(p => p.id === settings.pipelinesProvider)?.name || settings.pipelinesProvider }}
            </span>
            <span v-else-if="settings.pipelinesEnabled" class="text-muted">Enabled — no provider selected</span>
            <span v-else class="text-muted">Disabled</span>
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

/* Pipelines provider picker */
.provider-grid {
  display: grid; grid-template-columns: repeat(auto-fill, minmax(220px, 1fr));
  gap: 8px; margin: 0 0 14px;
}
.provider-card {
  display: flex; gap: 10px; padding: 10px 12px;
  border: 1px solid var(--border); border-radius: 6px; background: var(--bg);
  cursor: pointer; transition: border-color 0.15s, background 0.15s;
}
.provider-card:hover { border-color: var(--accent); }
.provider-card.active { border-color: var(--accent); background: rgba(79,142,247,0.08); }
.provider-radio { accent-color: var(--accent); margin-top: 2px; flex-shrink: 0; }
.provider-body { display: flex; flex-direction: column; gap: 3px; min-width: 0; }
.provider-name { font-size: 12.5px; font-weight: 600; color: var(--text); }
.provider-blurb { font-size: 11px; color: var(--text2); line-height: 1.4; }
.provider-auth { font-size: 10.5px; color: var(--text3); font-style: italic; }

.provider-config {
  border-top: 1px dashed var(--border); padding-top: 14px; margin-top: 4px;
  display: flex; flex-direction: column; gap: 0;
}
.field-row.two { display: grid; grid-template-columns: 1fr 1fr; gap: 12px; align-items: start; }
.field-row.two .field { margin-bottom: 14px; }

.provider-empty {
  font-size: 11.5px; color: var(--text3); font-style: italic;
  padding: 10px 12px; border: 1px dashed var(--border); border-radius: 6px; text-align: center;
}

.subsection { margin-top: 18px; padding-top: 14px; border-top: 1px dashed var(--border); }
.subsection-title {
  font-size: 11px; font-weight: 600; text-transform: uppercase; letter-spacing: 0.06em;
  color: var(--text2); margin-bottom: 4px;
}
.subsection-hint { font-size: 11.5px; color: var(--text3); margin-bottom: 10px; }
.notify-grid {
  display: grid; grid-template-columns: repeat(auto-fill, minmax(220px, 1fr));
  gap: 6px 12px;
}
.toggle-sub { color: var(--text3); font-size: 11px; margin-left: 4px; }

.dest-grid {
  display: grid; grid-template-columns: repeat(auto-fill, minmax(220px, 1fr));
  gap: 8px; margin-bottom: 12px;
}
.dest-card {
  display: flex; gap: 10px; padding: 10px 12px;
  border: 1px solid var(--border); border-radius: 6px; background: var(--bg);
  cursor: pointer; transition: border-color 0.15s, background 0.15s;
}
.dest-card:hover { border-color: var(--accent); }
.dest-card.active { border-color: var(--accent); background: rgba(79,142,247,0.08); }
.dest-config {
  border-top: 1px dashed var(--border); padding-top: 12px; margin-top: 4px;
}

.loading-state { flex: 1; display: flex; align-items: center; justify-content: center; color: var(--text3); font-size: 13px; }

/* Billing & Usage */
.usage-loading, .usage-error, .usage-empty {
  font-size: 12px; color: var(--text3); padding: 10px 0;
}
.usage-error { color: var(--red); }
.usage-grid {
  display: grid; grid-template-columns: repeat(auto-fill, minmax(180px, 1fr));
  gap: 10px; margin-bottom: 14px;
}
.usage-card {
  padding: 12px; background: var(--bg); border: 1px solid var(--border); border-radius: 6px;
}
.usage-label { font-size: 10.5px; text-transform: uppercase; letter-spacing: 0.06em; color: var(--text3); }
.usage-value {
  font-size: 18px; font-weight: 600; color: var(--text); margin-top: 2px;
  font-family: var(--mono);
}
.usage-unit { font-size: 11px; color: var(--text3); font-weight: 400; margin-left: 4px; }
.usage-sub { font-size: 11px; color: var(--text2); margin-top: 4px; display: flex; gap: 6px; align-items: center; }
.usage-sub .dot { opacity: 0.5; }

.budget-row { margin-bottom: 14px; }
.budget-label { font-size: 12px; color: var(--text2); margin-bottom: 4px; }
.budget-bar {
  height: 6px; background: var(--bg); border-radius: 3px; overflow: hidden;
  border: 1px solid var(--border);
}
.budget-bar-fill { height: 100%; background: var(--accent); transition: width 0.2s; }
.budget-bar-fill.warn { background: #d8a347; }
.budget-bar-fill.over { background: var(--red); }

.usage-table-wrap { overflow-x: auto; margin-bottom: 12px; }
.usage-table { width: 100%; border-collapse: collapse; font-size: 12px; }
.usage-table th {
  text-align: left; font-weight: 500; color: var(--text3);
  padding: 6px 10px; border-bottom: 1px solid var(--border);
  font-size: 10.5px; text-transform: uppercase; letter-spacing: 0.05em;
}
.usage-table th.num, .usage-table td.num { text-align: right; font-family: var(--mono); }
.usage-table td { padding: 6px 10px; border-bottom: 1px solid var(--border); color: var(--text2); }
.usage-table td.mono { color: var(--text); }

.usage-since { font-size: 11px; color: var(--text3); font-style: italic; margin-bottom: 10px; }
</style>
