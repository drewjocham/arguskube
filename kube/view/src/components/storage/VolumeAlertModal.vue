<script setup>
import { ref, computed, watch } from 'vue'
import { useVolumeAlertsStore } from '../../stores/volumeAlerts'

// Re-implement parseBytes locally so we don't drag VolumeList's helpers
// into the modal. Mirrors the same string forms ("100Gi", "512Mi", etc.).
function parseBytes(s) {
  if (!s || typeof s !== 'string') return 0
  const m = s.match(/^([\d.]+)\s*([KMGTP]i?B?|B)?$/i)
  if (!m) return 0
  const num = parseFloat(m[1])
  const unit = (m[2] || '').toUpperCase()
  const mult = {
    '': 1, B: 1,
    K: 1000, KB: 1000, KI: 1024, KIB: 1024,
    M: 1e6, MB: 1e6, MI: 1024 ** 2, MIB: 1024 ** 2,
    G: 1e9, GB: 1e9, GI: 1024 ** 3, GIB: 1024 ** 3,
    T: 1e12, TB: 1e12, TI: 1024 ** 4, TIB: 1024 ** 4,
    P: 1e15, PB: 1e15, PI: 1024 ** 5, PIB: 1024 ** 5,
  }[unit] || 1
  return Math.round(num * mult)
}
function fmtBytes(n) {
  if (!Number.isFinite(n) || n <= 0) return '0 B'
  const units = ['B', 'KiB', 'MiB', 'GiB', 'TiB', 'PiB']
  let i = 0
  let v = n
  while (v >= 1024 && i < units.length - 1) { v /= 1024; i++ }
  return `${v.toFixed(v < 10 ? 2 : 1)} ${units[i]}`
}

const props = defineProps({
  open: { type: Boolean, default: false },
  // Volume identity. scope = 'pv' | 'pvc'.
  scope: { type: String, required: true },
  namespace: { type: String, default: '' },
  name: { type: String, required: true },
  capacity: { type: String, default: '' },
})
const emit = defineEmits(['close'])

const alertsStore = useVolumeAlertsStore()
const existing = computed(() => alertsStore.get(props.scope, props.namespace, props.name))

const mode = ref('pct')          // 'pct' | 'abs'
const pctValue = ref(85)         // 0-100
const absValue = ref('')         // user input, parsed via parseBytes
const enabled = ref(true)
const error = ref('')

// Reset draft fields whenever the modal opens or the targeted volume changes.
watch(() => [props.open, props.name, props.scope, props.namespace], () => {
  if (!props.open) return
  error.value = ''
  const e = existing.value
  if (e) {
    mode.value = e.mode || 'pct'
    enabled.value = e.enabled !== false
    if (e.mode === 'abs') {
      absValue.value = fmtBytes(e.value)
      pctValue.value = 85
    } else {
      pctValue.value = e.value
      absValue.value = ''
    }
  } else {
    mode.value = 'pct'
    pctValue.value = 85
    absValue.value = ''
    enabled.value = true
  }
}, { immediate: true })

const capacityBytes = computed(() => parseBytes(props.capacity))

const validation = computed(() => {
  if (mode.value === 'pct') {
    const v = Number(pctValue.value)
    if (!Number.isFinite(v) || v <= 0 || v > 100) {
      return 'Choose a percentage between 1 and 100.'
    }
    return ''
  }
  const bytes = parseBytes(absValue.value)
  if (bytes <= 0) return 'Enter a value like 100Gi, 512Mi, or 1.5T.'
  if (capacityBytes.value && bytes >= capacityBytes.value) {
    return `Threshold must be lower than capacity (${props.capacity}).`
  }
  return ''
})

function save() {
  error.value = validation.value
  if (error.value) return
  const value = mode.value === 'pct' ? Number(pctValue.value) : parseBytes(absValue.value)
  alertsStore.set({
    scope: props.scope,
    namespace: props.namespace,
    name: props.name,
    capacity: props.capacity,
    mode: mode.value,
    value,
    enabled: enabled.value,
  })
  emit('close')
}

function clear() {
  alertsStore.remove(props.scope, props.namespace, props.name)
  emit('close')
}
</script>

<template>
  <div v-if="open" class="va-backdrop" @click.self="emit('close')">
    <div class="va-modal" role="dialog" aria-modal="true" aria-labelledby="va-title">
      <div class="va-header">
        <div id="va-title" class="va-title">
          {{ existing ? 'Edit alert' : 'Set alert' }}
          <span class="va-target mono">{{ name }}</span>
        </div>
        <button class="va-close" @click="emit('close')" aria-label="Close">×</button>
      </div>

      <div class="va-body">
        <p class="va-hint">
          Fire a notification when the volume's used capacity crosses the
          threshold below. Capacity: <strong class="mono">{{ capacity || '—' }}</strong>.
        </p>

        <div class="va-mode">
          <label class="va-mode-opt" :class="{ active: mode === 'pct' }">
            <input type="radio" value="pct" v-model="mode" />
            <span>Percentage</span>
          </label>
          <label class="va-mode-opt" :class="{ active: mode === 'abs' }">
            <input type="radio" value="abs" v-model="mode" />
            <span>Absolute size</span>
          </label>
        </div>

        <div v-if="mode === 'pct'" class="va-field">
          <label class="va-label">Used percentage threshold</label>
          <div class="va-pct-row">
            <input
              type="range"
              min="1"
              max="100"
              v-model.number="pctValue"
              class="va-slider"
            />
            <input
              type="number"
              min="1"
              max="100"
              v-model.number="pctValue"
              class="va-input va-pct-num mono"
            />
            <span class="va-suffix">%</span>
          </div>
          <div class="va-hint subtle">
            Fires the moment usage crosses this percentage.
          </div>
        </div>

        <div v-else class="va-field">
          <label class="va-label">Used capacity threshold</label>
          <input
            v-model="absValue"
            type="text"
            class="va-input mono"
            placeholder="e.g. 80Gi"
          />
          <div class="va-hint subtle">
            Accepts <code>Ki</code>, <code>Mi</code>, <code>Gi</code>,
            <code>Ti</code>. Must be less than the volume's capacity.
          </div>
        </div>

        <label class="va-toggle">
          <input type="checkbox" v-model="enabled" />
          <span>Alert is active</span>
        </label>

        <div v-if="error" class="va-error">{{ error }}</div>
      </div>

      <div class="va-footer">
        <button v-if="existing" class="btn danger" @click="clear">Remove alert</button>
        <div class="va-spacer"></div>
        <button class="btn" @click="emit('close')">Cancel</button>
        <button class="btn primary" @click="save">{{ existing ? 'Update' : 'Set alert' }}</button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.va-backdrop {
  position: fixed; inset: 0; z-index: 80;
  background: rgba(0,0,0,0.55);
  display: flex; align-items: center; justify-content: center; padding: 24px;
}
.va-modal {
  width: min(520px, 100%); background: var(--bg2);
  border: 1px solid var(--border); border-radius: 10px;
  display: flex; flex-direction: column; max-height: 90vh;
  box-shadow: 0 16px 40px rgba(0,0,0,0.45);
}
.va-header {
  display: flex; justify-content: space-between; align-items: center;
  padding: 14px 18px; border-bottom: 1px solid var(--border);
}
.va-title { font-size: 14px; font-weight: 600; color: var(--text); display: flex; gap: 8px; align-items: baseline; }
.va-target { color: var(--accent2); font-size: 13px; }
.va-close {
  background: transparent; border: 0; color: var(--text2);
  width: 24px; height: 24px; cursor: pointer; font-size: 18px; line-height: 1;
}
.va-close:hover { color: var(--text); }

.va-body { padding: 16px 18px; display: flex; flex-direction: column; gap: 14px; overflow: auto; }
.va-hint { font-size: 12.5px; color: var(--text2); margin: 0; line-height: 1.5; }
.va-hint.subtle { color: var(--text3); font-size: 11.5px; margin-top: 4px; }
.va-hint code, .va-hint strong { font-family: var(--mono); font-size: 11.5px; }

.va-mode { display: flex; gap: 8px; }
.va-mode-opt {
  flex: 1; display: flex; align-items: center; gap: 6px;
  padding: 8px 12px; border: 1px solid var(--border); border-radius: 6px;
  background: var(--bg); cursor: pointer; font-size: 12.5px; color: var(--text2);
}
.va-mode-opt.active { border-color: var(--accent); background: rgba(79,142,247,0.08); color: var(--text); }
.va-mode-opt input { accent-color: var(--accent); }

.va-field { display: flex; flex-direction: column; gap: 4px; }
.va-label { font-size: 12px; color: var(--text2); font-weight: 500; }
.va-input {
  background: var(--bg); border: 1px solid var(--border); color: var(--text);
  padding: 7px 10px; border-radius: 6px; font-size: 12.5px; outline: none;
}
.va-input:focus { border-color: var(--accent); }
.mono { font-family: var(--mono); }

.va-pct-row { display: flex; align-items: center; gap: 10px; }
.va-slider { flex: 1; accent-color: var(--accent); }
.va-pct-num { width: 70px; text-align: right; }
.va-suffix { color: var(--text3); font-size: 12px; }

.va-toggle { display: flex; gap: 6px; align-items: center; font-size: 12.5px; color: var(--text2); }
.va-toggle input { accent-color: var(--accent); }

.va-error {
  color: var(--red); font-size: 12px;
  padding: 6px 10px;
  background: rgba(217,72,72,0.08);
  border: 1px solid rgba(217,72,72,0.3); border-radius: 4px;
}

.va-footer {
  display: flex; gap: 8px; padding: 12px 18px;
  border-top: 1px solid var(--border);
}
.va-spacer { flex: 1; }
.btn {
  padding: 7px 14px; border-radius: 6px; border: 1px solid var(--border);
  background: var(--bg); color: var(--text2); font-size: 12px;
  cursor: pointer; transition: all 0.15s;
}
.btn:hover { background: var(--bg4); color: var(--text); }
.btn.primary { background: var(--accent); color: white; border-color: var(--accent); }
.btn.primary:hover { background: var(--accent2); }
.btn.danger { color: var(--red); border-color: var(--red); }
.btn.danger:hover { background: rgba(217,72,72,0.08); }
</style>
