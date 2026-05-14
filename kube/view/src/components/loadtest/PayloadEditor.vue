<script setup>
import { ref, computed, watch } from 'vue'
import { useDistLoadStore } from '../../stores/distload'

// PayloadEditor: tabbed payload input (upload / paste / type / AI / local
// file) with an always-visible editable preview underneath. We keep this
// as a single component because all five tabs share the same target
// (payloadBytes) and a split would force callers to track them all
// separately. The parent only needs to v-model the payload spec.

const props = defineProps({
  modelValue: {
    type: Object,
    required: true,
    // shape: { source, bytes, filename, filePath, fileMode, aiPrompt }
  },
})
const emit = defineEmits(['update:modelValue'])

const store = useDistLoadStore()

const TABS = [
  { id: 'upload', label: 'Upload' },
  { id: 'paste', label: 'Paste' },
  { id: 'type', label: 'Type' },
  { id: 'ai', label: 'Ask AI' },
  { id: 'file', label: 'Local file' },
]

const activeTab = computed({
  get: () => props.modelValue.source || 'paste',
  set: (v) => patch('source', v),
})
const bytes = computed({
  get: () => props.modelValue.bytes || '',
  set: (v) => patch('bytes', v),
})

function patch(field, value) {
  emit('update:modelValue', { ...props.modelValue, [field]: value })
}

// JSON lint is non-blocking — payloads may legitimately be non-JSON.
const lintError = ref('')
function lintJson() {
  const v = bytes.value
  if (!v.trim()) { lintError.value = ''; return }
  try { JSON.parse(v); lintError.value = '' }
  catch (e) { lintError.value = `Invalid JSON: ${e.message}` }
}
watch(activeTab, () => { lintError.value = '' })

// ── Upload tab ──────────────────────────────────────────────────────
function onFileChange(e) {
  const f = e.target.files?.[0]
  if (f) readUploaded(f)
}
function onDrop(e) {
  e.preventDefault()
  const f = e.dataTransfer?.files?.[0]
  if (f) readUploaded(f)
}
function readUploaded(file) {
  const reader = new FileReader()
  reader.onload = (ev) => {
    emit('update:modelValue', {
      ...props.modelValue,
      bytes: ev.target.result,
      filename: file.name,
    })
  }
  reader.readAsText(file)
}

// ── AI tab ──────────────────────────────────────────────────────────
const aiBusy = ref(false)
const aiError = ref('')
const aiPrompt = computed({
  get: () => props.modelValue.aiPrompt || '',
  set: (v) => patch('aiPrompt', v),
})
async function generateAi() {
  aiBusy.value = true
  aiError.value = ''
  try {
    // sizeHint is approximate — we ask for "match the editable
    // preview's current size or default to 1KB". Backend may ignore.
    const sizeHint = bytes.value?.length || 1024
    const out = await store.generatePayload(aiPrompt.value, sizeHint)
    if (typeof out === 'string') patch('bytes', out)
    else patch('bytes', JSON.stringify(out, null, 2))
  } catch (e) {
    aiError.value = e.message ?? String(e)
  } finally {
    aiBusy.value = false
  }
}

// ── Local file tab ──────────────────────────────────────────────────
const resolved = ref(null) // { kind, files, sample }
const resolveError = ref('')
const localPath = computed({
  get: () => props.modelValue.filePath || '',
  set: (v) => patch('filePath', v),
})
const fileMode = computed({
  get: () => props.modelValue.fileMode || 'template',
  set: (v) => patch('fileMode', v),
})

// Preview is editable for inline sources (upload/paste/type/ai) but
// read-only when the source is "file" — the engine reads from disk at
// runtime, so edits to the in-memory preview wouldn't actually be sent.
const previewReadonly = computed(() => source.value === 'file')
async function doResolve() {
  resolveError.value = ''
  resolved.value = null
  try {
    const out = await store.resolvePayloadPath(localPath.value)
    resolved.value = out
    if (out?.sample) patch('bytes', out.sample)
  } catch (e) {
    resolveError.value = e.message ?? String(e)
  }
}

// ── Size indicator ──────────────────────────────────────────────────
const sizeLabel = computed(() => {
  const n = (bytes.value || '').length
  if (n < 1024) return `${n} bytes`
  return `${n} bytes / ${(n / 1024).toFixed(1)} KB`
})
</script>

<template>
  <div class="payload-editor" aria-label="Payload editor">
    <div class="payload-tabs" role="tablist">
      <button
        v-for="t in TABS"
        :key="t.id"
        type="button"
        role="tab"
        :aria-selected="activeTab === t.id"
        class="payload-tab"
        :class="{ active: activeTab === t.id }"
        :data-testid="`distload-payload-tab-${t.id}`"
        @click="activeTab = t.id"
      >{{ t.label }}</button>
    </div>

    <!-- Upload -->
    <div v-show="activeTab === 'upload'" class="payload-panel">
      <div class="dropzone" @dragover.prevent @drop="onDrop" data-testid="distload-payload-dropzone">
        <span v-if="modelValue.filename" class="drop-filename">{{ modelValue.filename }}</span>
        <span v-else class="drop-hint">Drop a .json or .txt file here, or</span>
        <label class="file-pick-btn">
          Browse
          <input type="file" accept=".json,.txt,application/json,text/plain" class="sr-only" data-testid="distload-payload-file-input" @change="onFileChange" />
        </label>
      </div>
    </div>

    <!-- Paste -->
    <div v-show="activeTab === 'paste'" class="payload-panel">
      <textarea class="form-textarea" rows="6" placeholder="Paste payload here" :value="bytes" @input="bytes = $event.target.value" @blur="lintJson" />
    </div>

    <!-- Type -->
    <div v-show="activeTab === 'type'" class="payload-panel">
      <textarea class="form-textarea" rows="6" placeholder="Type payload" :value="bytes" @input="bytes = $event.target.value" @blur="lintJson" />
    </div>

    <!-- AI -->
    <div v-show="activeTab === 'ai'" class="payload-panel">
      <textarea class="form-textarea" rows="3" placeholder="Describe the payload you want (e.g., 'OrderCreated event with random fields')" :value="aiPrompt" @input="aiPrompt = $event.target.value" />
      <button type="button" class="btn-sm" :disabled="aiBusy || !aiPrompt.trim()" data-testid="distload-payload-ai-generate" @click="generateAi">
        {{ aiBusy ? 'Generating…' : 'Generate' }}
      </button>
      <p v-if="aiError" class="error-msg" role="alert">{{ aiError }}</p>
    </div>

    <!-- Local file -->
    <div v-show="activeTab === 'file'" class="payload-panel">
      <div class="file-row">
        <input class="form-input" placeholder="/path/to/file-or-dir" :value="localPath" @input="localPath = $event.target.value" data-testid="distload-payload-local-path" />
        <button type="button" class="btn-sm" data-testid="distload-payload-resolve" @click="doResolve">Resolve</button>
      </div>
      <p v-if="resolveError" class="error-msg" role="alert">{{ resolveError }}</p>
      <div v-if="resolved" class="resolved-block">
        <div class="resolved-kind">{{ resolved.kind }}{{ resolved.files?.length ? ` — ${resolved.files.length} file(s)` : '' }}</div>
        <div v-if="resolved.files?.length" class="resolved-list">
          <div v-for="f in resolved.files" :key="f.path" class="resolved-row">
            <span class="resolved-name">{{ f.name }}</span>
            <span class="resolved-size">{{ f.size }} B</span>
          </div>
        </div>
        <div v-if="resolved.kind === 'dir'" class="dir-mode">
          <label class="mode-opt">
            <input type="radio" name="dir-mode" value="template" :checked="fileMode === 'template'" @change="fileMode = 'template'" />
            Use one as template (generate <code>count</code> from it)
          </label>
          <label class="mode-opt">
            <input type="radio" name="dir-mode" value="exact" :checked="fileMode === 'exact'" @change="fileMode = 'exact'" />
            Send each exactly
          </label>
        </div>
      </div>
    </div>

    <!-- Always-visible preview. Read-only in file mode because the
         backend reads from disk at run time; editing here would silently
         discard the change. The user can switch to Paste/Type to make the
         preview authoritative. -->
    <div class="preview-block">
      <div class="preview-head">
        <span class="preview-label">{{ previewReadonly ? 'Preview (read-only — backend reads files at runtime)' : 'Editable preview' }}</span>
        <span class="preview-size">{{ sizeLabel }}</span>
      </div>
      <textarea
        class="form-textarea preview-area"
        rows="14"
        :value="bytes"
        :readonly="previewReadonly"
        data-testid="distload-payload-preview"
        @input="bytes = $event.target.value"
        @blur="lintJson"
      />
      <p v-if="lintError" class="lint-msg">{{ lintError }}</p>
    </div>
  </div>
</template>

<style scoped>
.payload-editor { display: flex; flex-direction: column; gap: 8px; }
.payload-tabs { display: flex; gap: 0; border-bottom: 1px solid var(--border); }
.payload-tab { background: none; border: none; border-bottom: 2px solid transparent; padding: 6px 14px; font-size: 12px; color: var(--text2); cursor: pointer; font-family: inherit; }
.payload-tab:hover { color: var(--text); }
.payload-tab.active { color: var(--accent, #4f8cff); border-bottom-color: var(--accent, #4f8cff); }
.payload-panel { display: flex; flex-direction: column; gap: 8px; }
.form-textarea, .form-input {
  background: var(--bg3); border: 1px solid var(--border); border-radius: 6px;
  color: var(--text); font-size: 13px; padding: 6px 10px; font-family: monospace;
  width: 100%; box-sizing: border-box;
}
.form-textarea { resize: vertical; }
.form-textarea:focus, .form-input:focus { outline: 2px solid var(--accent, #4f8cff); outline-offset: 1px; border-color: var(--accent, #4f8cff); }
.dropzone { border: 2px dashed var(--border); border-radius: 8px; padding: 24px; text-align: center; color: var(--text2); display: flex; flex-direction: column; align-items: center; gap: 8px; }
.drop-filename { color: var(--text); font-weight: 500; }
.file-pick-btn { background: var(--bg4); border: 1px solid var(--border); border-radius: 6px; padding: 5px 14px; font-size: 12px; cursor: pointer; color: var(--text); }
.btn-sm { padding: 5px 12px; border-radius: 6px; font-size: 12px; border: 1px solid var(--border); background: var(--bg3); color: var(--text); cursor: pointer; align-self: flex-start; }
.btn-sm:disabled { opacity: 0.5; cursor: not-allowed; }
.file-row { display: flex; gap: 6px; }
.resolved-block { display: flex; flex-direction: column; gap: 6px; padding: 8px; background: var(--bg3); border: 1px solid var(--border); border-radius: 6px; }
.resolved-kind { font-size: 11px; color: var(--text2); text-transform: uppercase; }
.resolved-list { max-height: 140px; overflow-y: auto; display: flex; flex-direction: column; gap: 2px; }
.resolved-row { display: flex; justify-content: space-between; font-size: 12px; font-family: monospace; color: var(--text); }
.resolved-size { color: var(--text2); }
.dir-mode { display: flex; flex-direction: column; gap: 4px; padding-top: 4px; border-top: 1px solid var(--border); font-size: 12px; }
.mode-opt { display: flex; align-items: center; gap: 6px; cursor: pointer; }
.preview-block { display: flex; flex-direction: column; gap: 4px; padding-top: 8px; border-top: 1px solid var(--border); }
.preview-head { display: flex; justify-content: space-between; }
.preview-label { font-size: 11px; font-weight: 600; text-transform: uppercase; color: var(--text2); }
.preview-size { font-size: 11px; color: var(--text2); font-family: monospace; }
.preview-area { font-family: monospace; }
.error-msg { font-size: 12px; color: #ef4444; margin: 0; }
.lint-msg { font-size: 11px; color: #f5a623; margin: 0; }
.sr-only { position: absolute; width: 1px; height: 1px; padding: 0; margin: -1px; overflow: hidden; clip: rect(0,0,0,0); white-space: nowrap; border: 0; }
</style>
