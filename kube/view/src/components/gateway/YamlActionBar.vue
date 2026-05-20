<script setup>
import { ref } from 'vue'
import { callGo } from '../../composables/useBridge'

// Shared Copy / Apply / Download bar for the gateway YAML outputs
// (IngressMigration, TrafficSplitter, future siblings). Centralizes
// three things that were duplicated or missing across components:
//
//   - Copy: clipboard, with a graceful fallback for environments
//     where navigator.clipboard isn't available.
//   - Apply: posts the YAML to the cluster via the existing
//     ApplyYaml App method. Shows inline status (idle / applying /
//     ok / error) instead of a toast so the user sees the result
//     next to the button that fired it.
//   - Download: triggers a normal browser download via an in-memory
//     blob URL. Filename defaults to <suggestedName>.yaml — caller
//     supplies the slug.

const props = defineProps({
  yaml: { type: String, required: true },
  // Suggested filename WITHOUT extension. The bar tacks ".yaml" on.
  suggestedName: { type: String, default: 'manifest' },
  // Disables Apply when the backend isn't reachable yet (e.g.
  // result hasn't been generated). Copy/Download stay enabled
  // because they only need the local string.
  canApply: { type: Boolean, default: true },
})

const copyState = ref('idle') // idle | copied
const applyState = ref('idle') // idle | applying | ok | error
const applyMessage = ref('')

async function onCopy() {
  try {
    if (navigator.clipboard?.writeText) {
      await navigator.clipboard.writeText(props.yaml)
    } else {
      const ta = document.createElement('textarea')
      ta.value = props.yaml
      document.body.appendChild(ta)
      ta.select()
      document.execCommand('copy')
      document.body.removeChild(ta)
    }
    copyState.value = 'copied'
    setTimeout(() => { copyState.value = 'idle' }, 1500)
  } catch (e) {
    // Best-effort — fall through silently. If even document.execCommand
    // fails the user can still right-click/select the pre block.
  }
}

async function onApply() {
  if (!props.yaml.trim()) return
  applyState.value = 'applying'
  applyMessage.value = ''
  try {
    await callGo('ApplyYaml', props.yaml)
    applyState.value = 'ok'
    applyMessage.value = 'Applied'
    // Reset after a beat so the next click reads as a fresh action.
    setTimeout(() => {
      if (applyState.value === 'ok') {
        applyState.value = 'idle'
        applyMessage.value = ''
      }
    }, 4000)
  } catch (e) {
    applyState.value = 'error'
    applyMessage.value = e?.message || String(e)
  }
}

function onDownload() {
  const blob = new Blob([props.yaml], { type: 'application/yaml;charset=utf-8' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = `${props.suggestedName || 'manifest'}.yaml`
  document.body.appendChild(a)
  a.click()
  document.body.removeChild(a)
  // Revoke after the click event has flushed — same trick the file-
  // save composables in the rest of the app use.
  setTimeout(() => URL.revokeObjectURL(url), 1000)
}
</script>

<template>
  <div class="yaml-action-bar">
    <button
      type="button"
      class="yaml-btn"
      :class="{ active: copyState === 'copied' }"
      :data-testid="`yaml-copy-${suggestedName}`"
      @click="onCopy"
    >{{ copyState === 'copied' ? 'Copied ✓' : 'Copy' }}</button>

    <button
      type="button"
      class="yaml-btn primary"
      :disabled="!canApply || !yaml.trim() || applyState === 'applying'"
      :data-testid="`yaml-apply-${suggestedName}`"
      @click="onApply"
    >{{ applyState === 'applying' ? 'Applying…' : 'Apply to cluster' }}</button>

    <button
      type="button"
      class="yaml-btn"
      :disabled="!yaml.trim()"
      :data-testid="`yaml-download-${suggestedName}`"
      @click="onDownload"
      :title="`Download as ${suggestedName}.yaml`"
    >Download</button>

    <span
      v-if="applyState === 'ok'"
      class="apply-status ok"
      role="status"
    >✓ {{ applyMessage }}</span>
    <span
      v-if="applyState === 'error'"
      class="apply-status err"
      role="alert"
    >✗ {{ applyMessage }}</span>
  </div>
</template>

<style scoped>
.yaml-action-bar {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
}
.yaml-btn {
  padding: 4px 12px;
  font-size: 11px;
  background: #2a2e35;
  border: 1px solid #3d424c;
  color: #fff;
  border-radius: 4px;
  cursor: pointer;
  font: inherit;
  font-size: 11px;
}
.yaml-btn:hover:not(:disabled) { background: #3a4049; }
.yaml-btn.active { background: #1d6f49; border-color: #1d6f49; }
.yaml-btn.primary {
  background: #6d4ade;
  border-color: #6d4ade;
}
.yaml-btn.primary:hover:not(:disabled) { background: #5a3bc7; border-color: #5a3bc7; }
.yaml-btn:disabled { opacity: 0.45; cursor: not-allowed; }

.apply-status {
  font-size: 11px;
  padding: 3px 8px;
  border-radius: 4px;
  word-break: break-word;
  max-width: 360px;
}
.apply-status.ok {
  background: rgba(62,207,142,0.14);
  color: #6ee7b7;
}
.apply-status.err {
  background: rgba(241,92,92,0.16);
  color: #fff;
  background-color: #b8392f;
}
</style>
