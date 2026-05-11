<script setup>
import { ref, computed, watch } from 'vue'
import {
  parseSecretRef,
  formatSecretRef,
  isResolvable,
  describeSecretRef,
  secretRefIsValid,
  SECRET_REF_KINDS,
  SECRET_REF_META,
} from '../../lib/secretRef'

// SecretRefInput — a single text input that doubles as a "where does this
// value come from?" picker. The user can either type a value directly
// (treated as inline) or pick a source from the dropdown to switch the
// input to "kind:value[#key]" form. The component emits the canonical
// formatted string back to the parent, plus a parsed object for
// convenience.
//
// Props:
//   modelValue   — string, the canonical "kind:value[#key]" form
//   placeholder  — text input placeholder
//   label        — optional <label> text rendered above the input
//   showKey      — show the optional "#key" subfield (default: auto, on
//                  for kinds where #key is meaningful)
//   compact      — render in a single tight row (used in tables)
//   passwordLike — mask the value as a password (only when kind=inline)
//
// Emits:
//   update:modelValue (string)  — formatted ref string
//   resolved-change   (parsed)  — the parsed { kind, value, key } object
//
// The component is intentionally dumb about where the resolver lives:
// resolving an aws-secret/vault/file reference happens server-side, so
// this component never tries to fetch real values. It just owns the
// editing surface.

const props = defineProps({
  modelValue: { type: String, default: '' },
  placeholder: { type: String, default: '' },
  label: { type: String, default: '' },
  showKey: { type: String, default: 'auto' }, // 'auto' | 'always' | 'never'
  compact: { type: Boolean, default: false },
  passwordLike: { type: Boolean, default: false },
  disabled: { type: Boolean, default: false },
})

const emit = defineEmits(['update:modelValue', 'resolved-change'])

// Internal editable state. We parse the incoming modelValue once and then
// own the three independent fields (kind, value, key) so the user can
// switch the source dropdown without losing the value they already typed.
const parsed = ref(parseSecretRef(props.modelValue))

// Re-parse if the parent replaces modelValue (loading from a config, undo).
watch(() => props.modelValue, (next) => {
  const reparsed = parseSecretRef(next)
  // Only adopt if it actually changed — otherwise we'd echo our own emits.
  const cur = parsed.value
  if (reparsed.kind !== cur.kind || reparsed.value !== cur.value || reparsed.key !== cur.key) {
    parsed.value = reparsed
  }
})

// When any field changes, re-format and emit.
function emitChange() {
  const next = formatSecretRef(parsed.value)
  emit('update:modelValue', next)
  emit('resolved-change', { ...parsed.value })
}

function setKind(k) {
  parsed.value = { ...parsed.value, kind: k }
  // Switching to inline drops the key (it's never meaningful inline).
  if (k === 'inline') parsed.value.key = ''
  emitChange()
}
function setValue(v) {
  parsed.value = { ...parsed.value, value: v }
  emitChange()
}
function setKey(k) {
  parsed.value = { ...parsed.value, key: k }
  emitChange()
}

const wantsKey = computed(() => {
  if (props.showKey === 'always') return true
  if (props.showKey === 'never') return false
  // auto: show #key for kinds that can hold structured values
  return ['aws-secret', 'gcp-secret', 'vault', 'azure-vault'].includes(parsed.value.kind)
})

const inputType = computed(() => {
  // Only mask when the value is being typed inline AND the parent asked.
  // For source labels, masking the path "vault:gh-pat" is silly — those
  // strings aren't secret in themselves.
  if (props.passwordLike && parsed.value.kind === 'inline') return 'password'
  return 'text'
})

const sourceMeta = computed(() => SECRET_REF_META[parsed.value.kind] || SECRET_REF_META.inline)
const sourceDescription = computed(() => describeSecretRef(parsed.value))
const valid = computed(() => secretRefIsValid(parsed.value))
const sourcedFromBackend = computed(() => isResolvable(parsed.value))

defineExpose({ parsed, valid, sourceDescription })
</script>

<template>
  <div class="secret-ref-input" :class="{ compact, invalid: !valid && parsed.kind !== 'inline' }">
    <label v-if="label" class="srf-label">{{ label }}</label>
    <div class="srf-row">
      <!-- Source picker. Single button + dropdown so the input stays the
           visually dominant element (instead of one button per kind). -->
      <select
        class="srf-kind"
        :value="parsed.kind"
        :disabled="disabled"
        :title="sourceMeta.hint"
        :data-kind="parsed.kind"
        @change="setKind($event.target.value)"
      >
        <option v-for="k in SECRET_REF_KINDS" :key="k" :value="k">
          {{ SECRET_REF_META[k].label }}
        </option>
      </select>

      <input
        class="srf-value mono"
        :type="inputType"
        :value="parsed.value"
        :placeholder="placeholder || (parsed.kind === 'inline' ? 'value' : 'reference')"
        :disabled="disabled"
        :data-kind="parsed.kind"
        autocomplete="off"
        spellcheck="false"
        @input="setValue($event.target.value)"
      />

      <input
        v-if="wantsKey"
        class="srf-key mono"
        type="text"
        :value="parsed.key"
        placeholder="#key (optional)"
        :disabled="disabled"
        autocomplete="off"
        spellcheck="false"
        @input="setKey($event.target.value)"
      />
    </div>

    <div class="srf-foot">
      <span class="srf-source-pill" :data-kind="parsed.kind" :title="sourceMeta.hint">
        {{ sourceDescription }}
      </span>
      <span v-if="sourcedFromBackend" class="srf-foot-hint">
        Resolved server-side at use time. The value is never persisted in plain text.
      </span>
      <span v-if="!valid && parsed.kind !== 'inline'" class="srf-error">
        Invalid {{ sourceMeta.label }} reference.
      </span>
    </div>
  </div>
</template>

<style scoped>
.secret-ref-input {
  display: flex;
  flex-direction: column;
  gap: 4px;
}
.secret-ref-input.compact { gap: 2px; }

.srf-label {
  font-size: 11px;
  color: var(--text2);
  text-transform: uppercase;
  letter-spacing: 0.05em;
  font-weight: 500;
}

.srf-row {
  display: flex;
  gap: 4px;
  align-items: stretch;
}

.srf-kind {
  background: var(--bg3);
  border: 1px solid var(--border2);
  color: var(--text2);
  padding: 5px 22px 5px 8px;
  border-radius: 4px;
  font: inherit;
  font-size: 11px;
  cursor: pointer;
  min-width: 110px;
  appearance: none;
  background-image: linear-gradient(45deg, transparent 50%, var(--text3) 50%),
                    linear-gradient(135deg, var(--text3) 50%, transparent 50%);
  background-position: calc(100% - 12px) 50%, calc(100% - 8px) 50%;
  background-size: 4px 4px, 4px 4px;
  background-repeat: no-repeat;
}
.srf-kind:disabled { opacity: 0.5; cursor: not-allowed; }
.srf-kind[data-kind="inline"] { color: var(--text2); }
.srf-kind[data-kind="env"]         { color: var(--accent2); }
.srf-kind[data-kind="file"]        { color: var(--accent2); }
.srf-kind[data-kind="volume"]      { color: var(--accent2); }
.srf-kind[data-kind="aws-secret"]  { color: #ff9900; }
.srf-kind[data-kind="gcp-secret"]  { color: #4285F4; }
.srf-kind[data-kind="azure-vault"] { color: #00a4ef; }
.srf-kind[data-kind="vault"]       { color: var(--purple); }

.srf-value {
  flex: 1; min-width: 0;
  background: var(--bg3);
  border: 1px solid var(--border2);
  color: var(--text);
  padding: 5px 8px;
  border-radius: 4px;
  font: inherit;
  font-size: 12px;
  outline: none;
  transition: border-color 0.12s;
}
.srf-value:focus { border-color: var(--accent); }
.srf-value:disabled { opacity: 0.5; }

.srf-key {
  width: 140px;
  background: var(--bg3);
  border: 1px solid var(--border2);
  color: var(--text2);
  padding: 5px 8px;
  border-radius: 4px;
  font: inherit;
  font-size: 11.5px;
  outline: none;
}
.srf-key:focus { border-color: var(--accent); }

.srf-foot {
  display: flex; align-items: center; gap: 8px;
  font-size: 10.5px;
  flex-wrap: wrap;
}
.srf-source-pill {
  display: inline-block;
  padding: 1px 6px;
  border-radius: 3px;
  background: var(--bg4);
  color: var(--text3);
  font-size: 10px;
  font-weight: 500;
}
.srf-source-pill[data-kind="aws-secret"]  { color: #ff9900; background: rgba(255,153,0,0.12); }
.srf-source-pill[data-kind="gcp-secret"]  { color: #4285F4; background: rgba(66,133,244,0.12); }
.srf-source-pill[data-kind="azure-vault"] { color: #00a4ef; background: rgba(0,164,239,0.12); }
.srf-source-pill[data-kind="vault"]       { color: var(--purple); background: rgba(167,139,250,0.12); }
.srf-source-pill[data-kind="env"]         { color: var(--accent2); background: rgba(79,142,247,0.12); }
.srf-source-pill[data-kind="file"]        { color: var(--accent2); background: rgba(79,142,247,0.12); }
.srf-source-pill[data-kind="volume"]      { color: var(--accent2); background: rgba(79,142,247,0.12); }

.srf-foot-hint { color: var(--text3); font-style: italic; }
.srf-error { color: var(--red); font-weight: 500; }

.secret-ref-input.invalid .srf-value { border-color: rgba(240,84,84,0.6); }
.secret-ref-input.compact .srf-foot { display: none; }
</style>
