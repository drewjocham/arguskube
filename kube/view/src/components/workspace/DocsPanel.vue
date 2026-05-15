<script setup>
// DocsPanel — Phase 2 Google Docs surface. Create + read + append. No
// full document editor here — we deliberately stay simple ("send text to
// a doc", "show the body") because the user always has docs.google.com
// for real editing. The link badge after a create makes the handoff easy.

import { computed, onMounted, ref, watch } from 'vue'
import { storeToRefs } from 'pinia'
import { useWorkspaceStore } from '../../stores/workspace'
import GoogleAccountHeader from './GoogleAccountHeader.vue'

const emit = defineEmits(['switch-tab'])

const store = useWorkspaceStore()
const { googleConnections, googleLoading, googleError, googleStatus } =
  storeToRefs(store)

const activeID = ref(null)
const title = ref('')
const body = ref('')
const docID = ref('')
const loadedTitle = ref('')
const loadedBody = ref('')
const appendText = ref('')
// Last 5 created/read this session. Local to the panel — keeps the
// store free of UX-only state.
const recent = ref([])

const MAX = 40000

const titleTooLong = computed(() => title.value.length > MAX)
const bodyTooLong = computed(() => body.value.length > MAX)
const appendTooLong = computed(() => appendText.value.length > MAX)

const createDisabled = computed(
  () =>
    googleLoading.value ||
    !activeID.value ||
    !title.value.trim() ||
    titleTooLong.value ||
    bodyTooLong.value,
)
const loadDisabled = computed(
  () => googleLoading.value || !activeID.value || !docID.value.trim(),
)
const appendDisabled = computed(
  () =>
    googleLoading.value ||
    !activeID.value ||
    !docID.value.trim() ||
    !appendText.value.trim() ||
    appendTooLong.value,
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

function pushRecent(doc) {
  if (!doc?.id) return
  const existing = recent.value.filter((d) => d.id !== doc.id)
  recent.value = [doc, ...existing].slice(0, 5)
}

async function onCreate() {
  if (createDisabled.value) return
  try {
    const doc = await store.createDoc(activeID.value, title.value, body.value)
    pushRecent(doc)
    title.value = ''
    body.value = ''
  } catch { /* surfaced via googleError */ }
}

async function onLoad() {
  if (loadDisabled.value) return
  try {
    const res = await store.readDoc(activeID.value, docID.value.trim())
    loadedTitle.value = res?.title || ''
    loadedBody.value = res?.body || ''
    pushRecent({ id: res?.id || docID.value.trim(), title: loadedTitle.value, url: '' })
  } catch { /* surfaced */ }
}

async function onAppend() {
  if (appendDisabled.value) return
  try {
    await store.appendDoc(activeID.value, docID.value.trim(), appendText.value)
    appendText.value = ''
  } catch { /* surfaced */ }
}
</script>

<template>
  <div class="docs-panel">
    <GoogleAccountHeader
      v-model="activeID"
      @switch-tab="(t) => emit('switch-tab', t)"
    />

    <template v-if="googleConnections.length">
      <section class="card">
        <div class="card-head">Create a doc</div>
        <input
          v-model="title"
          class="text-in"
          placeholder="Doc title"
          maxlength="500"
        />
        <textarea
          v-model="body"
          class="ta"
          rows="6"
          placeholder="Initial body (optional)"
        />
        <div class="row foot">
          <span class="char-count" :class="{ over: bodyTooLong }">
            {{ body.length.toLocaleString() }} / {{ MAX.toLocaleString() }}
          </span>
          <span class="grow" />
          <span v-if="googleStatus?.op === 'doc-created' && recent[0]?.url" class="ok-badge">
            ✓ Created —
            <a :href="recent[0].url" target="_blank" rel="noopener">open in Google Docs</a>
          </span>
          <button
            class="btn-primary"
            :disabled="createDisabled"
            data-testid="docs-create"
            @click="onCreate"
          >
            Create
          </button>
        </div>
      </section>

      <section class="card">
        <div class="card-head">Read &amp; append</div>
        <div class="row">
          <input
            v-model="docID"
            class="text-in mono"
            placeholder="Doc ID"
            spellcheck="false"
          />
          <button class="btn-ghost" :disabled="loadDisabled" @click="onLoad">
            Load
          </button>
        </div>
        <template v-if="loadedTitle || loadedBody">
          <div class="loaded-title">{{ loadedTitle }}</div>
          <textarea
            class="ta readonly"
            rows="8"
            readonly
            :value="loadedBody"
          />
        </template>
        <textarea
          v-model="appendText"
          class="ta"
          rows="4"
          placeholder="Text to append…"
        />
        <div class="row foot">
          <span class="char-count" :class="{ over: appendTooLong }">
            {{ appendText.length.toLocaleString() }} / {{ MAX.toLocaleString() }}
          </span>
          <span class="grow" />
          <span v-if="googleStatus?.op === 'doc-appended'" class="ok-badge">Appended ✓</span>
          <button
            class="btn-primary"
            :disabled="appendDisabled"
            @click="onAppend"
          >
            Append
          </button>
        </div>
      </section>

      <section v-if="recent.length" class="card recent">
        <div class="card-head">Recent (this session)</div>
        <ul>
          <li v-for="d in recent" :key="d.id">
            <span class="r-title">{{ d.title || '(untitled)' }}</span>
            <a v-if="d.url" :href="d.url" target="_blank" rel="noopener">open</a>
            <span v-else class="r-id">{{ d.id }}</span>
          </li>
        </ul>
      </section>

      <div v-if="googleError" class="err-banner">{{ googleError }}</div>
    </template>
  </div>
</template>

<style scoped>
.docs-panel {
  padding: 18px 22px;
  overflow-y: auto;
  flex: 1;
  min-height: 0;
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
  font-size: 11px;
  color: var(--text3);
  text-transform: uppercase;
  letter-spacing: 0.5px;
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
.text-in.mono { font-family: ui-monospace, SFMono-Regular, Menlo, monospace; font-size: 12px; }
.text-in:focus { outline: none; border-color: var(--accent); }

.ta {
  width: 100%;
  background: var(--bg3);
  border: 1px solid var(--border);
  border-radius: 6px;
  color: var(--text);
  padding: 8px 10px;
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 12.5px;
  line-height: 1.5;
  resize: vertical;
}
.ta:focus { outline: none; border-color: var(--accent); }
.ta.readonly { background: var(--bg2); color: var(--text2); }

.loaded-title { font-size: 13px; font-weight: 600; color: var(--text); }

.char-count { font-size: 11px; color: var(--text3); font-variant-numeric: tabular-nums; }
.char-count.over { color: #dc5050; font-weight: 600; }

.ok-badge {
  font-size: 11.5px; font-weight: 600;
  color: #86efac;
  background: rgba(74, 222, 128, 0.10);
  padding: 3px 8px; border-radius: 999px;
}
.ok-badge a { color: #86efac; text-decoration: underline; }

.btn-primary {
  padding: 7px 16px;
  border-radius: 6px;
  border: 1px solid var(--border2);
  background: var(--accent);
  color: white;
  font-size: 12.5px; font-weight: 500;
  cursor: pointer;
}
.btn-primary:hover:not(:disabled) { opacity: 0.88; }
.btn-primary:disabled { opacity: 0.5; cursor: not-allowed; }

.btn-ghost {
  padding: 6px 12px;
  border-radius: 6px;
  border: 1px solid var(--border2);
  background: transparent;
  color: var(--text2);
  font-size: 12px;
  cursor: pointer;
}
.btn-ghost:hover:not(:disabled) { background: var(--bg3); color: var(--text); }
.btn-ghost:disabled { opacity: 0.5; cursor: not-allowed; }

.recent ul { list-style: none; margin: 0; padding: 0; display: flex; flex-direction: column; gap: 4px; }
.recent li { display: flex; gap: 10px; align-items: baseline; font-size: 12px; }
.r-title { color: var(--text); flex: 1; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.r-id { font-family: ui-monospace, monospace; color: var(--text3); font-size: 11px; }
.recent a { color: var(--accent2, var(--accent)); }

.err-banner {
  padding: 8px 12px;
  background: rgba(220, 80, 80, 0.10);
  border: 1px solid rgba(220, 80, 80, 0.35);
  border-radius: 6px;
  color: var(--text);
  font-size: 12px;
  font-family: ui-monospace, monospace;
}
</style>
