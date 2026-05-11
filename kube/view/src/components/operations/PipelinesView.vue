<script setup>
import { ref, computed, onMounted, onBeforeUnmount } from 'vue'
import { marked } from 'marked'
import DOMPurify from 'dompurify'
import { callGo } from '../../composables/useWails'
import { useAppNavStore } from '../../stores/appNav'

const appNav = useAppNavStore()

const PROVIDERS = {
  github:           { label: 'GitHub Actions',     repoHint: (s) => s.githubOwner && s.githubRepo ? `${s.githubOwner}/${s.githubRepo}` : null },
  gitlab:           { label: 'GitLab CI/CD',       repoHint: (s) => s.gitlabProjectId || null },
  'aws-codebuild':  { label: 'AWS CodeBuild',      repoHint: (s) => s.awsProject || null },
  'gcp-cloudbuild': { label: 'Google Cloud Build', repoHint: (s) => s.gcpProject || null },
  circleci:         { label: 'CircleCI',           repoHint: (s) => s.circleciProjectSlug || null },
  azure:            { label: 'Azure Pipelines',    repoHint: (s) => s.azureOrganization && s.azureProject ? `${s.azureOrganization}/${s.azureProject}` : null },
}

const DESTINATION_LABELS = {
  local: 'In-app only',
  gdrive: 'Google Drive',
  s3: 'S3',
  email: 'Email',
  confluence: 'Confluence',
  notion: 'Notion',
  evernote: 'Evernote',
  onenote: 'Microsoft OneNote',
  amplenote: 'Amplenote',
  'standard-notes': 'Standard Notes',
  obsidian: 'Obsidian',
  joplin: 'Joplin',
  logseq: 'Logseq',
  bear: 'Bear',
}

const settings = ref(null)
const loading = ref(true)
const activeTab = ref('prs') // 'prs' | 'branches' | 'reports' | 'rules'

const provider = computed(() => settings.value?.pipelinesProvider || '')
const enabled = computed(() => !!settings.value?.pipelinesEnabled)
const providerLabel = computed(() => PROVIDERS[provider.value]?.label || '')
const repoHint = computed(() => settings.value && PROVIDERS[provider.value]?.repoHint(settings.value) || '')

async function loadSettings() {
  loading.value = true
  try {
    settings.value = await callGo('GetSettings')
  } catch (e) {
    console.error('[pipelines] load settings failed:', e)
  } finally {
    loading.value = false
  }
}

// --- Rules editor state ---
const rules = ref('')
const rulesLoaded = ref(false)
const rulesDirty = ref(false)
const rulesSaving = ref(false)
const rulesSaveMsg = ref('')
const dragOver = ref(false)
const fileInputRef = ref(null)

let rulesAutoSave = null

async function loadRules() {
  rulesLoaded.value = false
  rules.value = ''
  rulesDirty.value = false
  if (!provider.value) {
    rulesLoaded.value = true
    return
  }
  try {
    const result = await callGo('GetPRGuidelines', provider.value)
    rules.value = result || ''
  } catch (e) {
    console.error('[pipelines] load rules failed:', e)
  } finally {
    rulesLoaded.value = true
  }
}

function onRulesInput(e) {
  rules.value = e.target.value
  rulesDirty.value = true
  if (rulesAutoSave) clearTimeout(rulesAutoSave)
  rulesAutoSave = setTimeout(saveRules, 1500)
}

async function saveRules() {
  if (!provider.value) return
  if (rulesAutoSave) { clearTimeout(rulesAutoSave); rulesAutoSave = null }
  rulesSaving.value = true
  rulesSaveMsg.value = ''
  try {
    await callGo('SavePRGuidelines', provider.value, rules.value)
    rulesDirty.value = false
    rulesSaveMsg.value = rules.value ? 'Saved' : 'Cleared'
  } catch (e) {
    rulesSaveMsg.value = 'Error: ' + (e?.message || String(e))
  } finally {
    rulesSaving.value = false
    setTimeout(() => { rulesSaveMsg.value = '' }, 4000)
  }
}

function clearRules() {
  if (!confirm('Clear the PR guidelines for this provider? The on-disk file will be removed.')) return
  rules.value = ''
  rulesDirty.value = true
  saveRules()
}

// --- Drag-and-drop / Finder upload ---
const ACCEPTED_EXT = ['.md', '.markdown', '.txt', '.mdc', '.cursorrules']
const MAX_BYTES = 256 * 1024

function isAcceptedFile(file) {
  const name = (file.name || '').toLowerCase()
  return ACCEPTED_EXT.some(ext => name.endsWith(ext)) || file.type.startsWith('text/')
}

async function ingestFile(file) {
  if (!provider.value) {
    rulesSaveMsg.value = 'Error: select a pipeline provider in Settings first.'
    return
  }
  if (file.size > MAX_BYTES) {
    rulesSaveMsg.value = `Error: file too large (${file.size} bytes, max ${MAX_BYTES}).`
    return
  }
  if (!isAcceptedFile(file)) {
    rulesSaveMsg.value = `Error: unsupported file type. Use ${ACCEPTED_EXT.join(', ')}.`
    return
  }
  try {
    const text = await file.text()
    rules.value = text
    rulesDirty.value = true
    rulesSaveMsg.value = `Loaded ${file.name} — saving…`
    await saveRules()
  } catch (e) {
    rulesSaveMsg.value = 'Error reading file: ' + (e?.message || String(e))
  }
}

function onDrop(e) {
  e.preventDefault()
  dragOver.value = false
  const file = e.dataTransfer?.files?.[0]
  if (file) ingestFile(file)
}
function onDragOver(e) { e.preventDefault(); dragOver.value = true }
function onDragLeave(e) { if (e.target === e.currentTarget) dragOver.value = false }

function pickFile() { fileInputRef.value?.click() }
function onFilePicked(e) {
  const file = e.target.files?.[0]
  if (file) ingestFile(file)
  e.target.value = '' // allow re-uploading the same file
}

// --- Code review reports ---
const reports = ref([])
const reportsLoading = ref(false)
const reportsError = ref('')

const openReport = ref(null)        // metadata for the currently-open report
const openReportBody = ref('')      // markdown body
const openReportLoading = ref(false)

const showCreateDialog = ref(false)
const newReport = ref({ title: '', prRef: '', body: '' })
const creatingReport = ref(false)

const renderedReport = computed(() => {
  if (!openReportBody.value) return ''
  return DOMPurify.sanitize(marked.parse(openReportBody.value))
})

function formatDate(ts) {
  if (!ts) return ''
  try { return new Date(ts).toLocaleString() } catch { return String(ts) }
}

async function loadReports() {
  if (!provider.value) {
    reports.value = []
    return
  }
  reportsLoading.value = true
  reportsError.value = ''
  try {
    const res = await callGo('ListCodeReviewReports', provider.value)
    reports.value = res || []
  } catch (e) {
    reportsError.value = e?.message || String(e)
  } finally {
    reportsLoading.value = false
  }
}

async function openReportDoc(rep) {
  openReport.value = rep
  openReportBody.value = ''
  openReportLoading.value = true
  try {
    openReportBody.value = await callGo('GetCodeReviewReport', provider.value, rep.id)
  } catch (e) {
    openReportBody.value = `_Failed to load report: ${e?.message || e}_`
  } finally {
    openReportLoading.value = false
  }
}

function closeReport() {
  openReport.value = null
  openReportBody.value = ''
}

async function deleteReport(rep) {
  if (!confirm(`Delete review "${rep.title}"? This cannot be undone.`)) return
  try {
    await callGo('DeleteCodeReviewReport', provider.value, rep.id)
    if (openReport.value?.id === rep.id) closeReport()
    await loadReports()
  } catch (e) {
    alert('Delete failed: ' + (e?.message || e))
  }
}

function openCreateDialog() {
  newReport.value = { title: '', prRef: '', body: '' }
  showCreateDialog.value = true
}

async function submitCreate() {
  if (!newReport.value.title.trim() || creatingReport.value) return
  creatingReport.value = true
  try {
    const created = await callGo(
      'CreateCodeReviewReport',
      provider.value,
      newReport.value.title.trim(),
      newReport.value.prRef.trim(),
      newReport.value.body,
    )
    showCreateDialog.value = false
    await loadReports()
    if (created) openReportDoc(created)
  } catch (e) {
    alert('Create failed: ' + (e?.message || e))
  } finally {
    creatingReport.value = false
  }
}

// --- Live provider fetches (GitHub today; gitlab/azure to follow) ----------
//
// `<thing>State` is one of: 'idle' | 'loading' | 'ok' | 'error'. When 'error',
// errorKind carries the typed prefix the Go handler emits ('config_missing',
// 'auth_failed', 'rate_limited', 'not_found', 'other') so the UI can pick the
// right CTA without reparsing the message.

const prs = ref([])
const prsState = ref('idle')
const prsErrorKind = ref('')
const prsErrorMessage = ref('')

const branches = ref([])
const branchesState = ref('idle')
const branchesErrorKind = ref('')
const branchesErrorMessage = ref('')

function parseGoError(err) {
  // Go-side wraps every error as `[<kind>] message`. If we don't see a
  // bracket prefix, assume 'other' so the UI can still react.
  const text = err?.message || String(err || '')
  const m = text.match(/^\[(\w+)\]\s*(.*)$/)
  if (m) return { kind: m[1], message: m[2] }
  return { kind: 'other', message: text }
}

async function fetchPRs() {
  if (provider.value !== 'github') {
    prs.value = []
    prsState.value = 'idle'
    return
  }
  prsState.value = 'loading'
  prsErrorKind.value = ''
  prsErrorMessage.value = ''
  try {
    const res = await callGo('ListGitHubPullRequests')
    prs.value = Array.isArray(res) ? res : []
    prsState.value = 'ok'
  } catch (e) {
    const { kind, message } = parseGoError(e)
    prsErrorKind.value = kind
    prsErrorMessage.value = message
    prsState.value = 'error'
    prs.value = []
  }
}

async function fetchBranches() {
  if (provider.value !== 'github') {
    branches.value = []
    branchesState.value = 'idle'
    return
  }
  branchesState.value = 'loading'
  branchesErrorKind.value = ''
  branchesErrorMessage.value = ''
  try {
    const res = await callGo('ListGitHubBranches')
    branches.value = Array.isArray(res) ? res : []
    branchesState.value = 'ok'
  } catch (e) {
    const { kind, message } = parseGoError(e)
    branchesErrorKind.value = kind
    branchesErrorMessage.value = message
    branchesState.value = 'error'
    branches.value = []
  }
}

// Map the typed error kind to a human one-liner the user can act on.
function ghErrorTitle(kind) {
  switch (kind) {
    case 'config_missing': return 'GitHub isn’t configured yet'
    case 'auth_failed':    return 'GitHub rejected the token'
    case 'not_found':      return 'GitHub can’t find that owner / repo'
    case 'rate_limited':   return 'GitHub rate limit reached'
    default:               return 'GitHub call failed'
  }
}

// True when the user can fix this by visiting the GitHub config in Settings.
function ghErrorIsFixable(kind) {
  return kind === 'config_missing' || kind === 'auth_failed' || kind === 'not_found'
}

// Deep-link to Settings → Pipelines & CI/CD → GitHub provider config, with a
// returnTo so the user can flow back to whichever tab they were on.
function jumpToGitHubConfig() {
  appNav.requestNav({
    navId: 'settings',
    anchor: 'pipelines-github',
    returnTo: {
      navId: 'pipelines',
      anchor: 'pipelines-tab-' + activeTab.value,
      label: 'Pipelines — ' + (activeTab.value === 'prs' ? 'Pull Requests' : 'Branches'),
    },
  })
}

// --- Tab switching ---
function setTab(t) {
  activeTab.value = t
  if (t === 'rules' && !rulesLoaded.value) loadRules()
  if (t === 'reports') loadReports()
  if (t === 'prs') fetchPRs()
  if (t === 'branches') fetchBranches()
}

onMounted(async () => {
  await loadSettings()
  if (provider.value) loadRules()

  // On arrival via "Go back" from Settings, the appNav store hands us the
  // tab to focus. Default-fetch whichever tab we land on.
  const pending = appNav.consumeNav()
  if (pending && pending.navId === 'pipelines' && pending.anchor) {
    if (pending.anchor.startsWith('pipelines-tab-')) {
      const t = pending.anchor.replace('pipelines-tab-', '')
      if (['prs', 'branches', 'reports', 'rules'].includes(t)) setTab(t)
    }
  } else if (provider.value === 'github') {
    // No deep-link, but GitHub is configured — eagerly load the default
    // tab so the user immediately sees "loading" state instead of empty.
    if (activeTab.value === 'prs') fetchPRs()
    if (activeTab.value === 'branches') fetchBranches()
  }
})

onBeforeUnmount(() => {
  if (rulesAutoSave) {
    clearTimeout(rulesAutoSave)
    if (rulesDirty.value) saveRules()
  }
})
</script>

<template>
  <div class="pipelines-view">
    <!-- Header -->
    <div class="header">
      <div class="header-left">
        <div class="title">Pipelines</div>
        <div class="subtitle">
          <template v-if="loading">Loading…</template>
          <template v-else-if="!enabled">
            Pipeline integration is disabled. Enable it in Settings to manage PRs and branches.
          </template>
          <template v-else-if="!provider">
            No provider selected. Choose one in Settings.
          </template>
          <template v-else>
            <span class="provider-pill">{{ providerLabel }}</span>
            <span v-if="repoHint" class="repo-hint mono">{{ repoHint }}</span>
          </template>
        </div>
      </div>
    </div>

    <!-- Tab strip -->
    <div class="tabs">
      <button class="tab" :class="{ active: activeTab === 'prs' }" @click="setTab('prs')">
        Pull Requests
      </button>
      <button class="tab" :class="{ active: activeTab === 'branches' }" @click="setTab('branches')">
        Branches
      </button>
      <button class="tab" :class="{ active: activeTab === 'reports' }" @click="setTab('reports')">
        Reports
        <span v-if="reports.length" class="tab-badge">{{ reports.length }}</span>
      </button>
      <button class="tab" :class="{ active: activeTab === 'rules' }" @click="setTab('rules')">
        Rules &amp; Guidelines
      </button>
    </div>

    <!-- Body -->
    <div class="body">
      <!-- No provider selected -->
      <div v-if="!loading && (!enabled || !provider)" class="empty big">
        <div class="empty-title">Connect a pipeline provider</div>
        <div class="empty-body">
          Open <strong>Settings → Pipelines &amp; CI/CD</strong>, enable pipelines and pick a provider
          (GitHub Actions, GitLab CI/CD, AWS CodeBuild, Google Cloud Build, CircleCI, or Azure
          Pipelines). Once connected you can browse PRs and branches here, and ask the AI to
          summarize a PR or run a code review against your guidelines.
        </div>
      </div>

      <!-- PR tab -->
      <div v-else-if="activeTab === 'prs'" class="tab-pane">
        <div class="fetch-bar">
          <div class="fetch-status">
            <span v-if="prsState === 'loading'" class="dot-spinner" aria-label="loading"></span>
            <span v-else-if="prsState === 'ok'" class="dot ok"></span>
            <span v-else-if="prsState === 'error'" class="dot err"></span>
            <span v-else class="dot idle"></span>
            <span class="fetch-label">
              <template v-if="prsState === 'loading'">Calling api.github.com /repos/{{ repoHint || '…' }}/pulls…</template>
              <template v-else-if="prsState === 'ok'">{{ prs.length }} pull request{{ prs.length === 1 ? '' : 's' }} from <strong>{{ repoHint }}</strong></template>
              <template v-else-if="prsState === 'error'">{{ ghErrorTitle(prsErrorKind) }}</template>
              <template v-else>Idle.</template>
            </span>
          </div>
          <button class="action-btn" :disabled="prsState === 'loading' || provider !== 'github'" @click="fetchPRs">
            {{ prsState === 'loading' ? 'Refreshing…' : 'Refresh' }}
          </button>
        </div>

        <div v-if="prsState === 'loading'" class="fetch-skeleton">
          <div v-for="i in 4" :key="i" class="fetch-skel-row"></div>
        </div>

        <div v-else-if="prsState === 'error'" class="fetch-error">
          <div class="fetch-error-msg">{{ prsErrorMessage }}</div>
          <button
            v-if="ghErrorIsFixable(prsErrorKind)"
            class="action-btn primary"
            @click="jumpToGitHubConfig"
          >Configure GitHub in Settings →</button>
          <button v-else class="action-btn" @click="fetchPRs">Try again</button>
        </div>

        <div v-else-if="prsState === 'ok' && prs.length === 0" class="empty">
          <div class="empty-title">No open pull requests</div>
          <div class="empty-body">
            <strong>{{ repoHint }}</strong> has no open PRs right now. Open one on GitHub
            and click <em>Refresh</em>.
          </div>
        </div>

        <ul v-else-if="prsState === 'ok'" class="pr-list">
          <li v-for="pr in prs" :key="pr.number" class="pr-row" :class="{ draft: pr.draft }">
            <a class="pr-num mono" :href="pr.url" target="_blank" rel="noopener noreferrer">#{{ pr.number }}</a>
            <div class="pr-main">
              <div class="pr-title">
                <span v-if="pr.draft" class="pr-draft-tag">DRAFT</span>
                {{ pr.title }}
              </div>
              <div class="pr-meta">
                <span class="mono">{{ pr.author }}</span>
                <span class="dot-sep">·</span>
                <span class="mono">{{ pr.branch }}</span>
                <span class="dot-sep">→</span>
                <span class="mono">{{ pr.base }}</span>
              </div>
            </div>
            <div class="pr-actions">
              <button class="action-btn" disabled title="Wired up next: AI summary">✨ Summarize</button>
              <button class="action-btn" disabled title="Wired up next: AI code review">🔍 Review</button>
            </div>
          </li>
        </ul>

        <div v-else-if="provider !== 'github'" class="empty">
          <div class="empty-title">No live fetch for this provider yet</div>
          <div class="empty-body">
            Live PR listing is wired for GitHub today. Other providers will follow.
          </div>
        </div>
      </div>

      <!-- Branches tab -->
      <div v-else-if="activeTab === 'branches'" class="tab-pane">
        <div class="fetch-bar">
          <div class="fetch-status">
            <span v-if="branchesState === 'loading'" class="dot-spinner" aria-label="loading"></span>
            <span v-else-if="branchesState === 'ok'" class="dot ok"></span>
            <span v-else-if="branchesState === 'error'" class="dot err"></span>
            <span v-else class="dot idle"></span>
            <span class="fetch-label">
              <template v-if="branchesState === 'loading'">Calling api.github.com /repos/{{ repoHint || '…' }}/branches…</template>
              <template v-else-if="branchesState === 'ok'">{{ branches.length }} branch{{ branches.length === 1 ? '' : 'es' }} from <strong>{{ repoHint }}</strong></template>
              <template v-else-if="branchesState === 'error'">{{ ghErrorTitle(branchesErrorKind) }}</template>
              <template v-else>Idle.</template>
            </span>
          </div>
          <button class="action-btn" :disabled="branchesState === 'loading' || provider !== 'github'" @click="fetchBranches">
            {{ branchesState === 'loading' ? 'Refreshing…' : 'Refresh' }}
          </button>
        </div>

        <div v-if="branchesState === 'loading'" class="fetch-skeleton">
          <div v-for="i in 4" :key="i" class="fetch-skel-row"></div>
        </div>

        <div v-else-if="branchesState === 'error'" class="fetch-error">
          <div class="fetch-error-msg">{{ branchesErrorMessage }}</div>
          <button
            v-if="ghErrorIsFixable(branchesErrorKind)"
            class="action-btn primary"
            @click="jumpToGitHubConfig"
          >Configure GitHub in Settings →</button>
          <button v-else class="action-btn" @click="fetchBranches">Try again</button>
        </div>

        <div v-else-if="branchesState === 'ok' && branches.length === 0" class="empty">
          <div class="empty-title">No branches found</div>
          <div class="empty-body">The repo exists but returned an empty branch list.</div>
        </div>

        <ul v-else-if="branchesState === 'ok'" class="br-list">
          <li v-for="b in branches" :key="b.name" class="br-row">
            <span class="mono br-name">{{ b.name }}</span>
            <span v-if="b.protected" class="br-protected">protected</span>
            <span class="mono br-sha">{{ (b.sha || '').slice(0, 9) }}</span>
          </li>
        </ul>

        <div v-else-if="provider !== 'github'" class="empty">
          <div class="empty-title">No live fetch for this provider yet</div>
          <div class="empty-body">Live branch listing is wired for GitHub today.</div>
        </div>
      </div>

      <!-- Reports tab -->
      <div v-else-if="activeTab === 'reports'" class="tab-pane reports-pane">
        <div class="reports-header">
          <div class="reports-summary">
            Code review reports for <strong>{{ providerLabel }}</strong>.
            <span v-if="settings?.autoCodeReview" class="reports-auto">
              Auto-review is <strong>on</strong>
              ({{ DESTINATION_LABELS[settings.codeReviewDestination] || settings.codeReviewDestination || 'In-app only' }}).
            </span>
            <span v-else class="reports-auto-off">Auto-review is off — enable in Settings.</span>
          </div>
          <div class="reports-actions">
            <button class="action-btn" @click="loadReports" :disabled="reportsLoading">
              {{ reportsLoading ? 'Refreshing…' : 'Refresh' }}
            </button>
            <button class="action-btn primary" @click="openCreateDialog" :disabled="!provider">
              + New report
            </button>
          </div>
        </div>

        <div v-if="reportsError" class="report-error">Failed to load: {{ reportsError }}</div>

        <div v-if="!reports.length && !reportsLoading" class="empty">
          <div class="empty-title">No reports yet</div>
          <div class="empty-body">
            When auto-review is on (or you click <strong>+ New report</strong>), each review
            appears below as a document. Click a doc to read the rendered Markdown analysis.
          </div>
        </div>

        <div v-else class="reports-grid">
          <button
            v-for="rep in reports"
            :key="rep.id"
            class="report-card"
            @click="openReportDoc(rep)"
            :title="rep.title"
          >
            <svg class="report-icon" viewBox="0 0 24 24" width="36" height="36" aria-hidden="true">
              <path
                fill="currentColor"
                d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8l-6-6zm0 7V3.5L19.5 9H14z"
                opacity="0.85"
              />
              <path fill="var(--bg)" d="M8 13h8v1.5H8zm0 3h8v1.5H8zm0 3h5v1.5H8z" />
            </svg>
            <div class="report-meta">
              <div class="report-title">{{ rep.title }}</div>
              <div class="report-sub">
                <span v-if="rep.prRef" class="mono">{{ rep.prRef }}</span>
                <span v-if="rep.prRef" class="dot">·</span>
                <span>{{ formatDate(rep.createdAt) }}</span>
              </div>
            </div>
          </button>
        </div>
      </div>

      <!-- Rules tab -->
      <div v-else-if="activeTab === 'rules'" class="tab-pane rules-pane">
        <div class="rules-intro">
          <div>
            Markdown rules the AI uses when summarizing PRs and running code reviews for
            <strong>{{ providerLabel }}</strong>. Stored locally per provider.
          </div>
          <div class="rules-actions">
            <input
              ref="fileInputRef"
              type="file"
              :accept="ACCEPTED_EXT.join(',')"
              class="file-input"
              @change="onFilePicked"
            />
            <button class="action-btn" @click="pickFile" :disabled="!provider">
              Upload from Finder…
            </button>
            <button class="action-btn" @click="saveRules" :disabled="!provider || rulesSaving || !rulesDirty">
              {{ rulesSaving ? 'Saving…' : 'Save' }}
            </button>
            <button class="action-btn danger" @click="clearRules" :disabled="!provider || (!rules && !rulesDirty)">
              Clear
            </button>
            <span v-if="rulesSaveMsg" class="save-msg" :class="{ error: rulesSaveMsg.startsWith('Error') }">
              {{ rulesSaveMsg }}
            </span>
            <span v-else-if="rulesDirty" class="save-msg dirty">Unsaved changes</span>
          </div>
        </div>

        <div
          class="drop-zone"
          :class="{ active: dragOver }"
          @drop="onDrop"
          @dragover="onDragOver"
          @dragleave="onDragLeave"
        >
          <textarea
            class="rules-editor mono"
            spellcheck="false"
            :value="rules"
            :disabled="!provider || !rulesLoaded"
            :placeholder="provider
              ? '# PR Guidelines\n\n## Review checklist\n- All public APIs documented\n- Tests cover the happy path AND error paths\n- No new TODOs without a tracking issue\n\n## House style\n- Imports grouped: stdlib, third-party, local\n- One package per file\n\n_Drag a .md file here, or click Upload from Finder…_'
              : 'Select a pipeline provider in Settings to edit rules.'"
            @input="onRulesInput"
          ></textarea>
          <div v-if="dragOver" class="drop-overlay">
            <div class="drop-overlay-text">Drop file to load — replaces current rules</div>
          </div>
        </div>

        <div class="rules-footer">
          <span class="hint">
            Drop a <code>.md</code>, <code>.markdown</code>, <code>.txt</code>,
            <code>.mdc</code>, or <code>.cursorrules</code> file (max 256 KB).
          </span>
          <span v-if="rules" class="byte-count">{{ rules.length.toLocaleString() }} chars</span>
        </div>
      </div>
    </div>

    <!-- Report viewer modal -->
    <div v-if="openReport" class="modal-backdrop" @click.self="closeReport">
      <div class="modal report-modal">
        <div class="modal-header">
          <div class="modal-title">
            <svg class="modal-icon" viewBox="0 0 24 24" width="20" height="20" aria-hidden="true">
              <path fill="currentColor" d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8l-6-6zm0 7V3.5L19.5 9H14z" />
            </svg>
            <span>{{ openReport.title }}</span>
          </div>
          <div class="modal-actions">
            <button class="action-btn danger" @click="deleteReport(openReport)">Delete</button>
            <button class="action-btn" @click="closeReport">Close</button>
          </div>
        </div>
        <div class="modal-meta">
          <span v-if="openReport.prRef" class="mono">{{ openReport.prRef }}</span>
          <span v-if="openReport.prRef" class="dot">·</span>
          <span>{{ formatDate(openReport.createdAt) }}</span>
        </div>
        <div class="modal-body">
          <div v-if="openReportLoading" class="modal-loading">Loading…</div>
          <article v-else class="markdown" v-html="renderedReport"></article>
        </div>
      </div>
    </div>

    <!-- Create-report dialog -->
    <div v-if="showCreateDialog" class="modal-backdrop" @click.self="showCreateDialog = false">
      <div class="modal create-modal">
        <div class="modal-header">
          <div class="modal-title">New code review report</div>
        </div>
        <div class="modal-body create-body">
          <div class="field">
            <label class="field-label">Title</label>
            <input v-model="newReport.title" class="field-input" placeholder="e.g. PR #42 review" />
          </div>
          <div class="field">
            <label class="field-label">PR reference (optional)</label>
            <input v-model="newReport.prRef" class="field-input mono" placeholder="github:acme/repo#42" />
          </div>
          <div class="field">
            <label class="field-label">Markdown body</label>
            <textarea
              v-model="newReport.body"
              class="rules-editor mono"
              spellcheck="false"
              style="min-height: 200px;"
              placeholder="## Findings&#10;&#10;- ..."
            ></textarea>
          </div>
        </div>
        <div class="modal-footer">
          <button class="action-btn" @click="showCreateDialog = false" :disabled="creatingReport">Cancel</button>
          <button
            class="action-btn primary"
            @click="submitCreate"
            :disabled="!newReport.title.trim() || creatingReport"
          >
            {{ creatingReport ? 'Saving…' : 'Save report' }}
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.pipelines-view { flex: 1; display: flex; flex-direction: column; overflow: hidden; }

.header {
  height: 50px; display: flex; align-items: center; padding: 0 20px;
  border-bottom: 1px solid var(--border); background: var(--bg2); flex-shrink: 0;
}
.title { font-size: 14px; font-weight: 600; color: var(--text); }
.subtitle { font-size: 11px; color: var(--text3); margin-top: 1px; display: flex; align-items: center; gap: 8px; }
.provider-pill {
  background: rgba(79,142,247,0.15); color: var(--accent2);
  padding: 2px 8px; border-radius: 10px; font-weight: 600;
  text-transform: uppercase; letter-spacing: 0.04em; font-size: 10.5px;
}
.repo-hint { color: var(--text2); font-size: 11px; }

.tabs {
  display: flex; align-items: center; height: 38px;
  border-bottom: 1px solid var(--border); background: var(--bg2);
  padding: 0 16px; gap: 2px; flex-shrink: 0;
}
.tab {
  padding: 5px 12px; font-size: 12.5px; font-weight: 400; color: var(--text2);
  cursor: pointer; border-radius: 6px; transition: all 0.1s;
  background: transparent; border: 0; white-space: nowrap;
}
.tab:hover { background: var(--bg3); color: var(--text); }
.tab.active { background: rgba(79,142,247,0.12); color: var(--accent2); font-weight: 500; }

.body { flex: 1; overflow: hidden; display: flex; flex-direction: column; min-height: 0; }
.tab-pane { flex: 1; overflow: auto; padding: 20px; display: flex; flex-direction: column; }

.empty {
  margin: auto; max-width: 560px; padding: 24px; text-align: center;
  border: 1px dashed var(--border); border-radius: 8px; background: var(--bg3);
}
.empty.big { margin: 40px auto; }
.empty-title { font-size: 14px; font-weight: 600; color: var(--text); margin-bottom: 8px; }
.empty-body { font-size: 12.5px; color: var(--text2); line-height: 1.55; }

.ai-actions { display: flex; align-items: center; gap: 8px; margin-top: 16px; flex-wrap: wrap; justify-content: center; }
.ai-actions.preview { opacity: 0.85; }
.preview-note { font-size: 11px; color: var(--text3); font-style: italic; }

.action-btn {
  padding: 6px 14px; border-radius: 6px; border: 1px solid var(--border);
  background: var(--bg); color: var(--text2); font-size: 12px; font-weight: 500;
  cursor: pointer; transition: all 0.15s;
}
.action-btn:hover:not(:disabled) { background: var(--bg4); color: var(--text); }
.action-btn:disabled { opacity: 0.5; cursor: not-allowed; }
.action-btn.primary { background: var(--accent); color: white; border-color: var(--accent); }
.action-btn.primary:hover:not(:disabled) { background: var(--accent2); }
.action-btn.danger { color: var(--red); border-color: var(--red); }
.action-btn.danger:hover:not(:disabled) { background: rgba(217,72,72,0.08); }

/* Rules tab */
.rules-pane { gap: 12px; padding: 16px 20px; }

.rules-intro { display: flex; flex-direction: column; gap: 10px; font-size: 12.5px; color: var(--text2); line-height: 1.5; }
.rules-actions { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; }
.file-input { display: none; }

.save-msg { font-size: 12px; color: var(--green); }
.save-msg.error { color: var(--red); }
.save-msg.dirty { color: var(--text3); font-style: italic; }

.drop-zone {
  flex: 1; min-height: 240px; position: relative;
  border: 1px solid var(--border); border-radius: 8px; background: var(--bg);
  transition: border-color 0.15s, background 0.15s;
}
.drop-zone.active { border-color: var(--accent); background: rgba(79,142,247,0.06); }

.rules-editor {
  width: 100%; height: 100%; box-sizing: border-box; resize: none;
  background: transparent; color: var(--text);
  border: 0; outline: none; padding: 14px 16px;
  font-family: var(--mono); font-size: 12.5px; line-height: 1.5;
}
.rules-editor:disabled { opacity: 0.6; }

.drop-overlay {
  position: absolute; inset: 0; display: flex; align-items: center; justify-content: center;
  background: rgba(79,142,247,0.12);
  border: 2px dashed var(--accent); border-radius: 8px; pointer-events: none;
}
.drop-overlay-text { color: var(--accent2); font-weight: 600; font-size: 13px; }

.rules-footer { display: flex; justify-content: space-between; align-items: center; font-size: 11px; color: var(--text3); }
.rules-footer code {
  background: var(--bg3); padding: 1px 4px; border-radius: 3px;
  font-family: var(--mono); font-size: 10.5px; color: var(--text2);
}
.byte-count { font-family: var(--mono); }
.mono { font-family: var(--mono); }

.tab-badge {
  display: inline-block; margin-left: 6px; padding: 0 6px; min-width: 16px;
  font-size: 10.5px; font-weight: 600; line-height: 16px; text-align: center;
  background: var(--bg4); color: var(--text2); border-radius: 8px;
}
.tab.active .tab-badge { background: rgba(79,142,247,0.25); color: var(--accent2); }

/* Reports tab */
.reports-pane { gap: 12px; padding: 16px 20px; }
.reports-header {
  display: flex; justify-content: space-between; align-items: center; gap: 12px;
  flex-wrap: wrap;
}
.reports-summary { font-size: 12.5px; color: var(--text2); line-height: 1.5; }
.reports-auto { color: var(--green); margin-left: 4px; }
.reports-auto-off { color: var(--text3); margin-left: 4px; font-style: italic; }
.reports-actions { display: flex; gap: 8px; }
.report-error {
  background: rgba(217,72,72,0.08); color: var(--red);
  border: 1px solid rgba(217,72,72,0.3); border-radius: 6px;
  padding: 8px 12px; font-size: 12px;
}

.reports-grid {
  display: grid; grid-template-columns: repeat(auto-fill, minmax(220px, 1fr));
  gap: 10px;
}
.report-card {
  display: flex; gap: 10px; align-items: flex-start;
  padding: 12px; border: 1px solid var(--border); border-radius: 8px;
  background: var(--bg); cursor: pointer; text-align: left;
  transition: border-color 0.15s, transform 0.05s;
  font: inherit; color: inherit;
}
.report-card:hover { border-color: var(--accent); }
.report-card:active { transform: translateY(1px); }
.report-icon { color: var(--accent2); flex-shrink: 0; }
.report-meta { display: flex; flex-direction: column; gap: 4px; min-width: 0; }
.report-title {
  font-size: 12.5px; font-weight: 600; color: var(--text);
  overflow: hidden; text-overflow: ellipsis; white-space: nowrap;
}
.report-sub {
  font-size: 11px; color: var(--text3);
  display: flex; flex-wrap: wrap; align-items: center; gap: 4px;
}
.dot { opacity: 0.6; }

/* Modals */
.modal-backdrop {
  position: fixed; inset: 0; z-index: 100;
  background: rgba(0,0,0,0.55);
  display: flex; align-items: center; justify-content: center;
  padding: 32px;
}
.modal {
  background: var(--bg2); border: 1px solid var(--border); border-radius: 10px;
  display: flex; flex-direction: column; max-height: 90vh; min-width: 0;
  box-shadow: 0 16px 48px rgba(0,0,0,0.45);
}
.report-modal { width: min(820px, 100%); }
.create-modal { width: min(680px, 100%); }
.modal-header {
  display: flex; justify-content: space-between; align-items: center;
  padding: 14px 20px; border-bottom: 1px solid var(--border); gap: 10px;
}
.modal-title {
  display: flex; align-items: center; gap: 8px;
  font-size: 14px; font-weight: 600; color: var(--text); min-width: 0;
}
.modal-icon { color: var(--accent2); flex-shrink: 0; }
.modal-title span { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.modal-actions { display: flex; gap: 8px; flex-shrink: 0; }
.modal-meta {
  padding: 8px 20px; font-size: 11px; color: var(--text3);
  display: flex; gap: 6px; align-items: center; flex-wrap: wrap;
  border-bottom: 1px solid var(--border);
}
.modal-body { flex: 1; overflow: auto; padding: 18px 24px; }
.create-body { display: flex; flex-direction: column; gap: 12px; }
.modal-footer {
  display: flex; justify-content: flex-end; gap: 8px;
  padding: 12px 20px; border-top: 1px solid var(--border);
}
.modal-loading { color: var(--text3); font-size: 12.5px; padding: 24px; text-align: center; }

/* Markdown rendering */
.markdown { color: var(--text); font-size: 13px; line-height: 1.6; }
.markdown :deep(h1) { font-size: 18px; margin: 0 0 12px; padding-bottom: 6px; border-bottom: 1px solid var(--border); }
.markdown :deep(h2) { font-size: 15px; margin: 18px 0 10px; }
.markdown :deep(h3) { font-size: 13.5px; margin: 14px 0 8px; }
.markdown :deep(p) { margin: 8px 0; }
.markdown :deep(ul), .markdown :deep(ol) { padding-left: 22px; margin: 8px 0; }
.markdown :deep(li) { margin: 3px 0; }
.markdown :deep(code) {
  background: var(--bg3); padding: 1px 5px; border-radius: 3px;
  font-family: var(--mono); font-size: 12px;
}
.markdown :deep(pre) {
  background: var(--bg); border: 1px solid var(--border); border-radius: 6px;
  padding: 10px 12px; overflow: auto; margin: 10px 0;
}
.markdown :deep(pre code) { background: transparent; padding: 0; font-size: 11.5px; }
.markdown :deep(blockquote) {
  border-left: 3px solid var(--accent); padding: 0 12px; margin: 10px 0;
  color: var(--text2);
}
.markdown :deep(table) { border-collapse: collapse; margin: 10px 0; font-size: 12px; }
.markdown :deep(th), .markdown :deep(td) {
  border: 1px solid var(--border); padding: 5px 10px; text-align: left;
}
.markdown :deep(th) { background: var(--bg3); }
.markdown :deep(a) { color: var(--accent2); }

/* Live-fetch indicator + PR/Branch lists ------------------------------- */

.fetch-bar {
  display: flex; align-items: center; justify-content: space-between;
  gap: 12px; margin-bottom: 14px;
}
.fetch-status { display: flex; align-items: center; gap: 8px; font-size: 12.5px; color: var(--text2); }
.fetch-label { line-height: 1.4; }
.fetch-label strong { color: var(--text); }

/* The dot that summarizes API call state. The 'spinner' variant is a
   small animated ring that makes "an API call is happening" unmistakable
   even at a glance. */
.dot { display: inline-block; width: 9px; height: 9px; border-radius: 50%; flex-shrink: 0; }
.dot.idle { background: var(--text3); }
.dot.ok   { background: #3ecf8e; box-shadow: 0 0 0 0 rgba(62,207,142,0.0); animation: ok-pulse 2.5s ease-out infinite; }
.dot.err  { background: #f05454; }
@keyframes ok-pulse {
  0%   { box-shadow: 0 0 0 0 rgba(62,207,142,0.6); }
  70%  { box-shadow: 0 0 0 8px rgba(62,207,142,0); }
  100% { box-shadow: 0 0 0 0 rgba(62,207,142,0); }
}

.dot-spinner {
  display: inline-block; width: 13px; height: 13px; flex-shrink: 0;
  border: 2px solid rgba(79,142,247,0.25);
  border-top-color: #4f8ef7;
  border-radius: 50%;
  animation: dot-spin 0.8s linear infinite;
}
@keyframes dot-spin { to { transform: rotate(360deg); } }

/* Skeleton placeholders while a fetch is in flight — the user always
   has *something* to look at instead of a flat empty pane. */
.fetch-skeleton { display: flex; flex-direction: column; gap: 8px; }
.fetch-skel-row {
  height: 38px; border-radius: 6px;
  background: linear-gradient(90deg, var(--bg3) 0%, rgba(255,255,255,0.04) 50%, var(--bg3) 100%);
  background-size: 200% 100%;
  animation: skel-shimmer 1.4s ease-in-out infinite;
  border: 1px solid var(--border);
}
@keyframes skel-shimmer {
  0%   { background-position: 200% 0; }
  100% { background-position: -200% 0; }
}

/* Error block — copy + the CTA that takes the user to GitHub config. */
.fetch-error {
  border: 1px solid rgba(240,84,84,0.4);
  background: rgba(240,84,84,0.06);
  border-radius: 8px; padding: 14px 16px;
  display: flex; flex-direction: column; align-items: flex-start; gap: 10px;
}
.fetch-error-msg { color: var(--text); font-size: 12.5px; line-height: 1.5; }

/* PR list */
.pr-list { list-style: none; margin: 0; padding: 0; display: flex; flex-direction: column; gap: 6px; }
.pr-row {
  display: grid; grid-template-columns: 56px 1fr auto;
  gap: 12px; align-items: center;
  padding: 10px 12px; border-radius: 6px;
  background: var(--bg2); border: 1px solid var(--border);
}
.pr-row.draft { opacity: 0.7; }
.pr-num {
  color: var(--accent2); text-decoration: none; font-weight: 600;
  font-size: 12.5px;
}
.pr-num:hover { text-decoration: underline; }
.pr-main { display: flex; flex-direction: column; gap: 3px; min-width: 0; }
.pr-title { font-size: 13px; color: var(--text); font-weight: 500;
  white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.pr-meta { display: flex; gap: 6px; align-items: center; font-size: 11px; color: var(--text3); flex-wrap: wrap; }
.pr-meta .mono { color: var(--text2); }
.dot-sep { color: var(--text3); opacity: 0.7; }
.pr-draft-tag {
  font-size: 9.5px; font-weight: 700; letter-spacing: 0.05em;
  color: #f5a623;
  background: rgba(245,166,35,0.15);
  padding: 1px 6px; border-radius: 3px;
  margin-right: 6px; vertical-align: middle;
}
.pr-actions { display: flex; gap: 6px; }

/* Branch list */
.br-list { list-style: none; margin: 0; padding: 0; display: flex; flex-direction: column; gap: 4px; }
.br-row {
  display: flex; gap: 12px; align-items: center;
  padding: 7px 10px; border-radius: 5px;
  background: var(--bg2); border: 1px solid var(--border);
  font-size: 12px;
}
.br-name { flex: 1; color: var(--text); }
.br-sha { color: var(--text3); }
.br-protected {
  font-size: 10px; padding: 1px 6px; border-radius: 10px;
  background: rgba(167,139,250,0.18); color: #a78bfa;
  text-transform: uppercase; letter-spacing: 0.05em; font-weight: 600;
}
</style>
