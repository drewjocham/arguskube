<script setup>
import { computed, ref, watch } from 'vue'
import Select from '../common/Select.vue'

// Endpoint configuration modal — two tabs:
//   1. Chained calls (depth 1 only in the UI; data model allows deeper)
//   2. Expected response (visual JSON picker + raw "match whole response")
//
// The visual picker is the centrepiece: load a sample JSON, click [+]
// on any node to add a gjson-pathed assertion. This avoids users having
// to learn gjson syntax cold.

const props = defineProps({
  open: { type: Boolean, default: false },
  endpoint: { type: Object, required: true },
})
const emit = defineEmits(['update:open', 'update:endpoint'])

const METHOD_OPTIONS = [
  { value: 'POST', label: 'POST' },
  { value: 'GET', label: 'GET' },
  { value: 'PUT', label: 'PUT' },
  { value: 'PATCH', label: 'PATCH' },
  { value: 'DELETE', label: 'DELETE' },
]
const KIND_OPTIONS = [
  { value: 'exists', label: 'exists' },
  { value: 'equals', label: 'equals' },
  { value: 'type', label: 'type' },
  { value: 'length', label: 'length' },
]
const TYPE_OPTIONS = [
  { value: 'string', label: 'string' }, { value: 'number', label: 'number' },
  { value: 'integer', label: 'integer' }, { value: 'boolean', label: 'boolean' },
  { value: 'array', label: 'array' }, { value: 'object', label: 'object' },
  { value: 'null', label: 'null' },
]

const tab = ref('chain') // 'chain' | 'expect'

function close() { emit('update:open', false) }
function patch(field, value) { emit('update:endpoint', { ...props.endpoint, [field]: value }) }

// ── Expect tab ───────────────────────────────────────────────────────
const expect = computed(() => props.endpoint.expect || { status: 0, bodyMatches: '', fieldChecks: [] })

// Mode: matchWhole (uses bodyMatches) vs pickFields (uses fieldChecks).
// Derive initial from the data: bodyMatches non-empty → matchWhole.
const expectMode = ref(expect.value.bodyMatches ? 'matchWhole' : 'pickFields')

function patchExpect(field, value) {
  const next = { ...(props.endpoint.expect || { status: 0, bodyMatches: '', fieldChecks: [] }), [field]: value }
  patch('expect', next)
}

// ── Sample JSON & tree ───────────────────────────────────────────────
const sampleRaw = ref('')
const sampleParsed = ref(null)
const sampleError = ref('')

function loadSample() {
  if (!sampleRaw.value.trim()) { sampleParsed.value = null; sampleError.value = ''; return }
  try {
    sampleParsed.value = JSON.parse(sampleRaw.value)
    sampleError.value = ''
  } catch (e) {
    sampleParsed.value = null
    sampleError.value = `Invalid JSON: ${e.message}`
  }
}
watch(sampleRaw, loadSample)

async function pasteFromClipboard() {
  try {
    const text = await navigator.clipboard.readText()
    sampleRaw.value = text
  } catch (e) { sampleError.value = `Clipboard read failed: ${e.message}` }
}
function onDropFile(e) {
  e.preventDefault()
  const f = e.dataTransfer?.files?.[0]
  if (!f) return
  const reader = new FileReader()
  reader.onload = () => { sampleRaw.value = String(reader.result || '') }
  reader.readAsText(f)
}

// ── Tree flatten — render as a depth-tagged list. gjson path notation.
function flattenNode(value, path, out) {
  if (Array.isArray(value)) {
    out.push({ path, kind: 'array', depth: path.split('.').filter(Boolean).length, length: value.length })
    value.forEach((v, i) => flattenNode(v, path ? `${path}.${i}` : String(i), out))
  } else if (value && typeof value === 'object') {
    out.push({ path, kind: 'object', depth: path.split('.').filter(Boolean).length, keys: Object.keys(value) })
    for (const k of Object.keys(value)) flattenNode(value[k], path ? `${path}.${k}` : k, out)
  } else {
    out.push({ path, kind: 'leaf', depth: path.split('.').filter(Boolean).length, value })
  }
}
const treeRows = computed(() => {
  if (sampleParsed.value == null) return []
  const out = []
  flattenNode(sampleParsed.value, '', out)
  return out
})

// ── Field checks state ───────────────────────────────────────────────
const flashKey = ref(-1) // index of the most-recently-added row, briefly highlighted
function addCheck(check) {
  const cur = (props.endpoint.expect?.fieldChecks) || []
  patchExpect('fieldChecks', [...cur, check])
  flashKey.value = cur.length
  setTimeout(() => { if (flashKey.value === cur.length) flashKey.value = -1 }, 600)
}
function clickLeaf(row) {
  // path "" means root leaf — that's unusual but supported via "@this".
  addCheck({ path: row.path || '@this', kind: 'equals', value: row.value })
}
function clickArray(row) {
  // "items.#" is gjson syntax for length-of-array.
  addCheck({ path: row.path ? `${row.path}.#` : '#', kind: 'length', value: row.length })
}
function clickObject(row) {
  addCheck({ path: row.path || '@this', kind: 'exists' })
}
function updateCheck(i, field, val) {
  const cur = (props.endpoint.expect?.fieldChecks || []).slice()
  cur[i] = { ...cur[i], [field]: val }
  patchExpect('fieldChecks', cur)
}
function removeCheck(i) {
  const cur = (props.endpoint.expect?.fieldChecks || []).slice()
  cur.splice(i, 1)
  patchExpect('fieldChecks', cur)
}

// Mode toggle. When switching to matchWhole, clear fieldChecks visually
// (the user can switch back to restore). When switching to pickFields,
// clear bodyMatches. Matches "disables the picker" requirement.
function setExpectMode(m) {
  expectMode.value = m
  if (m === 'matchWhole') {
    patchExpect('fieldChecks', [])
  } else {
    patchExpect('bodyMatches', '')
  }
}

// ── Match-whole linting ──────────────────────────────────────────────
const bodyMatchError = ref('')
function lintBodyMatches(v) {
  if (!v?.trim()) { bodyMatchError.value = ''; return }
  try { JSON.parse(v); bodyMatchError.value = '' }
  catch (e) { bodyMatchError.value = `Invalid JSON: ${e.message}` }
}

// ── Chain tab ────────────────────────────────────────────────────────
const chain = computed(() => props.endpoint.chain || [])
function setChain(arr) { patch('chain', arr) }
function addChain() {
  setChain([...chain.value, { name: '', method: 'POST', url: '', headers: {}, body: '', expect: null, chain: [] }])
}
function removeChain(i) {
  const next = chain.value.slice(); next.splice(i, 1); setChain(next)
}
function patchChain(i, field, value) {
  const next = chain.value.map((c, idx) => idx === i ? { ...c, [field]: value } : c)
  setChain(next)
}
</script>

<template>
  <div v-if="open" class="modal-backdrop" @click.self="close">
    <div class="modal" role="dialog" aria-modal="true" data-testid="endpoint-config-modal">
      <header class="modal-head">
        <h3>Configure endpoint</h3>
        <button class="btn-x" type="button" @click="close" aria-label="Close">×</button>
      </header>

      <nav class="tabs">
        <button type="button" class="tab" :class="{ active: tab === 'chain' }" data-testid="modal-tab-chain"
          @click="tab = 'chain'">Chained calls</button>
        <button type="button" class="tab" :class="{ active: tab === 'expect' }" data-testid="modal-tab-expect"
          @click="tab = 'expect'">Expected response</button>
      </nav>

      <!-- Chain tab -->
      <section v-if="tab === 'chain'" class="modal-body">
        <p class="muted">Calls that run after this step succeeds. Nested chains (depth &gt; 1) can be authored manually via spec.json — out of scope here.</p>
        <div v-for="(c, i) in chain" :key="i" class="card">
          <div class="card-head">
            <strong>↳ {{ c.method || 'POST' }} {{ c.url || '(no url)' }}</strong>
            <button type="button" class="btn-row btn-del" @click="removeChain(i)" aria-label="Remove chained call">×</button>
          </div>
          <div class="card-body">
            <div class="row-2">
              <div>
                <label class="label">Name</label>
                <input class="input" :value="c.name || ''" @input="patchChain(i, 'name', $event.target.value)" />
              </div>
              <div>
                <label class="label">Method</label>
                <Select :modelValue="c.method || 'POST'" :options="METHOD_OPTIONS" width="100%"
                  @update:modelValue="patchChain(i, 'method', $event)" />
              </div>
            </div>
            <div class="row">
              <label class="label">URL</label>
              <input class="input" :value="c.url || ''" @input="patchChain(i, 'url', $event.target.value)" />
            </div>
            <div class="row">
              <label class="label">Body</label>
              <textarea class="input mono" rows="2" :value="c.body || ''"
                @input="patchChain(i, 'body', $event.target.value)"></textarea>
            </div>
          </div>
        </div>
        <button type="button" class="btn-add" data-testid="modal-add-chain" @click="addChain">+ Add chained call</button>
      </section>

      <!-- Expect tab -->
      <section v-else class="modal-body">
        <div class="row-2">
          <div>
            <label class="label">HTTP status (blank = any 2xx)</label>
            <input class="input" type="number" min="0" :value="expect.status || ''" data-testid="modal-status"
              @input="patchExpect('status', $event.target.value ? Number($event.target.value) : 0)" />
          </div>
          <div>
            <label class="label">Assertion style</label>
            <div class="seg">
              <button type="button" class="seg-btn" :class="{ active: expectMode === 'matchWhole' }"
                data-testid="modal-mode-match" @click="setExpectMode('matchWhole')">Match whole response</button>
              <button type="button" class="seg-btn" :class="{ active: expectMode === 'pickFields' }"
                data-testid="modal-mode-pick" @click="setExpectMode('pickFields')">Pick fields</button>
            </div>
          </div>
        </div>

        <!-- Match whole response -->
        <template v-if="expectMode === 'matchWhole'">
          <div class="row">
            <label class="label">Expected JSON body</label>
            <textarea class="input mono" rows="10" :value="expect.bodyMatches || ''" data-testid="modal-bodymatch"
              @input="patchExpect('bodyMatches', $event.target.value); lintBodyMatches($event.target.value)"
              @blur="lintBodyMatches($event.target.value)"></textarea>
            <div v-if="bodyMatchError" class="err">{{ bodyMatchError }}</div>
          </div>
        </template>

        <!-- Pick fields -->
        <template v-else>
          <div class="sample-pane">
            <div class="sample-head">
              <strong>Sample response</strong>
              <button type="button" class="btn-row" data-testid="modal-paste-clipboard" @click="pasteFromClipboard">
                Paste from clipboard
              </button>
            </div>
            <textarea class="input mono" rows="5" placeholder='{ "items": [...], "meta": {...} }'
              :value="sampleRaw" data-testid="modal-sample-input"
              @input="sampleRaw = $event.target.value"
              @dragover.prevent @drop="onDropFile"></textarea>
            <div v-if="sampleError" class="err">{{ sampleError }}</div>
          </div>

          <div v-if="treeRows.length" class="tree">
            <div v-for="(row, i) in treeRows" :key="i" class="tree-row" :style="{ paddingLeft: (row.depth * 14 + 4) + 'px' }">
              <span class="tree-path">{{ row.path || '<root>' }}</span>
              <template v-if="row.kind === 'object'">
                <span class="tag">object</span>
                <button class="add-btn" :data-testid="`tree-add-${i}`" @click="clickObject(row)" title="Assert exists">[+]</button>
              </template>
              <template v-else-if="row.kind === 'array'">
                <span class="tag">array ({{ row.length }})</span>
                <button class="add-btn" :data-testid="`tree-add-${i}`" @click="clickArray(row)" title="Assert length">[ ]</button>
              </template>
              <template v-else>
                <span class="tag leaf">{{ JSON.stringify(row.value) }}</span>
                <button class="add-btn" :data-testid="`tree-add-${i}`" @click="clickLeaf(row)" title="Assert equals">[+]</button>
              </template>
            </div>
          </div>

          <!-- Assertions list -->
          <div class="assertions">
            <h4>Field assertions ({{ (expect.fieldChecks || []).length }})</h4>
            <div v-if="!expect.fieldChecks?.length" class="muted">Click <code>[+]</code> on any node above to add an assertion.</div>
            <div v-for="(fc, i) in expect.fieldChecks || []" :key="i" class="assert-row" :class="{ flash: flashKey === i }">
              <code class="assert-path">{{ fc.path }}</code>
              <Select :modelValue="fc.kind" :options="KIND_OPTIONS" width="110px"
                @update:modelValue="updateCheck(i, 'kind', $event)" />
              <Select v-if="fc.kind === 'type'" :modelValue="fc.value || 'string'" :options="TYPE_OPTIONS"
                width="120px" @update:modelValue="updateCheck(i, 'value', $event)" />
              <input v-else-if="fc.kind !== 'exists'" class="input assert-val" :value="fc.value ?? ''"
                @input="updateCheck(i, 'value', $event.target.value)" />
              <span v-else class="muted">(no value)</span>
              <button type="button" class="btn-row btn-del" @click="removeCheck(i)" aria-label="Remove">×</button>
            </div>
          </div>
        </template>
      </section>

      <footer class="modal-foot">
        <button type="button" class="btn-primary" @click="close">Done</button>
      </footer>
    </div>
  </div>
</template>

<style scoped>
.modal-backdrop { position: fixed; inset: 0; background: rgba(0,0,0,0.55); display: flex; align-items: center; justify-content: center; z-index: 1000; }
.modal { background: var(--bg2); border: 1px solid var(--border); border-radius: 10px; width: min(720px, 95vw); max-height: 85vh; display: flex; flex-direction: column; }
.modal-head { display: flex; align-items: center; justify-content: space-between; padding: 10px 14px; border-bottom: 1px solid var(--border); }
.modal-head h3 { margin: 0; font-size: 14px; color: var(--text); }
.btn-x { background: none; border: none; font-size: 20px; color: var(--text2); cursor: pointer; line-height: 1; }
.tabs { display: flex; gap: 0; border-bottom: 1px solid var(--border); }
.tab { background: var(--bg3); border: none; padding: 8px 14px; font-size: 12px; color: var(--text2); cursor: pointer; font-family: inherit; border-right: 1px solid var(--border); }
.tab.active { background: var(--bg2); color: var(--text); border-bottom: 2px solid var(--accent); }
.modal-body { padding: 12px 14px; overflow-y: auto; flex: 1; display: flex; flex-direction: column; gap: 10px; }
.modal-foot { padding: 10px 14px; border-top: 1px solid var(--border); display: flex; justify-content: flex-end; }
.btn-primary { background: var(--accent); color: white; border: none; padding: 6px 16px; border-radius: 6px; font-size: 13px; font-weight: 600; cursor: pointer; }
.row { display: flex; flex-direction: column; gap: 4px; }
.row-2 { display: grid; grid-template-columns: 1fr 1fr; gap: 10px; }
.label { font-size: 11px; color: var(--text2); }
.input { background: var(--bg3); border: 1px solid var(--border); border-radius: 6px; padding: 6px 10px; font-size: 13px; color: var(--text); width: 100%; box-sizing: border-box; font-family: inherit; }
.input.mono { font-family: ui-monospace, SFMono-Regular, Menlo, monospace; font-size: 12px; }
.muted { color: var(--text3); font-size: 12px; }
.err { color: #d05858; font-size: 11px; }
.seg { display: inline-flex; border: 1px solid var(--border); border-radius: 8px; overflow: hidden; }
.seg-btn { background: var(--bg3); border: none; padding: 6px 12px; font-size: 12px; color: var(--text2); cursor: pointer; font-family: inherit; }
.seg-btn.active { background: var(--accent); color: #fff; }
.card { background: var(--bg3); border: 1px solid var(--border); border-radius: 6px; }
.card-head { display: flex; align-items: center; justify-content: space-between; padding: 6px 10px; font-size: 12px; }
.card-body { padding: 8px 10px; display: flex; flex-direction: column; gap: 8px; border-top: 1px solid var(--border); }
.btn-row { background: var(--bg3); border: 1px solid var(--border); border-radius: 6px; padding: 4px 10px; font-size: 12px; cursor: pointer; color: var(--text2); }
.btn-row:hover { background: var(--bg4); color: var(--text); }
.btn-del { color: #ef4444; border-color: rgba(239,68,68,0.3); }
.btn-add { background: var(--bg3); border: 1px dashed var(--border); border-radius: 6px; padding: 6px 10px; color: var(--text2); cursor: pointer; font-size: 12px; align-self: flex-start; }
.sample-pane { display: flex; flex-direction: column; gap: 6px; }
.sample-head { display: flex; align-items: center; justify-content: space-between; }
.tree { background: var(--bg3); border: 1px solid var(--border); border-radius: 6px; padding: 6px; max-height: 220px; overflow: auto; font-family: ui-monospace, SFMono-Regular, Menlo, monospace; font-size: 12px; }
.tree-row { display: flex; align-items: center; gap: 8px; padding: 2px 0; }
.tree-path { color: var(--text2); }
.tag { color: var(--accent2); font-size: 11px; }
.tag.leaf { color: var(--text); }
.add-btn { background: none; border: none; color: var(--accent); cursor: pointer; font-family: inherit; font-size: 11px; padding: 0 4px; }
.add-btn:hover { text-decoration: underline; }
.assertions h4 { margin: 8px 0 4px; font-size: 12px; color: var(--text); }
.assert-row { display: grid; grid-template-columns: 1fr 110px 1fr auto; gap: 6px; align-items: center; padding: 4px 0; transition: background-color 0.4s; }
.assert-row.flash { background: rgba(79,142,247,0.18); border-radius: 4px; }
.assert-path { font-family: ui-monospace, SFMono-Regular, Menlo, monospace; font-size: 11px; color: var(--text); background: var(--bg3); padding: 2px 6px; border-radius: 4px; }
.assert-val { font-size: 12px; }
</style>
