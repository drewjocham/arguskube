<script setup>
import { ref, watch, onMounted, onBeforeUnmount, inject, computed } from 'vue'
import { marked } from 'marked'
import DOMPurify from 'dompurify'
import { useRunbooks } from '../../composables/useWails'
import { parseRunbookSegments } from '../../utils/parseRunbookSegments'
import RunbookCodeBlock from './RunbookCodeBlock.vue'
import ProGateOverlay from '../shared/ProGateOverlay.vue'

const isAllowed = inject('isAllowed')
const canAutomate = isAllowed('runbook_automation')
const canCustomize = isAllowed('custom_runbooks')
const { runbooks, loading, saving, listRunbooks, getRunbook, saveRunbook, deleteRunbook, createRunbook } = useRunbooks()

const selectedRunbook = ref(null)
const editorContent = ref('')
const editMode = ref(false)
const showCreateDialog = ref(false)
const newRunbookName = ref('')
const newRunbookTrigger = ref('')
const notification = ref(null)

function showNotification(msg, duration = 3000) {
  notification.value = msg
  setTimeout(() => notification.value = null, duration)
}

// Debounce auto-save.
let saveTimer = null

onMounted(() => {
  listRunbooks()
})

onBeforeUnmount(() => {
  if (saveTimer) clearTimeout(saveTimer)
})

async function selectRunbook(rb) {
  // Save current before switching.
  flushSave()
  selectedRunbook.value = rb
  editMode.value = false
  try {
    editorContent.value = await getRunbook(rb.id)
  } catch {
    editorContent.value = `# ${rb.name}\n\nFailed to load content.`
  }
}

function flushSave() {
  if (saveTimer) {
    clearTimeout(saveTimer)
    saveTimer = null
  }
  if (selectedRunbook.value && editMode.value) {
    saveRunbook(selectedRunbook.value.id, editorContent.value)
  }
}

function onEditorInput(e) {
  editorContent.value = e.target.value
  if (saveTimer) clearTimeout(saveTimer)
  saveTimer = setTimeout(() => {
    if (selectedRunbook.value) {
      saveRunbook(selectedRunbook.value.id, editorContent.value)
      listRunbooks() // refresh metadata (step count etc)
    }
  }, 800)
}

function toggleEdit() {
  editMode.value = !editMode.value
}

async function handleCreate() {
  if (!newRunbookName.value.trim()) return
  try {
    const rb = await createRunbook(newRunbookName.value.trim(), newRunbookTrigger.value.trim())
    showCreateDialog.value = false
    newRunbookName.value = ''
    newRunbookTrigger.value = ''
    if (rb) {
      await selectRunbook(rb)
      editMode.value = true
    }
  } catch (e) {
    showNotification('Failed to create runbook: ' + (e?.message || e))
  }
}

async function handleDelete() {
  if (!selectedRunbook.value) return
  if (!confirm(`Delete runbook "${selectedRunbook.value.name}"?`)) return
  await deleteRunbook(selectedRunbook.value.id)
  selectedRunbook.value = null
  editorContent.value = ''
  editMode.value = false
}

// Preview-mode segments. We split the runbook into text + code segments so
// each code block becomes an interactive <RunbookCodeBlock> with Copy / Run
// / per-section terminal routing. Text segments are still rendered as
// markdown HTML, sanitized via DOMPurify before injection.
const previewSegments = computed(() => parseRunbookSegments(editorContent.value))

const currentRunbookId = computed(() => selectedRunbook.value?.id || 'untitled')

function renderTextSegment(text) {
  if (!text) return ''
  // marked outputs block HTML; DOMPurify ensures no script / on* / javascript:
  // sneaks in via a runbook authored elsewhere.
  const html = marked.parse(text)
  return DOMPurify.sanitize(html, {
    ADD_ATTR: ['target'],
  })
}
</script>

<template>
  <div class="runbooks-layout">
    <div v-if="notification" class="rb-notification">{{ notification }}</div>
    <!-- Left: Runbook list -->
    <div class="rb-list-panel">
      <div class="view-header">
        <div>
          <div class="view-title">Runbooks</div>
          <div class="view-sub">Response playbooks for incidents</div>
        </div>
        <button class="rb-create-btn" @click="canCustomize ? showCreateDialog = true : null" :title="canCustomize ? 'New runbook' : 'Upgrade to Pro for custom runbooks'" :class="{ 'pro-disabled': !canCustomize }">
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <line x1="12" y1="5" x2="12" y2="19"></line><line x1="5" y1="12" x2="19" y2="12"></line>
          </svg>
        </button>
      </div>

      <!-- Create dialog -->
      <div v-if="showCreateDialog" class="create-dialog">
        <input v-model="newRunbookName" class="create-input" placeholder="Runbook name" @keydown.enter="handleCreate" />
        <input v-model="newRunbookTrigger" class="create-input" placeholder="Trigger (e.g. OOMKilled)" @keydown.enter="handleCreate" />
        <div class="create-actions">
          <button class="btn-sm btn-ghost" @click="showCreateDialog = false">Cancel</button>
          <button class="btn-sm btn-accent" @click="handleCreate">Create</button>
        </div>
      </div>

      <!-- Loading -->
      <div v-if="loading" class="empty-state">Loading...</div>

      <!-- Runbook cards -->
      <div v-else class="runbooks-grid">
        <div
          v-for="rb in runbooks"
          :key="rb.id"
          class="runbook-card"
          :class="{ selected: selectedRunbook?.id === rb.id }"
          @click="selectRunbook(rb)"
        >
          <div class="rb-status" :class="'rb-' + rb.status"></div>
          <div class="rb-body">
            <div class="rb-name">{{ rb.name }}</div>
            <div class="rb-trigger">Trigger: {{ rb.trigger || '—' }}</div>
            <div class="rb-meta">
              <span>{{ rb.steps }} steps</span>
              <span class="rb-dot">·</span>
              <span>{{ rb.status }}</span>
            </div>
          </div>
          <div v-if="rb.status === 'draft'" class="rb-badge">DRAFT</div>
        </div>

        <div v-if="runbooks.length === 0 && !loading" class="empty-state">
          No runbooks yet. Click + to create one.
        </div>
      </div>
    </div>

    <!-- Right: Editor / Preview -->
    <div class="rb-editor-panel">
      <template v-if="selectedRunbook">
        <div class="editor-toolbar">
          <div class="editor-title">{{ selectedRunbook.name }}</div>
          <div class="editor-actions">
            <button class="toolbar-btn" :class="{ active: editMode }" @click="toggleEdit">
              {{ editMode ? 'Preview' : 'Edit' }}
            </button>
            <button class="toolbar-btn danger" @click="handleDelete">Delete</button>
          </div>
        </div>

        <!-- Edit mode: raw markdown -->
        <textarea
          v-if="editMode"
          class="rb-textarea"
          :value="editorContent"
          @input="onEditorInput"
          spellcheck="false"
        ></textarea>

        <!-- Preview mode: walk parsed segments so code blocks become
             interactive RunbookCodeBlock components (Copy + Run + session
             routing) while prose stays as sanitized markdown. -->
        <div v-else class="rb-preview">
          <template v-for="(seg, i) in previewSegments" :key="i">
            <div
              v-if="seg.type === 'text'"
              class="rb-text"
              v-html="renderTextSegment(seg.text)"
            ></div>
            <RunbookCodeBlock
              v-else
              :code="seg.code"
              :language="seg.language"
              :section="seg.section"
              :section-id="seg.sectionId"
              :code-index="seg.codeIndex"
              :runbook-id="currentRunbookId"
            />
          </template>
        </div>
      </template>

      <div v-else class="empty-editor">
        <p>Select a runbook or create a new one</p>
      </div>
    </div>
  </div>
</template>

<style scoped>
.rb-notification {
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  padding: 8px 16px;
  background: rgba(239,68,68,0.12);
  border-bottom: 1px solid rgba(239,68,68,0.2);
  font-size: 12px;
  color: #f87171;
  text-align: center;
  z-index: 10;
}

.runbooks-layout {
  display: flex;
  height: 100%;
  overflow: hidden;
  position: relative;
}

/* Left panel */
.rb-list-panel {
  width: 320px;
  border-right: 1px solid var(--border);
  display: flex;
  flex-direction: column;
  overflow: hidden;
  flex-shrink: 0;
}

.view-header {
  padding: 12px 14px;
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  border-bottom: 1px solid var(--border);
  flex-shrink: 0;
}
.view-title { font-size: 14px; font-weight: 500; color: var(--text); margin-bottom: 2px; }
.view-sub { font-size: 12px; color: var(--text3); }

.rb-create-btn {
  background: var(--bg3); border: 1px solid var(--border); color: var(--text2);
  width: 28px; height: 28px; border-radius: 6px; cursor: pointer;
  display: flex; align-items: center; justify-content: center;
  transition: all 0.1s;
}
.rb-create-btn:hover { background: var(--bg4); color: var(--accent2); border-color: var(--accent); }

/* Create dialog */
.create-dialog {
  padding: 10px 14px;
  border-bottom: 1px solid var(--border);
  display: flex;
  flex-direction: column;
  gap: 6px;
  background: var(--bg2);
}
.create-input {
  background: var(--bg); border: 1px solid var(--border2); padding: 7px 10px;
  border-radius: 4px; color: var(--text); font-size: 12px; outline: none;
}
.create-input:focus { border-color: var(--accent); }
.create-actions { display: flex; justify-content: flex-end; gap: 6px; margin-top: 2px; }
.btn-sm { padding: 5px 12px; border-radius: 4px; font-size: 11px; font-weight: 500; cursor: pointer; border: none; }
.btn-ghost { background: transparent; color: var(--text2); }
.btn-ghost:hover { background: var(--bg3); }
.btn-accent { background: rgba(79,142,247,0.15); color: var(--accent2); }
.btn-accent:hover { background: rgba(79,142,247,0.25); }

/* Runbook list */
.runbooks-grid {
  flex: 1;
  overflow-y: auto;
  padding: 8px;
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.runbook-card {
  display: flex; gap: 10px; padding: 10px 12px; cursor: pointer;
  background: var(--bg3); border: 1px solid var(--border); border-radius: var(--r);
  transition: all 0.12s; align-items: flex-start; position: relative;
}
.runbook-card:hover { background: var(--bg4); border-color: var(--border2); }
.runbook-card.selected { background: rgba(79,142,247,0.08); border-color: rgba(79,142,247,0.3); }

.rb-status { width: 3px; border-radius: 2px; align-self: stretch; flex-shrink: 0; }
.rb-ready { background: var(--green); }
.rb-draft { background: var(--text3); }

.rb-body { flex: 1; min-width: 0; }
.rb-name { font-size: 13px; font-weight: 500; color: var(--text); margin-bottom: 3px; }
.rb-trigger { font-size: 11px; color: var(--text2); margin-bottom: 3px; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.rb-meta { font-size: 10.5px; color: var(--text3); display: flex; gap: 3px; }
.rb-dot { opacity: 0.5; }

.rb-badge {
  font-size: 9px; font-weight: 600; font-family: var(--mono);
  padding: 1px 5px; border-radius: 3px;
  background: var(--bg5); color: var(--text3); letter-spacing: 0.05em;
  flex-shrink: 0;
}

/* Right panel */
.rb-editor-panel {
  flex: 1;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.editor-toolbar {
  padding: 10px 16px;
  display: flex;
  justify-content: space-between;
  align-items: center;
  border-bottom: 1px solid var(--border);
  flex-shrink: 0;
}
.editor-title { font-size: 14px; font-weight: 500; color: var(--text); }
.editor-actions { display: flex; gap: 6px; }
.toolbar-btn {
  padding: 5px 12px; border-radius: 6px; font-size: 12px; font-weight: 500;
  cursor: pointer; border: 1px solid var(--border2); background: var(--bg3);
  color: var(--text2); transition: all 0.1s;
}
.toolbar-btn:hover { background: var(--bg4); color: var(--text); }
.toolbar-btn.active { background: rgba(79,142,247,0.15); color: var(--accent2); border-color: rgba(79,142,247,0.3); }
.toolbar-btn.danger { color: var(--text3); }
.toolbar-btn.danger:hover { background: rgba(239,68,68,0.1); color: #ef4444; border-color: rgba(239,68,68,0.3); }

/* Textarea editor */
.rb-textarea {
  flex: 1;
  resize: none;
  padding: 20px 24px;
  background: var(--bg);
  color: var(--text);
  font-family: var(--mono);
  font-size: 13px;
  line-height: 1.6;
  border: none;
  outline: none;
  overflow-y: auto;
}

/* Markdown preview */
.rb-preview {
  flex: 1;
  overflow-y: auto;
  padding: 20px 32px;
  color: var(--text);
  font-size: 14px;
  line-height: 1.7;
}
.rb-preview :deep(h1) { font-size: 1.6em; font-weight: 600; color: var(--text); margin: 1em 0 0.5em; border-bottom: 1px solid var(--border); padding-bottom: 0.3em; }
.rb-preview :deep(h2) { font-size: 1.3em; font-weight: 600; color: var(--text); margin: 1.2em 0 0.4em; }
.rb-preview :deep(h3) { font-size: 1.1em; font-weight: 600; color: var(--text); margin: 1em 0 0.3em; }
.rb-preview :deep(p) { margin-bottom: 0.8em; color: var(--text2); }
.rb-preview :deep(ul), .rb-preview :deep(ol) { padding-left: 2em; margin-bottom: 0.8em; color: var(--text2); }
.rb-preview :deep(li) { margin-bottom: 0.3em; }
.rb-preview :deep(code) { background: var(--bg3); padding: 0.15em 0.4em; border-radius: 3px; font-family: var(--mono); font-size: 0.9em; }
.rb-preview :deep(pre) { background: var(--bg2); padding: 12px 16px; border-radius: 6px; overflow-x: auto; border: 1px solid var(--border); margin-bottom: 1em; }
.rb-preview :deep(pre code) { background: none; padding: 0; font-size: 12px; }
.rb-preview :deep(blockquote) { border-left: 3px solid var(--accent); padding-left: 12px; color: var(--text3); margin: 0.8em 0; }
.rb-preview :deep(strong) { color: var(--text); font-weight: 600; }
.rb-preview :deep(em) { color: var(--text2); }
.rb-preview :deep(hr) { border: none; border-top: 1px solid var(--border); margin: 1.5em 0; }

/* Empty states */
.empty-state { text-align: center; padding: 32px; color: var(--text3); font-size: 13px; }
.empty-editor { flex: 1; display: flex; align-items: center; justify-content: center; color: var(--text3); font-size: 14px; }
.rb-create-btn.pro-disabled { opacity: 0.4; cursor: not-allowed; }
.rb-create-btn.pro-disabled:hover { background: transparent; }
</style>
