/**
 * Background Task Persistence Store
 *
 * Keeps scan/analysis state alive across navigation so the user can
 * start a task, click away, and come back to see the last result.
 *
 * Use this instead of component-local refs for long-running operations.
 */

import { reactive, readonly } from 'vue'

// ─── Task Registry ───────────────────────────────────────────────────
const tasks = reactive({})

/**
 * Register (or update) a background task.
 *
 * @param {string} key  Unique key, e.g. "vulnerability-scan" or "argus-scan"
 * @param {object} state  { status, result, error, startedAt, completedAt }
 */
function setTask(key, state) {
  tasks[key] = { ...state, updatedAt: Date.now() }
}

/**
 * Retrieve a stored task snapshot.
 */
function getTask(key) {
  return tasks[key] || null
}

/**
 * Remove a task from the registry.
 */
function clearTask(key) {
  delete tasks[key]
}

/**
 * Begin tracking a task. Sets status → "running".
 */
function startTask(key, meta = {}) {
  setTask(key, {
    status: 'running',
    result: null,
    error: null,
    startedAt: new Date().toISOString(),
    completedAt: null,
    ...meta,
  })
}

/**
 * Mark a task as completed with its result.
 */
function completeTask(key, result) {
  const existing = getTask(key) || {}
  setTask(key, {
    ...existing,
    status: 'completed',
    result,
    error: null,
    completedAt: new Date().toISOString(),
  })
}

/**
 * Mark a task as failed with an error.
 */
function failTask(key, error) {
  const existing = getTask(key) || {}
  setTask(key, {
    ...existing,
    status: 'failed',
    error,
    completedAt: new Date().toISOString(),
  })
}

// ─── Convenience accessors ───────────────────────────────────────────
const allTasks = readonly(tasks)

function hasTask(key) {
  return key in tasks
}

function isRunning(key) {
  return tasks[key]?.status === 'running'
}

function lastResult(key) {
  const t = tasks[key]
  return t?.status === 'completed' ? t.result : null
}

export function useBackgroundTasks() {
  return {
    tasks: allTasks,
    setTask,
    getTask,
    clearTask,
    startTask,
    completeTask,
    failTask,
    hasTask,
    isRunning,
    lastResult,
  }
}
