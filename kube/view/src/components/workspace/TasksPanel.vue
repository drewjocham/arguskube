<script setup>
// TasksPanel — Phase 2 Google Tasks surface. List → filter → toggle/edit/
// delete. Inline title editing on click→blur because the user's mental
// model is "click the title to change it" (matches the Tasks web UI).

import { computed, onMounted, ref, watch } from 'vue'
import { storeToRefs } from 'pinia'
import { useWorkspaceStore } from '../../stores/workspace'
import { invalidateCache } from '../../composables/useBridge'
import GoogleAccountHeader from './GoogleAccountHeader.vue'
import Select from '../common/Select.vue'

const emit = defineEmits(['switch-tab'])

const store = useWorkspaceStore()
const { googleConnections, taskLists, tasks, googleLoading, googleError, googleStatus } =
  storeToRefs(store)

const activeID = ref(null)
const listID = ref(null)
const filter = ref('active')  // 'all' | 'active' | 'completed'

const addOpen = ref(false)
const addTitle = ref('')
const addDue = ref('')
const addNotes = ref('')

// Inline edit state — taskID currently being renamed.
const editingID = ref(null)
const editingText = ref('')

const listOptions = computed(() => {
  const arr = (activeID.value && taskLists.value[activeID.value]) || []
  return arr.map((l) => ({ value: l.id, label: l.title }))
})

const currentTasks = computed(() => {
  if (!activeID.value || !listID.value) return []
  return tasks.value[`${activeID.value}:${listID.value}`] || []
})

const filteredTasks = computed(() => {
  const list = currentTasks.value
  if (filter.value === 'active') return list.filter((t) => t.status !== 'completed')
  if (filter.value === 'completed') return list.filter((t) => t.status === 'completed')
  return list
})

const addDisabled = computed(
  () => googleLoading.value || !activeID.value || !listID.value || !addTitle.value.trim(),
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

// Whenever the connection flips, reload lists and clear the selected
// list (the IDs are per-account).
watch(
  activeID,
  async (id) => {
    listID.value = null
    if (!id) return
    const lists = await store.loadTaskLists(id)
    if (lists.length) listID.value = lists[0].id
  },
  { immediate: true },
)

// Loading tasks when list changes. Cached so quick filter flips don't
// hammer the API.
watch(listID, async (id) => {
  if (!activeID.value || !id) return
  await store.loadTasks(activeID.value, id)
})

async function refreshLists() {
  if (!activeID.value) return
  invalidateCache('ListGoogleTaskLists')
  await store.loadTaskLists(activeID.value)
}

async function toggleStatus(t) {
  const nextStatus = t.status === 'completed' ? 'needsAction' : 'completed'
  try {
    await store.updateTask(activeID.value, listID.value, t.id, { status: nextStatus })
  } catch { /* surfaced */ }
}

function startEdit(t) {
  editingID.value = t.id
  editingText.value = t.title
}

async function commitEdit(t) {
  const trimmed = editingText.value.trim()
  const id = editingID.value
  editingID.value = null
  if (!id || !trimmed || trimmed === t.title) return
  try {
    await store.updateTask(activeID.value, listID.value, id, { title: trimmed })
  } catch { /* surfaced */ }
}

async function removeTask(t) {
  try {
    await store.deleteTask(activeID.value, listID.value, t.id)
  } catch { /* surfaced */ }
}

async function submitAdd() {
  if (addDisabled.value) return
  const patch = {
    title: addTitle.value.trim(),
    notes: addNotes.value.trim() || undefined,
    due: addDue.value || undefined,
    status: 'needsAction',
  }
  try {
    await store.createTask(activeID.value, listID.value, patch)
    addTitle.value = ''
    addNotes.value = ''
    addDue.value = ''
    addOpen.value = false
  } catch { /* surfaced */ }
}
</script>

<template>
  <div class="tasks-panel">
    <GoogleAccountHeader
      v-model="activeID"
      @switch-tab="(t) => emit('switch-tab', t)"
    />

    <template v-if="googleConnections.length">
      <section class="card">
        <div class="row">
          <label>Task list</label>
          <Select
            v-model="listID"
            :options="listOptions"
            :disabled="googleLoading || !listOptions.length"
            :placeholder="googleLoading ? 'Loading…' : 'Pick a list'"
            width="260px"
            aria-label="Task list"
          />
          <button class="btn-ghost" :disabled="googleLoading" @click="refreshLists">
            Refresh lists
          </button>
        </div>

        <div class="filters">
          <button
            v-for="f in ['all', 'active', 'completed']"
            :key="f"
            class="pill"
            :class="{ active: filter === f }"
            :data-testid="`tasks-filter-${f}`"
            @click="filter = f"
          >{{ f }}</button>
        </div>

        <ul class="tasks">
          <li
            v-for="t in filteredTasks"
            :key="t.id"
            class="task"
            :class="{ done: t.status === 'completed' }"
          >
            <input
              type="checkbox"
              :checked="t.status === 'completed'"
              :data-testid="`tasks-toggle-${t.id}`"
              @change="toggleStatus(t)"
            />
            <input
              v-if="editingID === t.id"
              v-model="editingText"
              class="edit-in"
              autofocus
              @keyup.enter="commitEdit(t)"
              @keyup.esc="editingID = null"
              @blur="commitEdit(t)"
            />
            <span v-else class="title" @click="startEdit(t)">{{ t.title || '(untitled)' }}</span>
            <span v-if="t.due" class="due">{{ t.due }}</span>
            <button class="x" title="Delete" @click="removeTask(t)">×</button>
          </li>
          <li v-if="!filteredTasks.length" class="empty-row">
            No tasks {{ filter !== 'all' ? `(${filter})` : '' }}.
          </li>
        </ul>

        <div class="add-area">
          <button
            v-if="!addOpen"
            class="btn-ghost add-btn"
            :disabled="!listID"
            @click="addOpen = true"
          >+ Add task</button>
          <div v-else class="add-form">
            <input
              v-model="addTitle"
              class="text-in"
              placeholder="Task title"
              autofocus
              @keyup.enter="submitAdd"
              @keyup.esc="addOpen = false"
            />
            <input
              v-model="addDue"
              class="text-in"
              type="date"
              :placeholder="'Due'"
              style="flex: 0 0 150px"
            />
            <textarea
              v-model="addNotes"
              class="ta"
              rows="2"
              placeholder="Notes (optional)"
            />
            <div class="row foot">
              <span class="grow" />
              <button class="btn-ghost" @click="addOpen = false">Cancel</button>
              <button
                class="btn-primary"
                :disabled="addDisabled"
                @click="submitAdd"
              >Add</button>
            </div>
          </div>
        </div>

        <div v-if="googleStatus?.op?.startsWith('task-')" class="ok-badge">
          {{ googleStatus.op.replace('task-', '') }} ✓
        </div>
      </section>

      <div v-if="googleError" class="err-banner">{{ googleError }}</div>
    </template>
  </div>
</template>

<style scoped>
.tasks-panel {
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

.row { display: flex; align-items: center; gap: 10px; }
.row.foot { gap: 8px; }
.row label { font-size: 12px; color: var(--text2); min-width: 64px; }
.grow { flex: 1; }

.filters { display: flex; gap: 6px; }
.pill {
  padding: 3px 12px;
  border-radius: 999px;
  border: 1px solid var(--border);
  background: transparent;
  color: var(--text2);
  font-size: 11.5px;
  text-transform: capitalize;
  cursor: pointer;
}
.pill:hover { background: var(--bg3); color: var(--text); }
.pill.active {
  background: rgba(79,142,247,0.14);
  color: var(--accent2, var(--accent));
  border-color: var(--accent);
}

.tasks { list-style: none; margin: 0; padding: 0; display: flex; flex-direction: column; gap: 2px; }
.task {
  display: flex; align-items: center; gap: 10px;
  padding: 6px 8px;
  border-radius: 6px;
  font-size: 13px; color: var(--text);
}
.task:hover { background: var(--bg3); }
.task.done .title { color: var(--text3); text-decoration: line-through; }
.title { flex: 1; cursor: text; }
.due { font-size: 11px; color: var(--text3); font-variant-numeric: tabular-nums; }
.x {
  border: 0; background: transparent; color: var(--text3);
  font-size: 16px; cursor: pointer; padding: 0 4px;
}
.x:hover { color: #dc5050; }
.edit-in {
  flex: 1;
  background: var(--bg3);
  border: 1px solid var(--accent);
  border-radius: 4px;
  color: var(--text);
  padding: 3px 7px;
  font-size: 13px;
}
.empty-row { padding: 12px 4px; color: var(--text3); font-size: 12px; font-style: italic; }

.add-area { margin-top: 4px; }
.add-btn { width: 100%; padding: 6px; }
.add-form { display: flex; flex-direction: column; gap: 8px; }
.add-form .row { display: flex; align-items: center; gap: 8px; }

.text-in {
  flex: 1;
  background: var(--bg3);
  border: 1px solid var(--border);
  border-radius: 6px;
  color: var(--text);
  padding: 6px 10px;
  font-size: 13px;
}
.text-in:focus { outline: none; border-color: var(--accent); }
.ta {
  background: var(--bg3);
  border: 1px solid var(--border);
  border-radius: 6px;
  color: var(--text);
  padding: 6px 10px;
  font-size: 12.5px;
  resize: vertical;
}
.ta:focus { outline: none; border-color: var(--accent); }

.btn-primary {
  padding: 6px 14px; border-radius: 6px;
  border: 1px solid var(--border2);
  background: var(--accent); color: white;
  font-size: 12.5px; cursor: pointer;
}
.btn-primary:hover:not(:disabled) { opacity: 0.88; }
.btn-primary:disabled { opacity: 0.5; cursor: not-allowed; }

.btn-ghost {
  padding: 5px 12px; border-radius: 6px;
  border: 1px solid var(--border2);
  background: transparent; color: var(--text2);
  font-size: 12px; cursor: pointer;
}
.btn-ghost:hover:not(:disabled) { background: var(--bg3); color: var(--text); }
.btn-ghost:disabled { opacity: 0.5; cursor: not-allowed; }

.ok-badge {
  align-self: flex-start;
  font-size: 11.5px; font-weight: 600;
  color: #4ade80;
  background: rgba(74,222,128,0.10);
  padding: 3px 8px; border-radius: 999px;
}

.add-form .row .text-in[type="date"] { color: var(--text2); }

.err-banner {
  padding: 8px 12px;
  background: rgba(220,80,80,0.10);
  border: 1px solid rgba(220,80,80,0.35);
  border-radius: 6px;
  color: var(--text); font-size: 12px;
  font-family: ui-monospace, monospace;
}
</style>
