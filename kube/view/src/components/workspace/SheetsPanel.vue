<script setup>
// SheetsPanel — Phase 2 Google Sheets surface. Create + read range +
// inline-edit + write back. The read renders the matrix as an editable
// HTML table so the user can tweak a cell and write it back without
// leaving Argus. Range validation is intentionally loose — a permissive
// regex that catches obvious garbage; the Sheets API will reject the
// rest with a clearer error than we could.

import { computed, onMounted, ref, watch } from 'vue'
import { storeToRefs } from 'pinia'
import { useWorkspaceStore } from '../../stores/workspace'
import GoogleAccountHeader from './GoogleAccountHeader.vue'

const emit = defineEmits(['switch-tab'])

const store = useWorkspaceStore()
const { googleConnections, googleLoading, googleError, googleStatus } =
  storeToRefs(store)

const activeID = ref(null)
const createTitle = ref('')
const sheetID = ref('')
const range = ref('Sheet1!A1:C10')
const matrix = ref([])         // string[][]
const tabs = ref([])           // string[] from GetGoogleSheet
const created = ref(null)      // last-created Sheet for the link badge

// A1 notation: optional sheet name + ! + start + optional :end. Sheet
// names can include letters, digits, _ and spaces. The regex is loose by
// design — the backend gives precise errors.
const A1_RX = /^([\w ]+!)?[A-Z]+\d+(:[A-Z]+\d+)?$/

const rangeValid = computed(() => A1_RX.test(range.value.trim()))

const createDisabled = computed(
  () => googleLoading.value || !activeID.value || !createTitle.value.trim(),
)
const readDisabled = computed(
  () =>
    googleLoading.value ||
    !activeID.value ||
    !sheetID.value.trim() ||
    !rangeValid.value,
)
const writeDisabled = computed(
  () =>
    googleLoading.value ||
    !activeID.value ||
    !sheetID.value.trim() ||
    !rangeValid.value ||
    !matrix.value.length,
)

onMounted(async () => {
  await store.loadServices()
  if (!googleConnections.value.length) await store.loadConnections()
  if (!activeID.value && googleConnections.value.length) {
    activeID.value = googleConnections.value[0].id
  }
})

watch(googleConnections, (conns) => {
  if (!activeID.value && conns.length) activeID.value = conns[0].id
})

async function onCreate() {
  if (createDisabled.value) return
  try {
    const sheet = await store.createSheet(activeID.value, createTitle.value)
    created.value = sheet
    if (sheet?.id) sheetID.value = sheet.id
    tabs.value = sheet?.tabs || []
    createTitle.value = ''
  } catch { /* surfaced */ }
}

async function onRead() {
  if (readDisabled.value) return
  try {
    // Get sheet metadata too so we can display tab names. Cheap call.
    try {
      const meta = await store.getSheet(activeID.value, sheetID.value.trim())
      tabs.value = meta?.tabs || []
    } catch { /* metadata is nice-to-have */ }
    const rows = await store.readSheetRange(
      activeID.value, sheetID.value.trim(), range.value.trim(),
    )
    // Defensive clone so editing the local matrix doesn't mutate the
    // store's cached read.
    matrix.value = Array.isArray(rows)
      ? rows.map((r) => Array.isArray(r) ? [...r] : [])
      : []
  } catch { /* surfaced */ }
}

async function onWrite() {
  if (writeDisabled.value) return
  try {
    await store.writeSheetRange(
      activeID.value, sheetID.value.trim(), range.value.trim(), matrix.value,
    )
  } catch { /* surfaced */ }
}

// A1 column letter (0 -> A, 25 -> Z, 26 -> AA) for table headers.
function colLetter(i) {
  let n = i, s = ''
  do { s = String.fromCharCode(65 + (n % 26)) + s; n = Math.floor(n / 26) - 1 } while (n >= 0)
  return s
}

function setCell(r, c, v) {
  // Vue reactivity on nested arrays: replace the row reference so a
  // re-render fires reliably.
  const row = [...(matrix.value[r] || [])]
  row[c] = v
  const next = [...matrix.value]
  next[r] = row
  matrix.value = next
}
</script>

<template>
  <div class="sheets-panel">
    <GoogleAccountHeader
      v-model="activeID"
      @switch-tab="(t) => emit('switch-tab', t)"
    />

    <template v-if="googleConnections.length">
      <section class="card">
        <div class="card-head">Create a sheet</div>
        <div class="row">
          <input
            v-model="createTitle"
            class="text-in"
            placeholder="Sheet title"
            maxlength="500"
          />
          <span v-if="googleStatus?.op === 'sheet-created' && created?.url" class="ok-badge">
            ✓ Created — <a :href="created.url" target="_blank" rel="noopener">open</a>
          </span>
          <button
            class="btn-primary"
            :disabled="createDisabled"
            data-testid="sheets-create"
            @click="onCreate"
          >
            Create
          </button>
        </div>
      </section>

      <section class="card">
        <div class="card-head">Open &amp; read</div>
        <div class="row">
          <input
            v-model="sheetID"
            class="text-in mono"
            placeholder="Sheet ID"
            spellcheck="false"
          />
          <input
            v-model="range"
            class="text-in mono range-in"
            :class="{ invalid: !rangeValid }"
            placeholder="Sheet1!A1:C10"
            spellcheck="false"
            data-testid="sheets-range"
          />
          <button
            class="btn-ghost"
            :disabled="readDisabled"
            data-testid="sheets-read"
            @click="onRead"
          >
            Read
          </button>
        </div>
        <div v-if="!rangeValid" class="hint">
          Use A1 notation, e.g. <code>Sheet1!A1:C10</code> or <code>A1:B5</code>.
        </div>

        <div v-if="matrix.length" class="table-wrap">
          <table class="grid">
            <thead>
              <tr>
                <th v-for="(_, ci) in matrix[0]" :key="ci" scope="col">
                  {{ colLetter(ci) }}
                </th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="(row, ri) in matrix" :key="ri">
                <td v-for="(cell, ci) in row" :key="ci">
                  <input
                    :value="cell"
                    @input="(e) => setCell(ri, ci, e.target.value)"
                  />
                </td>
              </tr>
            </tbody>
          </table>
        </div>

        <div class="row foot">
          <span class="grow" />
          <span v-if="googleStatus?.op === 'sheet-written'" class="ok-badge">Saved ✓</span>
          <button
            class="btn-primary"
            :disabled="writeDisabled"
            data-testid="sheets-write"
            @click="onWrite"
          >
            Write current edits
          </button>
        </div>
      </section>

      <section v-if="tabs.length" class="card tabs-foot">
        <div class="card-head">Tabs</div>
        <div class="tab-chips">
          <span v-for="t in tabs" :key="t" class="chip">{{ t }}</span>
        </div>
      </section>

      <div v-if="googleError" class="err-banner">{{ googleError }}</div>
    </template>
  </div>
</template>

<style scoped>
.sheets-panel {
  padding: 18px 22px; overflow-y: auto; flex: 1; min-height: 0;
  display: flex; flex-direction: column; gap: 14px;
}

.card {
  display: flex; flex-direction: column; gap: 10px;
  padding: 12px 14px;
  background: var(--bg2);
  border: 1px solid var(--border);
  border-radius: 8px;
}
.card-head {
  font-size: 11px; color: var(--text3);
  text-transform: uppercase; letter-spacing: 0.5px;
}

.row { display: flex; align-items: center; gap: 10px; }
.row.foot { gap: 10px; }
.grow { flex: 1; }

.text-in {
  flex: 1;
  background: var(--bg3);
  border: 1px solid var(--border);
  border-radius: 6px;
  color: var(--text);
  padding: 7px 10px;
  font-size: 13px;
}
.text-in.mono { font-family: ui-monospace, monospace; font-size: 12px; }
.text-in.range-in { flex: 0 0 200px; }
.text-in.invalid { border-color: #dc5050; }
.text-in:focus { outline: none; border-color: var(--accent); }

.hint { font-size: 11.5px; color: var(--text3); }
.hint code {
  background: var(--bg3); padding: 1px 5px; border-radius: 3px;
  font-family: ui-monospace, monospace; font-size: 11px;
}

.table-wrap {
  overflow: auto;
  border: 1px solid var(--border);
  border-radius: 6px;
  background: var(--bg3);
  max-height: 360px;
}
.grid { border-collapse: collapse; width: 100%; }
.grid th { border: 1px solid var(--border); background: var(--bg3); color: var(--text2); font-size: 11px; font-weight: 600; padding: 3px 6px; min-width: 90px; text-align: center; }
.grid td { border: 1px solid var(--border); padding: 0; min-width: 90px; }
.grid input {
  width: 100%; box-sizing: border-box;
  background: transparent; border: 0;
  color: var(--text);
  padding: 4px 7px;
  font-family: ui-monospace, monospace; font-size: 12px;
}
.grid input:focus {
  outline: 2px solid var(--accent);
  outline-offset: -2px;
  background: var(--bg2);
}

.tab-chips { display: flex; gap: 6px; flex-wrap: wrap; }
.chip {
  font-size: 11.5px; color: var(--text2);
  background: var(--bg3);
  border: 1px solid var(--border);
  border-radius: 999px;
  padding: 2px 10px;
}

.ok-badge {
  font-size: 11.5px; font-weight: 600;
  color: #86efac;
  background: rgba(74,222,128,0.10);
  padding: 3px 8px; border-radius: 999px;
}
.ok-badge a { color: #86efac; text-decoration: underline; }

.btn-primary {
  padding: 7px 16px; border-radius: 6px;
  border: 1px solid var(--border2);
  background: var(--accent); color: white;
  font-size: 12.5px; font-weight: 500; cursor: pointer;
}
.btn-primary:hover:not(:disabled) { opacity: 0.88; }
.btn-primary:disabled { opacity: 0.5; cursor: not-allowed; }

.btn-ghost {
  padding: 6px 12px; border-radius: 6px;
  border: 1px solid var(--border2);
  background: transparent; color: var(--text2);
  font-size: 12px; cursor: pointer;
}
.btn-ghost:hover:not(:disabled) { background: var(--bg3); color: var(--text); }
.btn-ghost:disabled { opacity: 0.5; cursor: not-allowed; }

.err-banner {
  padding: 8px 12px;
  background: rgba(220,80,80,0.10);
  border: 1px solid rgba(220,80,80,0.35);
  border-radius: 6px;
  color: var(--text); font-size: 12px;
  font-family: ui-monospace, monospace;
}
</style>
