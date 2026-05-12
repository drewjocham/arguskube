<script setup>
defineProps({
  tool:    { type: String, required: true },
  probe:   { type: Object, default: null },     // { found, version, path, error }
  probing: { type: Boolean, default: false },
})
defineEmits(['test'])
</script>

<template>
  <div class="probe-row">
    <span class="probe-dot"
      :class="{
        found:    probe?.found,
        missing:  probe && !probe.found,
        unknown:  !probe,
      }"
    ></span>

    <div class="probe-text">
      <template v-if="probing">Probing <code>{{ tool }} --version</code>…</template>
      <template v-else-if="!probe">
        Click <strong>Detect</strong> to check if <code>{{ tool }}</code> is on PATH.
      </template>
      <template v-else-if="probe.found">
        <span class="probe-version">{{ probe.version || 'installed' }}</span>
        <span class="probe-path mono">{{ probe.path }}</span>
      </template>
      <template v-else>
        <span class="probe-missing">{{ probe.error || `${tool} not found on PATH` }}</span>
      </template>
    </div>

    <button class="probe-btn" :disabled="probing" @click="$emit('test', tool)">
      {{ probing ? '…' : (probe ? 'Re-test' : 'Detect') }}
    </button>
  </div>
</template>

<style scoped>
.probe-row {
  display: grid;
  grid-template-columns: 8px 1fr auto;
  gap: 8px;
  align-items: center;
  padding: 6px 0;
  border-top: 1px dashed rgba(255, 255, 255, 0.06);
  border-bottom: 1px dashed rgba(255, 255, 255, 0.06);
  font-size: 11.5px;
  color: var(--text2);
}
.probe-dot {
  width: 8px; height: 8px; border-radius: 50%; flex-shrink: 0;
  background: var(--text3);
}
.probe-dot.found   { background: #3ecf8e; box-shadow: 0 0 0 0 rgba(62,207,142,0); animation: probe-ok 2.5s ease-out infinite; }
.probe-dot.missing { background: #f5a623; }
.probe-dot.unknown { background: var(--text3); }
@keyframes probe-ok {
  0%   { box-shadow: 0 0 0 0 rgba(62,207,142,0.55); }
  70%  { box-shadow: 0 0 0 7px rgba(62,207,142,0); }
  100% { box-shadow: 0 0 0 0 rgba(62,207,142,0); }
}

.probe-text {
  display: flex; flex-wrap: wrap; gap: 6px; min-width: 0;
}
.probe-version { color: var(--text); font-weight: 500; }
.probe-path { color: var(--text3); font-size: 10.5px; opacity: 0.85; }
.probe-missing { color: #f5a623; }

.probe-btn {
  background: transparent; border: 1px solid var(--border);
  color: var(--text2); padding: 3px 9px; border-radius: 4px;
  font-size: 10.5px; cursor: pointer; transition: border-color 0.12s, color 0.12s;
}
.probe-btn:hover:not(:disabled) { border-color: var(--accent); color: var(--text); }
.probe-btn:disabled { opacity: 0.4; cursor: not-allowed; }
</style>
