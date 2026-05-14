<script setup>
// Generic panel renderer — the shell uses this to mount a panel by id
// without knowing which feature provides it. Resolves the manifest's
// lazy component on first render so the chunk only loads when the
// panel is actually shown.
import { computed } from 'vue'
import { resolvePanel } from './registry'

const props = defineProps({
  id: { type: String, required: true },
})

const Component = computed(() => resolvePanel(props.id))
</script>

<template>
  <component :is="Component" v-if="Component" v-bind="$attrs" />
  <div v-else class="feature-missing">
    Unknown panel: {{ id }}
  </div>
</template>

<style scoped>
.feature-missing {
  padding: 12px;
  font-size: 12px;
  color: var(--text3);
  font-family: var(--mono, monospace);
}
</style>
