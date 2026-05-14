<script setup>
// DatabaseAnalysis — shows the raw analyzer output as a tabbed JSON
// viewer. The Vue layer is intentionally dumb; the Python agent is
// what renders the prose recommendation. For Phase 3 we just want a
// "I can see the metrics" surface so a user can verify the connection
// is producing useful numbers.

import { ref, onMounted, watch } from 'vue'
import { useDBAgent } from '../../composables/useDBAgent'

const props = defineProps({
  connection: { type: Object, required: true },
})
const emit = defineEmits(['close'])

const SECTIONS = ['overview', 'resources', 'connections', 'indexes', 'queries', 'replication']

const { analyze, error } = useDBAgent()
const active = ref('overview')
const data = ref(null)
const loading = ref(false)

async function load() {
  loading.value = true
  data.value = null
  try {
    data.value = await analyze(props.connection.id, active.value)
  } catch (e) {
    // error surfaces via composable.
  } finally {
    loading.value = false
  }
}

onMounted(load)
watch(active, load)
</script>

<template>
  <div class="modal-backdrop" @click.self="emit('close')">
    <div class="modal wide">
      <header>
        <h3>Analyze: {{ props.connection.name }}</h3>
        <button class="ghost" @click="emit('close')">Close</button>
      </header>

      <nav class="tabs">
        <button v-for="s in SECTIONS" :key="s"
                :class="{ active: active === s }"
                @click="active = s">
          {{ s }}
        </button>
      </nav>

      <div v-if="error" class="error">{{ error }}</div>

      <div v-if="loading" class="placeholder">Running…</div>

      <pre v-else-if="data" class="json">{{ JSON.stringify(data.data, null, 2) }}</pre>

      <div v-else class="placeholder">No data.</div>
    </div>
  </div>
</template>

<style scoped>
.modal-backdrop { position: fixed; inset: 0; background: rgba(0,0,0,0.4); display: flex; align-items: center; justify-content: center; z-index: 1000; }
.modal.wide { background: white; padding: 1.5rem; border-radius: 8px; width: 80vw; max-width: 1000px; max-height: 85vh; display: flex; flex-direction: column; }
.modal header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 1rem; }
.tabs { display: flex; gap: 0.25rem; border-bottom: 1px solid #ddd; margin-bottom: 1rem; }
.tabs button { background: transparent; border: none; padding: 0.5rem 0.8rem; cursor: pointer; border-bottom: 2px solid transparent; }
.tabs button.active { border-bottom-color: #2563eb; color: #2563eb; font-weight: 600; }
.json { flex: 1; background: #0f172a; color: #e2e8f0; padding: 1rem; border-radius: 4px; overflow: auto; font-size: 0.85rem; }
.ghost { background: transparent; border: 1px solid #ccc; padding: 0.3rem 0.8rem; border-radius: 4px; cursor: pointer; }
.error { background: #fef2f2; color: #991b1b; padding: 0.5rem; border-radius: 4px; margin-bottom: 1rem; }
.placeholder { padding: 2rem; text-align: center; color: #888; }
</style>
