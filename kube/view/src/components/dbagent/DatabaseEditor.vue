<script setup>
// DatabaseEditor — modal form for create/edit. The password field is
// always blank on edit; leaving it blank means "keep the existing one"
// (the backend's UpsertDBConnection handles that fallback).

import { ref, computed, watch } from 'vue'
import { useDBAgent } from '../../composables/useDBAgent'

const props = defineProps({
  connection: { type: Object, default: null }, // null = create
})
const emit = defineEmits(['saved', 'cancel'])

const { upsert, error } = useDBAgent()

const DEFAULT_PORTS = {
  postgres: 5432, mysql: 3306, oracle: 1521, mssql: 1433, clickhouse: 9000,
}
const SSL_MODES = ['', 'disable', 'require', 'verify-ca', 'verify-full']

const form = ref(blankForm())
const submitting = ref(false)
const tagsText = ref('')

function blankForm() {
  return {
    id: '',
    name: '',
    db_type: 'postgres',
    host: '',
    port: 5432,
    user: '',
    password: '',
    db_name: '',
    ssl_mode: 'require',
    pool_size: 0,
    tags: [],
    enabled: true,
  }
}

watch(() => props.connection, c => {
  if (c) {
    form.value = { ...c, password: '' }
    tagsText.value = (c.tags || []).join(', ')
  } else {
    form.value = blankForm()
    tagsText.value = ''
  }
}, { immediate: true })

watch(() => form.value.db_type, t => {
  if (DEFAULT_PORTS[t]) form.value.port = DEFAULT_PORTS[t]
})

const isSqlite = computed(() => form.value.db_type === 'sqlite')

async function save() {
  form.value.tags = tagsText.value
    .split(',').map(s => s.trim()).filter(Boolean)
  submitting.value = true
  try {
    await upsert(form.value)
    emit('saved')
  } catch (e) {
    // error is surfaced from the composable; keep the modal open.
  } finally {
    submitting.value = false
  }
}
</script>

<template>
  <div class="modal-backdrop" @click.self="emit('cancel')">
    <div class="modal">
      <header><h3>{{ props.connection ? 'Edit database' : 'Add database' }}</h3></header>

      <div v-if="error" class="error">{{ error }}</div>

      <form @submit.prevent="save">
        <label>
          Name
          <input v-model="form.name" required placeholder="prod-pg-primary" />
        </label>

        <label>
          Type
          <select v-model="form.db_type">
            <option value="postgres">PostgreSQL</option>
            <option value="mysql">MySQL</option>
            <option value="oracle">Oracle</option>
            <option value="mssql">SQL Server</option>
            <option value="sqlite">SQLite</option>
            <option value="clickhouse">ClickHouse</option>
          </select>
        </label>

        <template v-if="!isSqlite">
          <div class="row">
            <label class="grow">
              Host
              <input v-model="form.host" required placeholder="db.internal" />
            </label>
            <label class="port">
              Port
              <input v-model.number="form.port" type="number" min="1" max="65535" />
            </label>
          </div>

          <label>
            Username
            <input v-model="form.user" autocomplete="off" />
          </label>

          <label>
            Password
            <input v-model="form.password" type="password" autocomplete="new-password"
                   :placeholder="props.connection?.has_password ? '(unchanged)' : ''" />
          </label>

          <label>
            Database name
            <input v-model="form.db_name" />
          </label>

          <label>
            SSL mode
            <select v-model="form.ssl_mode">
              <option v-for="m in SSL_MODES" :key="m" :value="m">{{ m || '(default)' }}</option>
            </select>
          </label>
        </template>

        <template v-else>
          <label>
            File path
            <input v-model="form.db_name" required placeholder="/var/lib/argus/app.db" />
          </label>
        </template>

        <label>
          Tags (comma-separated)
          <input v-model="tagsText" placeholder="prod, primary" />
        </label>

        <label class="row inline">
          <input v-model="form.enabled" type="checkbox" />
          <span>Enabled</span>
        </label>

        <footer>
          <button type="button" class="ghost" @click="emit('cancel')">Cancel</button>
          <button type="submit" class="primary" :disabled="submitting">
            {{ submitting ? 'Saving…' : 'Save' }}
          </button>
        </footer>
      </form>
    </div>
  </div>
</template>

<style scoped>
.modal-backdrop { position: fixed; inset: 0; background: rgba(0,0,0,0.4); display: flex; align-items: center; justify-content: center; z-index: 1000; }
.modal { background: white; padding: 1.5rem; border-radius: 8px; min-width: 480px; max-width: 600px; max-height: 90vh; overflow-y: auto; }
.modal header h3 { margin: 0 0 1rem 0; }
form > label { display: block; margin-bottom: 0.75rem; }
form > label > input, form > label > select { display: block; width: 100%; padding: 0.4rem 0.6rem; border: 1px solid #ccc; border-radius: 4px; margin-top: 0.2rem; }
.row { display: flex; gap: 0.5rem; }
.row.inline { align-items: center; }
.grow { flex: 1; }
.port { width: 100px; }
footer { display: flex; justify-content: flex-end; gap: 0.5rem; margin-top: 1rem; }
.primary { background: #2563eb; color: white; border: none; padding: 0.5rem 1rem; border-radius: 4px; cursor: pointer; }
.primary:disabled { opacity: 0.5; cursor: wait; }
.ghost { background: transparent; border: 1px solid #ccc; padding: 0.5rem 1rem; border-radius: 4px; cursor: pointer; }
.error { background: #fef2f2; color: #991b1b; padding: 0.5rem; border-radius: 4px; margin-bottom: 1rem; }
</style>
