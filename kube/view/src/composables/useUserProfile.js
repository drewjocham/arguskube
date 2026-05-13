import { ref } from 'vue'
import { callGo } from './useBridge'

// useUserProfile wires the §6 "learning agents" surface:
//
//   - recordView(viewID) — fire on every navigation. The backend
//     persists this to a small SQLite log. We swallow errors silently:
//     a missed observation is no worse than the user not having opened
//     that view, and we never want a nav event to fail the nav itself.
//
//   - pollSuggestion(viewID) — async; returns 0 or 1 Suggestion plus a
//     suppression reason. The card component drives the polling, not
//     this composable, so timer logic stays where it's visible.
//
//   - mute / accept / dismiss — outcome bookkeeping. Always
//     fire-and-forget; a network blip never strands the UI.
//
//   - clearActivity — destructive, surfaced as "Forget my activity"
//     in Settings. The only call here that returns a real Promise
//     callers should await.

const suggestion = ref(null)
const suppressed = ref(false)
const suppressionReason = ref('')

function safe(p) {
  // Wrap any Promise so it never throws past us. The frontend's
  // existing bridge already logs HTTP failures; we don't want to
  // double-log or surface those errors in the UI.
  return Promise.resolve(p).catch(() => null)
}

export function useUserProfile() {
  function recordView(viewID, kubeCtx = '', namespace = '') {
    if (!viewID) return
    safe(callGo('RecordView', viewID, kubeCtx, namespace))
  }

  async function pollSuggestion(viewID) {
    try {
      const res = await callGo('GetNextSuggestion', viewID || '')
      suggestion.value = res?.suggestion || null
      suppressed.value = !!res?.suppressed
      suppressionReason.value = res?.reason || ''
      return suggestion.value
    } catch (_) {
      suggestion.value = null
      suppressed.value = false
      suppressionReason.value = ''
      return null
    }
  }

  function mute(muteKey) {
    if (!muteKey) return
    safe(callGo('MuteSuggestion', muteKey))
    // Optimistic local clear — the card disappears immediately;
    // the next poll re-renders from the truth on the backend.
    suggestion.value = null
  }

  function accept(muteKey, kind) {
    safe(callGo('AcceptSuggestion', muteKey || '', kind || ''))
    suggestion.value = null
  }

  function dismiss(muteKey, kind) {
    safe(callGo('DismissSuggestion', muteKey || '', kind || ''))
    suggestion.value = null
  }

  async function clearActivity() {
    try { await callGo('ClearUserActivity'); return true }
    catch { return false }
  }

  function reset() {
    suggestion.value = null
    suppressed.value = false
    suppressionReason.value = ''
  }

  return {
    suggestion,
    suppressed,
    suppressionReason,
    recordView,
    pollSuggestion,
    mute,
    accept,
    dismiss,
    clearActivity,
    reset,
  }
}

export const __test = { _safe: safe }
