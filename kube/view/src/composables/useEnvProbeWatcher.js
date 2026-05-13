import { callGo } from './useBridge'
import { useWailsEvent } from './useEvents'
import { useSetupChecklistStore } from '../stores/setupChecklist'

// useEnvProbeWatcher subscribes to "argus:envprobe" events and turns
// each one into a checklist row. The backend publishes one event per
// probe per sweep (DNS, TLS, clock-skew today; envprobe.* in general),
// so this composable is purely reactive — no polling, no derived state.
//
// Each probe owns a stable id (envprobe.dns, envprobe.tls, …). We
// upsert by that id so the row updates in place across sweeps without
// duplicating. When the user clicks the row's action button, we route
// the actionId through a small dispatcher table; the most common
// dispatch is "re-check", which calls RunEnvProbes() on the backend.

const PRIORITY = 30 // sits below kubeconfig.context, above optional rows

function statusOf(raw) {
  switch (raw) {
    case 'ok':
    case 'warn':
    case 'todo':
    case 'error':
      return raw
    case 'running':
      return 'running'
    default:
      return 'warn'
  }
}

// Action dispatcher. Probe events carry an actionId; this map turns
// that id into a frontend function. Keeping it local to this composable
// means the producer is self-contained — adding a new probe action is
// one entry here plus the backend handler.
function buildDispatch(actions) {
  return {
    'envprobe.recheck': () => actions.reprobeAll(),
    'envprobe.trust-corp-ca': () => actions.reprobeAll(), // future: real CA-trust flow
    'envprobe.open-datetime': () => actions.reprobeAll(),
    // Signed-images: the long-term action is "apply policy with
    // dry-run-server preview", but the desktop-side backend for that
    // doesn't exist yet. In the meantime we open the policies README
    // (rendered with the user's repo at release time) so the user can
    // apply by hand without leaving the flow.
    'envprobe.apply-trust-policy': () => actions.openTrustPolicyDocs(),
  }
}

export function useEnvProbeWatcher() {
  const checklist = useSetupChecklistStore()

  async function reprobeAll() {
    try { await callGo('RunEnvProbes') }
    catch (_) { /* errors land in the ribbon via the backend's status events */ }
  }

  function openTrustPolicyDocs() {
    // Open the in-repo README that lists the three policy variants. In
    // a Wails desktop build window.open hands off to the system browser.
    const url = 'https://github.com/drewjocham/arguskube/tree/main/kube/deploy/policies'
    try { window.open(url, '_blank', 'noopener,noreferrer') } catch (_) { /* ignore */ }
  }

  const dispatch = buildDispatch({ reprobeAll, openTrustPolicyDocs })

  function rowFromEvent(e) {
    const status = statusOf(e?.status)
    const id = String(e?.id || '')
    if (!id) return null
    const row = {
      id,
      title: String(e?.title || id),
      status,
      detail: String(e?.detail || ''),
      priority: PRIORITY,
      source: 'envprobe',
    }
    if (status !== 'ok' && e?.actionLabel && e?.actionId) {
      const fn = dispatch[e.actionId]
      row.action = {
        label: String(e.actionLabel),
        dispatch: fn ? () => fn() : () => reprobeAll(),
        actionId: String(e.actionId),
      }
    }
    return row
  }

  useWailsEvent('argus:envprobe', (data) => {
    const row = rowFromEvent(data)
    if (row) checklist.upsert(row)
  })

  return { reprobeAll }
}

// Test-only surface so unit tests can exercise the event → row mapping
// without spinning up the full Wails event bridge.
export const __test = {
  buildRow(event, opts = {}) {
    const checklist = opts.checklist || useSetupChecklistStore()
    const reprobe = opts.reprobe || (() => {})
    const openDocs = opts.openTrustPolicyDocs || (() => {})
    const dispatch = buildDispatch({ reprobeAll: reprobe, openTrustPolicyDocs: openDocs })
    const status = statusOf(event?.status)
    const id = String(event?.id || '')
    if (!id) return null
    const row = {
      id,
      title: String(event?.title || id),
      status,
      detail: String(event?.detail || ''),
      priority: PRIORITY,
      source: 'envprobe',
    }
    if (status !== 'ok' && event?.actionLabel && event?.actionId) {
      const fn = dispatch[event.actionId]
      row.action = {
        label: String(event.actionLabel),
        dispatch: fn ? () => fn() : () => reprobe(),
        actionId: String(event.actionId),
      }
    }
    checklist.upsert(row)
    return row
  },
  PRIORITY,
}
