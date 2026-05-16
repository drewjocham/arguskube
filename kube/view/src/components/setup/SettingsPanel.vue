<script setup>
import { ref, computed, onMounted, nextTick, reactive } from 'vue'
import { storeToRefs } from 'pinia'
import { callGo, useContexts } from '../../composables/useWails'
import { useAppearanceStore } from '../../stores/appearance'
import { useNotificationsStore } from '../../stores/notifications'
import { useSpotCheck } from '../../composables/useSpotCheck'
import { useAddonsStore } from '../../stores/addons'
import { useNotificationChannelsStore } from '../../stores/notificationChannels'
import { useAppNavStore } from '../../stores/appNav'
import { useCredentialAlertsStore } from '../../stores/credentialAlerts'
import { useExternalSecretsStore } from '../../stores/externalSecrets'
import { useNotificationGuardStore } from '../../stores/notificationGuard'
import { useWatcherRegistryStore } from '../../stores/watcherRegistry'
import { runDueNow as watcherRunDueNow, runWatcherById } from '../../composables/useWatcherEngine'
import Select from '../common/Select.vue'
import RevealableInput from '../common/RevealableInput.vue'
import SecretsToolProbeRow from './SecretsToolProbeRow.vue'
import SetupChecklist from './SetupChecklist.vue'
import PrivacyControls from './PrivacyControls.vue'
import { useNavVisibilityStore } from '../../stores/navVisibility'

const SECTION_GROUPS = [
  {
    label: 'System',
    sections: [
      { id: 'setup-checklist', label: 'Checklist' },
      { id: 'privacy-controls', label: 'Privacy' },
      { id: 'vault', label: 'Vault' },
      { id: 'kube-connection', label: 'Kubernetes' },
    ],
  },
  {
    label: 'Access',
    sections: [
      { id: 'sign-in-integrations', label: 'Sign-in & OAuth' },
      { id: 'security-tools', label: 'Security' },
    ],
  },
  {
    label: 'Appearance',
    sections: [
      { id: 'appearance', label: 'Appearance' },
      { id: 'settings-navigation', label: 'Navigation' },
    ],
  },
  {
    label: 'Notifications',
    sections: [
      { id: 'notifications-general', label: 'General' },
      { id: 'notification-channels', label: 'Channels' },
    ],
  },
  {
    label: 'Agent',
    sections: [
      { id: 'watchers-notifications', label: 'Watchers' },
      { id: 'agent-profile', label: 'Profile' },
    ],
  },
  {
    label: 'Integrations',
    sections: [
      { id: 'ai-integrations', label: 'AI & LLM' },
      { id: 'arguscd-section', label: 'Argo CD' },
      { id: 'pipelines-section', label: 'Pipelines' },
    ],
  },
  {
    label: 'Operations',
    sections: [
      { id: 'billing-usage', label: 'Billing' },
      { id: 'addons-jobs', label: 'Add-ons' },
    ],
  },
]

const activeSection = ref('')

function scrollToSection(id) {
  const el = document.getElementById(id)
  if (el) el.scrollIntoView({ behavior: 'smooth', block: 'start' })
}

const credentialAlerts = useCredentialAlertsStore()
const esStore = useExternalSecretsStore()
const guard = useNotificationGuardStore()
const registry = useWatcherRegistryStore()

// The watcher engine itself is mounted globally in App.vue. From here we
// only call into the module-level run helpers — they share the same
// in-flight guard as the timer, so a manual "Re-check" can never collide
// with the scheduled tick.
const watchersRunning = ref(false)
const watcherRunningId = ref('')
const watchersAnchorRef = ref(null)

async function runAllWatchersNow() {
  watchersRunning.value = true
  try { await watcherRunDueNow({ force: true }) }
  finally { watchersRunning.value = false }
}
async function runOneWatcher(id) {
  watcherRunningId.value = id
  try { await runWatcherById(id) }
  finally { watcherRunningId.value = '' }
}

function watcherStatusOf(w) {
  return registry.results[w.id]?.status || ''
}

function formatLocal(epochMs) {
  try { return new Date(epochMs).toLocaleString() } catch { return String(epochMs) }
}
function formatRelative(epochMs) {
  if (!epochMs) return ''
  const diff = Date.now() - epochMs
  if (diff < 60_000) return 'just now'
  if (diff < 3_600_000) return Math.round(diff / 60_000) + ' min ago'
  if (diff < 86_400_000) return Math.round(diff / 3_600_000) + ' h ago'
  return formatLocal(epochMs)
}

// Probe one of the local secrets-encryption CLIs (kubeseal, sops, gpg,
// age) by asking the backend to run `<bin> --version`. Result lands in
// the externalSecrets store; the row re-renders with version + path.
async function testSecretsTool(tool) {
  esStore.setProbing(tool, true)
  try {
    const res = await callGo('TestSecretsTool', tool)
    esStore.setProbeResult(tool, res || { tool, found: false })
  } catch (e) {
    esStore.setProbeResult(tool, {
      tool, found: false, error: e?.message || String(e),
    })
  } finally {
    esStore.setProbing(tool, false)
  }
}

const notifChannelsStore = useNotificationChannelsStore()
const appNav = useAppNavStore()
const channelsAnchorRef = ref(null)
const githubAnchorRef = ref(null)
const vaultAnchorRef = ref(null)
const returnBanner = ref({ state: 'none', label: '', target: null })

// --- Vault state -----------------------------------------------------------
const vaultEntries = ref([])
const vaultLoading = ref(false)
const vaultError = ref('')
const vaultProbing = ref({})        // { providerId: true } while a Test runs

const vaultSecrets = ref([])
const vaultSecretsError = ref('')
const secretDraft = ref({ key: '', value: '', notes: '' })
const secretSaving = ref(false)

const VAULT_STATUS_LABEL = {
  valid:   'Valid',
  present: 'Configured',
  missing: 'Missing',
  expired: 'Expired',
  invalid: 'Invalid',
  error:   'Error',
}

function vaultStatusLabel(s) { return VAULT_STATUS_LABEL[s] || s || 'unknown' }

function formatVaultCheck(ts) {
  if (!ts) return ''
  try {
    const d = new Date(ts)
    const diffMs = Date.now() - d.getTime()
    if (diffMs < 60_000) return 'just now'
    if (diffMs < 3_600_000) return Math.round(diffMs / 60_000) + ' min ago'
    return d.toLocaleString()
  } catch { return String(ts) }
}

async function refreshVault() {
  vaultLoading.value = true
  vaultError.value = ''
  try {
    const res = await callGo('GetVaultStatus')
    vaultEntries.value = Array.isArray(res) ? res : []
  } catch (e) {
    vaultError.value = e?.message || String(e)
  } finally {
    vaultLoading.value = false
  }
}

async function testVaultEntry(entry) {
  if (!entry?.id) return
  vaultProbing.value = { ...vaultProbing.value, [entry.id]: true }
  try {
    const updated = await callGo('TestVaultProvider', entry.id)
    if (updated && updated.id) {
      vaultEntries.value = vaultEntries.value.map((e) => (e.id === updated.id ? updated : e))
      // Manual Test feeds the same dedupe + delivery path the background
      // monitor uses, so a token that comes back expired/invalid will fire
      // a global notification (toast top-right + bell-panel entry).
      credentialAlerts.observe(updated)
    }
  } catch (e) {
    const errMsg = e?.message || String(e)
    const failed = { ...entry, status: 'error', message: errMsg }
    vaultEntries.value = vaultEntries.value.map((row) =>
      row.id === entry.id ? failed : row
    )
    credentialAlerts.observe(failed)
  } finally {
    const next = { ...vaultProbing.value }
    delete next[entry.id]
    vaultProbing.value = next
  }
}

// Anchor IDs that live inside the Pipelines section render conditionally
// based on which provider radio is selected. Selecting it before scrolling
// is what makes the deep-link actually land on populated input fields.
const PROVIDER_RADIO_FOR_ANCHOR = {
  'pipelines-github':   'github',
  'pipelines-gitlab':   'gitlab',
  'pipelines-aws':      'aws-codebuild',
  'pipelines-gcp':      'gcp-cloudbuild',
  'pipelines-circleci': 'circleci',
  'pipelines-azure':    'azure',
}

// In-page deep-link from a Vault row to the configuration section that owns
// the credential. Same UX shape as the volume / pipelines flows: scroll the
// target section into view, pulse-banner top-right with "I will take you
// back when you're done", 4 s fade → "← Go back to Vault".
function configureVaultEntry(entry) {
  const anchor = entry?.configureAnchor
  if (!anchor) return

  // Pre-select the right pipelines provider so its fields render before we
  // try to scroll to them.
  const radioValue = PROVIDER_RADIO_FOR_ANCHOR[anchor]
  if (radioValue && form.value) {
    form.value.pipelinesProvider = radioValue
  }

  // The provider block renders next tick after the radio change; wait once
  // before scrolling so we resolve the actual element instead of the
  // hidden conditional placeholder.
  nextTick(() => {
    const el = document.getElementById(anchor)
    if (el && typeof el.scrollIntoView === 'function') {
      el.scrollIntoView({ block: 'start', behavior: 'smooth' })
    } else {
      // Fall back to the parent Pipelines section if a sub-anchor isn't
      // wired up yet — better than a no-op click.
      const fallback = document.getElementById('pipelines-section')
      if (fallback) fallback.scrollIntoView({ block: 'start', behavior: 'smooth' })
    }
  })

  returnBanner.value = {
    state: 'pulse',
    label: 'Vault',
    target: { anchor: 'vault', inPage: true },
  }
  setTimeout(() => {
    returnBanner.value = { ...returnBanner.value, state: 'go-back' }
  }, 4000)
}

async function loadVaultSecretsFromBackend() {
  vaultSecretsError.value = ''
  try {
    const res = await callGo('ListVaultSecrets')
    vaultSecrets.value = Array.isArray(res) ? res : []
  } catch (e) {
    vaultSecretsError.value = e?.message || String(e)
    vaultSecrets.value = []
  }
}

async function saveCustomSecret() {
  const k = secretDraft.value.key.trim()
  if (!k || !secretDraft.value.value) return
  secretSaving.value = true
  vaultSecretsError.value = ''
  try {
    await callGo('SetVaultSecret', k, secretDraft.value.value, secretDraft.value.notes || '')
    secretDraft.value = { key: '', value: '', notes: '' }
    await loadVaultSecretsFromBackend()
  } catch (e) {
    vaultSecretsError.value = e?.message || String(e)
  } finally {
    secretSaving.value = false
  }
}

async function removeCustomSecret(key) {
  if (!confirm(`Remove the secret "${key}"? This cannot be undone.`)) return
  try {
    await callGo('DeleteVaultSecret', key)
    await loadVaultSecretsFromBackend()
  } catch (e) {
    vaultSecretsError.value = e?.message || String(e)
  }
}

const NEW_CHANNEL_KINDS = [
  { id: 'desktop', label: 'Desktop', hint: 'Native OS notification (no extra config).' },
  { id: 'email',   label: 'Email',   hint: 'Send to a single recipient. Provide an address.' },
  { id: 'slack',   label: 'Slack',   hint: 'POST to a Slack incoming webhook URL.' },
  { id: 'google-chat', label: 'Google Chat', hint: 'POST to a Google Chat webhook URL.' },
  { id: 'webhook', label: 'Webhook', hint: 'POST a JSON body to any URL of your choice.' },
]

function addChannel(kind) { notifChannelsStore.add(kind) }
function removeChannel(id) { notifChannelsStore.remove(id) }
function updateChannel(id, patch) { notifChannelsStore.update(id, patch) }
function channelPlaceholder(kind) {
  if (kind === 'email') return 'you@example.com'
  if (kind === 'slack') return 'https://hooks.slack.com/services/T…/B…/…'
  if (kind === 'google-chat') return 'https://chat.googleapis.com/v1/spaces/…/messages'
  if (kind === 'webhook') return 'https://example.com/notify'
  return ''
}
function goBackToOrigin() {
  const t = returnBanner.value.target
  returnBanner.value = { state: 'none', label: '', target: null }
  appNav.clearReturn()
  // In-page jump (Vault → some other section in Settings): no router
  // transition; just scroll back to the originating anchor inside this
  // same view.
  if (t?.inPage && t?.anchor) {
    const el = document.getElementById(t.anchor)
    if (el && typeof el.scrollIntoView === 'function') {
      el.scrollIntoView({ block: 'start', behavior: 'smooth' })
    }
    return
  }
  if (t && t.navId) appNav.requestNav({ navId: t.navId, anchor: t.anchor })
}

const addonsStore = useAddonsStore()
const jobInputsDraft = ref({})
const jobDeliveriesDraft = ref({})
const runningJob = ref('')

function _draftForJob(jobId) {
  if (!jobInputsDraft.value[jobId]) {
    jobInputsDraft.value[jobId] = { ...(addonsStore.jobInputs[jobId] || {}) }
  }
  return jobInputsDraft.value[jobId]
}
function _deliveryDraftForJob(jobId) {
  if (!jobDeliveriesDraft.value[jobId]) {
    jobDeliveriesDraft.value[jobId] = { ...(addonsStore.jobDeliveries[jobId] || {}) }
  }
  return jobDeliveriesDraft.value[jobId]
}

async function runJob(job) {
  runningJob.value = job.id
  for (const inp of job.inputs || []) {
    if (jobInputsDraft.value[job.id]?.[inp.id] != null) {
      addonsStore.setJobInput(job.id, inp.id, jobInputsDraft.value[job.id][inp.id])
    }
  }
  for (const d of job.deliveries || []) {
    if (jobDeliveriesDraft.value[job.id]?.[d.id] != null) {
      addonsStore.setJobDelivery(job.id, d.id, jobDeliveriesDraft.value[job.id][d.id])
    }
  }
  const runId = addonsStore.recordRun({ jobId: job.id, status: 'pending' })
  try {
    addonsStore.completeRun(runId, {
      status: 'queued',
      result: 'Job staged. Backend runner not yet implemented.',
    })
  } catch (e) {
    addonsStore.completeRun(runId, { status: 'error', error: e?.message || String(e) })
  } finally {
    runningJob.value = ''
  }
}

const agentProfile = reactive({
  autoInvestigate: true,
  autoDocument: true,
  canAck: false,
  canSilence: false,
  canAdjustParams: false,
  silenceWindow: 3600000000000,
  fatigueThreshold: 5,
})
const silenceWindowMin = ref(60)
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

const appearance = useAppearanceStore()
const { theme: appTheme, brightness, contrast, opacity, blur, saturation, density, fontSize } = storeToRefs(appearance)
const fontSizeRange = appearance.ranges.fontSize

// Google Auth step-by-step guide toggle
const googleGuideOpen = ref(false)

// Navigation visibility — which sidebar sections show. Read once and
// destructure the reactive `sections` getter for the toggle UI.
const navVisibility = useNavVisibilityStore()
const navSections = computed(() => navVisibility.sections)
const ranges = appearance.ranges

const notifStore = useNotificationsStore()
const { settings: notifSettings } = storeToRefs(notifStore)
const draftNotifMax = ref(notifSettings.value.maxItems)
function applyNotifMax() {
  notifStore.setMaxItems(draftNotifMax.value)
  draftNotifMax.value = notifStore.settings.maxItems
}

const { runAll: runSpotChecksNow } = useSpotCheck()
const spotChecksRunning = ref(false)
async function triggerSpotChecks() {
  spotChecksRunning.value = true
  try {
    await runSpotChecksNow()
  } finally {
    setTimeout(() => { spotChecksRunning.value = false }, 1500)
  }
}

const settings = ref(null)
const loading = ref(true)
const saving = ref(false)
const saveMessage = ref('')

const form = ref({
  kubeconfigPath: '',
  currentContext: '',
  namespace: '',
  deepseekApiKey: '',
  llmBaseUrl: '',
  llmModel: '',
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

  // Sign-in & integrations: OAuth client credentials operators used to
  // set via env vars. The backend masks secrets on read; the masked
  // sentinel ("••••") is skipped on save so an unedited form preserves
  // the on-disk value.
  googleClientId: '',
  googleClientSecret: '',
  oidcIssuer: '',
  oidcClientId: '',
  oidcClientSecret: '',
  oidcDisplayName: '',
  appleServicesId: '',
  appleTeamId: '',
  appleKeyId: '',
  applePrivateKey: '',
  appleDisplayName: '',
  allowLocalSignup: true,
  passkeyEnabled: false,
  passkeyRpId: 'localhost',
  passkeyRpName: 'Argus',
  passkeyRpOrigin: 'http://localhost:8080',
  workspaceGoogleClientId: '',
  workspaceGoogleClientSecret: '',
  slackClientId: '',
  slackClientSecret: '',
  slackSigningSecret: '',
})

const PIPELINE_PROVIDERS = [
  { id: 'github',          name: 'GitHub Actions' },
  { id: 'gitlab',          name: 'GitLab CI/CD' },
  { id: 'aws-codebuild',   name: 'AWS CodeBuild' },
  { id: 'gcp-cloudbuild',  name: 'Google Cloud Build' },
  { id: 'circleci',        name: 'CircleCI' },
  { id: 'azure',           name: 'Azure Pipelines' },
]

const REVIEW_DESTINATIONS = [
  { id: 'local',          label: 'In-app only' },
  { id: 'gdrive',         label: 'Google Drive' },
  { id: 's3',             label: 'S3' },
  { id: 'email',          label: 'Email' },
  { id: 'confluence',     label: 'Confluence' },
  { id: 'notion',         label: 'Notion' },
  { id: 'evernote',       label: 'Evernote' },
  { id: 'onenote',        label: 'Microsoft OneNote' },
  { id: 'amplenote',      label: 'Amplenote' },
  { id: 'standard-notes', label: 'Standard Notes' },
  { id: 'obsidian',       label: 'Obsidian' },
  { id: 'joplin',         label: 'Joplin' },
  { id: 'logseq',         label: 'Logseq' },
  { id: 'bear',           label: 'Bear' },
]

const usage = ref(null)
const usageLoading = ref(false)
const usageError = ref('')
const billing = ref({ inputCostPer1M: 0, outputCostPer1M: 0, monthlyBudget: 0 })
const billingSaving = ref(false)
const billingSaveMsg = ref('')

function fmtNumber(n) {
  if (n === undefined || n === null) return '\u2014'
  return n.toLocaleString()
}
function fmtCost(usd) {
  if (!Number.isFinite(usd) || usd === 0) return '\u2014'
  return '$' + usd.toFixed(usd < 1 ? 4 : 2)
}
function fmtTimestamp(ts) {
  if (!ts) return '\u2014'
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
        llmBaseUrl: result.llmBaseUrl || '',
        llmModel: result.llmModel || '',
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

        googleClientId: result.googleClientId || '',
        googleClientSecret: result.googleClientSecret || '',
        oidcIssuer: result.oidcIssuer || '',
        oidcClientId: result.oidcClientId || '',
        oidcClientSecret: result.oidcClientSecret || '',
        oidcDisplayName: result.oidcDisplayName || '',
        appleServicesId: result.appleServicesId || '',
        appleTeamId: result.appleTeamId || '',
        appleKeyId: result.appleKeyId || '',
        applePrivateKey: result.applePrivateKey || '',
        appleDisplayName: result.appleDisplayName || '',
        allowLocalSignup: result.allowLocalSignup ?? true,
        passkeyEnabled: result.passkeyEnabled || false,
        passkeyRpId: result.passkeyRpId || 'localhost',
        passkeyRpName: result.passkeyRpName || 'Argus',
        passkeyRpOrigin: result.passkeyRpOrigin || 'http://localhost:8080',
        workspaceGoogleClientId: result.workspaceGoogleClientId || '',
        workspaceGoogleClientSecret: result.workspaceGoogleClientSecret || '',
        slackClientId: result.slackClientId || '',
        slackClientSecret: result.slackClientSecret || '',
        slackSigningSecret: result.slackSigningSecret || '',
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
      mcpServersConfig: form.value.mcpServersConfig,
      agentInstructions: form.value.agentInstructions,
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

      googleClientId: form.value.googleClientId,
      googleClientSecret: form.value.googleClientSecret,
      oidcIssuer: form.value.oidcIssuer,
      oidcClientId: form.value.oidcClientId,
      oidcClientSecret: form.value.oidcClientSecret,
      oidcDisplayName: form.value.oidcDisplayName,
      appleServicesId: form.value.appleServicesId,
      appleTeamId: form.value.appleTeamId,
      appleKeyId: form.value.appleKeyId,
      applePrivateKey: form.value.applePrivateKey,
      appleDisplayName: form.value.appleDisplayName,
      allowLocalSignup: form.value.allowLocalSignup,
      passkeyEnabled: form.value.passkeyEnabled,
      passkeyRpId: form.value.passkeyRpId,
      passkeyRpName: form.value.passkeyRpName,
      passkeyRpOrigin: form.value.passkeyRpOrigin,
      workspaceGoogleClientId: form.value.workspaceGoogleClientId,
      workspaceGoogleClientSecret: form.value.workspaceGoogleClientSecret,
      slackClientId: form.value.slackClientId,
      slackClientSecret: form.value.slackClientSecret,
      slackSigningSecret: form.value.slackSigningSecret,
    })
    saveMessage.value = 'Sign-in providers reloaded — no restart needed.'
    await loadSettings()
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
    argocdTestResult.value = 'Connected'
  } catch (e) {
    argocdTestResult.value = 'Failed: ' + (e?.message || String(e))
  } finally {
    argocdTesting.value = false
    setTimeout(() => { argocdTestResult.value = '' }, 6000)
  }
}

onMounted(async () => {
  loadSettings()
  listContexts()
  loadAgentProfile()
  loadUsage()
  refreshVault()
  loadVaultSecretsFromBackend()

  // Auto-probe local encryption CLIs once on mount so the External
  // Secrets cards land with their detection state already filled in.
  // Each call is bounded server-side; failures are silent.
  for (const tool of ['kubeseal', 'sops', 'gpg', 'age']) {
    testSecretsTool(tool)
  }

  // Set up IntersectionObserver for sticky-nav active state
  nextTick(() => {
    const sectionIds = SECTION_GROUPS.flatMap(g => g.sections.map(s => s.id))
    const els = sectionIds.map(id => document.getElementById(id)).filter(Boolean)
    if (els.length) {
      const obs = new IntersectionObserver((entries) => {
        for (const entry of entries) {
          if (entry.isIntersecting) {
            activeSection.value = entry.target.id
          }
        }
      }, { rootMargin: '-80px 0px -60% 0px' })
      for (const el of els) obs.observe(el)
    }
  })

  const pending = appNav.consumeNav()
  const ret = appNav.peekReturn()
  if (pending && pending.navId === 'settings') {
    await nextTick()

    // Resolve which section anchor to scroll to + which return label to use.
    let anchorEl = null
    if (pending.anchor === 'notification-channels') {
      anchorEl = channelsAnchorRef.value
    } else if (pending.anchor === 'sign-in-integrations') {
      anchorEl = document.getElementById('sign-in-integrations')
    } else if (pending.anchor === 'pipelines-github') {
      anchorEl = githubAnchorRef.value
      // Pre-select the GitHub provider so the corresponding fields render.
      // The form is the source of truth for the radio + the conditional
      // field-group that owns the token / owner / repo inputs.
      if (form.value) form.value.pipelinesProvider = 'github'
    }

    if (anchorEl && typeof anchorEl.scrollIntoView === 'function') {
      anchorEl.scrollIntoView({ block: 'start', behavior: 'smooth' })
    }

    if (ret && ret.navId) {
      const label = ret.label || (pending.anchor === 'pipelines-github' ? 'Pipelines' : 'volume')
      returnBanner.value = { state: 'pulse', label, target: ret }
      setTimeout(() => {
        returnBanner.value = { ...returnBanner.value, state: 'go-back' }
      }, 4000)
    }
  }
})
</script>

<template>
  <div class="settings-view">
    <div class="header">
      <h1 class="title">Settings</h1>
    </div>

    <!-- In-page deep-link banner. Pinned to the top-right of the settings
         scroll viewport so it's always visible regardless of which section
         the user is currently looking at. Only shows for in-page jumps
         from the Vault. -->
    <transition name="nc-banner-fade">
      <div
        v-if="returnBanner.state === 'pulse' && returnBanner.target?.inPage"
        class="nc-banner pulse vault-banner"
        key="pulse"
      >
        <span class="nc-banner-text">I will take you back when you're done.</span>
      </div>
      <button
        v-else-if="returnBanner.state === 'go-back' && returnBanner.target?.inPage"
        class="nc-banner go-back vault-banner"
        key="go-back"
        @click="goBackToOrigin"
      >← Go back to <span class="mono">{{ returnBanner.label }}</span></button>
    </transition>

    <nav class="settings-nav">
      <template v-for="group in SECTION_GROUPS" :key="group.label">
        <span class="nav-category">{{ group.label }}</span>
        <button
          v-for="s in group.sections"
          :key="s.id"
          class="nav-item"
          :class="{ active: activeSection === s.id }"
          @click="scrollToSection(s.id)"
        >{{ s.label }}</button>
      </template>
    </nav>

    <div class="scroll" v-if="!loading">
      <div id="setup-checklist">
        <SetupChecklist />
      </div>

      <div id="privacy-controls">
        <PrivacyControls />
      </div>

      <!-- Vault: every credential the app uses, in one auditable place. -->
      <div class="section vault-section" id="vault" ref="vaultAnchorRef">
        <div class="section-header-row">
          <h2 class="section-title">Vault</h2>
          <button class="vault-refresh" @click="refreshVault" :disabled="vaultLoading" title="Re-read configured tokens">
            {{ vaultLoading ? 'Refreshing…' : 'Refresh' }}
          </button>
        </div>
        <p class="hint">
          Every external credential the app uses — OAuth providers, API tokens, cluster
          credentials — with status indicators so missing or expired tokens are obvious.
          <strong>Test</strong> live-probes a provider; <strong>Configure</strong> jumps
          to the exact section below where you fill the value in.
        </p>

        <div v-if="vaultError" class="vault-error">{{ vaultError }}</div>

        <div class="vault-grid">
          <div
            v-for="entry in vaultEntries"
            :key="entry.id"
            class="vault-row"
            :data-status="entry.status"
          >
            <div class="vault-meta">
              <div class="vault-label">
                <span class="vault-kind" :data-kind="entry.kind">{{ entry.kind }}</span>
                {{ entry.label }}
              </div>
              <div class="vault-msg">{{ entry.message || ' ' }}</div>
              <div v-if="entry.lastCheckedAt" class="vault-checked">
                checked {{ formatVaultCheck(entry.lastCheckedAt) }}
              </div>
            </div>

            <div class="vault-status-pill" :data-status="entry.status">
              {{ vaultStatusLabel(entry.status) }}
            </div>

            <div class="vault-row-actions">
              <button
                v-if="entry.probable"
                class="vault-btn"
                :disabled="vaultProbing[entry.id] || !entry.configured"
                @click="testVaultEntry(entry)"
                :title="entry.configured ? 'Run a live probe against the provider' : 'Configure first'"
              >{{ vaultProbing[entry.id] ? 'Testing…' : 'Test' }}</button>

              <button
                class="vault-btn primary"
                @click="configureVaultEntry(entry)"
                :title="'Jump to ' + entry.label + ' configuration'"
              >{{ entry.configured ? 'Edit' : 'Configure' }}</button>
            </div>
          </div>
        </div>

        <!-- External Secrets: first-class integration for the encryption
             tooling teams pair with Kubernetes (SealedSecrets / SOPS / PGP)
             plus External Secrets Operator backends (AWS / GCP / Azure /
             HashiCorp Vault). Local CLIs are auto-detected; ESO config
             names which backends the user wants the operator wired to. -->
        <div class="subsection" id="external-secrets">
          <div class="subsection-title-row">
            <div class="subsection-title">External Secrets</div>
            <div v-if="esStore.localToolCount" class="es-tool-badge" :class="{ none: esStore.localToolFound === 0 }">
              {{ esStore.localToolFound }} / {{ esStore.localToolCount }} CLIs detected
            </div>
          </div>
          <p class="hint" style="margin-top:-2px;">
            Local CLI tooling is auto-detected on PATH. External Secrets Operator
            backends are toggles for the in-cluster controller — credentials live
            in cluster Secrets, never in this app.
          </p>

          <div class="es-group-title">Local encryption tooling</div>
          <div class="es-grid">
            <!-- Bitnami SealedSecrets (kubeseal) -->
            <div class="es-card" :class="{ enabled: esStore.config.kubeseal.enabled }">
              <div class="es-card-head">
                <div class="es-card-title">
                  <span class="es-tool-name">Bitnami Sealed Secrets</span>
                  <span class="es-tool-bin mono">kubeseal</span>
                </div>
                <label class="es-switch">
                  <input type="checkbox" :checked="esStore.config.kubeseal.enabled"
                    @change="esStore.setEnabled('kubeseal', $event.target.checked)" />
                  <span class="es-switch-track"></span>
                </label>
              </div>
              <SecretsToolProbeRow tool="kubeseal" :probe="esStore.probes.kubeseal" :probing="!!esStore.probing.kubeseal" @test="testSecretsTool('kubeseal')" />
              <div v-if="esStore.config.kubeseal.enabled" class="es-card-fields">
                <div class="es-field-row">
                  <label>Controller namespace</label>
                  <input class="input mono short" :value="esStore.config.kubeseal.controllerNamespace"
                    @input="esStore.setSection('kubeseal', { controllerNamespace: $event.target.value })" />
                </div>
                <div class="es-field-row">
                  <label>Controller name</label>
                  <input class="input mono short" :value="esStore.config.kubeseal.controllerName"
                    @input="esStore.setSection('kubeseal', { controllerName: $event.target.value })" />
                </div>
              </div>
            </div>

            <!-- SOPS -->
            <div class="es-card" :class="{ enabled: esStore.config.sops.enabled }">
              <div class="es-card-head">
                <div class="es-card-title">
                  <span class="es-tool-name">SOPS</span>
                  <span class="es-tool-bin mono">sops</span>
                </div>
                <label class="es-switch">
                  <input type="checkbox" :checked="esStore.config.sops.enabled"
                    @change="esStore.setEnabled('sops', $event.target.checked)" />
                  <span class="es-switch-track"></span>
                </label>
              </div>
              <SecretsToolProbeRow tool="sops" :probe="esStore.probes.sops" :probing="!!esStore.probing.sops" @test="testSecretsTool('sops')" />
              <div v-if="esStore.config.sops.enabled" class="es-card-fields">
                <div class="es-field-row">
                  <label>Default key file</label>
                  <input class="input mono short" :value="esStore.config.sops.defaultKeyFile"
                    placeholder="~/.config/sops/age/keys.txt"
                    @input="esStore.setSection('sops', { defaultKeyFile: $event.target.value })" />
                </div>
                <p class="es-hint">
                  Pairs with <code>age</code> below for keyless workflows, or with GPG for legacy SOPS files (<code>secrets.enc.json</code>).
                </p>
              </div>
            </div>

            <!-- GPG / PGP -->
            <div class="es-card" :class="{ enabled: esStore.config.gpg.enabled }">
              <div class="es-card-head">
                <div class="es-card-title">
                  <span class="es-tool-name">GPG / PGP</span>
                  <span class="es-tool-bin mono">gpg</span>
                </div>
                <label class="es-switch">
                  <input type="checkbox" :checked="esStore.config.gpg.enabled"
                    @change="esStore.setEnabled('gpg', $event.target.checked)" />
                  <span class="es-switch-track"></span>
                </label>
              </div>
              <SecretsToolProbeRow tool="gpg" :probe="esStore.probes.gpg" :probing="!!esStore.probing.gpg" @test="testSecretsTool('gpg')" />
              <div v-if="esStore.config.gpg.enabled" class="es-card-fields">
                <div class="es-field-row">
                  <label>Default key id</label>
                  <input class="input mono short" :value="esStore.config.gpg.defaultKeyId"
                    placeholder="ABCD1234EF…"
                    @input="esStore.setSection('gpg', { defaultKeyId: $event.target.value })" />
                </div>
              </div>
            </div>

            <!-- age -->
            <div class="es-card" :class="{ enabled: esStore.config.age.enabled }">
              <div class="es-card-head">
                <div class="es-card-title">
                  <span class="es-tool-name">age</span>
                  <span class="es-tool-bin mono">age</span>
                </div>
                <label class="es-switch">
                  <input type="checkbox" :checked="esStore.config.age.enabled"
                    @change="esStore.setEnabled('age', $event.target.checked)" />
                  <span class="es-switch-track"></span>
                </label>
              </div>
              <SecretsToolProbeRow tool="age" :probe="esStore.probes.age" :probing="!!esStore.probing.age" @test="testSecretsTool('age')" />
              <div v-if="esStore.config.age.enabled" class="es-card-fields">
                <div class="es-field-row">
                  <label>Default key file</label>
                  <input class="input mono short" :value="esStore.config.age.defaultKeyFile"
                    placeholder="~/.config/sops/age/keys.txt"
                    @input="esStore.setSection('age', { defaultKeyFile: $event.target.value })" />
                </div>
              </div>
            </div>
          </div>

          <div class="es-group-title">External Secrets Operator backends</div>
          <p class="hint" style="margin-top:-4px; margin-bottom:8px;">
            Toggle a backend to mark which providers this cluster's ESO instance should be aware of.
            The actual credentials are stored as <code>SecretStore</code>/<code>ClusterSecretStore</code>
            CRs in the cluster — this section just records the user-facing intent.
          </p>
          <div class="es-grid">
            <!-- AWS -->
            <div class="es-card" :class="{ enabled: esStore.config.esoAws.enabled }">
              <div class="es-card-head">
                <div class="es-card-title"><span class="es-tool-name">AWS Secrets Manager</span></div>
                <label class="es-switch">
                  <input type="checkbox" :checked="esStore.config.esoAws.enabled"
                    @change="esStore.setEnabled('esoAws', $event.target.checked)" />
                  <span class="es-switch-track"></span>
                </label>
              </div>
              <div v-if="esStore.config.esoAws.enabled" class="es-card-fields">
                <div class="es-field-row">
                  <label>Region</label>
                  <input class="input mono short" :value="esStore.config.esoAws.region"
                    @input="esStore.setSection('esoAws', { region: $event.target.value })" />
                </div>
                <div class="es-field-row">
                  <label>Auth Secret name</label>
                  <input class="input mono short" :value="esStore.config.esoAws.authRef"
                    placeholder="aws-secret-store-auth"
                    @input="esStore.setSection('esoAws', { authRef: $event.target.value })" />
                </div>
              </div>
            </div>

            <!-- GCP -->
            <div class="es-card" :class="{ enabled: esStore.config.esoGcp.enabled }">
              <div class="es-card-head">
                <div class="es-card-title"><span class="es-tool-name">Google Secret Manager</span></div>
                <label class="es-switch">
                  <input type="checkbox" :checked="esStore.config.esoGcp.enabled"
                    @change="esStore.setEnabled('esoGcp', $event.target.checked)" />
                  <span class="es-switch-track"></span>
                </label>
              </div>
              <div v-if="esStore.config.esoGcp.enabled" class="es-card-fields">
                <div class="es-field-row">
                  <label>Project ID</label>
                  <input class="input mono short" :value="esStore.config.esoGcp.projectId"
                    @input="esStore.setSection('esoGcp', { projectId: $event.target.value })" />
                </div>
                <div class="es-field-row">
                  <label>Auth Secret name</label>
                  <input class="input mono short" :value="esStore.config.esoGcp.authRef"
                    placeholder="gcp-sa-key"
                    @input="esStore.setSection('esoGcp', { authRef: $event.target.value })" />
                </div>
              </div>
            </div>

            <!-- Azure -->
            <div class="es-card" :class="{ enabled: esStore.config.esoAzure.enabled }">
              <div class="es-card-head">
                <div class="es-card-title"><span class="es-tool-name">Azure Key Vault</span></div>
                <label class="es-switch">
                  <input type="checkbox" :checked="esStore.config.esoAzure.enabled"
                    @change="esStore.setEnabled('esoAzure', $event.target.checked)" />
                  <span class="es-switch-track"></span>
                </label>
              </div>
              <div v-if="esStore.config.esoAzure.enabled" class="es-card-fields">
                <div class="es-field-row">
                  <label>Vault URL</label>
                  <input class="input mono short" :value="esStore.config.esoAzure.vaultUrl"
                    placeholder="https://my-vault.vault.azure.net"
                    @input="esStore.setSection('esoAzure', { vaultUrl: $event.target.value })" />
                </div>
                <div class="es-field-row">
                  <label>Tenant ID</label>
                  <input class="input mono short" :value="esStore.config.esoAzure.tenantId"
                    @input="esStore.setSection('esoAzure', { tenantId: $event.target.value })" />
                </div>
                <div class="es-field-row">
                  <label>Auth Secret name</label>
                  <input class="input mono short" :value="esStore.config.esoAzure.authRef"
                    placeholder="azure-credentials"
                    @input="esStore.setSection('esoAzure', { authRef: $event.target.value })" />
                </div>
              </div>
            </div>

            <!-- HashiCorp Vault -->
            <div class="es-card" :class="{ enabled: esStore.config.esoVault.enabled }">
              <div class="es-card-head">
                <div class="es-card-title"><span class="es-tool-name">HashiCorp Vault</span></div>
                <label class="es-switch">
                  <input type="checkbox" :checked="esStore.config.esoVault.enabled"
                    @change="esStore.setEnabled('esoVault', $event.target.checked)" />
                  <span class="es-switch-track"></span>
                </label>
              </div>
              <div v-if="esStore.config.esoVault.enabled" class="es-card-fields">
                <div class="es-field-row">
                  <label>Server address</label>
                  <input class="input mono short" :value="esStore.config.esoVault.address"
                    placeholder="https://vault.example.com"
                    @input="esStore.setSection('esoVault', { address: $event.target.value })" />
                </div>
                <div class="es-field-row">
                  <label>Engine path</label>
                  <input class="input mono short" :value="esStore.config.esoVault.path"
                    placeholder="secret/data"
                    @input="esStore.setSection('esoVault', { path: $event.target.value })" />
                </div>
                <div class="es-field-row">
                  <label>Auth method</label>
                  <Select
                    :model-value="esStore.config.esoVault.authMethod"
                    @change="(val) => esStore.setSection('esoVault', { authMethod: val })"
                    :options="[{value:'kubernetes',label:'kubernetes'},{value:'approle',label:'approle'},{value:'token',label:'token'},{value:'iam',label:'aws-iam'}]"
                    size="sm"
                  />
                </div>
              </div>
            </div>
          </div>
        </div>

        <!-- Custom secrets: user-managed key/value entries persisted to
             ~/.argus/vault/secrets.json, masked on read. Useful for
             storing one-off API keys / tokens the app doesn't have a
             dedicated UI for yet. -->
        <div class="subsection">
          <div class="subsection-title">Custom Secrets</div>
          <p class="hint" style="margin-top:-2px;">
            Free-form key/value entries kept on this machine, masked when displayed.
            Use them for anything the dedicated panels above don't cover.
          </p>

          <div v-if="vaultSecretsError" class="vault-error">{{ vaultSecretsError }}</div>

          <div class="secret-add-row">
            <input v-model="secretDraft.key" class="input mono short" placeholder="Key, e.g. JIRA_TOKEN" />
            <input v-model="secretDraft.value" type="password" class="input mono" placeholder="Value (won't be re-displayed)" />
            <input v-model="secretDraft.notes" class="input" placeholder="Notes (optional)" />
            <button
              class="vault-btn primary"
              :disabled="!secretDraft.key.trim() || !secretDraft.value || secretSaving"
              @click="saveCustomSecret"
            >{{ secretSaving ? 'Saving…' : 'Add / Update' }}</button>
          </div>

          <div v-if="!vaultSecrets.length" class="vault-empty">No custom secrets stored yet.</div>
          <div v-else class="secret-list">
            <div v-for="s in vaultSecrets" :key="s.key" class="secret-row">
              <div class="secret-key mono">{{ s.key }}</div>
              <div class="secret-value mono">{{ s.valueMask || '••••' }}</div>
              <div class="secret-notes">{{ s.notes || ' ' }}</div>
              <div class="secret-stamp">updated {{ formatVaultCheck(s.updatedAt) }}</div>
              <button class="vault-btn danger" @click="removeCustomSecret(s.key)" title="Remove">Delete</button>
            </div>
          </div>
        </div>
      </div>

      <!-- Sign-in & integrations. Replaces the previous "set ARGUS_*
           env vars and restart" UX: paste OAuth client credentials in
           here, hit Save, and the backend hot-reloads providers. -->
      <div class="section" id="sign-in-integrations">
        <h2 class="section-title">Sign-in &amp; integrations</h2>
        <p class="hint" style="margin: -6px 0 14px; font-size: 12px; color: var(--text3); line-height: 1.45;">
          Paste OAuth client credentials below to enable Sign-In providers and
          Workspace Connect buttons. Saved values are encrypted on disk;
          no backend restart needed.
        </p>

        <!-- Collapsible Google Auth step-by-step guide -->
        <div class="guide-card">
          <div class="guide-header" @click="googleGuideOpen = !googleGuideOpen">
            <span class="guide-title">How Google Auth Works</span>
            <span class="guide-toggle">{{ googleGuideOpen ? '−' : '+' }}</span>
          </div>
          <div v-if="googleGuideOpen" class="guide-body">
            <div class="guide-step">
              <span class="step-num">1</span>
              <div class="step-content">
                <strong>Developer: register the app in GCP once</strong>
                <p class="step-hint">The app administrator creates OAuth credentials once at <a href="https://console.cloud.google.com/apis/credentials" target="_blank" rel="noopener" class="guide-link">Google Cloud Console</a> and enters them below. Choose <strong>Desktop app</strong> as the application type — the PKCE flow with loopback IP handles auth securely without a client secret on the client side.</p>
              </div>
            </div>
            <div class="guide-step">
              <span class="step-num">2</span>
              <div class="step-content">
                <strong>End-user: clicks "Connect" — no GCP access needed</strong>
                <p class="step-hint">Users see a single <strong>Connect Google Workspace</strong> button. Clicking it opens a browser popup to Google's login page where they authorize the app. After granting permission, the browser says <em>"Success, close this tab"</em> and the app is connected.</p>
              </div>
            </div>
            <div class="guide-step">
              <span class="step-num">3</span>
              <div class="step-content">
                <strong>Behind the scenes: PKCE + loopback flow</strong>
                <p class="step-hint">The app starts a temporary local server on <code>http://127.0.0.1:54321</code>, opens the browser to Google's authorization URL, catches the redirect with the auth code, exchanges it for tokens, and stores the refresh token securely (macOS Keychain / Windows Credential Locker / Linux Secret Service).</p>
              </div>
            </div>
            <div class="guide-step">
              <span class="step-num">4</span>
              <div class="step-content">
                <strong>Silent token refresh</strong>
                <p class="step-hint">Access tokens expire after 1 hour. The app uses the stored refresh token to silently obtain new tokens without bothering the user again. No further GCP interaction is ever needed.</p>
              </div>
            </div>
          </div>
        </div>

        <!-- Sub-section: Sign-in providers (used by the LoginView). -->
        <div class="sub-section">
          <h3 class="subsection-title">Sign-in providers</h3>

          <div class="field-group">
            <div class="field"><label class="label">Google client ID</label>
              <input v-model="form.googleClientId" type="text" class="input mono" placeholder="123…-abc.apps.googleusercontent.com" />
            </div>
            <div class="field"><label class="label">Google client secret</label>
              <RevealableInput v-model="form.googleClientSecret" input-class="input mono" placeholder="GOCSPX-…" />
            </div>
          </div>

          <div class="field-group">
            <div class="field"><label class="label">OIDC issuer URL</label>
              <input v-model="form.oidcIssuer" type="text" class="input mono" placeholder="https://acme.okta.com" />
            </div>
            <div class="field"><label class="label">OIDC display name</label>
              <input v-model="form.oidcDisplayName" type="text" class="input" placeholder="Corporate SSO" />
            </div>
          </div>
          <div class="field-group">
            <div class="field"><label class="label">OIDC client ID</label>
              <input v-model="form.oidcClientId" type="text" class="input mono" />
            </div>
            <div class="field"><label class="label">OIDC client secret</label>
              <RevealableInput v-model="form.oidcClientSecret" input-class="input mono" />
            </div>
          </div>

          <div class="field-group">
            <div class="field"><label class="label">Apple services ID</label>
              <input v-model="form.appleServicesId" type="text" class="input mono" placeholder="com.argus.signin" />
            </div>
            <div class="field"><label class="label">Apple team ID</label>
              <input v-model="form.appleTeamId" type="text" class="input mono" placeholder="ABCD123456" />
            </div>
          </div>
          <div class="field-group">
            <div class="field"><label class="label">Apple key ID</label>
              <input v-model="form.appleKeyId" type="text" class="input mono" placeholder="KEYID67890" />
            </div>
            <div class="field"><label class="label">Apple display name</label>
              <input v-model="form.appleDisplayName" type="text" class="input" placeholder="Apple" />
            </div>
          </div>
          <div class="field">
            <label class="label">Apple private key (.p8 contents)</label>
            <RevealableInput v-model="form.applePrivateKey" input-class="input mono"
              placeholder="-----BEGIN PRIVATE KEY-----…" />
          </div>

          <div class="field-group">
            <div class="field">
              <label class="toggle">
                <input type="checkbox" v-model="form.passkeyEnabled" />
                <span>Enable passkeys (WebAuthn)</span>
              </label>
            </div>
            <div class="field">
              <label class="toggle">
                <input type="checkbox" v-model="form.allowLocalSignup" />
                <span>Allow email/password sign-up</span>
              </label>
            </div>
          </div>
          <div v-if="form.passkeyEnabled" class="field-group">
            <div class="field"><label class="label">Passkey RP ID</label>
              <input v-model="form.passkeyRpId" type="text" class="input mono" placeholder="localhost" />
            </div>
            <div class="field"><label class="label">Passkey RP name</label>
              <input v-model="form.passkeyRpName" type="text" class="input" placeholder="Argus" />
            </div>
            <div class="field"><label class="label">Passkey RP origin</label>
              <input v-model="form.passkeyRpOrigin" type="text" class="input mono" placeholder="http://localhost:8080" />
            </div>
          </div>
        </div>

        <!-- Sub-section: Workspace OAuth (distinct from sign-in Google
             because the scopes/consent screen are independent). -->
        <div class="sub-section">
          <h3 class="subsection-title">Workspace OAuth clients</h3>
          <p class="hint" style="margin: 0 0 10px; font-size: 12px; color: var(--text3);">
            These power the Connect buttons on the Workspace page. Slack and
            Google Workspace each need their own client ID/secret — separate
            from the sign-in credentials above.
          </p>

          <div class="field-group">
            <div class="field"><label class="label">Google Workspace client ID</label>
              <input v-model="form.workspaceGoogleClientId" type="text" class="input mono" />
            </div>
            <div class="field"><label class="label">Google Workspace client secret</label>
              <RevealableInput v-model="form.workspaceGoogleClientSecret" input-class="input mono" />
            </div>
          </div>

          <div class="field-group">
            <div class="field"><label class="label">Slack client ID</label>
              <input v-model="form.slackClientId" type="text" class="input mono" />
            </div>
            <div class="field"><label class="label">Slack client secret</label>
              <RevealableInput v-model="form.slackClientSecret" input-class="input mono" />
            </div>
          </div>
          <div class="field">
            <label class="label">Slack signing secret (Events API)</label>
            <RevealableInput v-model="form.slackSigningSecret" input-class="input mono" />
          </div>
        </div>
      </div>

      <div class="section" id="appearance">
        <h2 class="section-title">Appearance</h2>

        <div class="field">
          <label class="label">Theme</label>
          <div class="chip-row">
            <button type="button" class="chip" :class="{ active: appTheme === 'dark' }" @click="appearance.setTheme('dark')">
              <span class="swatch" data-swatch="dark"></span>Dark
            </button>
            <button type="button" class="chip" :class="{ active: appTheme === 'light' }" @click="appearance.setTheme('light')">
              <span class="swatch" data-swatch="light"></span>Light
            </button>
            <button type="button" class="chip" :class="{ active: appTheme === 'auto' }" @click="appearance.setTheme('auto')">
              <span class="swatch" data-swatch="auto"></span>Auto
            </button>
          </div>
        </div>

        <div class="field">
          <div class="slider-head">
            <label class="label">Brightness</label>
            <span class="slider-val">{{ brightness }}%</span>
          </div>
          <input type="range" class="slider" :min="ranges.brightness[0]" :max="ranges.brightness[1]" :value="brightness" @input="appearance.setBrightness(Number($event.target.value))" />
        </div>

        <div class="field">
          <div class="slider-head">
            <label class="label">Contrast</label>
            <span class="slider-val">{{ contrast }}%</span>
          </div>
          <input type="range" class="slider" :min="ranges.contrast[0]" :max="ranges.contrast[1]" :value="contrast" @input="appearance.setContrast(Number($event.target.value))" />
        </div>

        <div class="field">
          <div class="slider-head">
            <label class="label">Saturation</label>
            <span class="slider-val">{{ saturation }}%</span>
          </div>
          <input type="range" class="slider" :min="ranges.saturation[0]" :max="ranges.saturation[1]" :value="saturation" @input="appearance.setSaturation(Number($event.target.value))" />
        </div>

        <div class="field">
          <div class="slider-head">
            <label class="label">Window opacity</label>
            <span class="slider-val">{{ opacity }}%</span>
          </div>
          <input type="range" class="slider" :min="ranges.opacity[0]" :max="ranges.opacity[1]" :value="opacity" @input="appearance.setOpacity(Number($event.target.value))" />
        </div>

        <div class="field">
          <div class="slider-head">
            <label class="label">Window blur</label>
            <span class="slider-val">{{ blur }}px</span>
          </div>
          <input type="range" class="slider" :min="ranges.blur[0]" :max="ranges.blur[1]" :value="blur" @input="appearance.setBlur(Number($event.target.value))" />
        </div>

        <div class="field">
          <label class="label">UI density</label>
          <div class="chip-row">
            <button v-for="d in ['compact', 'normal', 'comfortable']" :key="d" type="button" class="chip" :class="{ active: density === d }" @click="appearance.setDensity(d)">{{ d.charAt(0).toUpperCase() + d.slice(1) }}</button>
          </div>
        </div>

        <div class="field">
          <div class="slider-head">
            <label class="label">Font size</label>
            <span class="slider-val">{{ fontSize }}px</span>
          </div>
          <input
            type="range"
            class="slider"
            :min="fontSizeRange[0]"
            :max="fontSizeRange[1]"
            :value="fontSize"
            data-testid="appearance-font-size"
            @input="appearance.setFontSize(Number($event.target.value))"
          />
        </div>

        <button class="text-btn" @click="appearance.reset()">Reset to defaults</button>
      </div>

      <!-- Navigation visibility — which sidebar sections show. Core
           sections are always present; optional ones reveal when their
           matching subsystem is detected, or via the toggles here. -->
      <div class="section" id="settings-navigation" data-testid="settings-navigation">
        <h2 class="section-title">Navigation</h2>
        <p class="hint">
          Choose which sections appear in the sidebar. Core sections stay
          visible; optional ones can be toggled at any time.
        </p>
        <div class="nav-toggle-grid">
          <div
            v-for="sec in navSections"
            :key="sec.id"
            class="nav-toggle-row"
            :class="{ core: sec.core }"
            :data-testid="`nav-toggle-${sec.id}`"
          >
            <div class="nav-toggle-info">
              <div class="nav-toggle-label">
                {{ sec.label }}
                <span v-if="sec.core" class="core-badge">Core</span>
              </div>
              <div v-if="sec.hint" class="nav-toggle-hint">{{ sec.hint }}</div>
            </div>
            <label class="toggle" :title="sec.core ? 'Core sections are always visible' : ''">
              <input
                type="checkbox"
                :checked="navVisibility.isVisible(sec.id)"
                :disabled="sec.core"
                :data-testid="`nav-toggle-input-${sec.id}`"
                @change="navVisibility.toggle(sec.id)"
                class="toggle-input"
              />
              <span class="toggle-track"><span class="toggle-thumb"></span></span>
            </label>
          </div>
        </div>
        <button
          class="text-btn"
          data-testid="nav-toggle-reset"
          @click="navVisibility.resetToDefaults()"
        >Reset to defaults</button>
      </div>

      <div class="section" id="notifications-general">
        <h2 class="section-title">Notifications</h2>

        <div class="field">
          <label class="label">Max notifications kept</label>
          <div class="row">
            <input v-model.number="draftNotifMax" type="number" min="1" max="5000" step="50" class="input short" />
            <button class="btn" @click="applyNotifMax">Apply</button>
          </div>
          <p class="hint">Limits notifications stored in the bell panel. Range 1\u20135000.</p>
        </div>

        <div class="field">
          <label class="label">Spot-checks</label>
          <button class="btn" :disabled="spotChecksRunning" @click="triggerSpotChecks">{{ spotChecksRunning ? 'Running\u2026' : 'Run spot-checks' }}</button>
        </div>
      </div>

      <div class="section nc-section" id="notification-channels" ref="channelsAnchorRef">
        <div class="nc-header">
          <h2 class="section-title" style="margin:0;">Notification Channels</h2>
          <transition name="nc-banner-fade">
            <div v-if="returnBanner.state === 'pulse' && returnBanner.target?.navId !== 'pipelines'" class="nc-banner pulse" key="pulse">
              <span class="nc-banner-text">I will take you back when you're done.</span>
            </div>
            <button
              v-else-if="returnBanner.state === 'go-back' && returnBanner.target?.navId !== 'pipelines'"
              class="nc-banner go-back"
              key="go-back"
              @click="goBackToOrigin"
            >\u2190 Go back to <span class="mono">{{ returnBanner.label }}</span></button>
          </transition>
        </div>
        <p class="hint" style="margin-top:6px;">
          Alerts go to every enabled channel. Without at least one, alerts will fire silently in the bell panel only.
        </p>

        <div class="nc-add">
          <span class="nc-add-label">Add channel:</span>
          <button v-for="k in NEW_CHANNEL_KINDS" :key="k.id" class="nc-add-btn" @click="addChannel(k.id)" :title="k.hint">+ {{ k.label }}</button>
        </div>

        <div v-if="!notifChannelsStore.channels.length" class="nc-empty">
          No channels yet. Add one above so alerts can be delivered.
        </div>

        <div v-else class="nc-list">
          <div v-for="ch in notifChannelsStore.channels" :key="ch.id" class="nc-row" :class="{ disabled: !ch.enabled }">
            <label class="nc-toggle" :title="ch.enabled ? 'Disable' : 'Enable'">
              <input type="checkbox" :checked="ch.enabled" @change="updateChannel(ch.id, { enabled: $event.target.checked })" />
            </label>
            <div class="nc-kind" :data-kind="ch.kind">{{ ch.kind }}</div>
            <input class="nc-label-in" :value="ch.label" @input="updateChannel(ch.id, { label: $event.target.value })" placeholder="Label" />
            <input v-if="ch.kind !== 'desktop'" class="nc-target-in mono" :value="ch.target" @input="updateChannel(ch.id, { target: $event.target.value })" :placeholder="channelPlaceholder(ch.kind)" />
            <span v-else class="nc-target-noop">no target needed</span>
            <button class="nc-remove" @click="removeChannel(ch.id)" title="Remove">\u00d7</button>
          </div>
        </div>
      </div>

      <!-- Watchers & Notifications: the user-visible surface for the
           generic watcher framework + the anti-spam guard. Every
           expirable thing in the app (credentials today, certs/licenses
           later) registers itself with the registry and shows up here. -->
      <div class="section" id="watchers-notifications" ref="watchersAnchorRef">
        <div class="section-header-row">
          <h2 class="section-title">Watchers &amp; Notifications</h2>
          <button class="vault-refresh" @click="runAllWatchersNow" :disabled="watchersRunning">
            {{ watchersRunning ? 'Running\u2026' : 'Re-check all' }}
          </button>
        </div>
        <p class="hint">
          Anything in the app that can expire (tokens, certs, licenses, \u2026) is
          registered here. Each watcher polls on its own interval; results
          flow through a single anti-spam guard so the user is never
          duplicate-notified or drowned in repeats.
        </p>

        <!-- Spam-guard config -->
        <div class="subsection">
          <div class="subsection-title">Anti-spam guard</div>
          <p class="hint" style="margin-top:-2px;">
            If a single source fires more than the threshold inside the
            window, the rest are suppressed and one acknowledgeable warning
            takes their place. Cap is hard-clamped to 24 hours.
          </p>
          <div class="watcher-guard-grid">
            <label class="toggle" style="grid-column: 1 / -1;">
              <input type="checkbox" class="toggle-input" :checked="guard.settings.enabled"
                @change="guard.setSettings({ enabled: $event.target.checked })" />
              <span class="toggle-track"><span class="toggle-thumb"></span></span>
              <span class="toggle-label">Anti-spam guard enabled</span>
            </label>
            <div class="watcher-field">
              <label>Spam threshold</label>
              <input type="number" min="1" max="100" class="input mono short"
                :value="guard.settings.spamThreshold"
                @change="guard.setSettings({ spamThreshold: Math.max(1, Number($event.target.value) || 1) })" />
            </div>
            <div class="watcher-field">
              <label>Spam window (min)</label>
              <input type="number" min="1" max="240" class="input mono short"
                :value="Math.round(guard.settings.spamWindowMs / 60000)"
                @change="guard.setSettings({ spamWindowMs: Math.max(1, Number($event.target.value) || 1) * 60000 })" />
            </div>
            <div class="watcher-field">
              <label>Default silence (min)</label>
              <input type="number" min="1" :max="24 * 60" class="input mono short"
                :value="Math.round(guard.settings.defaultSilenceMs / 60000)"
                @change="guard.setSettings({ defaultSilenceMs: Math.max(1, Number($event.target.value) || 1) * 60000 })" />
            </div>
          </div>
        </div>

        <!-- Active silences -->
        <div v-if="guard.activeSilences.length" class="subsection">
          <div class="subsection-title">Active silences</div>
          <div class="watcher-silence-list">
            <div v-for="s in guard.activeSilences" :key="s.source" class="watcher-silence-row" :class="{ unack: !s.acknowledged }">
              <div class="watcher-silence-meta">
                <div class="watcher-silence-label">
                  <span class="watcher-reason" :data-reason="s.reason">{{ s.reason }}</span>
                  {{ s.label }}
                </div>
                <div class="watcher-silence-detail">
                  Until <span class="mono">{{ formatLocal(s.until) }}</span>
                  <template v-if="s.pendingCount"> \u00b7 {{ s.pendingCount }} suppressed</template>
                </div>
              </div>
              <div class="watcher-silence-actions">
                <button v-if="!s.acknowledged" class="vault-btn primary" @click="guard.acknowledge(s.source)">Acknowledge</button>
                <button class="vault-btn" @click="guard.unsilence(s.source)">Unsilence</button>
              </div>
            </div>
          </div>
        </div>

        <!-- Registered watchers -->
        <div class="subsection">
          <div class="subsection-title">Registered watchers ({{ registry.list.length }})</div>
          <div v-if="!registry.list.length" class="vault-empty">
            No watchers registered yet. They auto-register as features (credentials, certs, \u2026) come online.
          </div>
          <div v-else class="watcher-list">
            <div v-for="w in registry.list" :key="w.id" class="watcher-row" :data-status="watcherStatusOf(w)">
              <div class="watcher-meta">
                <div class="watcher-label">
                  <span class="watcher-kind">{{ w.kind }}</span>
                  {{ w.label }}
                </div>
                <div class="watcher-msg">
                  <template v-if="registry.results[w.id]?.message">{{ registry.results[w.id].message }}</template>
                  <template v-else>No probe result yet.</template>
                </div>
                <div class="watcher-checked" v-if="registry.lastCheckedAt[w.id]">
                  last check {{ formatRelative(registry.lastCheckedAt[w.id]) }}
                </div>
              </div>
              <div class="watcher-status-pill" :data-status="watcherStatusOf(w)">{{ watcherStatusOf(w) || 'pending' }}</div>
              <div class="watcher-row-actions">
                <Select
                  :model-value="w.intervalMs"
                  @change="(val) => registry.setInterval(w.id, Number(val))"
                  :options="[{value:60000,label:'every 1 min'},{value:300000,label:'every 5 min'},{value:900000,label:'every 15 min'},{value:1800000,label:'every 30 min'},{value:3600000,label:'every 1 h'},{value:21600000,label:'every 6 h'},{value:86400000,label:'every 24 h'}]"
                  size="sm"
                />
                <label class="watcher-enable" :title="w.enabled ? 'Disable watcher' : 'Enable watcher'">
                  <input type="checkbox" :checked="w.enabled"
                    @change="registry.setEnabled(w.id, $event.target.checked)" />
                  <span>{{ w.enabled ? 'on' : 'off' }}</span>
                </label>
                <button class="vault-btn" @click="runOneWatcher(w.id)" :disabled="!w.enabled || watcherRunningId === w.id">
                  {{ watcherRunningId === w.id ? 'Running\u2026' : 'Re-check' }}
                </button>
                <button v-if="!guard.silences[w.id]" class="vault-btn"
                  @click="guard.silence(w.id, guard.settings.defaultSilenceMs, { label: w.label, anchor: w.configureAnchor, reason: 'manual' })">Silence</button>
                <button v-else class="vault-btn primary" @click="guard.unsilence(w.id)">Unsilence</button>
              </div>
            </div>
          </div>
        </div>
      </div>

      <div class="section" id="agent-profile">
        <h2 class="section-title">Agent Profile</h2>

        <label class="toggle">
          <input type="checkbox" class="toggle-input" v-model="agentProfile.autoInvestigate" />
          <span class="toggle-track"><span class="toggle-thumb"></span></span>
          <span class="toggle-label">Auto-investigate new alerts</span>
        </label>

        <label class="toggle">
          <input type="checkbox" class="toggle-input" v-model="agentProfile.autoDocument" />
          <span class="toggle-track"><span class="toggle-thumb"></span></span>
          <span class="toggle-label">Auto-document fires, silences, and dismissals</span>
        </label>

        <label class="toggle">
          <input type="checkbox" class="toggle-input" v-model="agentProfile.canAck" />
          <span class="toggle-track"><span class="toggle-thumb"></span></span>
          <span class="toggle-label">Allow agent to acknowledge alerts</span>
        </label>

        <label class="toggle">
          <input type="checkbox" class="toggle-input" v-model="agentProfile.canSilence" />
          <span class="toggle-track"><span class="toggle-thumb"></span></span>
          <span class="toggle-label">Allow agent to silence noisy alerts</span>
        </label>

        <label class="toggle">
          <input type="checkbox" class="toggle-input" v-model="agentProfile.canAdjustParams" />
          <span class="toggle-track"><span class="toggle-thumb"></span></span>
          <span class="toggle-label">Allow agent to adjust alert parameters</span>
        </label>

        <div class="field">
          <label class="label">Silence window (minutes)</label>
          <input type="number" min="1" max="1440" step="5" v-model.number="silenceWindowMin" class="input short" />
        </div>

        <div class="field">
          <label class="label">Fatigue threshold</label>
          <input type="number" min="1" max="100" step="1" v-model.number="agentProfile.fatigueThreshold" class="input short" />
          <p class="hint">Fires a warning after this many silences of the same alert.</p>
        </div>

        <div class="row">
          <button class="btn" :disabled="agentProfileSaving" @click="saveAgentProfile">{{ agentProfileSaving ? 'Saving\u2026' : 'Apply' }}</button>
          <span v-if="agentProfileMsg" class="msg" :class="{ ok: !agentProfileMsg.startsWith('Error'), fail: agentProfileMsg.startsWith('Error') }">{{ agentProfileMsg }}</span>
        </div>
      </div>

      <div class="section" id="kube-connection">
        <h2 class="section-title">Kubernetes Connection</h2>

        <div class="field">
          <label class="label">Kubeconfig path</label>
          <input v-model="form.kubeconfigPath" type="text" class="input mono" placeholder="/Users/you/.kube/config" />
        </div>

        <div class="field">
          <label class="label">Context</label>
          <div class="row">
            <Select
              v-model="form.currentContext"
              :options="[{value:'',label:'Auto (current context)'}, ...contexts.map(ctx => ({value:ctx.name,label:ctx.name + (ctx.active ? ' (active)' : '')}))]"
              size="sm"
              aria-label="Kubernetes context"
            />
            <button class="icon-btn" @click="listContexts" :disabled="ctxLoading">{{ ctxLoading ? '\u2026' : '\u21BB' }}</button>
          </div>
        </div>

        <div class="field">
          <label class="label">Default namespace</label>
          <input v-model="form.namespace" type="text" class="input" placeholder="All namespaces" />
        </div>
      </div>

      <div class="section" id="ai-integrations">
        <h2 class="section-title">AI & Integrations</h2>

        <div class="field">
          <label class="label">API key</label>
          <input v-model="form.deepseekApiKey" type="password" class="input mono" placeholder="sk-\u2026" />
        </div>

        <div class="field">
          <label class="label">LLM base URL</label>
          <input v-model="form.llmBaseUrl" type="text" class="input mono" placeholder="https://api.deepseek.com/v1" />
        </div>

        <div class="field">
          <label class="label">LLM model</label>
          <input v-model="form.llmModel" type="text" class="input mono" placeholder="deepseek-chat" />
        </div>

        <div class="field">
          <label class="label">Anomstack URL</label>
          <input v-model="form.anomstackUrl" type="text" class="input mono" placeholder="http://localhost:8087" />
        </div>

        <div class="field">
          <label class="label">Prometheus URL</label>
          <input v-model="form.prometheusUrl" type="text" class="input mono" placeholder="http://prometheus:9090" />
        </div>

        <div class="field">
          <label class="label">Agent instructions</label>
          <textarea v-model="form.agentInstructions" class="input mono" rows="3" placeholder="Analyze cluster health based on recent events and alerts."></textarea>
          <button class="btn" style="margin-top: 8px;" @click="callGo('TriggerAgentAnalysis')">Test agent analysis</button>
        </div>

        <div class="field">
          <label class="label">MCP servers config</label>
          <textarea v-model="form.mcpServersConfig" class="input mono code-area" rows="4" placeholder='{ "mcpServers": { "my-server": { "command": "npx", "args": ["-y", "mcp-server"] } } }'></textarea>
        </div>
      </div>

      <div class="section" id="arguscd-section">
        <h2 class="section-title">Argo CD</h2>

        <div class="field">
          <label class="label">Server URL</label>
          <input v-model="form.argocdUrl" type="text" class="input mono" placeholder="https://argocd.example.com" />
        </div>

        <div class="field">
          <label class="label">API token</label>
          <input v-model="form.argocdToken" type="password" class="input mono" placeholder="eyJhbGciOi\u2026" />
        </div>

        <div class="field">
          <label class="toggle">
            <input type="checkbox" v-model="form.argocdInsecure" class="toggle-input" />
            <span class="toggle-track"><span class="toggle-thumb"></span></span>
            <span class="toggle-label">Skip TLS verification</span>
          </label>
        </div>

        <div class="row">
          <button class="btn" @click="testArgoCD" :disabled="argocdTesting || !form.argocdUrl">{{ argocdTesting ? 'Testing\u2026' : 'Test connection' }}</button>
          <span v-if="argocdTestResult" class="msg" :class="{ ok: !argocdTestResult.startsWith('Failed'), fail: argocdTestResult.startsWith('Failed') }">{{ argocdTestResult }}</span>
        </div>
      </div>

      <div class="section" id="security-tools">
        <h2 class="section-title">Security Scanning</h2>

        <div class="field">
          <label class="label">Snyk token</label>
          <input v-model="form.snykToken" type="password" class="input mono" placeholder="(optional)" />
        </div>

        <div class="field">
          <label class="label">Trivy binary path</label>
          <input v-model="form.trivyBinary" type="text" class="input mono" placeholder="trivy" />
        </div>

        <div class="field">
          <label class="label">Falco endpoint</label>
          <input v-model="form.falcoUrl" type="text" class="input mono" placeholder="http://falco:8765" />
        </div>
      </div>

      <div class="section" id="pipelines-section">
        <h2 class="section-title">Pipelines & CI/CD</h2>

        <label class="toggle">
          <input type="checkbox" v-model="form.pipelinesEnabled" class="toggle-input" />
          <span class="toggle-track"><span class="toggle-thumb"></span></span>
          <span class="toggle-label">Enable pipeline integration</span>
        </label>

        <div v-if="form.pipelinesEnabled" class="sub-section">
          <p class="hint" style="margin-bottom: 10px;">Select your CI/CD provider.</p>
          <div class="grid">
            <label v-for="p in PIPELINE_PROVIDERS" :key="p.id" class="grid-card" :class="{ active: form.pipelinesProvider === p.id }">
              <input type="radio" name="pipeline-provider" :value="p.id" v-model="form.pipelinesProvider" class="card-radio" />
              <span class="card-label">{{ p.name }}</span>
            </label>
          </div>

          <div v-if="form.pipelinesProvider === 'github'" id="pipelines-github" ref="githubAnchorRef" class="field-group gh-anchor">
            <transition name="nc-banner-fade">
              <div v-if="returnBanner.state === 'pulse' && returnBanner.target?.navId === 'pipelines'" class="nc-banner pulse gh-banner" key="pulse">
                <span class="nc-banner-text">I will take you back when you're done.</span>
              </div>
              <button
                v-else-if="returnBanner.state === 'go-back' && returnBanner.target?.navId === 'pipelines'"
                class="nc-banner go-back gh-banner"
                key="go-back"
                @click="goBackToOrigin"
              >\u2190 Go back to <span class="mono">{{ returnBanner.label }}</span></button>
            </transition>
            <div class="field"><label class="label">Token</label><input v-model="form.githubToken" type="password" class="input mono" placeholder="ghp_\u2026" /></div>
            <div class="field-row two">
              <div class="field"><label class="label">Owner</label><input v-model="form.githubOwner" type="text" class="input mono" placeholder="acme" /></div>
              <div class="field"><label class="label">Repository</label><input v-model="form.githubRepo" type="text" class="input mono" placeholder="argus" /></div>
            </div>
            <div class="field"><label class="label">Workflow file</label><input v-model="form.githubWorkflow" type="text" class="input mono" placeholder="deploy.yml" /></div>
          </div>

          <div v-if="form.pipelinesProvider === 'gitlab'" id="pipelines-gitlab" class="field-group">
            <div class="field"><label class="label">GitLab URL</label><input v-model="form.gitlabUrl" type="text" class="input mono" placeholder="https://gitlab.com" /></div>
            <div class="field"><label class="label">Trigger token</label><input v-model="form.gitlabToken" type="password" class="input mono" placeholder="glpat-\u2026" /></div>
            <div class="field-row two">
              <div class="field"><label class="label">Project ID</label><input v-model="form.gitlabProjectId" type="text" class="input mono" placeholder="12345" /></div>
              <div class="field"><label class="label">Ref</label><input v-model="form.gitlabRef" type="text" class="input mono" placeholder="main" /></div>
            </div>
          </div>

          <div v-if="form.pipelinesProvider === 'aws-codebuild'" id="pipelines-aws" class="field-group">
            <div class="field-row two">
              <div class="field"><label class="label">Region</label><input v-model="form.awsRegion" type="text" class="input mono" placeholder="us-east-1" /></div>
              <div class="field"><label class="label">Project</label><input v-model="form.awsProject" type="text" class="input mono" placeholder="my-build-project" /></div>
            </div>
            <div class="field"><label class="label">Access key ID</label><input v-model="form.awsAccessKey" type="text" class="input mono" placeholder="AKIA\u2026" /></div>
            <div class="field"><label class="label">Secret access key</label><input v-model="form.awsSecretKey" type="password" class="input mono" placeholder="(optional)" /></div>
          </div>

          <div v-if="form.pipelinesProvider === 'gcp-cloudbuild'" id="pipelines-gcp" class="field-group">
            <div class="field-row two">
              <div class="field"><label class="label">Project</label><input v-model="form.gcpProject" type="text" class="input mono" placeholder="my-gcp-project" /></div>
              <div class="field"><label class="label">Region</label><input v-model="form.gcpRegion" type="text" class="input mono" placeholder="global" /></div>
            </div>
            <div class="field"><label class="label">Service account key path</label><input v-model="form.gcpCredentials" type="text" class="input mono" placeholder="/path/to/sa.json" /></div>
          </div>

          <div v-if="form.pipelinesProvider === 'circleci'" id="pipelines-circleci" class="field-group">
            <div class="field"><label class="label">API token</label><input v-model="form.circleciToken" type="password" class="input mono" placeholder="CCI_\u2026" /></div>
            <div class="field"><label class="label">Project slug</label><input v-model="form.circleciProjectSlug" type="text" class="input mono" placeholder="github/acme/kube-watcher" /></div>
          </div>

          <div v-if="form.pipelinesProvider === 'azure'" id="pipelines-azure" class="field-group">
            <div class="field-row two">
              <div class="field"><label class="label">Organization</label><input v-model="form.azureOrganization" type="text" class="input mono" placeholder="my-org" /></div>
              <div class="field"><label class="label">Project</label><input v-model="form.azureProject" type="text" class="input mono" placeholder="my-project" /></div>
            </div>
            <div class="field-row two">
              <div class="field"><label class="label">Pipeline ID</label><input v-model="form.azurePipelineId" type="text" class="input mono" placeholder="42" /></div>
              <div class="field"><label class="label">Branch ref</label><input v-model="form.azureBranch" type="text" class="input mono" placeholder="refs/heads/main" /></div>
            </div>
            <div class="field"><label class="label">Personal access token</label><input v-model="form.azureToken" type="password" class="input mono" /></div>
          </div>

          <div v-if="form.pipelinesProvider === 'aws-codebuild' || form.pipelinesProvider === 'gcp-cloudbuild'" class="field">
            <p class="hint">Leave credentials blank to use the ambient cloud provider credential chain.</p>
          </div>

          <div class="sub-section">
            <h3 class="sub-title">PR notifications</h3>
            <div class="grid tight">
              <label class="toggle"><input type="checkbox" v-model="form.notifyOnPrOpened" class="toggle-input" /><span class="toggle-track"><span class="toggle-thumb"></span></span><span class="toggle-label">Opened</span></label>
              <label class="toggle"><input type="checkbox" v-model="form.notifyOnPrUpdated" class="toggle-input" /><span class="toggle-track"><span class="toggle-thumb"></span></span><span class="toggle-label">Updated</span></label>
              <label class="toggle"><input type="checkbox" v-model="form.notifyOnPrCommented" class="toggle-input" /><span class="toggle-track"><span class="toggle-thumb"></span></span><span class="toggle-label">Commented</span></label>
              <label class="toggle"><input type="checkbox" v-model="form.notifyOnPrMerged" class="toggle-input" /><span class="toggle-track"><span class="toggle-thumb"></span></span><span class="toggle-label">Merged</span></label>
            </div>
          </div>

          <div class="sub-section" id="auto-code-review">
            <h3 class="sub-title">Auto code review</h3>
            <label class="toggle">
              <input type="checkbox" v-model="form.autoCodeReview" class="toggle-input" />
              <span class="toggle-track"><span class="toggle-thumb"></span></span>
              <span class="toggle-label">Run AI code review on every PR</span>
            </label>
            <div v-if="form.autoCodeReview" class="field" style="margin-top: 10px;">
              <label class="label">Publish to</label>
              <div class="grid">
                <label v-for="d in REVIEW_DESTINATIONS" :key="d.id" class="grid-card compact" :class="{ active: form.codeReviewDestination === d.id }">
                  <input type="radio" name="review-destination" :value="d.id" v-model="form.codeReviewDestination" class="card-radio" />
                  <span class="card-label">{{ d.label }}</span>
                </label>
              </div>
              <div v-if="form.codeReviewDestination === 'gdrive'" class="field" style="margin-top: 8px;"><label class="label">Folder ID</label><input v-model="form.gdriveFolderId" type="text" class="input mono" placeholder="1A2b3C4d\u2026" /></div>
              <div v-if="form.codeReviewDestination === 's3'" class="field" style="margin-top: 8px;"><label class="label">Key prefix</label><input v-model="form.codeReviewS3Prefix" type="text" class="input mono" placeholder="code-reviews/" /></div>
              <div v-if="form.codeReviewDestination === 'email'" class="field" style="margin-top: 8px;"><label class="label">Recipients</label><input v-model="form.codeReviewEmailTo" type="text" class="input mono" placeholder="dev-leads@example.com" /></div>
              <div v-if="form.codeReviewDestination === 'confluence'" class="field-group">
                <div class="field"><label class="label">Base URL</label><input v-model="form.confluenceUrl" type="text" class="input mono" placeholder="https://acme.atlassian.net/wiki" /></div>
                <div class="field"><label class="label">Email</label><input v-model="form.confluenceEmail" type="text" class="input mono" placeholder="you@acme.com" /></div>
                <div class="field"><label class="label">API token</label><input v-model="form.confluenceToken" type="password" class="input mono" /></div>
                <div class="field-row two">
                  <div class="field"><label class="label">Space key</label><input v-model="form.confluenceSpaceKey" type="text" class="input mono" placeholder="ENG" /></div>
                  <div class="field"><label class="label">Parent page ID</label><input v-model="form.confluenceParentPageId" type="text" class="input mono" /></div>
                </div>
              </div>
              <div v-if="form.codeReviewDestination === 'notion'" class="field-group">
                <div class="field"><label class="label">Integration token</label><input v-model="form.notionToken" type="password" class="input mono" placeholder="ntn_\u2026" /></div>
                <div class="field"><label class="label">Database ID</label><input v-model="form.notionDatabaseId" type="text" class="input mono" /></div>
              </div>
              <div v-if="form.codeReviewDestination === 'evernote'" class="field-group">
                <div class="field"><label class="label">Developer token</label><input v-model="form.evernoteToken" type="password" class="input mono" placeholder="S=\u2026" /></div>
                <div class="field"><label class="label">Notebook GUID</label><input v-model="form.evernoteNotebookGuid" type="text" class="input mono" /></div>
              </div>
              <div v-if="form.codeReviewDestination === 'onenote'" class="field-group">
                <div class="field"><label class="label">Access token</label><input v-model="form.onenoteToken" type="password" class="input mono" /></div>
                <div class="field"><label class="label">Section ID</label><input v-model="form.onenoteSectionId" type="text" class="input mono" /></div>
              </div>
              <div v-if="form.codeReviewDestination === 'amplenote'" class="field"><label class="label">API key</label><input v-model="form.amplenoteApiKey" type="password" class="input mono" /></div>
              <div v-if="form.codeReviewDestination === 'standard-notes'" class="field-group">
                <div class="field"><label class="label">Server URL</label><input v-model="form.standardNotesUrl" type="text" class="input mono" /></div>
                <div class="field"><label class="label">Session token</label><input v-model="form.standardNotesToken" type="password" class="input mono" /></div>
              </div>
              <div v-if="form.codeReviewDestination === 'obsidian'" class="field"><label class="label">Vault path</label><input v-model="form.obsidianVaultPath" type="text" class="input mono" /></div>
              <div v-if="form.codeReviewDestination === 'joplin'" class="field-group">
                <div class="field"><label class="label">Web clipper URL</label><input v-model="form.joplinUrl" type="text" class="input mono" /></div>
                <div class="field"><label class="label">Token</label><input v-model="form.joplinToken" type="password" class="input mono" /></div>
              </div>
              <div v-if="form.codeReviewDestination === 'logseq'" class="field"><label class="label">Graph path</label><input v-model="form.logseqGraphPath" type="text" class="input mono" /></div>
              <div v-if="form.codeReviewDestination === 'bear'" class="field"><label class="label">API token</label><input v-model="form.bearToken" type="password" class="input mono" /></div>
            </div>
          </div>
        </div>
      </div>

      <div class="section" id="billing-usage">
        <h2 class="section-title">Billing & Usage</h2>

        <div v-if="usageLoading" class="hint">Loading usage\u2026</div>
        <div v-else-if="usageError" class="msg fail">Failed to load: {{ usageError }}</div>
        <div v-else-if="usage">
          <div class="usage-grid">
            <div class="usage-card">
              <div class="usage-lbl">Today</div>
              <div class="usage-val">{{ fmtNumber((usage.today?.in || 0) + (usage.today?.out || 0)) }}<span class="usage-unit">tokens</span></div>
              <div class="usage-sub">{{ fmtNumber(usage.today?.calls) }} calls &middot; {{ fmtCost(usage.today?.estCostUsd) }}</div>
            </div>
            <div class="usage-card">
              <div class="usage-lbl">This month</div>
              <div class="usage-val">{{ fmtNumber((usage.month?.in || 0) + (usage.month?.out || 0)) }}<span class="usage-unit">tokens</span></div>
              <div class="usage-sub">{{ fmtNumber(usage.month?.calls) }} calls &middot; {{ fmtCost(usage.month?.estCostUsd) }}</div>
            </div>
            <div class="usage-card">
              <div class="usage-lbl">Lifetime</div>
              <div class="usage-val">{{ fmtNumber((usage.lifetime?.in || 0) + (usage.lifetime?.out || 0)) }}<span class="usage-unit">tokens</span></div>
              <div class="usage-sub">{{ fmtNumber(usage.lifetime?.calls) }} calls &middot; {{ fmtCost(usage.lifetime?.estCostUsd) }}</div>
            </div>
          </div>

          <div v-if="billing.monthlyBudget > 0" class="budget-row">
            <div class="budget-lbl">
              {{ fmtCost(usage.month?.estCostUsd) }} of ${{ Number(billing.monthlyBudget).toFixed(2) }}
              ({{ monthBudgetUsedPct }}%)
            </div>
            <div class="budget-bar"><div class="budget-fill" :class="{ over: monthBudgetUsedPct >= 100, warn: monthBudgetUsedPct >= 80 }" :style="{ width: monthBudgetUsedPct + '%' }"></div></div>
          </div>

          <div v-if="usage.byModel?.length" class="table-wrap">
            <table class="table">
              <thead><tr><th>Model</th><th class="num">Calls</th><th class="num">Input</th><th class="num">Output</th><th class="num">Cost</th></tr></thead>
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

          <p v-if="usage.firstRecordedAt" class="hint">Tracking since {{ fmtTimestamp(usage.firstRecordedAt) }}</p>
        </div>

        <div class="sub-section">
          <h3 class="sub-title">Cost rates & budget</h3>
          <div class="field-row two">
            <div class="field"><label class="label">Input $/1M tokens</label><input v-model.number="billing.inputCostPer1M" type="number" step="0.01" min="0" class="input mono short" /></div>
            <div class="field"><label class="label">Output $/1M tokens</label><input v-model.number="billing.outputCostPer1M" type="number" step="0.01" min="0" class="input mono short" /></div>
          </div>
          <div class="field">
            <label class="label">Monthly budget ($)</label>
            <input v-model.number="billing.monthlyBudget" type="number" step="1" min="0" class="input mono short" placeholder="0 (disabled)" />
          </div>
          <div class="row" style="gap: 8px;">
            <button class="btn primary" @click="saveBilling" :disabled="billingSaving">{{ billingSaving ? 'Saving\u2026' : 'Save rates' }}</button>
            <button class="btn" @click="loadUsage" :disabled="usageLoading">Refresh</button>
            <button class="btn danger" @click="clearUsage">Reset usage</button>
            <span v-if="billingSaveMsg" class="msg" :class="{ fail: billingSaveMsg.startsWith('Error') }">{{ billingSaveMsg }}</span>
          </div>
        </div>
      </div>

      <div class="section" id="addons-jobs">
        <div class="section-h">
          <h2 class="section-title" style="margin-bottom: 0;">Add-ons & Jobs</h2>
        </div>

        <div v-for="addon in addonsStore.addons" :key="addon.id" class="addon-row">
          <div class="addon-info">
            <div class="addon-name">{{ addon.name }}</div>
            <p class="addon-desc">{{ addon.summary }}</p>
          </div>
          <label class="toggle">
            <input type="checkbox" class="toggle-input" :checked="addonsStore.isEnabled(addon.id)" @change="addonsStore.setEnabled(addon.id, $event.target.checked)" />
            <span class="toggle-track"><span class="toggle-thumb"></span></span>
          </label>
        </div>

        <div v-for="job in addonsStore.jobs" :key="job.id" class="job-card">
          <div class="job-head">
            <div class="job-name">{{ job.name }}</div>
            <button class="btn" :disabled="runningJob === job.id" @click="runJob(job)">{{ runningJob === job.id ? 'Running\u2026' : 'Run' }}</button>
          </div>
          <p class="job-desc">{{ job.summary }}</p>

          <div v-if="job.inputs?.length" class="job-inputs">
            <label v-for="inp in job.inputs" :key="inp.id" class="job-input-row">
              <span class="job-input-lbl">{{ inp.label }}</span>
              <input type="text" class="input mono" :placeholder="inp.placeholder" :value="_draftForJob(job.id)[inp.id] ?? ''" @input="_draftForJob(job.id)[inp.id] = $event.target.value" />
            </label>
          </div>

          <div v-if="job.deliveries?.length" class="job-inputs">
            <p class="hint" style="margin-bottom: 4px;">Deliver result to:</p>
            <label v-for="d in job.deliveries" :key="d.id" class="job-input-row">
              <span class="job-input-lbl">{{ d.label }}</span>
              <input type="text" class="input mono" :placeholder="d.placeholder" :value="_deliveryDraftForJob(job.id)[d.id] ?? ''" @input="_deliveryDraftForJob(job.id)[d.id] = $event.target.value" />
            </label>
          </div>

          <div v-if="addonsStore.runsByJob(job.id).length" class="job-runs">
            <p class="hint" style="margin-bottom: 4px;">Recent runs</p>
            <div v-for="r in addonsStore.runsByJob(job.id).slice(0, 5)" :key="r.id" class="job-run">
              <span class="job-run-status" :data-status="r.status">{{ r.status }}</span>
              <span class="job-run-time">{{ new Date(r.startedAt).toLocaleString() }}</span>
              <span v-if="r.error" class="job-run-err">{{ r.error }}</span>
              <span v-else-if="r.result" class="job-run-res">{{ r.result }}</span>
            </div>
          </div>
        </div>
      </div>
    </div>

    <div class="save-bar" v-if="!loading">
      <button class="btn primary save-btn" @click="saveSettings" :disabled="saving">{{ saving ? 'Saving\u2026' : 'Save settings' }}</button>
      <span v-if="saveMessage" class="msg" :class="{ fail: saveMessage.startsWith('Error') }">{{ saveMessage }}</span>
    </div>

    <div v-if="loading" class="loading-state">Loading\u2026</div>
  </div>
</template>

<style scoped>
.settings-view {
  flex: 1;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.header {
  padding: 16px 24px;
  border-bottom: 1px solid var(--border);
  flex-shrink: 0;
}

.title {
  font-size: 20px;
  font-weight: 500;
  color: var(--text);
  line-height: 1;
}

/* Sticky section navigation */
.settings-nav {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 8px 24px;
  border-bottom: 1px solid var(--border);
  background: var(--bg2);
  overflow-x: auto;
  flex-shrink: 0;
  scrollbar-width: thin;
}

.settings-nav::-webkit-scrollbar {
  height: 3px;
}

.nav-category {
  font-size: 9.5px;
  font-weight: 600;
  letter-spacing: 0.06em;
  text-transform: uppercase;
  color: var(--text3);
  margin-right: 2px;
  white-space: nowrap;
}

.nav-item {
  background: none;
  border: 1px solid transparent;
  color: var(--text2);
  font-size: 11.5px;
  font-weight: 500;
  padding: 4px 10px;
  border-radius: 5px;
  cursor: pointer;
  white-space: nowrap;
  transition: border-color 0.15s, color 0.15s, background 0.15s;
  font-family: inherit;
}

.nav-item:hover {
  color: var(--text);
  background: var(--bg3);
  border-color: var(--border);
}

.nav-item.active {
  color: var(--accent);
  border-color: var(--accent);
  background: rgba(79, 142, 247, 0.1);
}

/* Scroll-margin for checklist/privacy wrappers */
#setup-checklist,
#privacy-controls {
  scroll-margin-top: 100px;
}

.scroll {
  flex: 1;
  overflow-y: auto;
  padding: 20px 24px;
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.section {
  background: var(--bg2);
  border: 1px solid var(--border);
  border-radius: 8px;
  padding: 20px;
  scroll-margin-top: 100px;
}

.section-h {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 8px;
}

.section-title {
  font-size: 13px;
  font-weight: 600;
  color: var(--text);
  margin-bottom: 16px;
}

.sub-section {
  margin-top: 16px;
  padding-top: 16px;
  border-top: 1px solid var(--border);
}

.sub-title {
  font-size: 12px;
  font-weight: 600;
  color: var(--text2);
  margin-bottom: 8px;
}

.field {
  margin-bottom: 14px;
}

.field:last-child {
  margin-bottom: 0;
}

.label {
  display: block;
  font-size: 12.5px;
  font-weight: 500;
  color: var(--text2);
  margin-bottom: 8px;
}

.input {
  width: 100%;
  padding: 10px 12px;
  border-radius: 6px;
  border: 1px solid var(--border);
  background: var(--bg);
  color: var(--text);
  font-size: 13px;
  outline: none;
  transition: border-color 0.15s;
  box-sizing: border-box;
}

.input:focus {
  border-color: var(--accent);
}

.input.mono {
  font-family: var(--mono);
  font-size: 12px;
}

.input.short {
  max-width: 160px;
}

.input::placeholder {
  color: var(--text3);
}

.code-area {
  font-family: var(--mono);
  font-size: 11.5px;
  line-height: 1.5;
}


.row {
  display: flex;
  gap: 8px;
  align-items: center;
}

.field-row {
  display: flex;
  gap: 12px;
  align-items: start;
}

.field-row.two {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 12px;
  align-items: start;
}

.hint {
  font-size: 11.5px;
  color: var(--text3);
  margin-top: 4px;
  line-height: 1.4;
}

.chip-row {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.chip {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 7px 14px;
  border-radius: 8px;
  border: 1px solid var(--border);
  background: var(--bg);
  color: var(--text2);
  font-size: 12.5px;
  font-weight: 500;
  cursor: pointer;
  transition: border-color 0.15s, background 0.15s, color 0.15s;
}

.chip:hover {
  background: var(--bg3);
  color: var(--text);
}

.chip.active {
  border-color: var(--accent);
  background: rgba(79, 142, 247, 0.1);
  color: var(--text);
}

.swatch {
  display: inline-block;
  width: 16px;
  height: 16px;
  border-radius: 50%;
  border: 1px solid var(--border2);
}

.swatch[data-swatch="dark"]  { background: linear-gradient(135deg, #1a1c1e 50%, #2f3236 50%); }
.swatch[data-swatch="light"] { background: linear-gradient(135deg, #f7f8fa 50%, #e2e6eb 50%); }
.swatch[data-swatch="auto"]  { background: linear-gradient(135deg, #1a1c1e 50%, #f7f8fa 50%); }

.slider-head {
  display: flex;
  justify-content: space-between;
  align-items: baseline;
  margin-bottom: 6px;
}

.slider-val {
  font-family: var(--mono);
  font-size: 11.5px;
  color: var(--text2);
  font-variant-numeric: tabular-nums;
}

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

.slider::-webkit-slider-thumb:active {
  cursor: grabbing;
  transform: scale(1.1);
}

.slider::-moz-range-thumb {
  width: 16px;
  height: 16px;
  border-radius: 50%;
  background: var(--accent);
  border: 2px solid var(--bg2);
  cursor: grab;
}

.toggle {
  display: flex;
  align-items: center;
  gap: 10px;
  cursor: pointer;
  padding: 4px 0;
}

.toggle-input {
  position: absolute;
  opacity: 0;
  width: 0;
  height: 0;
  pointer-events: none;
}

.toggle-track {
  position: relative;
  width: 36px;
  height: 20px;
  flex-shrink: 0;
  border-radius: 10px;
  background: var(--bg4);
  border: 1px solid var(--border);
  transition: background 0.2s;
}

.toggle-input:checked + .toggle-track {
  background: var(--accent);
  border-color: var(--accent);
}

.toggle-thumb {
  position: absolute;
  top: 2px;
  left: 2px;
  width: 14px;
  height: 14px;
  border-radius: 50%;
  background: white;
  transition: transform 0.2s;
}

.toggle-input:checked + .toggle-track .toggle-thumb {
  transform: translateX(16px);
}

.toggle-label {
  font-size: 12.5px;
  color: var(--text2);
  user-select: none;
}

.btn {
  padding: 7px 16px;
  border-radius: 6px;
  border: 1px solid var(--border);
  background: var(--bg);
  color: var(--text2);
  font-size: 12px;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.15s;
  font-family: inherit;
}

.btn:hover {
  background: var(--bg4);
  color: var(--text);
}

.btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.btn.primary {
  background: var(--accent);
  color: white;
  border-color: var(--accent);
}

.btn.primary:hover {
  background: var(--accent2);
}

.btn.danger {
  color: var(--red);
  border-color: var(--red);
}

.btn.danger:hover {
  background: rgba(217, 72, 72, 0.08);
}

.save-btn {
  padding: 8px 24px;
  font-size: 13px;
}

.icon-btn {
  width: 34px;
  height: 34px;
  border-radius: 6px;
  border: 1px solid var(--border);
  background: var(--bg);
  color: var(--text2);
  cursor: pointer;
  font-size: 14px;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: all 0.15s;
}

.icon-btn:hover {
  background: var(--bg4);
  color: var(--text);
}

.text-btn {
  background: none;
  border: none;
  color: var(--accent);
  font-size: 12px;
  font-weight: 500;
  cursor: pointer;
  padding: 0;
  font-family: inherit;
}

/* Navigation visibility toggle list (§C2) */
.nav-toggle-grid {
  display: flex;
  flex-direction: column;
  gap: 6px;
  margin: 10px 0;
}
.nav-toggle-row {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 10px 12px;
  background: var(--bg2, #1a1a1a);
  border: 1px solid var(--border, #2a2a2a);
  border-radius: 6px;
}
.nav-toggle-info { flex: 1; min-width: 0; }
.nav-toggle-label {
  font-size: 13px;
  color: var(--text, #e5e5e5);
  display: flex;
  align-items: center;
  gap: 6px;
}
.nav-toggle-hint {
  font-size: 11px;
  color: var(--text2, #b0b0b0);
  margin-top: 2px;
}
.core-badge {
  font-size: 9px;
  font-weight: 700;
  letter-spacing: 0.04em;
  text-transform: uppercase;
  padding: 1px 5px;
  border-radius: 3px;
  background: var(--bg3, #222);
  color: var(--text3, #5a5a5a);
}
.toggle {
  position: relative;
  display: inline-flex;
  align-items: center;
  cursor: pointer;
}
.toggle-input {
  position: absolute;
  opacity: 0;
  width: 0;
  height: 0;
  margin: 0;
}
.toggle-track {
  display: inline-block;
  width: 32px;
  height: 18px;
  background: var(--bg3, #222);
  border: 1px solid var(--border, #2a2a2a);
  border-radius: 9px;
  position: relative;
  transition: background 0.15s, border-color 0.15s;
}
.toggle-thumb {
  position: absolute;
  top: 2px;
  left: 2px;
  width: 12px;
  height: 12px;
  background: var(--text2, #b0b0b0);
  border-radius: 50%;
  transition: transform 0.15s, background 0.15s;
}
.toggle-input:checked + .toggle-track {
  background: var(--accent2, #4a9eff);
  border-color: var(--accent2, #4a9eff);
}
.toggle-input:checked + .toggle-track .toggle-thumb {
  transform: translateX(14px);
  background: #fff;
}
.toggle-input:focus-visible + .toggle-track {
  outline: 1px solid var(--accent2, #4a9eff);
  outline-offset: 2px;
}

.text-btn:hover {
  text-decoration: underline;
}

.msg {
  font-size: 12px;
}

.msg.ok {
  color: var(--green);
}

.msg.fail {
  color: var(--red);
}

.grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(180px, 1fr));
  gap: 6px;
}

.grid.tight {
  grid-template-columns: repeat(auto-fill, minmax(130px, 1fr));
}

.grid-card {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 12px;
  border: 1px solid var(--border);
  border-radius: 6px;
  background: var(--bg);
  cursor: pointer;
  transition: border-color 0.15s, background 0.15s;
}

.grid-card:hover {
  border-color: var(--accent);
}

.grid-card.active {
  border-color: var(--accent);
  background: rgba(79, 142, 247, 0.06);
}

.grid-card.compact {
  padding: 6px 10px;
}

.card-radio {
  accent-color: var(--accent);
  flex-shrink: 0;
}

.card-label {
  font-size: 12px;
  color: var(--text);
  font-weight: 500;
}

.field-group {
  margin-top: 12px;
}

.usage-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(160px, 1fr));
  gap: 10px;
  margin-bottom: 14px;
}

.usage-card {
  padding: 14px;
  background: var(--bg);
  border: 1px solid var(--border);
  border-radius: 8px;
}

.usage-lbl {
  font-size: 10.5px;
  text-transform: uppercase;
  letter-spacing: 0.06em;
  color: var(--text3);
  margin-bottom: 2px;
}

.usage-val {
  font-size: 20px;
  font-weight: 600;
  color: var(--text);
  font-family: var(--mono);
}

.usage-unit {
  font-size: 11px;
  color: var(--text3);
  font-weight: 400;
  margin-left: 4px;
}

.usage-sub {
  font-size: 11px;
  color: var(--text2);
  margin-top: 4px;
}

.budget-row {
  margin-bottom: 14px;
}

.budget-lbl {
  font-size: 12px;
  color: var(--text2);
  margin-bottom: 4px;
}

.budget-bar {
  height: 6px;
  background: var(--bg);
  border-radius: 3px;
  overflow: hidden;
  border: 1px solid var(--border);
}

.budget-fill {
  height: 100%;
  background: var(--accent);
  transition: width 0.2s;
}

.budget-fill.warn {
  background: #d8a347;
}

.budget-fill.over {
  background: var(--red);
}

.table-wrap {
  overflow-x: auto;
  margin-bottom: 12px;
}

.table {
  width: 100%;
  border-collapse: collapse;
  font-size: 12px;
}

.table th {
  text-align: left;
  font-weight: 500;
  color: var(--text3);
  padding: 6px 10px;
  border-bottom: 1px solid var(--border);
  font-size: 10.5px;
  text-transform: uppercase;
  letter-spacing: 0.05em;
}

.table th.num,
.table td.num {
  text-align: right;
  font-family: var(--mono);
}

.table td {
  padding: 6px 10px;
  border-bottom: 1px solid var(--border);
  color: var(--text2);
}

.table td.mono {
  color: var(--text);
  font-family: var(--mono);
  font-size: 11.5px;
}

.addon-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 10px 0;
  border-bottom: 1px solid var(--border);
}

.addon-row:last-child {
  border-bottom: none;
}

.addon-info {
  flex: 1;
  min-width: 0;
}

.addon-name {
  font-size: 12.5px;
  font-weight: 500;
  color: var(--text);
  margin-bottom: 2px;
}

.addon-desc {
  font-size: 11.5px;
  color: var(--text3);
  line-height: 1.5;
}

.job-card {
  background: var(--bg);
  border: 1px solid var(--border);
  border-radius: 6px;
  padding: 14px;
  margin-top: 10px;
}

.job-card:last-child {
  margin-bottom: 0;
}

.job-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 6px;
}

.job-name {
  font-size: 13px;
  font-weight: 500;
  color: var(--text);
}

.job-desc {
  font-size: 11.5px;
  color: var(--text3);
  line-height: 1.5;
  margin-bottom: 10px;
}

.job-inputs {
  display: flex;
  flex-direction: column;
  gap: 6px;
  margin-bottom: 8px;
}

.job-input-row {
  display: flex;
  align-items: center;
  gap: 8px;
}

.job-input-lbl {
  flex: 0 0 130px;
  font-size: 11.5px;
  color: var(--text2);
}

.job-runs {
  margin-top: 8px;
  padding-top: 8px;
  border-top: 1px dashed var(--border);
}

.job-run {
  display: flex;
  gap: 8px;
  align-items: baseline;
  font-size: 11px;
  color: var(--text2);
  padding: 4px 0;
  border-bottom: 1px solid var(--border);
}

.job-run:last-child {
  border-bottom: 0;
}

.job-run-status {
  flex: 0 0 65px;
  font-family: var(--mono);
  font-size: 10px;
  text-transform: uppercase;
  letter-spacing: 0.04em;
  padding: 1px 6px;
  border-radius: 3px;
  background: var(--bg2);
  color: var(--text3);
}

.job-run-status[data-status="queued"]  { background: rgba(245, 166, 35, 0.12); color: var(--amber2); }
.job-run-status[data-status="success"] { background: rgba(62, 207, 142, 0.12); color: var(--green2); }
.job-run-status[data-status="error"]   { background: rgba(240, 84, 84, 0.12); color: var(--red2); }

.job-run-time {
  flex: 0 0 155px;
  font-family: var(--mono);
  font-size: 10px;
  color: var(--text3);
}

.job-run-res {
  color: var(--text2);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.job-run-err {
  color: var(--red2);
}

.save-bar {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-shrink: 0;
  padding: 12px 24px;
  border-top: 1px solid var(--border);
  background: var(--bg);
}

.loading-state {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--text3);
  font-size: 13px;
}

.nc-section { position: relative; }
.nc-header { display: flex; justify-content: space-between; align-items: flex-start; gap: 16px; }
.nc-banner {
  align-self: flex-start; padding: 7px 12px; border-radius: 6px;
  font-size: 11.5px; font-weight: 500; white-space: nowrap; cursor: default;
  background: rgba(79,142,247,0.12); border: 1px solid rgba(79,142,247,0.30); color: #d2e0f7;
}
.nc-banner.pulse { animation: nc-banner-pulse 1.6s ease-in-out infinite; }

/* When the banner lives inside the GitHub config block (deep-link from
   the Pipelines view), pin it to the top-right of that section so the
   user immediately sees both "where the fix lives" and "how to get back". */
.gh-anchor { position: relative; }
.gh-banner {
  position: absolute; top: -8px; right: 0;
  z-index: 2;
}
.nc-banner.go-back {
  cursor: pointer; color: #cbdaf6;
  background: rgba(79,142,247,0.18); border-color: rgba(79,142,247,0.5);
  transition: background 0.15s, border-color 0.15s, transform 0.15s;
}
.nc-banner.go-back:hover {
  background: rgba(79,142,247,0.28); border-color: rgba(79,142,247,0.7); transform: translateY(-1px);
}
.nc-banner-text {
  display: inline-block; overflow: hidden; vertical-align: bottom; white-space: nowrap;
  animation: nc-type-in 1.2s steps(40, end) 0s 1 normal both;
}
@keyframes nc-type-in {
  from { max-width: 0; }
  to   { max-width: 30ch; }
}
@keyframes nc-banner-pulse {
  0%, 100% { background: rgba(79,142,247,0.12); box-shadow: 0 0 0 0 rgba(79,142,247,0); }
  50%      { background: rgba(79,142,247,0.22); box-shadow: 0 0 0 4px rgba(79,142,247,0.12); }
}
.nc-banner-fade-enter-active,
.nc-banner-fade-leave-active { transition: opacity 0.4s, transform 0.4s; }
.nc-banner-fade-enter-from,
.nc-banner-fade-leave-to { opacity: 0; transform: translateY(-4px); }

.nc-add {
  display: flex; align-items: center; gap: 8px; flex-wrap: wrap;
  margin: 12px 0 10px; padding: 8px 10px; border-radius: 6px;
  background: rgba(255,255,255,0.025); border: 1px dashed rgba(255,255,255,0.08);
}
.nc-add-label { font-size: 11px; color: var(--text3); margin-right: 4px; }
.nc-add-btn {
  background: var(--bg); border: 1px solid var(--border); color: var(--text2);
  padding: 4px 10px; font-size: 11px; border-radius: 4px; cursor: pointer;
  transition: border-color 0.12s, color 0.12s;
}
.nc-add-btn:hover { border-color: var(--accent); color: var(--text); }

.nc-empty {
  padding: 14px; border: 1px dashed var(--border); border-radius: 6px;
  background: var(--bg3); color: var(--text3); font-size: 12px;
  font-style: italic; text-align: center;
}

.nc-list { display: flex; flex-direction: column; gap: 6px; }
.nc-row {
  display: grid; grid-template-columns: auto 92px 1fr 2fr auto;
  gap: 8px; align-items: center; padding: 6px 8px; border-radius: 5px;
  background: var(--bg3); border: 1px solid var(--border); transition: opacity 0.15s;
}
.nc-row.disabled { opacity: 0.55; }
.nc-toggle input { accent-color: var(--accent); width: 14px; height: 14px; cursor: pointer; }
.nc-kind {
  text-transform: uppercase; letter-spacing: 0.06em; font-size: 10.5px;
  font-weight: 600; padding: 2px 8px; border-radius: 10px; text-align: center; color: var(--text);
}
.nc-kind[data-kind="desktop"]    { background: rgba(167,139,250,0.18); color: #a78bfa; }
.nc-kind[data-kind="email"]      { background: rgba(62,207,142,0.16);  color: #3ecf8e; }
.nc-kind[data-kind="slack"]      { background: rgba(245,166,35,0.16);  color: #f5a623; }
.nc-kind[data-kind="google-chat"] { background: rgba(66,133,244,0.16); color: #4285f4; }
.nc-kind[data-kind="webhook"]    { background: rgba(79,142,247,0.16);  color: #4f8ef7; }
.nc-label-in, .nc-target-in {
  background: var(--bg); border: 1px solid var(--border); color: var(--text);
  padding: 5px 8px; border-radius: 4px; font-size: 11.5px; outline: none;
}
.nc-label-in:focus, .nc-target-in:focus { border-color: var(--accent); }
.nc-target-noop { color: var(--text3); font-size: 11px; font-style: italic; padding-left: 6px; }
.nc-remove {
  background: transparent; border: 1px solid var(--border); color: var(--text3);
  width: 22px; height: 22px; border-radius: 4px; cursor: pointer;
  font-size: 14px; line-height: 1; padding: 0;
}
.nc-remove:hover { border-color: var(--red); color: var(--red); }

/* ---- Vault ---- */

.section-header-row {
  display: flex; justify-content: space-between; align-items: center; gap: 12px;
}
.vault-refresh {
  background: transparent; border: 1px solid var(--border); color: var(--text2);
  padding: 4px 12px; border-radius: 4px; font-size: 11px; cursor: pointer;
  transition: border-color 0.12s, color 0.12s;
}
.vault-refresh:hover:not(:disabled) { border-color: var(--accent); color: var(--text); }
.vault-refresh:disabled { opacity: 0.5; cursor: not-allowed; }

.vault-error {
  margin: 8px 0; padding: 8px 12px;
  background: rgba(240,84,84,0.08); border: 1px solid rgba(240,84,84,0.3); border-radius: 5px;
  color: var(--red); font-size: 11.5px;
}

.vault-grid {
  display: flex; flex-direction: column; gap: 6px; margin-top: 12px;
}
.vault-row {
  display: grid; grid-template-columns: 1fr auto auto;
  gap: 12px; align-items: center;
  padding: 9px 12px; border-radius: 6px;
  background: var(--bg3); border: 1px solid var(--border);
}
.vault-row[data-status="missing"] { opacity: 0.65; }
.vault-row[data-status="invalid"] { border-color: rgba(240,84,84,0.45); background: rgba(240,84,84,0.04); }
.vault-row[data-status="expired"] { border-color: rgba(245,166,35,0.45); background: rgba(245,166,35,0.04); }
.vault-row[data-status="error"]   { border-color: rgba(240,84,84,0.45); background: rgba(240,84,84,0.04); }

.vault-meta { min-width: 0; display: flex; flex-direction: column; gap: 2px; }
.vault-label { font-size: 12.5px; color: var(--text); font-weight: 500; display: flex; gap: 8px; align-items: center; }
.vault-msg   { font-size: 11px; color: var(--text2); }
.vault-checked { font-size: 10.5px; color: var(--text3); font-style: italic; }

.vault-kind {
  font-size: 9.5px; letter-spacing: 0.07em; text-transform: uppercase;
  font-weight: 600; padding: 1px 6px; border-radius: 9px; color: var(--text);
}
.vault-kind[data-kind="oauth"]      { background: rgba(167,139,250,0.18); color: #a78bfa; }
.vault-kind[data-kind="token"]      { background: rgba(79,142,247,0.18);  color: #4f8ef7; }
.vault-kind[data-kind="credential"] { background: rgba(62,207,142,0.18);  color: #3ecf8e; }

.vault-status-pill {
  font-size: 11px; font-weight: 600; letter-spacing: 0.04em;
  padding: 2px 9px; border-radius: 10px; text-transform: uppercase;
  background: var(--bg4); color: var(--text2);
}
.vault-status-pill[data-status="valid"]   { background: rgba(62,207,142,0.18); color: #3ecf8e; }
.vault-status-pill[data-status="present"] { background: rgba(79,142,247,0.18); color: #4f8ef7; }
.vault-status-pill[data-status="missing"] { background: rgba(255,255,255,0.06); color: var(--text3); }
.vault-status-pill[data-status="expired"] { background: rgba(245,166,35,0.18); color: #f5a623; }
.vault-status-pill[data-status="invalid"] { background: rgba(240,84,84,0.18);  color: #f05454; }
.vault-status-pill[data-status="error"]   { background: rgba(240,84,84,0.18);  color: #f05454; }

.vault-row-actions { display: flex; gap: 6px; }
.vault-btn {
  background: var(--bg2); border: 1px solid var(--border); color: var(--text2);
  padding: 4px 10px; font-size: 11px; border-radius: 4px; cursor: pointer;
  transition: border-color 0.12s, color 0.12s, background 0.12s;
}
.vault-btn:hover:not(:disabled) { border-color: var(--accent); color: var(--text); background: var(--bg3); }
.vault-btn:disabled { opacity: 0.4; cursor: not-allowed; }
.vault-btn.primary {
  background: rgba(79,142,247,0.16); border-color: rgba(79,142,247,0.45); color: #4f8ef7;
}
.vault-btn.primary:hover:not(:disabled) {
  background: rgba(79,142,247,0.26); border-color: rgba(79,142,247,0.65); color: #6ea7fb;
}
.vault-btn.danger { color: #f05454; border-color: rgba(240,84,84,0.4); }
.vault-btn.danger:hover:not(:disabled) {
  background: rgba(240,84,84,0.1); border-color: var(--red); color: var(--red);
}

/* Custom-secrets sub-section */
.vault-empty {
  font-size: 11.5px; color: var(--text3); font-style: italic;
  padding: 10px 12px; border: 1px dashed var(--border); border-radius: 5px; text-align: center;
}
.secret-add-row {
  display: grid; grid-template-columns: minmax(160px, 0.8fr) minmax(180px, 1.2fr) minmax(120px, 1fr) auto;
  gap: 8px; margin: 10px 0;
}
.input.short { max-width: 220px; }

.secret-list { display: flex; flex-direction: column; gap: 5px; }
.secret-row {
  display: grid; grid-template-columns: minmax(160px, 0.8fr) minmax(120px, 0.6fr) minmax(120px, 1fr) auto auto;
  gap: 10px; align-items: center;
  padding: 6px 10px; border-radius: 5px;
  background: var(--bg3); border: 1px solid var(--border);
  font-size: 11.5px;
}
.secret-key { color: var(--text); font-weight: 500; }
.secret-value { color: var(--text3); }
.secret-notes { color: var(--text2); }
.secret-stamp { color: var(--text3); font-size: 10.5px; font-style: italic; }

/* Vault in-page deep-link banner — pinned to the top-right of the
   settings viewport so it's always visible during the configure flow. */
.vault-banner {
  position: fixed; top: 64px; right: 22px; z-index: 50;
}

/* ---- External Secrets sub-section ---- */

.subsection-title-row {
  display: flex; justify-content: space-between; align-items: center; gap: 12px;
}
.es-tool-badge {
  font-size: 10.5px; font-weight: 600; letter-spacing: 0.04em;
  padding: 2px 9px; border-radius: 10px;
  background: rgba(62,207,142,0.15); color: #3ecf8e;
  text-transform: uppercase;
}
.es-tool-badge.none { background: var(--bg4); color: var(--text3); }

.es-group-title {
  font-size: 10.5px; font-weight: 600; letter-spacing: 0.06em;
  text-transform: uppercase; color: var(--text3);
  margin: 18px 0 8px;
}

.es-grid {
  display: grid; grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
  gap: 10px;
}
.es-card {
  display: flex; flex-direction: column; gap: 8px;
  padding: 10px 12px; border-radius: 7px;
  background: var(--bg3); border: 1px solid var(--border);
  transition: border-color 0.15s, background 0.15s;
}
.es-card.enabled { border-color: rgba(79,142,247,0.45); background: rgba(79,142,247,0.04); }
.es-card-head { display: flex; justify-content: space-between; align-items: center; gap: 8px; }
.es-card-title { display: flex; flex-direction: column; gap: 2px; min-width: 0; }
.es-tool-name { font-size: 12.5px; color: var(--text); font-weight: 600; }
.es-tool-bin {
  font-size: 10.5px; color: var(--text3);
  background: rgba(255,255,255,0.05); padding: 1px 6px; border-radius: 3px; align-self: flex-start;
}

/* iOS-style switch — keeps the card compact while still being easy to hit. */
.es-switch { position: relative; flex-shrink: 0; cursor: pointer; }
.es-switch input { position: absolute; opacity: 0; width: 0; height: 0; }
.es-switch-track {
  display: inline-block; width: 30px; height: 17px; border-radius: 10px;
  background: var(--bg4); position: relative; transition: background 0.18s;
}
.es-switch-track::before {
  content: ''; position: absolute; top: 2px; left: 2px;
  width: 13px; height: 13px; border-radius: 50%;
  background: white; transition: transform 0.18s;
  box-shadow: 0 1px 3px rgba(0,0,0,0.4);
}
.es-switch input:checked + .es-switch-track { background: var(--accent); }
.es-switch input:checked + .es-switch-track::before { transform: translateX(13px); }

.es-card-fields {
  display: flex; flex-direction: column; gap: 6px;
  padding-top: 4px;
  border-top: 1px dashed rgba(255,255,255,0.06);
}
.es-field-row {
  display: grid; grid-template-columns: 130px 1fr; gap: 8px; align-items: center;
}
.es-field-row label {
  font-size: 11px; color: var(--text2);
}
.es-field-row .input { font-size: 11.5px; padding: 4px 8px; }
.es-hint {
  font-size: 11px; color: var(--text3); margin: 4px 0 0; line-height: 1.45;
}
.es-hint code {
  background: rgba(255,255,255,0.06); padding: 1px 5px;
  border-radius: 3px; font-family: var(--mono); font-size: 10.5px;
}

/* ---- Watchers & Notifications section ---- */

.watcher-guard-grid {
  display: grid; grid-template-columns: repeat(auto-fit, minmax(180px, 1fr));
  gap: 10px 16px; margin: 8px 0 4px;
  padding: 10px 12px; border: 1px solid var(--border); border-radius: 6px;
  background: rgba(255,255,255,0.025);
}
.watcher-field {
  display: flex; flex-direction: column; gap: 4px;
  font-size: 11.5px; color: var(--text2);
}
.watcher-field label {
  font-size: 10.5px; letter-spacing: 0.04em;
  text-transform: uppercase; color: var(--text3);
}

/* Active silences */
.watcher-silence-list { display: flex; flex-direction: column; gap: 6px; }
.watcher-silence-row {
  display: flex; justify-content: space-between; align-items: center; gap: 12px;
  padding: 8px 12px; border-radius: 6px;
  background: var(--bg3); border: 1px solid var(--border);
}
.watcher-silence-row.unack {
  border-color: rgba(245,166,35,0.5);
  background: rgba(245,166,35,0.06);
  animation: silence-pulse 2.4s ease-in-out infinite;
}
@keyframes silence-pulse {
  0%, 100% { box-shadow: 0 0 0 0 rgba(245,166,35,0.0); }
  50%      { box-shadow: 0 0 0 4px rgba(245,166,35,0.18); }
}
.watcher-silence-meta { display: flex; flex-direction: column; gap: 2px; min-width: 0; }
.watcher-silence-label { font-size: 12.5px; color: var(--text); display: flex; gap: 8px; align-items: center; }
.watcher-silence-detail { font-size: 11px; color: var(--text2); }
.watcher-silence-actions { display: flex; gap: 6px; flex-shrink: 0; }
.watcher-reason {
  font-size: 9.5px; letter-spacing: 0.07em; text-transform: uppercase;
  font-weight: 600; padding: 1px 6px; border-radius: 9px;
}
.watcher-reason[data-reason="spam"]   { background: rgba(245,166,35,0.2); color: #f5a623; }
.watcher-reason[data-reason="manual"] { background: rgba(79,142,247,0.2);  color: #4f8ef7; }
.watcher-reason[data-reason="argus"]  { background: rgba(167,139,250,0.2); color: #a78bfa; }

/* Watchers list */
.watcher-list { display: flex; flex-direction: column; gap: 6px; }
.watcher-row {
  display: grid;
  grid-template-columns: 1fr auto auto;
  gap: 12px; align-items: center;
  padding: 9px 12px; border-radius: 6px;
  background: var(--bg3); border: 1px solid var(--border);
}
.watcher-row[data-status="expired"] { border-color: rgba(245,166,35,0.45); background: rgba(245,166,35,0.04); }
.watcher-row[data-status="invalid"] { border-color: rgba(240,84,84,0.45);  background: rgba(240,84,84,0.04); }
.watcher-row[data-status="error"]   { border-color: rgba(240,84,84,0.45);  background: rgba(240,84,84,0.04); }
.watcher-meta { min-width: 0; display: flex; flex-direction: column; gap: 2px; }
.watcher-label { font-size: 12.5px; color: var(--text); font-weight: 500; display: flex; gap: 8px; align-items: center; }
.watcher-msg { font-size: 11px; color: var(--text2); }
.watcher-checked { font-size: 10.5px; color: var(--text3); font-style: italic; }
.watcher-kind {
  font-size: 9.5px; letter-spacing: 0.06em; text-transform: uppercase;
  font-weight: 600; padding: 1px 6px; border-radius: 9px;
  background: rgba(79,142,247,0.18); color: #4f8ef7;
}
.watcher-status-pill {
  font-size: 11px; font-weight: 600; letter-spacing: 0.04em;
  padding: 2px 9px; border-radius: 10px; text-transform: uppercase;
  background: var(--bg4); color: var(--text2); white-space: nowrap;
}
.watcher-status-pill[data-status="valid"]   { background: rgba(62,207,142,0.18); color: #3ecf8e; }
.watcher-status-pill[data-status="present"] { background: rgba(79,142,247,0.18); color: #4f8ef7; }
.watcher-status-pill[data-status="ok"]      { background: rgba(62,207,142,0.18); color: #3ecf8e; }
.watcher-status-pill[data-status="warn"]    { background: rgba(245,166,35,0.18); color: #f5a623; }
.watcher-status-pill[data-status="expired"] { background: rgba(245,166,35,0.18); color: #f5a623; }
.watcher-status-pill[data-status="invalid"] { background: rgba(240,84,84,0.18);  color: #f05454; }
.watcher-status-pill[data-status="error"]   { background: rgba(240,84,84,0.18);  color: #f05454; }
.watcher-row-actions { display: flex; gap: 6px; align-items: center; flex-wrap: wrap; }
.watcher-enable {
  display: inline-flex; align-items: center; gap: 5px;
  font-size: 11px; color: var(--text2); cursor: pointer;
}
.watcher-enable input { accent-color: var(--accent); }

/* Google Auth step-by-step guide */
.guide-card {
  background: var(--bg3);
  border: 1px solid var(--border);
  border-radius: 7px;
  margin-bottom: 16px;
  overflow: hidden;
}

.guide-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 10px 14px;
  cursor: pointer;
  user-select: none;
  transition: background 0.12s;
}

.guide-header:hover {
  background: rgba(255,255,255,0.03);
}

.guide-title {
  font-size: 12.5px;
  font-weight: 600;
  color: var(--accent);
}

.guide-toggle {
  font-size: 16px;
  font-weight: 600;
  color: var(--text3);
  line-height: 1;
  width: 20px;
  text-align: center;
}

.guide-body {
  border-top: 1px solid var(--border);
  padding: 12px 14px 6px;
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.guide-step {
  display: flex;
  gap: 10px;
  align-items: flex-start;
}

.step-num {
  flex-shrink: 0;
  width: 22px;
  height: 22px;
  border-radius: 50%;
  background: var(--accent);
  color: white;
  font-size: 11px;
  font-weight: 700;
  display: flex;
  align-items: center;
  justify-content: center;
  margin-top: 1px;
}

.step-content {
  flex: 1;
  min-width: 0;
}

.step-content strong {
  color: var(--text);
  font-weight: 600;
}

.step-hint {
  font-size: 11.5px;
  color: var(--text2);
  margin: 3px 0 0;
  line-height: 1.5;
}

.step-hint code {
  font-family: var(--mono);
  font-size: 11px;
  background: rgba(255,255,255,0.06);
  padding: 1px 5px;
  border-radius: 3px;
  color: var(--text);
}

.guide-link {
  display: inline-block;
  font-family: var(--mono);
  font-size: 11.5px;
  color: var(--accent);
  text-decoration: none;
  margin: 2px 0;
  word-break: break-all;
}

.guide-link:hover {
  text-decoration: underline;
}

.guide-app-types {
  display: flex;
  flex-direction: column;
  gap: 5px;
  margin: 5px 0 2px;
}

.guide-app-option {
  padding: 6px 10px;
  border-radius: 5px;
  background: var(--bg);
  border: 1px solid var(--border);
  display: flex;
  flex-direction: column;
  gap: 1px;
}

.guide-app-label {
  font-size: 11.5px;
  font-weight: 600;
  color: var(--text);
}

.guide-app-desc {
  font-size: 11px;
  color: var(--text3);
  line-height: 1.4;
}
</style>
