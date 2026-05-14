<script setup>
import { computed } from 'vue'
import Select from '../common/Select.vue'

// RampControls owns the four ramp profiles (constant / linear / step /
// spike) and the per-profile fields. We surface them as a flat
// modelValue so the parent's submit handler can pass it straight into
// the spec's rampProfile/rampRate plus its ramp-specific extras.

const props = defineProps({
  modelValue: {
    type: Object,
    required: true,
    // shape: { profile, rate, durationSec, rampTo, stepBy, stepEverySec,
    //          spikeCount, spikeSize, spikeIdleSec }
  },
})
const emit = defineEmits(['update:modelValue'])

function patch(field, value) {
  emit('update:modelValue', { ...props.modelValue, [field]: value })
}

const PROFILES = [
  { value: 'constant', label: 'Constant rate' },
  { value: 'linear', label: 'Linear ramp-up' },
  { value: 'step', label: 'Step ramp' },
  { value: 'spike', label: 'Spike bursts' },
]

const profile = computed(() => props.modelValue.profile || 'constant')
</script>

<template>
  <div class="ramp-controls">
    <Select
      :modelValue="profile"
      :options="PROFILES"
      testid="distload-ramp-profile"
      width="100%"
      @update:modelValue="patch('profile', $event)"
    />
    <div class="ramp-fields">
      <template v-if="profile === 'constant'">
        <div class="row-2col">
          <div class="form-group">
            <label class="form-label">Rate (msg/s)</label>
            <input type="number" min="1" class="form-input" :value="modelValue.rate" @input="patch('rate', Number($event.target.value))" />
          </div>
          <div class="form-group">
            <label class="form-label">Max Duration (sec, 0 = unlimited)</label>
            <input type="number" min="0" class="form-input" :value="modelValue.durationSec" @input="patch('durationSec', Number($event.target.value))" />
          </div>
        </div>
      </template>

      <template v-else-if="profile === 'linear'">
        <div class="row-3col">
          <div class="form-group">
            <label class="form-label">Start rate</label>
            <input type="number" min="1" class="form-input" :value="modelValue.rate" @input="patch('rate', Number($event.target.value))" />
          </div>
          <div class="form-group">
            <label class="form-label">End rate</label>
            <input type="number" min="1" class="form-input" :value="modelValue.rampTo" @input="patch('rampTo', Number($event.target.value))" />
          </div>
          <div class="form-group">
            <label class="form-label">Duration (sec)</label>
            <input type="number" min="1" class="form-input" :value="modelValue.durationSec" @input="patch('durationSec', Number($event.target.value))" />
          </div>
        </div>
      </template>

      <template v-else-if="profile === 'step'">
        <div class="row-2col">
          <div class="form-group">
            <label class="form-label">Start rate</label>
            <input type="number" min="1" class="form-input" :value="modelValue.rate" @input="patch('rate', Number($event.target.value))" />
          </div>
          <div class="form-group">
            <label class="form-label">Step by (msg/s)</label>
            <input type="number" min="1" class="form-input" :value="modelValue.stepBy" @input="patch('stepBy', Number($event.target.value))" />
          </div>
          <div class="form-group">
            <label class="form-label">Step every (sec)</label>
            <input type="number" min="1" class="form-input" :value="modelValue.stepEverySec" @input="patch('stepEverySec', Number($event.target.value))" />
          </div>
          <div class="form-group">
            <label class="form-label">Max Duration (sec, 0 = unlimited)</label>
            <input type="number" min="0" class="form-input" :value="modelValue.durationSec" @input="patch('durationSec', Number($event.target.value))" />
          </div>
        </div>
      </template>

      <template v-else-if="profile === 'spike'">
        <div class="row-3col">
          <div class="form-group">
            <label class="form-label">Spike count</label>
            <input type="number" min="1" class="form-input" :value="modelValue.spikeCount" @input="patch('spikeCount', Number($event.target.value))" />
          </div>
          <div class="form-group">
            <label class="form-label">Spike size (msg/burst)</label>
            <input type="number" min="1" class="form-input" :value="modelValue.spikeSize" @input="patch('spikeSize', Number($event.target.value))" />
          </div>
          <div class="form-group">
            <label class="form-label">Idle between (sec)</label>
            <input type="number" min="1" class="form-input" :value="modelValue.spikeIdleSec" @input="patch('spikeIdleSec', Number($event.target.value))" />
          </div>
        </div>
      </template>
    </div>
  </div>
</template>

<style scoped>
.ramp-controls { display: flex; flex-direction: column; gap: 12px; }
.row-2col { display: grid; grid-template-columns: 1fr 1fr; gap: 12px; }
.row-3col { display: grid; grid-template-columns: 1fr 1fr 1fr; gap: 12px; }
.form-group { display: flex; flex-direction: column; gap: 4px; }
.form-label { font-size: 12px; color: var(--text2); }
.form-input {
  background: var(--bg3); border: 1px solid var(--border); border-radius: 6px;
  color: var(--text); font-size: 13px; padding: 6px 10px; font-family: inherit;
  width: 100%; box-sizing: border-box;
}
.form-input:focus { outline: 2px solid var(--accent, #4f8cff); outline-offset: 1px; border-color: var(--accent, #4f8cff); }
</style>
