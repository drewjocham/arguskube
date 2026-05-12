import { describe, it, expect, beforeEach } from 'vitest'
import { useBackgroundTasks } from '../useBackgroundTasks'

describe('useBackgroundTasks', () => {
  let tasks
  beforeEach(() => {
    tasks = useBackgroundTasks()
  })

  it('startTask creates a running task with timestamps', () => {
    tasks.startTask('scan-1')
    const t = tasks.getTask('scan-1')
    expect(t.status).toBe('running')
    expect(t.result).toBeNull()
    expect(t.error).toBeNull()
    expect(t.startedAt).toBeTypeOf('string')
    expect(t.completedAt).toBeNull()
  })

  it('completeTask marks task as completed with result', () => {
    tasks.startTask('scan-1')
    tasks.completeTask('scan-1', { findings: 3 })
    const t = tasks.getTask('scan-1')
    expect(t.status).toBe('completed')
    expect(t.result).toEqual({ findings: 3 })
    expect(t.error).toBeNull()
    expect(t.completedAt).toBeTypeOf('string')
  })

  it('failTask marks task as failed with error', () => {
    tasks.startTask('scan-1')
    tasks.failTask('scan-1', new Error('timeout'))
    const t = tasks.getTask('scan-1')
    expect(t.status).toBe('failed')
    expect(t.error).toEqual(new Error('timeout'))
    expect(t.result).toBeNull()
  })

  it('overwriting a running task preserves meta', () => {
    tasks.startTask('scan-1', { namespace: 'prod' })
    tasks.completeTask('scan-1', 'done')
    const t = tasks.getTask('scan-1')
    expect(t.namespace).toBe('prod')
    expect(t.status).toBe('completed')
  })

  it('completeTask on non-existent key still sets completed state', () => {
    tasks.completeTask('ghost', 'result')
    const t = tasks.getTask('ghost')
    expect(t.status).toBe('completed')
    expect(t.result).toBe('result')
  })

  it('failTask on non-existent key still sets failed state', () => {
    tasks.failTask('ghost', new Error('err'))
    const t = tasks.getTask('ghost')
    expect(t.status).toBe('failed')
  })

  it('hasTask returns true for existing task', () => {
    tasks.startTask('scan-1')
    expect(tasks.hasTask('scan-1')).toBe(true)
  })

  it('hasTask returns false for non-existing task', () => {
    expect(tasks.hasTask('nope')).toBe(false)
  })

  it('isRunning returns true only when status is running', () => {
    tasks.startTask('scan-1')
    expect(tasks.isRunning('scan-1')).toBe(true)
    tasks.completeTask('scan-1', 'ok')
    expect(tasks.isRunning('scan-1')).toBe(false)
  })

  it('isRunning returns false for non-existing key', () => {
    expect(tasks.isRunning('nope')).toBe(false)
  })

  it('lastResult returns result only when status is completed', () => {
    tasks.startTask('scan-1')
    expect(tasks.lastResult('scan-1')).toBeNull()
    tasks.completeTask('scan-1', 'done')
    expect(tasks.lastResult('scan-1')).toBe('done')
  })

  it('lastResult returns null for failed tasks', () => {
    tasks.startTask('scan-1')
    tasks.failTask('scan-1', new Error('err'))
    expect(tasks.lastResult('scan-1')).toBeNull()
  })

  it('lastResult returns null for non-existing key', () => {
    expect(tasks.lastResult('nope')).toBeNull()
  })

  it('clearTask removes the task entirely', () => {
    tasks.startTask('scan-1')
    expect(tasks.hasTask('scan-1')).toBe(true)
    tasks.clearTask('scan-1')
    expect(tasks.hasTask('scan-1')).toBe(false)
    expect(tasks.getTask('scan-1')).toBeNull()
  })

  it('getTask returns null for non-existing key', () => {
    expect(tasks.getTask('nope')).toBeNull()
  })

  it('startTask accepts optional meta that flows through', () => {
    tasks.startTask('scan-1', { foo: 'bar' })
    const t = tasks.getTask('scan-1')
    expect(t.foo).toBe('bar')
  })

  it('startTask sets updatedAt timestamp', () => {
    const before = Date.now()
    tasks.startTask('scan-1')
    const t = tasks.getTask('scan-1')
    expect(t.updatedAt).toBeGreaterThanOrEqual(before)
  })


})
