<script setup>
import { ref } from 'vue'
import { storeToRefs } from 'pinia'
import { useWorkspaceStore } from '../../stores/workspace'

const store = useWorkspaceStore()
const { icloudConnections, icloudNotes, icloudReminders } = storeToRefs(store)
const activeTab = ref('notes')
</script>

<template>
  <div class="panel icloud-panel" style="padding:12px">
    <h4>iCloud</h4>

    <div v-if="!icloudConnections.length" class="muted" style="margin-top:8px">
      Not connected. Go to the <strong>Connections</strong> tab and click Connect on the iCloud tile.
    </div>

    <div v-if="icloudConnections.length" class="card" style="padding:12px;border:1px solid var(--border);border-radius:6px;margin-top:8px">
      <div style="margin-bottom:12px">
        <strong>Connected as:</strong> {{ icloudConnections[0]?.display_name }}
      </div>

      <nav style="display:flex;gap:4px;margin-bottom:12px;border-bottom:1px solid var(--border)">
        <button v-for="t in ['notes', 'reminders', 'calendar']" :key="t"
          style="padding:6px 12px;border:none;background:none;cursor:pointer;font-size:0.9em"
          :style="activeTab === t ? {color:'var(--accent)',borderBottom:'2px solid var(--accent)',fontWeight:'600'} : {color:'var(--muted)'}"
          @click="activeTab = t">{{ t.charAt(0).toUpperCase() + t.slice(1) }}</button>
      </nav>

      <div v-if="activeTab === 'notes'">
        <p class="muted" style="font-size:0.9em">Notes via <code>memo</code> CLI on macOS. Install the apple-notes Hermes skill.</p>
      </div>
      <div v-if="activeTab === 'reminders'">
        <p class="muted" style="font-size:0.9em">Reminders via <code>remindctl</code> CLI on macOS. Install the apple-reminders Hermes skill.</p>
      </div>
      <div v-if="activeTab === 'calendar'">
        <p class="muted" style="font-size:0.9em">iCloud Calendar via CalDAV — in progress.</p>
      </div>
    </div>
  </div>
</template>

<style scoped>
.muted { color: var(--muted); }
</style>
