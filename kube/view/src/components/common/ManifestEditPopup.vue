<script setup>
defineProps({
  open: { type: Boolean, default: false },
  loading: { type: Boolean, default: false },
  applying: { type: Boolean, default: false },
  editing: { type: Boolean, default: false },
  content: { type: String, default: '' },
  kind: { type: String, default: '' },
  name: { type: String, default: '' },
  namespace: { type: String, default: '' },
})

const emit = defineEmits(['update:content', 'update:editing', 'apply', 'close'])

function onInput(e) {
  emit('update:content', e.target.value)
}

function toggleEdit() {
  emit('update:editing', true)
}

function toggleView() {
  emit('update:editing', false)
}
</script>

<template>
  <div v-if="open" class="popup-overlay" @click.self="emit('close')">
    <div class="popup-panel manifest-popup" @click.stop>
      <div class="popup-header">
        <div class="popup-title">
          <span class="popup-kind">{{ kind }}</span>
          <span class="popup-name font-mono">{{ name }}</span>
          <span class="popup-ns font-mono" v-if="namespace">{{ namespace }}</span>
        </div>
        <div class="popup-actions">
          <button v-if="!editing" class="action-btn" @click="toggleEdit">✏️ Edit</button>
          <button v-else class="action-btn primary" @click="toggleView">📖 View</button>
          <button
            class="action-btn primary"
            :disabled="applying || !content.trim()"
            @click="emit('apply')"
          >
            {{ applying ? '⏳ Applying…' : '🚀 Redeploy' }}
          </button>
          <button class="action-btn close" @click="emit('close')">✕</button>
        </div>
      </div>
      <div class="manifest-body">
        <div v-if="loading" class="manifest-loading">
          <div class="spinner"></div>
          <span>Loading manifest…</span>
        </div>
        <textarea
          v-else-if="editing"
          class="manifest-editor font-mono"
          :value="content"
          @input="onInput"
          spellcheck="false"
        ></textarea>
        <pre v-else class="manifest-viewer font-mono">{{ content }}</pre>
      </div>
      <div class="popup-footer">
        <span class="hint">Edit toggles the editor. Changes are live after clicking Redeploy.</span>
      </div>
    </div>
  </div>
</template>

<style scoped>
.popup-overlay {
  position: fixed; top: 0; left: 0; right: 0; bottom: 0;
  background: rgba(0, 0, 0, 0.65); z-index: 1000;
  display: flex; align-items: center; justify-content: center;
  backdrop-filter: blur(4px);
  animation: fade-in 0.15s ease-out;
}
@keyframes fade-in { from { opacity: 0; } to { opacity: 1; } }

.popup-panel {
  background: #1a1b1e; border: 1px solid rgba(255, 255, 255, 0.1);
  border-radius: 12px; max-width: 800px; width: 90%;
  max-height: 85vh; display: flex; flex-direction: column;
  box-shadow: 0 20px 60px rgba(0, 0, 0, 0.5);
}
.popup-header {
  display: flex; align-items: center; justify-content: space-between;
  padding: 16px 20px; border-bottom: 1px solid rgba(255, 255, 255, 0.08);
  flex-shrink: 0;
}
.popup-title { display: flex; align-items: center; gap: 8px; font-size: 14px; }
.popup-kind { color: #a78bfa; font-weight: 600; }
.popup-name { color: #e8eaec; }
.popup-ns { color: #6b7078; }
.font-mono { font-family: var(--mono); }

.popup-actions { display: flex; align-items: center; gap: 8px; }
.action-btn {
  background: rgba(255, 255, 255, 0.06);
  border: 1px solid rgba(255, 255, 255, 0.1);
  color: #b0b4ba;
  padding: 4px 10px;
  border-radius: 4px;
  font-size: 11px;
  cursor: pointer;
  transition: all 0.2s;
  white-space: nowrap;
}
.action-btn:hover { background: rgba(255, 255, 255, 0.12); color: #e8eaec; }
.action-btn.primary { background: rgba(167, 139, 250, 0.15); border-color: rgba(167, 139, 250, 0.3); color: #a78bfa; }
.action-btn.primary:hover { background: rgba(167, 139, 250, 0.25); }
.action-btn.primary:disabled { opacity: 0.4; cursor: not-allowed; }
.action-btn.close { background: transparent; border: none; color: #6b7078; font-size: 16px; padding: 4px 8px; }
.action-btn.close:hover { color: #e8eaec; }

.manifest-loading { display: flex; align-items: center; gap: 12px; padding: 32px; color: #8b8f96; font-size: 13px; justify-content: center; }
.spinner {
  width: 18px; height: 18px;
  border: 2px solid rgba(167, 139, 250, 0.3);
  border-top-color: #a78bfa;
  border-radius: 50%;
  animation: spin 0.7s linear infinite;
  flex-shrink: 0;
}
@keyframes spin { to { transform: rotate(360deg); } }

.manifest-body { flex: 1; overflow: auto; min-height: 300px; }
.manifest-viewer {
  padding: 16px 20px; margin: 0; font-size: 12px; line-height: 1.6;
  color: #c9d1d9; white-space: pre; tab-size: 2;
  overflow: auto; max-height: 55vh;
}
.manifest-editor {
  width: 100%; min-height: 300px; height: 55vh;
  padding: 16px 20px; font-size: 12px; line-height: 1.6;
  background: #121314; color: #c9d1d9; border: none; outline: none;
  resize: vertical; tab-size: 2;
  font-family: var(--mono);
}
.manifest-editor:focus { background: #0d0e0f; }

.popup-footer {
  padding: 10px 20px; border-top: 1px solid rgba(255, 255, 255, 0.06);
  flex-shrink: 0;
}
.hint { font-size: 11px; color: #6b7078; }
</style>
