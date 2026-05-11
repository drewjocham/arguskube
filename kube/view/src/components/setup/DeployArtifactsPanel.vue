<script setup>
import { ref, computed, onMounted, watch } from 'vue'
import { callGo } from '../../composables/useBridge'

function flavorLabel(f) {
  switch (f) {
    case 'helm':    return '⛵ Helm'
    case 'docker':  return '🐳 Docker'
    case 'compose': return '🧱 Docker Compose'
    default:        return f
  }
}

// DeployArtifactsPanel — per-tool deployment helper. Renders the
// install command + downloadable file (helm values, docker-compose…)
// for each available flavor, plus a drop zone for an .env file. When
// required env vars are missing it surfaces them inline with editable
// fields; the user fixes them and re-validates without leaving the
// panel.

const props = defineProps({
  // Restrict the catalog to a single tool name. Empty = show all.
  tool: { type: String, default: '' },
})

const allArtifacts = ref([])
const loadError = ref('')
const selectedFlavor = ref('')

const filtered = computed(() => {
  if (!props.tool) return allArtifacts.value
  return allArtifacts.value.filter(a => a.tool === props.tool)
})

const current = computed(() => {
  return filtered.value.find(a => a.flavor === selectedFlavor.value) || filtered.value[0] || null
})

onMounted(async () => {
  try {
    allArtifacts.value = (await callGo('GetDeployArtifacts')) || []
    if (filtered.value.length) selectedFlavor.value = filtered.value[0].flavor
  } catch (e) {
    loadError.value = e?.message || String(e)
  }
})

// Keep selectedFlavor consistent with the available list.
watch(filtered, (list) => {
  if (!list.find(a => a.flavor === selectedFlavor.value)) {
    selectedFlavor.value = list[0]?.flavor || ''
  }
})

// ── Env file handling ──────────────────────────────────────────────
const envFileName = ref('')
const envFileBody = ref('')
const validation = ref(null) // { missing, present, vars }
const validating = ref(false)
const validationError = ref('')

// Track edits to missing vars so the user can fix in-place without
// uploading a fresh .env.
const draftMissing = ref({})

async function validate() {
  if (!current.value) return
  validating.value = true
  validationError.value = ''
  try {
    // Merge the user's typed values back into the env body so the
    // backend re-checks against the current state, not just the
    // original upload.
    let body = envFileBody.value
    for (const [k, v] of Object.entries(draftMissing.value)) {
      if (!v) continue
      if (body && !body.endsWith('\n')) body += '\n'
      body += `${k}=${v}\n`
    }
    validation.value = await callGo('ValidateEnvFile', current.value.tool, current.value.flavor, body)
  } catch (e) {
    validationError.value = e?.message || String(e)
  } finally {
    validating.value = false
  }
}

function onFileSelected(file) {
  if (!file) return
  envFileName.value = file.name
  const reader = new FileReader()
  reader.onload = () => {
    envFileBody.value = String(reader.result || '')
    draftMissing.value = {}
    validate()
  }
  reader.readAsText(file)
}

function onDrop(e) {
  e.preventDefault()
  const f = e.dataTransfer?.files?.[0]
  if (f) onFileSelected(f)
}

function onPick(e) {
  const f = e.target.files?.[0]
  if (f) onFileSelected(f)
}

function clearEnv() {
  envFileName.value = ''
  envFileBody.value = ''
  validation.value = null
  draftMissing.value = {}
}

// ── Copy / download helpers ────────────────────────────────────────
const copied = ref('')

function copyText(text, label) {
  if (!text) return
  navigator.clipboard?.writeText(text).then(() => {
    copied.value = label
    setTimeout(() => { copied.value = '' }, 2000)
  })
}

function downloadFile(name, text) {
  const blob = new Blob([text], { type: 'text/plain' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = name
  document.body.appendChild(a)
  a.click()
  document.body.removeChild(a)
  setTimeout(() => URL.revokeObjectURL(url), 1000)
}

const missingNames = computed(() => validation.value?.missing || [])
const allRequired = computed(() => current.value?.envVars?.filter(v => v.required) || [])

function hintFor(name) {
  if (!current.value) return ''
  const spec = (current.value.envVars || []).find(v => v.name === name)
  return spec?.hint || ''
}
</script>

<template>
  <div class="deploy-panel">
    <div v-if="loadError" class="deploy-error">Could not load deploy catalog: {{ loadError }}</div>

    <div v-else-if="!filtered.length" class="deploy-empty">
      No deployment recipes for this tool yet.
    </div>

    <template v-else>
      <!-- Flavor tabs -->
      <div class="flavor-tabs">
        <button
          v-for="art in filtered"
          :key="art.flavor"
          :class="['flavor-tab', { active: art.flavor === selectedFlavor }]"
          @click="selectedFlavor = art.flavor"
        >{{ flavorLabel(art.flavor) }}</button>
      </div>

      <div v-if="current" class="deploy-card">
        <div class="deploy-desc">{{ current.description }}</div>

        <!-- Command block -->
        <div class="block">
          <div class="block-head">
            <span class="block-title">Command</span>
            <button class="block-action" @click="copyText(current.commandText, 'command')">
              {{ copied === 'command' ? '✓ Copied' : 'Copy' }}
            </button>
          </div>
          <pre class="block-pre">{{ current.commandText }}</pre>
        </div>

        <!-- Optional file body -->
        <div v-if="current.fileText" class="block">
          <div class="block-head">
            <span class="block-title">{{ current.fileName }}</span>
            <div class="block-actions">
              <button class="block-action" @click="copyText(current.fileText, 'file')">
                {{ copied === 'file' ? '✓ Copied' : 'Copy' }}
              </button>
              <button class="block-action" @click="downloadFile(current.fileName, current.fileText)">Download</button>
            </div>
          </div>
          <pre class="block-pre file">{{ current.fileText }}</pre>
        </div>

        <!-- Env file uploader -->
        <div v-if="allRequired.length" class="block">
          <div class="block-head">
            <span class="block-title">Environment file</span>
            <div class="block-actions">
              <input
                ref="fileInput"
                type="file"
                accept=".env,text/plain"
                style="display:none"
                @change="onPick"
                id="env-file-input"
              />
              <label for="env-file-input" class="block-action label-as-button">Choose .env</label>
              <button v-if="envFileBody" class="block-action" @click="clearEnv">Clear</button>
            </div>
          </div>

          <div
            class="env-drop"
            :class="{ filled: !!envFileBody }"
            @dragover.prevent
            @drop="onDrop"
          >
            <template v-if="!envFileBody">
              <div class="env-drop-hint">
                Drop your <code>.env</code> here, or click <strong>Choose .env</strong>.
              </div>
              <div class="env-drop-sub">
                Required variables: <code v-for="v in allRequired" :key="v.name">{{ v.name }}</code>
              </div>
            </template>
            <template v-else>
              <div class="env-loaded">
                <span class="env-icon">📄</span>
                <span class="env-name">{{ envFileName || '.env' }}</span>
                <span class="env-meta">{{ Object.keys(validation?.vars || {}).length }} vars parsed</span>
              </div>
            </template>
          </div>

          <!-- Validation -->
          <div v-if="validating" class="env-status">Validating…</div>
          <div v-else-if="validationError" class="env-status fail">{{ validationError }}</div>
          <div v-else-if="validation && validation.missing.length" class="env-status warn">
            Missing required values — fill in below to continue.
            <div class="env-missing-grid">
              <div v-for="name in missingNames" :key="name" class="env-missing-row">
                <label class="env-missing-label">{{ name }}</label>
                <input
                  type="text"
                  class="env-missing-input"
                  v-model="draftMissing[name]"
                  :placeholder="hintFor(name)"
                />
              </div>
            </div>
            <button class="env-recheck" @click="validate">Re-validate</button>
          </div>
          <div v-else-if="validation" class="env-status ok">
            All required variables present. Run the command above to deploy.
          </div>
        </div>
      </div>
    </template>
  </div>
</template>

<style scoped>
.deploy-panel { display: flex; flex-direction: column; gap: 12px; }

.deploy-error, .deploy-empty {
  padding: 12px;
  background: rgba(245,166,35,0.08);
  border: 1px solid rgba(245,166,35,0.3);
  border-radius: 6px;
  color: var(--amber2);
  font-size: 12.5px;
}

.flavor-tabs { display: flex; gap: 6px; }
.flavor-tab {
  padding: 5px 12px;
  border-radius: 999px;
  border: 1px solid var(--border);
  background: var(--bg);
  color: var(--text2);
  font: inherit;
  font-size: 12px;
  cursor: pointer;
  transition: all 0.15s;
}
.flavor-tab:hover { background: var(--bg4); color: var(--text); }
.flavor-tab.active { background: var(--accent); color: #fff; border-color: var(--accent); }

.deploy-card { display: flex; flex-direction: column; gap: 12px; }
.deploy-desc { font-size: 12.5px; color: var(--text2); line-height: 1.5; }

.block {
  background: var(--bg); border: 1px solid var(--border); border-radius: 6px;
  padding: 10px;
}
.block-head { display: flex; justify-content: space-between; align-items: center; margin-bottom: 8px; }
.block-title { font-size: 11.5px; font-weight: 500; color: var(--text2); }
.block-actions { display: flex; gap: 6px; }
.block-action {
  padding: 3px 10px;
  border-radius: 4px;
  border: 1px solid var(--border2);
  background: var(--bg3);
  color: var(--text);
  font: inherit;
  font-size: 11px;
  cursor: pointer;
  transition: background 0.15s;
}
.block-action:hover { background: var(--bg4); }
.label-as-button { display: inline-block; }

.block-pre {
  margin: 0;
  padding: 10px 12px;
  background: var(--bg2);
  border: 1px solid rgba(255,255,255,0.04);
  border-radius: 4px;
  color: var(--text);
  font-family: var(--mono);
  font-size: 11.5px;
  white-space: pre-wrap;
  word-break: break-word;
  overflow-x: auto;
  max-height: 220px;
  overflow-y: auto;
}
.block-pre.file { max-height: 320px; }

.env-drop {
  border: 1.5px dashed var(--border2);
  border-radius: 6px;
  padding: 14px;
  text-align: center;
  background: var(--bg2);
  transition: border-color 0.15s;
}
.env-drop.filled { border-style: solid; background: var(--bg); }
.env-drop-hint { font-size: 12.5px; color: var(--text); margin-bottom: 4px; }
.env-drop-hint code { font-family: var(--mono); font-size: 11px; }
.env-drop-sub { font-size: 11px; color: var(--text3); display: flex; flex-wrap: wrap; gap: 4px; justify-content: center; }
.env-drop-sub code {
  font-family: var(--mono);
  background: var(--bg3); padding: 1px 4px; border-radius: 3px;
}

.env-loaded { display: flex; align-items: center; gap: 8px; justify-content: center; font-size: 12.5px; color: var(--text); }
.env-name { font-family: var(--mono); font-size: 12px; }
.env-meta { font-size: 11px; color: var(--text3); }

.env-status { margin-top: 8px; font-size: 11.5px; padding: 8px 10px; border-radius: 4px; }
.env-status.ok   { background: rgba(62,207,142,0.08); color: var(--green2); border: 1px solid rgba(62,207,142,0.25); }
.env-status.warn { background: rgba(245,166,35,0.08); color: var(--amber2); border: 1px solid rgba(245,166,35,0.25); }
.env-status.fail { background: rgba(240,84,84,0.08);  color: var(--red2);   border: 1px solid rgba(240,84,84,0.25); }

.env-missing-grid { display: flex; flex-direction: column; gap: 6px; margin-top: 8px; }
.env-missing-row { display: flex; gap: 6px; align-items: center; }
.env-missing-label { font-family: var(--mono); font-size: 11px; color: var(--text); min-width: 200px; }
.env-missing-input {
  flex: 1; padding: 4px 8px;
  background: var(--bg2); border: 1px solid var(--border);
  border-radius: 4px; color: var(--text); font: inherit; font-size: 12px;
  font-family: var(--mono);
  outline: none;
}
.env-missing-input:focus { border-color: var(--accent); }

.env-recheck {
  margin-top: 8px;
  padding: 5px 12px;
  border-radius: 4px;
  border: 1px solid var(--accent);
  background: rgba(79,142,247,0.1);
  color: var(--accent2);
  font: inherit;
  font-size: 11.5px;
  cursor: pointer;
}
.env-recheck:hover { background: rgba(79,142,247,0.2); }
</style>
