<script setup>
// GoogleAccountHeader — shared header for Docs/Sheets/Tasks. The three
// panels all key off a single Google connection so we factor the picker +
// avatar + empty-state CTA out here. The host component owns the active
// connection id via v-model; this component just renders + emits.

import { computed } from 'vue'
import { storeToRefs } from 'pinia'
import { useWorkspaceStore } from '../../stores/workspace'
import Select from '../common/Select.vue'

const props = defineProps({
  modelValue: { type: [String, null], default: null },
})
const emit = defineEmits(['update:modelValue', 'switch-tab'])

const store = useWorkspaceStore()
const { googleConnections } = storeToRefs(store)

const connectionOptions = computed(() =>
  googleConnections.value.map((c) => ({
    value: c.id,
    label: c.display_name || c.email || '(unnamed account)',
  })),
)

const active = computed(
  () => googleConnections.value.find((c) => c.id === props.modelValue) || null,
)

function avatarLetter(name) {
  return (name || 'G').trim().charAt(0).toUpperCase() || 'G'
}

function pick(id) {
  emit('update:modelValue', id)
}
</script>

<template>
  <!-- Zero-connection empty state. Friendlier than hiding the tab; the CTA
       drops the user onto Connections so they can wire it up. -->
  <div v-if="!googleConnections.length" class="empty">
    <div class="empty-icon" aria-hidden="true">G</div>
    <h3>No Google account connected</h3>
    <p>Link Google from the Connections tab to use this panel.</p>
    <button class="btn-primary" @click="emit('switch-tab', 'connections')">
      Go to Connections
    </button>
  </div>

  <header v-else class="head">
    <div class="ws-info">
      <div class="ws-avatar" :title="active?.display_name || ''">
        <img v-if="active?.avatar_url" :src="active.avatar_url" alt="" />
        <span v-else>{{ avatarLetter(active?.display_name || active?.email) }}</span>
      </div>
      <div class="ws-meta">
        <div class="ws-name">{{ active?.display_name || 'Google' }}</div>
        <div v-if="active?.email" class="ws-id">{{ active.email }}</div>
      </div>
    </div>
    <div v-if="googleConnections.length > 1" class="ws-picker">
      <label for="google-account-select-wrap">Account</label>
      <div id="google-account-select-wrap">
        <Select
          :modelValue="modelValue"
          :options="connectionOptions"
          size="sm"
          width="220px"
          aria-label="Google account"
          @update:modelValue="pick"
        />
      </div>
    </div>
  </header>
</template>

<style scoped>
.empty {
  margin: 60px auto;
  max-width: 420px;
  text-align: center;
  background: var(--bg2);
  border: 1px solid var(--border);
  border-radius: 10px;
  padding: 32px;
}
.empty-icon {
  width: 48px; height: 48px;
  border-radius: 50%;
  background: #1a56db; color: white;
  display: flex; align-items: center; justify-content: center;
  font-size: 22px; font-weight: 700;
  margin: 0 auto 14px;
}
.empty h3 { margin: 0 0 6px; font-size: 15px; color: var(--text); }
.empty p { margin: 0 0 16px; font-size: 12.5px; color: var(--text2); }

.head {
  display: flex; align-items: center; justify-content: space-between; gap: 12px;
  padding: 12px 14px;
  background: var(--bg2);
  border: 1px solid var(--border);
  border-radius: 8px;
}
.ws-info { display: flex; align-items: center; gap: 10px; min-width: 0; }
.ws-avatar {
  width: 32px; height: 32px; border-radius: 50%;
  background: #1a56db; color: white;
  display: flex; align-items: center; justify-content: center;
  font-weight: 700; overflow: hidden; flex-shrink: 0;
}
.ws-avatar img { width: 100%; height: 100%; object-fit: cover; }
.ws-meta { min-width: 0; }
.ws-name { font-size: 14px; font-weight: 600; color: var(--text); }
.ws-id { font-size: 11px; color: var(--text3); }
.ws-picker { display: flex; align-items: center; gap: 8px; }
.ws-picker label { font-size: 11.5px; color: var(--text2); }

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
</style>
