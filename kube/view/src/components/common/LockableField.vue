<script setup>
import { ref, computed } from 'vue'

// LockableField — a credential input with two states:
//
//   1. LOCKED (default when modelValue is non-empty on mount):
//      Renders a read-only placeholder ("••••••••") with a greyed
//      background. The actual value is NEVER in the DOM in this
//      state — no `value` attribute, no inner text — so a casual
//      "view source" or screenshot doesn't leak the secret.
//      A checkbox below the field reads "Unlock to edit".
//
//   2. UNLOCKED (after the user ticks the checkbox, or when the
//      field starts empty):
//      Behaves like a normal text input. Typing updates modelValue
//      via v-model. A subsequent tick of the checkbox returns to
//      locked but PRESERVES modelValue — re-locking is purely a
//      visual hide. (Earlier draft cleared on re-lock; that erased
//      unsaved edits and surprised users who'd typed and changed
//      their mind. The security goal is "value not visible when
//      locked", not "value erased from JS memory" — modelValue
//      living in component state isn't the same as being on screen.)
//
// Why not just use type="password"? type="password" still puts the
// real value into the input's `value` attribute and onto the DOM
// node. Any browser extension, devtools session, or page-scraping
// screenshot tool sees the cleartext. LockableField only renders
// the value when the user has explicitly asked to see it.
//
// Props:
//   modelValue   — current value (string). Empty string means "no
//                  credential configured yet".
//   placeholder  — placeholder shown in the input when unlocked-and-empty
//   id           — forwarded for <label for>
//   disabled     — disables both the input and the unlock toggle
//   inputClass   — class to apply to the inner input element so the
//                  consumer can reuse their existing input styling
//                  (e.g. "input mono")
//   lockedHint   — text shown next to the unlock checkbox; defaults
//                  to "Unlock to edit"
//
// Emits:
//   update:modelValue — same as a native input via v-model
//   lock-change       — true (locked) / false (unlocked) when the
//                       state changes

const props = defineProps({
  modelValue:  { type: String, default: '' },
  placeholder: { type: String, default: '' },
  id:          { type: String, default: '' },
  disabled:    { type: Boolean, default: false },
  inputClass:  { type: [String, Array, Object], default: '' },
  lockedHint:  { type: String, default: 'Unlock to edit' },
})

const emit = defineEmits(['update:modelValue', 'lock-change'])

// Start locked iff there's an existing value to protect. Empty
// fields start unlocked so the first-time-config flow doesn't
// require an extra click before typing.
const locked = ref(props.modelValue !== '')

const masked = '•'.repeat(10)  // 10 bullets — long enough to suggest
                                    // "real value", short enough not to
                                    // wrap on narrow forms

const placeholderWhenLocked = computed(() =>
  props.modelValue !== '' ? masked : (props.placeholder || ''),
)

function onUnlockToggle(event) {
  if (props.disabled) return
  const wantLocked = !event.target.checked
  // Re-locking does NOT clear modelValue — see header comment for
  // the rationale. The value just stops being rendered in the DOM.
  locked.value = wantLocked
  emit('lock-change', wantLocked)
}

function onInput(event) {
  emit('update:modelValue', event.target.value)
}
</script>

<template>
  <div class="lockable-field" :class="{ disabled }">
    <!-- LOCKED state: a read-only div that LOOKS like the input but
         carries no value. Real input is omitted entirely so the DOM
         doesn't contain the secret.

         Aria-disabled communicates the locked state to screen readers;
         role="textbox" keeps it discoverable as a form control. -->
    <div
      v-if="locked"
      :id="id || undefined"
      :class="['lf-locked', inputClass]"
      role="textbox"
      aria-readonly="true"
      :aria-disabled="disabled || undefined"
      :title="modelValue !== '' ? 'Credential configured — unlock to view or replace' : ''"
    >{{ placeholderWhenLocked }}</div>

    <!-- UNLOCKED state: a real input bound to v-model. -->
    <input
      v-else
      :id="id || undefined"
      :class="['lf-input', inputClass]"
      type="text"
      :value="modelValue"
      :placeholder="placeholder"
      :disabled="disabled"
      autocomplete="off"
      spellcheck="false"
      @input="onInput"
    />

    <label class="lf-unlock">
      <input
        type="checkbox"
        :checked="!locked"
        :disabled="disabled"
        @change="onUnlockToggle"
      />
      <span>{{ lockedHint }}</span>
    </label>
  </div>
</template>

<style scoped>
.lockable-field {
  display: flex;
  flex-direction: column;
  gap: 4px;
  min-width: 0;
  width: 100%;
}
.lockable-field.disabled { opacity: 0.6; }

/* Locked state: greyed background + dotted border so it visually
   reads as "not editable right now" without looking broken. Letter-
   spacing widens the bullets so they don't squish to a single mass. */
.lf-locked {
  padding: 6px 10px;
  background: var(--bg3, #232830);
  border: 1px dashed var(--border, #3a4250);
  border-radius: 6px;
  color: var(--text3, #7a8492);
  font-family: var(--mono, ui-monospace, SFMono-Regular, Menlo, monospace);
  letter-spacing: 2px;
  user-select: none;
  cursor: default;
  /* Match the height of the real input. */
  min-height: 28px;
  display: flex;
  align-items: center;
}

.lf-input {
  /* Defer to the inputClass for actual styling — same pattern as
     RevealableInput. */
  font: inherit;
  background: inherit;
  border: inherit;
  color: inherit;
  outline: none;
  border-radius: inherit;
}

.lf-unlock {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  font-size: 11px;
  color: var(--text3, #7a8492);
  cursor: pointer;
  /* Tighten the checkbox+label to the left under the field. */
  align-self: flex-start;
}
.lf-unlock input[type="checkbox"] {
  cursor: pointer;
}
.lf-unlock:hover { color: var(--text2, #aab2bd); }
</style>
