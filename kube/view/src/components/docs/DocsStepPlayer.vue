<script setup>
import { ref, watch, onBeforeUnmount, computed } from 'vue'
import { useGuidesStore } from '../../stores/guides'
import { useAppNavStore } from '../../stores/appNav'
import { useSectionTabsStore } from '../../stores/sectionTabs'
import DocsHighlight from './DocsHighlight.vue'

const guides = useGuidesStore()
const appNav = useAppNavStore()
const sectionTabs = useSectionTabsStore()

const autoAdvance = ref(true)
let autoTimer = null

const currentStep = computed(() => guides.activeStep)
const stepIndex = computed(() => guides.activeStepIndex)
const stepCount = computed(() => guides.stepCount)
const guideLabel = computed(() => guides.activeGuide?.label || '')

const highlightedSelector = computed(() => currentStep.value?.targetSelector || '')

function scheduleNext() {
  clearAutoTimer()
  if (!autoAdvance.value) return
  if (guides.isLastStep) return
  const ms = currentStep.value?.durationMs || 5000
  autoTimer = setTimeout(() => {
    guides.nextStep()
  }, ms)
}

function clearAutoTimer() {
  if (autoTimer) {
    clearTimeout(autoTimer)
    autoTimer = null
  }
}

function onPrev() {
  clearAutoTimer()
  guides.prevStep()
}

function onNext() {
  clearAutoTimer()
  guides.nextStep()
}

function onGoThere() {
  clearAutoTimer()
  const step = currentStep.value
  if (!step?.navigation) return
  const { sectionId, tabId } = step.navigation
  if (tabId && sectionId) {
    sectionTabs.setTab(sectionId, tabId)
  }
  appNav.requestNav({ navId: sectionId })
}

function onDone() {
  clearAutoTimer()
  guides.reset()
}

function toggleAutoAdvance() {
  autoAdvance.value = !autoAdvance.value
  if (autoAdvance.value) scheduleNext()
}

watch(stepIndex, () => {
  if (autoAdvance.value) scheduleNext()
})

watch(autoAdvance, (val) => {
  if (val) scheduleNext()
  else clearAutoTimer()
})

onBeforeUnmount(clearAutoTimer)
</script>

<template>
  <div class="step-player" v-if="currentStep">
    <DocsHighlight :target-selector="highlightedSelector" />

    <div class="step-header">
      <div class="step-breadcrumb">
        <button class="step-back-link" @click="guides.reset()">
          <svg width="14" height="14" viewBox="0 0 14 14" fill="none">
            <path d="M9 3L5 7l4 4" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/>
          </svg>
          {{ guideLabel }}
        </button>
      </div>
      <div class="step-counter">
        <button
          class="step-auto-toggle"
          :class="{ paused: !autoAdvance }"
          :title="autoAdvance ? 'Pause auto-advance' : 'Resume auto-advance'"
          @click="toggleAutoAdvance"
        >
          <svg v-if="autoAdvance" width="12" height="12" viewBox="0 0 12 12" fill="currentColor">
            <circle cx="6" cy="6" r="5"/>
          </svg>
          <svg v-else width="12" height="12" viewBox="0 0 12 12" fill="none" stroke="currentColor" stroke-width="1.3">
            <rect x="3" y="2" width="2" height="8" rx="0.5"/>
            <rect x="7" y="2" width="2" height="8" rx="0.5"/>
          </svg>
        </button>
        <span class="step-num">Step {{ stepIndex + 1 }} / {{ stepCount }}</span>
      </div>
    </div>

    <div class="step-progress">
      <button
        v-for="(_, i) in guides.activeGuide?.steps"
        :key="i"
        class="step-dot"
        :class="{ active: i === stepIndex, done: i < stepIndex }"
        @click="guides.goToStep(i)"
        :aria-label="'Go to step ' + (i + 1)"
      ></button>
    </div>

    <div class="step-body">
      <h3 class="step-title">{{ currentStep.title }}</h3>
      <p class="step-instruction">{{ currentStep.instruction }}</p>
    </div>

    <div class="step-actions">
      <div class="step-nav">
        <button
          class="step-btn secondary"
          :disabled="guides.isFirstStep"
          @click="onPrev"
        >
          <svg width="12" height="12" viewBox="0 0 12 12" fill="none">
            <path d="M7 3L4 6l3 3" stroke="currentColor" stroke-width="1.4" stroke-linecap="round" stroke-linejoin="round"/>
          </svg>
          Prev
        </button>
        <button
          v-if="!guides.isLastStep"
          class="step-btn primary"
          @click="onNext"
        >
          Next
          <svg width="12" height="12" viewBox="0 0 12 12" fill="none">
            <path d="M5 3l3 3-3 3" stroke="currentColor" stroke-width="1.4" stroke-linecap="round" stroke-linejoin="round"/>
          </svg>
        </button>
      </div>

      <div class="step-go">
        <button
          class="step-btn go-there"
          @click="onGoThere"
          :title="'Navigate to ' + (currentStep.navigation?.tabId || currentStep.navigation?.sectionId || '')"
        >
          Go there
          <svg width="12" height="12" viewBox="0 0 12 12" fill="none">
            <path d="M3 9l6-6M4 3h5v5" stroke="currentColor" stroke-width="1.4" stroke-linecap="round" stroke-linejoin="round"/>
          </svg>
        </button>
        <button
          v-if="guides.isLastStep"
          class="step-btn done-btn"
          @click="onDone"
        >
          Back to guides
        </button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.step-player {
  flex: 1;
  display: flex;
  flex-direction: column;
  padding: 24px;
  gap: 16px;
  overflow-y: auto;
  min-height: 0;
}

.step-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.step-breadcrumb {
  display: flex;
  align-items: center;
}

.step-back-link {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  background: none;
  border: none;
  color: var(--text2);
  font: inherit;
  font-size: 12px;
  cursor: pointer;
  padding: 4px 8px;
  border-radius: 4px;
  transition: all 0.1s;
}

.step-back-link:hover {
  background: var(--bg3);
  color: var(--text);
}

.step-counter {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 12px;
  color: var(--text3);
}

.step-auto-toggle {
  background: none;
  border: 1px solid var(--border);
  border-radius: 50%;
  width: 22px;
  height: 22px;
  display: flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  color: var(--text3);
  transition: all 0.15s;
}

.step-auto-toggle:hover {
  background: var(--bg3);
  color: var(--text);
}

.step-auto-toggle.paused {
  border-color: var(--accent2);
  color: var(--accent2);
}

.step-num {
  font-variant-numeric: tabular-nums;
}

.step-progress {
  display: flex;
  gap: 6px;
  align-items: center;
}

.step-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  border: 1px solid var(--border2);
  background: var(--bg3);
  cursor: pointer;
  padding: 0;
  transition: all 0.2s;
}

.step-dot.active {
  background: var(--accent2);
  border-color: var(--accent2);
  box-shadow: 0 0 6px rgba(74, 158, 255, 0.4);
}

.step-dot.done {
  background: var(--text3);
  border-color: var(--text3);
}

.step-dot:hover {
  border-color: var(--accent2);
}

.step-body {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 12px;
  padding: 20px;
  background: var(--bg2);
  border: 1px solid var(--border);
  border-radius: 8px;
}

.step-title {
  font-size: 18px;
  font-weight: 500;
  color: var(--text);
  margin: 0;
}

.step-instruction {
  font-size: 13.5px;
  line-height: 1.7;
  color: var(--text2);
  margin: 0;
  white-space: pre-wrap;
}

.step-actions {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.step-nav {
  display: flex;
  gap: 8px;
}

.step-go {
  display: flex;
  gap: 8px;
}

.step-btn {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 7px 16px;
  border-radius: 6px;
  font: inherit;
  font-size: 12.5px;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.1s;
  border: 1px solid transparent;
}

.step-btn.secondary {
  background: var(--bg3);
  border-color: var(--border2);
  color: var(--text2);
}

.step-btn.secondary:hover:not(:disabled) {
  background: var(--bg4);
  color: var(--text);
}

.step-btn.secondary:disabled {
  opacity: 0.4;
  cursor: default;
}

.step-btn.primary {
  background: var(--bg3);
  border-color: var(--accent2);
  color: var(--accent2);
}

.step-btn.primary:hover {
  background: rgba(74, 158, 255, 0.1);
}

.step-btn.go-there {
  background: rgba(74, 158, 255, 0.15);
  border-color: rgba(74, 158, 255, 0.3);
  color: var(--accent2);
}

.step-btn.go-there:hover {
  background: rgba(74, 158, 255, 0.25);
}

.step-btn.done-btn {
  background: var(--bg3);
  border-color: var(--border2);
  color: var(--text2);
}

.step-btn.done-btn:hover {
  background: var(--bg4);
  color: var(--text);
}
</style>
