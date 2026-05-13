import { describe, it, expect, beforeEach, vi } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { __test } from '../useEnvProbeWatcher'
import { useSetupChecklistStore } from '../../stores/setupChecklist'

// We exercise the event → checklist mapping via the test-only surface
// the composable exposes. The Wails event bridge itself is covered by
// existing tests; here we care about the producer logic.

describe('useEnvProbeWatcher mapping', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('upserts an ok row without an action button', () => {
    const reprobe = vi.fn()
    __test.buildRow({
      id: 'envprobe.dns', title: 'DNS OK',
      status: 'ok', detail: 'prod → 10.0.0.1',
    }, { reprobe })
    const row = useSetupChecklistStore().items.find(i => i.id === 'envprobe.dns')
    expect(row.status).toBe('ok')
    expect(row.action).toBeNull()
  })

  it('upserts a todo row with the action wired to the local dispatcher', () => {
    const reprobe = vi.fn()
    __test.buildRow({
      id: 'envprobe.tls', title: 'TLS chain to API server',
      status: 'todo',
      detail: 'API server certificate signed by an untrusted CA — likely a corporate proxy.',
      actionLabel: 'Trust corporate CA', actionId: 'envprobe.trust-corp-ca',
    }, { reprobe })
    const row = useSetupChecklistStore().items.find(i => i.id === 'envprobe.tls')
    expect(row.status).toBe('todo')
    expect(row.action.label).toBe('Trust corporate CA')
    expect(typeof row.action.dispatch).toBe('function')
    // Clicking the action should reach reprobeAll for now (M5 will swap
    // in the real CA trust flow).
    row.action.dispatch()
    expect(reprobe).toHaveBeenCalled()
  })

  it('maps unknown statuses to warn so a broken backend cannot silently hide', () => {
    __test.buildRow({ id: 'envprobe.x', title: 'X', status: 'banana' })
    const row = useSetupChecklistStore().items.find(i => i.id === 'envprobe.x')
    expect(row.status).toBe('warn')
  })

  it('falls back to a generic re-check when actionId is unknown', () => {
    const reprobe = vi.fn()
    __test.buildRow({
      id: 'envprobe.future', title: 'Future probe',
      status: 'todo',
      actionLabel: 'Fix', actionId: 'envprobe.not-yet-mapped',
    }, { reprobe })
    const row = useSetupChecklistStore().items.find(i => i.id === 'envprobe.future')
    expect(row.action.label).toBe('Fix')
    row.action.dispatch()
    expect(reprobe).toHaveBeenCalled()
  })

  it('drops events with no id', () => {
    const r = __test.buildRow({ title: 'orphan', status: 'ok' })
    expect(r).toBeNull()
    expect(useSetupChecklistStore().items).toEqual([])
  })

  it('subsequent events replace the existing row in place', () => {
    __test.buildRow({ id: 'envprobe.dns', title: 'DNS', status: 'todo', detail: 'down' })
    __test.buildRow({ id: 'envprobe.dns', title: 'DNS OK', status: 'ok', detail: 'up' })
    const rows = useSetupChecklistStore().items.filter(i => i.id === 'envprobe.dns')
    expect(rows).toHaveLength(1)
    expect(rows[0].status).toBe('ok')
  })

  it('signed-images apply-trust-policy action opens the policy docs', () => {
    const openTrustPolicyDocs = vi.fn()
    __test.buildRow({
      id: 'envprobe.signed-images',
      title: 'Cluster requires signed images',
      status: 'todo',
      detail: 'Kyverno detected.',
      actionLabel: 'Apply Argus trust policy',
      actionId: 'envprobe.apply-trust-policy',
    }, { openTrustPolicyDocs })
    const row = useSetupChecklistStore().items.find(i => i.id === 'envprobe.signed-images')
    expect(row.action.label).toBe('Apply Argus trust policy')
    row.action.dispatch()
    expect(openTrustPolicyDocs).toHaveBeenCalled()
  })

  it('does not attach an action to ok rows even if actionLabel is sent', () => {
    __test.buildRow({
      id: 'envprobe.dns', title: 'DNS OK', status: 'ok',
      actionLabel: 'Re-check', actionId: 'envprobe.recheck',
    })
    const row = useSetupChecklistStore().items.find(i => i.id === 'envprobe.dns')
    expect(row.action).toBeNull()
  })
})
