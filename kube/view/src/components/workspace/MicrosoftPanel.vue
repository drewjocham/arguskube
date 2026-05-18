<script setup>
import { computed, onMounted, ref } from 'vue'
import { storeToRefs } from 'pinia'
import { useWorkspaceStore } from '../../stores/workspace'

const store = useWorkspaceStore()
const { connections } = storeToRefs(store)

const msConns = computed(() => connections.value.filter((c) => c.service === 'microsoft'))
</script>

<template>
  <div class="panel" style="padding:12px">
    <h4>Microsoft 365</h4>
    <p class="muted" v-if="!msConns.length">
      Connect via the Connections tab. Setup guide:
      <a href="https://portal.azure.com/#view/Microsoft_AAD_RegisteredApps/ApplicationsListBlade" target="_blank">portal.azure.com → App registrations</a>.
      Redirect URI: <code>{{ window?.location?.origin || 'http://127.0.0.1:8080' }}/workspace/oauth/callback</code>.
      API permissions: Microsoft Graph → Calendars.ReadWrite, Files.ReadWrite, Tasks.ReadWrite, User.Read.
    </p>
    <div v-for="c in msConns" :key="c.id" class="card" style="margin:8px 0;padding:8px;border:1px solid var(--border);border-radius:6px">
      <strong>{{ c.display_name }}</strong>
      <span class="muted" v-if="c.email"> — {{ c.email }}</span>
    </div>
  </div>
</template>

<style scoped>
.muted { color: var(--muted); font-size: 0.9em; }
</style>
