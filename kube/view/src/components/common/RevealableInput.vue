<script setup>
import { ref, computed } from 'vue'

// RevealableInput — a text/password input with an inline eye toggle so
// the user can confirm what they typed (especially useful for long
// API tokens where one wrong character silently breaks everything).
//
// Defaults to masked (type="password"). Clicking the eye flips to a
// visible input; clicking again re-masks. The toggle is a button so
// keyboards + screen readers can use it; it ships with aria-label and
// aria-pressed reflecting state.
//
// This is a thin wrapper around a native input — v-model passes
// through, all standard attributes still work via $attrs. The only
// styling we own is the trailing eye icon and a 32px right-padding
// reservation for it.
//
// Props beyond v-model + the eye toggle:
//   id                — forwarded to the input for <label for>
//   placeholder       — input placeholder
//   autocomplete      — important for password managers; defaults to
//                       "off" so we don't accidentally invite the
//                       browser to autofill an API token field
//   disabled          — disables both the input AND the eye button
//   initiallyRevealed — start in plaintext (rare; used for "edit a
//                       known-good token" flows where masking adds no
//                       value)
//   ariaLabel         — passes through to the input for screen readers
//
// Emits:
//   update:modelValue — same as a native input via v-model
//   reveal-change     — true/false when the eye is toggled

const props = defineProps({
  modelValue: { type: String, default: '' },
  id:          { type: String, default: '' },
  placeholder: { type: String, default: '' },
  autocomplete:{ type: String, default: 'off' },
  disabled:    { type: Boolean, default: false },
  initiallyRevealed: { type: Boolean, default: false },
  ariaLabel:   { type: String, default: '' },
  required:    { type: Boolean, default: false },
  minlength:   { type: Number, default: 0 },
  // Forwarded onto the inner <input> so callers can apply their
  // existing input styling (e.g. "input mono" classes) without having
  // to wrap-with-a-wrapper-with-a-wrapper.
  inputClass:  { type: [String, Array, Object], default: '' },
})

const emit = defineEmits(['update:modelValue', 'reveal-change'])

const revealed = ref(props.initiallyRevealed)

const inputType = computed(() => (revealed.value ? 'text' : 'password'))

function onInput(event) {
  emit('update:modelValue', event.target.value)
}

function toggleReveal() {
  if (props.disabled) return
  revealed.value = !revealed.value
  emit('reveal-change', revealed.value)
}

defineExpose({ revealed, toggleReveal })
</script>

<template>
  <div class="revealable-input" :class="{ disabled }">
    <input
      :id="id"
      :class="['ri-input', inputClass]"
      :type="inputType"
      :value="modelValue"
      :placeholder="placeholder"
      :autocomplete="autocomplete"
      :disabled="disabled"
      :required="required"
      :minlength="minlength || undefined"
      :aria-label="ariaLabel || undefined"
      spellcheck="false"
      @input="onInput"
    />
    <button
      type="button"
      class="ri-toggle"
      :disabled="disabled"
      :aria-label="revealed ? 'Hide value' : 'Show value'"
      :aria-pressed="revealed"
      :title="revealed ? 'Hide value' : 'Show value'"
      tabindex="0"
      @click="toggleReveal"
    >
      <!-- Eye-off (slashed) when revealed; eye-open when masked. -->
      <svg v-if="revealed" width="16" height="16" viewBox="0 0 24 24" fill="none"
           stroke="currentColor" stroke-width="1.8" stroke-linecap="round"
           stroke-linejoin="round" aria-hidden="true">
        <path d="M17.94 17.94A10.07 10.07 0 0 1 12 20c-7 0-11-8-11-8a18.45 18.45 0 0 1 5.06-5.94M9.9 4.24A9.12 9.12 0 0 1 12 4c7 0 11 8 11 8a18.5 18.5 0 0 1-2.16 3.19m-6.72-1.07a3 3 0 1 1-4.24-4.24"/>
        <line x1="1" y1="1" x2="23" y2="23"/>
      </svg>
      <svg v-else width="16" height="16" viewBox="0 0 24 24" fill="none"
           stroke="currentColor" stroke-width="1.8" stroke-linecap="round"
           stroke-linejoin="round" aria-hidden="true">
        <path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z"/>
        <circle cx="12" cy="12" r="3"/>
      </svg>
    </button>
  </div>
</template>

<style scoped>
.revealable-input {
  position: relative;
  display: flex;
  align-items: stretch;
  min-width: 0;
  /* Inherit width so consumers control layout via the wrapper. */
  width: 100%;
}
.revealable-input.disabled { opacity: 0.6; }

.ri-input {
  flex: 1;
  min-width: 0;
  /* Reserve 32px on the right for the eye button so the typed value
     doesn't slide under it. */
  padding-right: 32px;
  font: inherit;
  /* Defer the rest of the styling (border, background, padding-left)
     to the consumer's existing input styles via $attrs class
     inheritance — the consumer can pass class="input mono" etc. */
  background: inherit;
  border: inherit;
  color: inherit;
  outline: none;
  border-radius: inherit;
}

.ri-toggle {
  position: absolute;
  right: 4px;
  top: 50%;
  transform: translateY(-50%);
  width: 24px;
  height: 24px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  background: transparent;
  border: 0;
  border-radius: 4px;
  color: var(--text3);
  cursor: pointer;
  padding: 0;
  transition: color 0.12s, background 0.12s;
}
.ri-toggle:hover:not(:disabled) {
  color: var(--text);
  background: rgba(255, 255, 255, 0.06);
}
.ri-toggle:focus-visible {
  outline: 2px solid var(--accent);
  outline-offset: 1px;
}
.ri-toggle:disabled { cursor: not-allowed; }
</style>
