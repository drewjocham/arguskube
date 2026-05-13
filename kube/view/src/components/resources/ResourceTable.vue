<script setup>
import { ref, computed, watch, onMounted } from 'vue'
import { useResources } from '../../composables/useWails'
import Select from '../common/Select.vue'

const props = defineProps({
  resourceKind: { type: String, required: true },
})

const emit = defineEmits(['select'])

const { result, namespaces, loading, error, listResources, listNamespaces } = useResources()

const selectedNs = ref('')
const search = ref('')
const sortKey = ref('name')
const sortAsc = ref(true)
const selectedName = ref(null)

// Fetch namespaces on mount.
onMounted(async () => {
  await listNamespaces()
  refresh()
})

// Refresh when resource kind or namespace changes.
watch([() => props.resourceKind, selectedNs], () => {
  selectedName.value = null
  refresh()
})

async function refresh(force = false) {
  await listResources(props.resourceKind, selectedNs.value, force)
}

// Filter + sort items.
const filteredItems = computed(() => {
  if (!result.value?.items) return []
  let items = result.value.items

  if (search.value) {
    const q = search.value.toLowerCase()
    items = items.filter(item =>
      item.name.toLowerCase().includes(q) ||
      item.namespace.toLowerCase().includes(q) ||
      item.status.toLowerCase().includes(q)
    )
  }

  items = [...items].sort((a, b) => {
    let va, vb
    if (sortKey.value === 'name') {
      va = a.name; vb = b.name
    } else if (sortKey.value === 'namespace') {
      va = a.namespace; vb = b.namespace
    } else if (sortKey.value === 'status') {
      va = a.status; vb = b.status
    } else if (sortKey.value === 'age') {
      va = a.age; vb = b.age
    } else {
      va = a.fields?.[sortKey.value] || ''
      vb = b.fields?.[sortKey.value] || ''
    }
    const cmp = String(va).localeCompare(String(vb), undefined, { numeric: true })
    return sortAsc.value ? cmp : -cmp
  })

  return items
})

const columns = computed(() => result.value?.schema?.columns || [])
const kindLabel = computed(() => result.value?.schema?.kind || props.resourceKind)

// Determine if this resource is namespace-scoped.
const isClusterScoped = computed(() =>
  ['nodes', 'namespaces', 'pvs', 'storageclasses'].includes(props.resourceKind)
)

function toggleSort(key) {
  if (sortKey.value === key) {
    sortAsc.value = !sortAsc.value
  } else {
    sortKey.value = key
    sortAsc.value = true
  }
}

function sortIndicator(key) {
  if (sortKey.value !== key) return ''
  return sortAsc.value ? '↑' : '↓'
}

function selectRow(item) {
  selectedName.value = item.name
  emit('select', { kind: props.resourceKind, namespace: item.namespace, name: item.name })
}

function statusDotColor(color) {
  const map = {
    green: 'var(--green)',
    red: 'var(--red)',
    amber: 'var(--amber)',
    blue: 'var(--accent)',
    gray: 'var(--text3)',
  }
  return map[color] || map.gray
}
</script>

<template>
  <div class="resource-table">
    <!-- Toolbar -->
    <div class="toolbar">
      <div class="toolbar-left">
        <span class="kind-label">{{ kindLabel }}</span>
        <span class="item-count">{{ filteredItems.length }} items</span>
      </div>
      <div class="toolbar-right">
        <Select v-if="!isClusterScoped" v-model="selectedNs" :options="[{value:'',label:'All namespaces'}, ...namespaces.map(ns => ({value: ns, label: ns}))]" size="sm" aria-label="Namespace" />
        <div class="search-box">
          <svg class="search-icon" width="12" height="12" viewBox="0 0 12 12">
            <circle cx="5" cy="5" r="3.5" stroke="currentColor" stroke-width="1.2" fill="none"/>
            <line x1="7.5" y1="7.5" x2="10.5" y2="10.5" stroke="currentColor" stroke-width="1.2" stroke-linecap="round"/>
          </svg>
          <input v-model="search" type="text" :placeholder="`Search ${kindLabel}...`" class="search-input" />
        </div>
        <button class="refresh-btn" @click="refresh(true)" :disabled="loading">
          <svg :class="{ spinning: loading }" width="13" height="13" viewBox="0 0 13 13">
            <path d="M11 6.5a4.5 4.5 0 11-1.5-3.4" stroke="currentColor" stroke-width="1.3" fill="none" stroke-linecap="round"/>
            <path d="M11 2v2h-2" stroke="currentColor" stroke-width="1.3" fill="none" stroke-linecap="round" stroke-linejoin="round"/>
          </svg>
        </button>
      </div>
    </div>

    <!-- Error state -->
    <div v-if="error" class="table-message error">{{ error }}</div>

    <!-- Loading state -->
    <div v-else-if="loading && !result" class="table-message">Loading…</div>

    <!-- Empty state -->
    <div v-else-if="filteredItems.length === 0" class="table-message">No {{ kindLabel }} found</div>

    <!-- Table -->
    <div v-else class="table-scroll">
      <table class="table">
        <thead>
          <tr>
            <th class="col-status"></th>
            <th class="col-name" @click="toggleSort('name')">
              Name <span class="sort-arrow">{{ sortIndicator('name') }}</span>
            </th>
            <th v-if="!isClusterScoped" class="col-ns" @click="toggleSort('namespace')">
              Namespace <span class="sort-arrow">{{ sortIndicator('namespace') }}</span>
            </th>
            <th
              v-for="col in columns"
              :key="col.key"
              class="col-field"
              @click="toggleSort(col.key)"
            >
              {{ col.header }} <span class="sort-arrow">{{ sortIndicator(col.key) }}</span>
            </th>
            <th class="col-status-text" @click="toggleSort('status')">
              Status <span class="sort-arrow">{{ sortIndicator('status') }}</span>
            </th>
            <th class="col-age" @click="toggleSort('age')">
              Age <span class="sort-arrow">{{ sortIndicator('age') }}</span>
            </th>
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="item in filteredItems"
            :key="item.name + item.namespace"
            :class="{ selected: selectedName === item.name }"
            @click="selectRow(item)"
          >
            <td class="col-status">
              <div class="status-dot" :style="{ background: statusDotColor(item.statusColor) }"></div>
            </td>
            <td class="col-name cell-name">{{ item.name }}</td>
            <td v-if="!isClusterScoped" class="col-ns cell-ns">{{ item.namespace }}</td>
            <td v-for="col in columns" :key="col.key" class="col-field cell-field">
              {{ item.fields?.[col.key] || '—' }}
            </td>
            <td class="col-status-text">
              <span class="status-pill" :class="'status-' + item.statusColor">{{ item.status }}</span>
            </td>
            <td class="col-age cell-age">{{ item.age }}</td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>

<style scoped>
.resource-table {
  /* flex: 1 + min-width: 0 lets this fill the row in
     .resource-layout. Without them ResourceTable sized to its
     content's natural width and looked "oddly stopped" — visible
     on storageclasses where the parent uses flex-row layout to
     leave room for an optional ResourceDetail panel. */
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  height: 100%;
  overflow: hidden;
}

/* Toolbar */
.toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 8px 14px;
  border-bottom: 1px solid var(--border);
  background: var(--bg2);
  flex-shrink: 0;
  gap: 12px;
}

.toolbar-left {
  display: flex;
  align-items: center;
  gap: 8px;
}

.kind-label {
  font-size: 13px;
  font-weight: 500;
  color: var(--text);
}

.item-count {
  font-size: 11px;
  color: var(--text3);
  font-family: var(--mono);
}

.toolbar-right {
  display: flex;
  align-items: center;
  gap: 8px;
}


.search-box {
  display: flex;
  align-items: center;
  gap: 5px;
  background: var(--bg3);
  border: 1px solid var(--border2);
  border-radius: 5px;
  padding: 4px 8px;
  transition: border-color 0.15s;
}
.search-box:focus-within { border-color: var(--accent); }
.search-icon { color: var(--text3); flex-shrink: 0; }

.search-input {
  background: none;
  border: none;
  color: var(--text);
  font-size: 11.5px;
  font-family: var(--font);
  outline: none;
  width: 140px;
}
.search-input::placeholder { color: var(--text3); }

.refresh-btn {
  background: var(--bg3);
  border: 1px solid var(--border2);
  border-radius: 5px;
  color: var(--text2);
  padding: 4px 6px;
  cursor: pointer;
  display: flex;
  align-items: center;
  transition: all 0.15s;
}
.refresh-btn:hover { background: var(--bg4); color: var(--text); }
.refresh-btn:disabled { opacity: 0.4; cursor: default; }
.spinning { animation: spin 0.8s linear infinite; }

/* Table */
.table-scroll {
  flex: 1;
  overflow: auto;
}

.table {
  width: 100%;
  border-collapse: collapse;
  font-size: 12px;
}

thead {
  position: sticky;
  top: 0;
  z-index: 1;
}

th {
  background: var(--bg3);
  color: var(--text3);
  font-weight: 500;
  font-size: 10.5px;
  text-transform: uppercase;
  letter-spacing: 0.04em;
  padding: 6px 10px;
  text-align: left;
  white-space: nowrap;
  cursor: pointer;
  user-select: none;
  border-bottom: 1px solid var(--border);
}
th:hover { color: var(--text2); }

.sort-arrow {
  font-size: 9px;
  color: var(--accent2);
  margin-left: 2px;
}

td {
  padding: 6px 10px;
  border-bottom: 1px solid var(--border);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  max-width: 200px;
}

tr {
  cursor: pointer;
  transition: background 0.1s;
}
tr:hover { background: var(--bg3); }
tr.selected { background: rgba(79,142,247,0.08); }

.col-status { width: 24px; text-align: center; }
.status-dot { width: 6px; height: 6px; border-radius: 50%; margin: 0 auto; }

.cell-name { color: var(--accent2); font-weight: 500; font-family: var(--mono); font-size: 11.5px; }
.cell-ns { color: var(--accent2); font-size: 11.5px; }
.cell-field { color: var(--text2); font-family: var(--mono); font-size: 11px; }
.cell-age { color: var(--text3); font-family: var(--mono); font-size: 11px; }

.status-pill {
  display: inline-block;
  padding: 1px 7px;
  border-radius: 10px;
  font-size: 10px;
  font-weight: 500;
  font-family: var(--mono);
}
.status-green { background: rgba(62,207,142,0.12); color: var(--green2); }
.status-red { background: rgba(240,84,84,0.12); color: var(--red2); }
.status-amber { background: rgba(245,166,35,0.12); color: var(--amber2); }
.status-blue { background: rgba(79,142,247,0.12); color: var(--accent2); }
.status-gray { background: var(--bg4); color: var(--text3); }

/* Messages */
.table-message {
  padding: 40px;
  text-align: center;
  color: var(--text3);
  font-size: 13px;
}
.table-message.error { color: var(--red2); }
</style>
