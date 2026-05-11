// addons — Pinia store driving the Settings → Addons panel.
//
// Placeholder implementation. Holds an in-memory addon catalog plus
// per-job inputs, delivery preferences, and a run-history log. The
// surface mirrors the call sites in SettingsPanel.vue:
//
//   addonsStore.addons              — list of addon catalog entries
//   addonsStore.isEnabled(id)       — bool
//   addonsStore.setEnabled(id, b)
//   addonsStore.jobs                — list of job descriptors
//   addonsStore.jobInputs           — { [jobId]: { [inputId]: value } }
//   addonsStore.jobDeliveries       — { [jobId]: { [deliveryId]: value } }
//   addonsStore.setJobInput(jobId, inputId, value)
//   addonsStore.setJobDelivery(jobId, deliveryId, value)
//   addonsStore.recordRun({jobId, status})  → runId
//   addonsStore.completeRun(runId, {status, result, error})
//   addonsStore.runsByJob(jobId)    — newest-first run history slice
//
// Replace with the real backend-backed store when that work lands.

import { defineStore } from 'pinia'
import { reactive, ref } from 'vue'

let nextRun = 1
function newRunId() {
  return `run-${Date.now().toString(36)}-${(nextRun++).toString(36)}`
}

export const useAddonsStore = defineStore('addons', () => {
  // Placeholder catalog — empty until the backend exposes a real list.
  const addons = ref([])
  const enabled = reactive({})

  function isEnabled(id) {
    return Boolean(enabled[id])
  }
  function setEnabled(id, on) {
    enabled[id] = Boolean(on)
  }

  // Each job is { id, name, inputs: [{id,label,...}], deliveries: [...] }
  const jobs = ref([])
  const jobInputs = reactive({})
  const jobDeliveries = reactive({})

  function setJobInput(jobId, inputId, value) {
    if (!jobInputs[jobId]) jobInputs[jobId] = {}
    jobInputs[jobId][inputId] = value
  }
  function setJobDelivery(jobId, deliveryId, value) {
    if (!jobDeliveries[jobId]) jobDeliveries[jobId] = {}
    jobDeliveries[jobId][deliveryId] = value
  }

  // Run-history bookkeeping. Each run: { id, jobId, status, startedAt,
  // completedAt?, result?, error? }
  const runs = ref([])

  function recordRun({ jobId, status = 'pending' } = {}) {
    const id = newRunId()
    runs.value.unshift({
      id,
      jobId: jobId || '',
      status,
      startedAt: Date.now(),
    })
    return id
  }
  function completeRun(runId, patch = {}) {
    const r = runs.value.find((x) => x.id === runId)
    if (!r) return
    if (patch.status) r.status = patch.status
    if (patch.result !== undefined) r.result = patch.result
    if (patch.error !== undefined) r.error = patch.error
    r.completedAt = Date.now()
  }
  function runsByJob(jobId) {
    return runs.value.filter((r) => r.jobId === jobId)
  }

  return {
    addons,
    enabled,
    isEnabled,
    setEnabled,
    jobs,
    jobInputs,
    jobDeliveries,
    setJobInput,
    setJobDelivery,
    runs,
    recordRun,
    completeRun,
    runsByJob,
  }
})
