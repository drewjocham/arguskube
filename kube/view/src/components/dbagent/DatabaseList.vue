<script setup>
// DatabaseList — the entry view of the DBAgent feature. Shows every
// registered DB with a status pill (enabled / disabled), and routes the
// user to the editor or the analysis view.
//
// Empty state nudges the user toward "Add database" so a first-time
// install isn't a blank page.

import { ref, onMounted, computed } from 'vue'
import { useDBAgent } from '../../composables/useDBAgent'
import DatabaseEditor from './DatabaseEditor.vue'
import DatabaseAnalysis from './DatabaseAnalysis.vue'

const { connections, loading, error, list, remove, testConn } = useDBAgent()

const editing = ref(null)         // null | 'new' | DBConnectionView
const analyzing = ref(null)       // null | DBConnectionView
const testResults = ref({})       // id -> { ok, message, latency_ms }
const search = ref('')

onMounted(() => list())

const filtered = computed(() => {
  const q = search.value.trim().toLowerCase()
  if (!q) return connections.value
  return connections.value.filter(c =>
    c.name.toLowerCase().includes(q) ||
    c.db_type.toLowerCase().includes(q) ||
    (c.host || '').toLowerCase().includes(q),
  )
})

async function runTest(c) {
  testResults.value[c.id] = { ok: false, message: 'testing…', latency_ms: 0 }
  testResults.value[c.id] = await testConn(c.id)
}

async function confirmDelete(c) {
  if (!window.confirm(`Remove "${c.name}"? Encrypted credentials will be deleted.`)) return
  await remove(c.id)
  delete testResults.value[c.id]
}

function onSaved() {
  editing.value = null
  list()
}
</script>

<template>
  <div class="dbagent-list">
    <header class="dbagent-header">
      <h2>Databases</h2>
      <div class="actions">
        <input v-model="search" placeholder="Search…" class="search" />
        <button class="primary" @click="editing = 'new'">+ Add database</button>
      </div>
    </header>

    <div v-if="error" class="error">{{ error }}</div>

    <div v-if="loading && !connections.length" class="placeholder">Loading…</div>

    <div v-else-if="!connections.length" class="empty">
      <p>No databases registered.</p>
      <p class="hint">
        Register a Postgres, MySQL, ClickHouse, or SQLite instance to start collecting
        capacity, query, and index analytics.
      </p>
    </div>

    <table v-else class="dbagent-table">
      <thead>
        <tr>
          <th>Name</th>
          <th>Type</th>
          <th>Host</th>
          <th>Status</th>
          <th>Test</th>
          <th></th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="c in filtered" :key="c.id">
          <td>
            <strong>{{ c.name }}</strong>
            <div v-if="c.tags?.length" class="tags">
              <span v-for="t in c.tags" :key="t" class="tag">{{ t }}</span>
            </div>
          </td>
          <td>{{ c.db_type }}</td>
          <td>{{ c.db_type === 'sqlite' ? c.db_name : `${c.host}:${c.port}` }}</td>
          <td>
            <span :class="['pill', c.enabled ? 'pill-on' : 'pill-off']">
              {{ c.enabled ? 'enabled' : 'disabled' }}
            </span>
          </td>
          <td>
            <button class="ghost" @click="runTest(c)">Test</button>
            <span v-if="testResults[c.id]" :class="['test-result', testResults[c.id].ok ? 'ok' : 'fail']">
              {{ testResults[c.id].ok
                ? `ok (${testResults[c.id].latency_ms} ms)`
                : testResults[c.id].message }}
            </span>
          </td>
          <td class="row-actions">
            <button class="ghost" @click="analyzing = c">Analyze</button>
            <button class="ghost" @click="editing = c">Edit</button>
            <button class="danger" @click="confirmDelete(c)">Delete</button>
          </td>
        </tr>
      </tbody>
    </table>

    <DatabaseEditor v-if="editing" :connection="editing === 'new' ? null : editing"
                    @saved="onSaved" @cancel="editing = null" />

    <DatabaseAnalysis v-if="analyzing" :connection="analyzing" @close="analyzing = null" />
  </div>
</template>

<style scoped>
.dbagent-list { padding: 1rem; }
.dbagent-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 1rem; }
.actions { display: flex; gap: 0.5rem; }
.search { padding: 0.4rem 0.6rem; border-radius: 4px; border: 1px solid #ccc; }
.primary { background: #2563eb; color: white; border: none; padding: 0.4rem 0.8rem; border-radius: 4px; cursor: pointer; }
.ghost { background: transparent; border: 1px solid #ccc; padding: 0.3rem 0.6rem; border-radius: 4px; cursor: pointer; }
.danger { background: transparent; border: 1px solid #dc2626; color: #dc2626; padding: 0.3rem 0.6rem; border-radius: 4px; cursor: pointer; }
.dbagent-table { width: 100%; border-collapse: collapse; }
.dbagent-table th, .dbagent-table td { padding: 0.5rem; text-align: left; border-bottom: 1px solid #eee; }
.tags { margin-top: 0.25rem; }
.tag { display: inline-block; background: #f1f5f9; padding: 0.1rem 0.4rem; border-radius: 8px; font-size: 0.75rem; margin-right: 0.25rem; }
.pill { padding: 0.15rem 0.5rem; border-radius: 10px; font-size: 0.75rem; }
.pill-on { background: #d1fae5; color: #065f46; }
.pill-off { background: #fee2e2; color: #991b1b; }
.test-result { font-size: 0.85rem; margin-left: 0.5rem; }
.test-result.ok { color: #047857; }
.test-result.fail { color: #b91c1c; }
.row-actions { display: flex; gap: 0.25rem; }
.empty { text-align: center; padding: 2rem; color: #666; }
.hint { font-size: 0.9rem; opacity: 0.75; }
.error { background: #fef2f2; color: #991b1b; padding: 0.5rem; border-radius: 4px; margin-bottom: 1rem; }
.placeholder { padding: 2rem; text-align: center; color: #888; }
</style>
