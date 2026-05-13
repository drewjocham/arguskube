import { watch } from 'vue'
import { useSetupChecklistStore } from '../stores/setupChecklist'
import { useAutoContext } from './useAutoContext'

// Bridges the §2 auto-context resolver into the §3 setup checklist.
// Pure watcher: reads useAutoContext's reactive state and upserts (or
// removes) a single checklist row whenever the resolution changes.
//
// The probe owns its own row id ("kubeconfig.context") so it can update
// in place without duplicating entries. Confidence drives the row's
// status:
//
//   active-reachable    → ok      ("Connected to <ctx>")
//   fallback-reachable  → warn    ("Active context unreachable — using <ctx>")
//   active-unreachable  → todo    ("Cluster <ctx> not reachable") + reprobe action
//   none                → error   ("No kubeconfig contexts found")
//
// Mounting this composable inside App.vue (which already calls
// useAutoContext.resolve() on auth-true) means the row appears at the
// first paint after sign-in and updates live as probes complete.

const ROW_ID = 'kubeconfig.context'
const PRIORITY = 10 // load-bearing — sits near the top of the checklist

export function useAutoContextProbe() {
  const checklist = useSetupChecklistStore()
  const { resolution, loading, error, reprobe } = useAutoContext()

  // While the first probe is in flight, render a "Checking…" row so the
  // user sees activity even before any result lands.
  watch(loading, (l) => {
    if (l) {
      checklist.upsert({
        id: ROW_ID,
        title: 'Detecting your cluster',
        status: 'running',
        priority: PRIORITY,
        source: 'auto-context',
      })
    }
  }, { immediate: true })

  watch(error, (e) => {
    if (!e) return
    checklist.upsert({
      id: ROW_ID,
      title: 'Could not read kubeconfig',
      status: 'error',
      priority: PRIORITY,
      detail: e,
      source: 'auto-context',
      action: { label: 'Retry', dispatch: () => reprobe() },
    })
  })

  watch(resolution, (r) => {
    if (!r) return
    const { chosen, confidence, reachableCount, probes } = r
    if (confidence === 'none') {
      checklist.upsert({
        id: ROW_ID,
        title: 'No kubeconfig contexts found',
        status: 'error',
        priority: PRIORITY,
        detail: 'Add a context with kubectl, then click Retry.',
        source: 'auto-context',
        action: { label: 'Retry', dispatch: () => reprobe() },
      })
      return
    }
    if (confidence === 'active-reachable') {
      checklist.upsert({
        id: ROW_ID,
        title: `Connected to ${chosen}`,
        status: 'ok',
        priority: PRIORITY,
        detail: `${reachableCount} of ${probes?.length ?? reachableCount} contexts reachable.`,
        source: 'auto-context',
      })
      return
    }
    if (confidence === 'fallback-reachable') {
      checklist.upsert({
        id: ROW_ID,
        title: `Active context unreachable — using ${chosen}`,
        status: 'warn',
        priority: PRIORITY,
        detail: 'You can switch back from the sidebar once the original context is reachable.',
        source: 'auto-context',
        action: { label: 'Re-check', dispatch: () => reprobe() },
      })
      return
    }
    if (confidence === 'active-unreachable') {
      const detail = pickError(probes, chosen)
        || 'Common causes: VPN off, corporate proxy, expired credentials.'
      checklist.upsert({
        id: ROW_ID,
        title: `Cluster ${chosen} not reachable`,
        status: 'todo',
        priority: PRIORITY,
        detail,
        source: 'auto-context',
        action: { label: 'Re-check', dispatch: () => reprobe() },
      })
    }
  })
}

function pickError(probes, name) {
  if (!Array.isArray(probes)) return ''
  for (const p of probes) {
    if (p?.name === name && p?.error) return String(p.error)
  }
  return ''
}
