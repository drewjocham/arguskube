// appNav — cross-view navigation requests + return-context plumbing.
//
// Why a store: activeNav lives in App.vue as a local ref, but features deep
// inside center-panel components occasionally need to push the user to a
// different section + remember where they were so the user can be brought
// back. Threading emit chains up through every wrapper is painful; a tiny
// store is much cleaner.
//
// Usage from a deep-nested component:
//
//   const appNav = useAppNavStore()
//
//   // jump to another nav with optional in-page anchor + remember origin
//   appNav.requestNav({
//     navId: 'settings',
//     anchor: 'notification-channels',
//     returnTo: {
//       navId: 'pvcs',
//       anchor: 'pvc-default-data-api',
//       label: 'data-api',
//     },
//   })
//
//   // later, in the destination view's "go back" handler:
//   const ret = appNav.consumeReturn()
//   if (ret) appNav.requestNav(ret)
//
// App.vue watches `pending` and assigns it to its activeNav ref, then calls
// `consumeNav()` so the same target doesn't fire twice.

import { defineStore } from 'pinia'
import { ref } from 'vue'

export const useAppNavStore = defineStore('appNav', () => {
  // pending nav requests are { navId: string, anchor?: string }
  const pending = ref(null)
  // returnContext is { navId, anchor?, label? } — captured at the time the
  // user follows a deep link, consumed when they hit a "Go back" affordance.
  const returnContext = ref(null)

  function requestNav({ navId, anchor = '', returnTo = null }) {
    if (!navId) return
    if (returnTo && returnTo.navId) returnContext.value = { ...returnTo }
    pending.value = { navId, anchor }
  }

  function consumeNav() {
    const v = pending.value
    pending.value = null
    return v
  }

  function consumeReturn() {
    const v = returnContext.value
    returnContext.value = null
    return v
  }

  function peekReturn() {
    return returnContext.value
  }

  function clearReturn() {
    returnContext.value = null
  }

  return { pending, returnContext, requestNav, consumeNav, consumeReturn, peekReturn, clearReturn }
})
