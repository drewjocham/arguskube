import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { callGo, cachedCallGo, invalidateCache, FAST_TTL } from '../composables/useBridge'

// Workspace store — owns the user's third-party integration connections
// (Google Docs/Sheets/Tasks). Tokens never travel
// through this store; the backend keeps them encrypted at rest and
// only the redacted WorkspaceConnectionView shape comes over the wire.
//
// Session-token resolution: the Wails GetSessionToken binding is the
// authoritative source (Keychain-backed). In SaaS/web mode the binding
// is absent and the bridge falls back to the bearer in localStorage —
// the backend will read it from the Authorization header instead, so
// we pass "" for the sessionToken argument and let the HTTP layer
// re-attach the bearer transparently.
//
// OAuth popup contract: startConnect() opens the system browser at the
// returned auth URL. The OAuth callback page is expected to post back
//   { type: 'workspace:complete', service, state, code }
// via window.opener.postMessage. We poll for closed-without-complete
// so the spinner doesn't hang forever if the user cancels the flow.

const OAUTH_TIMEOUT_MS = 5 * 60 * 1000
const POPUP_POLL_MS = 500

function getSessionTokenSync() {
  // The Keychain getter is async, which makes wiring it into every
  // action awkward. We rely on the HTTP fallback to fill it in for
  // SaaS mode; in Wails mode the bound method ignores the arg and
  // reads the session from the in-process state anyway (see
  // app_workspace.go workspaceUserID — it pulls from auth.store).
  try {
    if (typeof localStorage === 'undefined') return ''
    const raw = localStorage.getItem('argus.auth.session')
    if (!raw) return ''
    const parsed = JSON.parse(raw)
    return parsed?.token || ''
  } catch { return '' }
}

export const useWorkspaceStore = defineStore('workspace', () => {
  const services = ref([])
  const connections = ref([])
  const loading = ref(false)
  const error = ref(null)

  // Per-connection channel cache. Keyed by connectionID so flipping between
  // two Slack workspaces doesn't blow away the other's list.
  const slackChannels = ref({})
  const slackLoading = ref(false)
  const slackSendError = ref(null)
  // { text, at: epochMs, channelID } — set on a successful send; the UI
  // shows it as a transient confirmation and the timer below clears it.
  const slackSendStatus = ref(null)
  let slackStatusTimer = null

  // ---------- Phase 3 — Google Chat (shares the same `google` connection) -
  // Mirrors the Slack pattern: per-connection space cache + send-status
  // timer. Spaces are listed via the chat.spaces.readonly scope that
  // Phase 3 added to GoogleProvider — existing connections from Phase 2
  // need a reconnect to pick up the scope, surfaced in the UI.
  const gchatSpaces = ref({})
  const gchatLoading = ref(false)
  const gchatSendError = ref(null)
  const gchatSendStatus = ref(null)
  let gchatStatusTimer = null

  // ---------- Phase 2 — Google Workspace (Docs / Sheets / Tasks) ----------
  // Per-connection caches. Docs+Sheets aren't preloaded today (no list-all
  // backend method), but the maps stay here so the panels can shove
  // recently-touched items into them and share between tab switches.
  const docs = ref({})        // { [connectionID]: Doc[] }
  const sheets = ref({})      // { [connectionID]: Sheet[] }
  const taskLists = ref({})   // { [connectionID]: TaskList[] }
  const tasks = ref({})       // { [`${connectionID}:${listID}`]: Task[] }
  const googleLoading = ref(false)
  const googleError = ref(null)
  // Transient ✓ banner data: { op, at }. Cleared after 4s.
  const googleStatus = ref(null)
  let googleStatusTimer = null

  // ---------- Phase 2 — Google Calendar -----------------------------------
  // Events cache: { [connectionID]: Event[] }. Time-range queries
  // populate this; the panel re-queries when the range changes.
  const calendarEvents = ref({})
  const calendarLoading = ref(false)
  const calendarError = ref(null)
  const calendarStatus = ref(null)
  let calendarStatusTimer = null

  const connectionsByService = computed(() => {
    const out = {}
    for (const c of connections.value) {
      if (!out[c.service]) out[c.service] = []
      out[c.service].push(c)
    }
    return out
  })

  // Convenience getter for the SlackPanel — empty array when none.
  const slackConnections = computed(() =>
    connections.value.filter((c) => c.service === 'slack'),
  )

  // All three Google panels share the same `service: 'google'` connection.
  const googleConnections = computed(() =>
    connections.value.filter((c) => c.service === 'google'),
  )

  async function loadServices() {
    try {
      const result = await cachedCallGo('ListWorkspaceServices', [], FAST_TTL)
      services.value = Array.isArray(result) ? result : []
    } catch (e) {
      error.value = e?.message || String(e)
      services.value = []
    }
  }

  async function loadConnections() {
    loading.value = true
    error.value = null
    try {
      const token = getSessionTokenSync()
      const result = await callGo('ListWorkspaceConnections', token)
      connections.value = Array.isArray(result) ? result : []
    } catch (e) {
      error.value = e?.message || String(e)
    } finally {
      loading.value = false
    }
  }

  async function startConnect(service) {
    error.value = null

    // STAGE 1 — ask the backend for an auth URL. This is the path
    // where a missing/misconfigured client_id silently broke before:
    // the call threw, the caller's catch was empty, store.error was
    // never updated, and the user saw nothing happen at all.
    let auth
    try {
      const token = getSessionTokenSync()
      // The redirectURL is left blank: the backend picks the right
      // loopback listener for desktop builds (or the SaaS callback
      // path in cloud builds). Passing "" keeps the frontend out of
      // that decision.
      auth = await callGo('StartWorkspaceConnect', token, service, '')
    } catch (e) {
      // Two common causes here: (1) provider isn't configured server-
      // side (missing GOOGLE_CLIENT_ID/SECRET) → backend returns a
      // descriptive error string, (2) the session isn't valid →
      // 401. Surface both verbatim — they're already actionable.
      error.value = humanizeStartError(service, e)
      throw e
    }
    if (!auth?.url) {
      error.value = `Could not start the ${service} sign-in flow — the backend did not return a redirect URL. Check that the OAuth client credentials are configured in Settings.`
      throw new Error(error.value)
    }

    // STAGE 2 — open the popup. Browsers will block window.open
    // when it isn't synchronous with a user gesture, or when the
    // user has a popup blocker. Previously a null popup meant the
    // promise waited 5 minutes then rejected with "Connection timed
    // out" — leaving the user clueless. Fail loud and fast.
    const popup = window.open(auth.url, '_blank', 'noopener,noreferrer,width=520,height=720')
    if (!popup) {
      error.value = 'The browser blocked the sign-in pop-up. Allow pop-ups for Argus, then click Connect again.'
      throw new Error(error.value)
    }

    // STAGE 3 — wait for the callback. The callback page posts back
    // one of:
    //   workspace:complete  → success (state/code present)
    //   workspace:error     → upstream failure (consent denied,
    //                          expired state, missing code, etc.)
    // The pre-fix backend only postMessaged on success; failures
    // just rendered HTML in the popup and the parent timed out.
    return await new Promise((resolve, reject) => {
      const start = Date.now()
      let done = false
      const cleanup = () => {
        done = true
        window.removeEventListener('message', onMessage)
        if (poll) clearInterval(poll)
      }
      const onMessage = async (ev) => {
        const d = ev.data
        if (!d || (d.type !== 'workspace:complete' && d.type !== 'workspace:error')) return
        // Service / state must match the attempt we started — drops
        // stale messages from previous attempts in the same tab.
        if (d.type === 'workspace:complete' && (d.service !== service || d.state !== auth.state)) return
        cleanup()
        if (d.type === 'workspace:error') {
          // Backend already gave us an actionable upstream message.
          error.value = d.error || 'The provider rejected the sign-in attempt.'
          try { popup?.close?.() } catch { /* cross-origin close — harmless */ }
          reject(new Error(error.value))
          return
        }
        try {
          const conn = await callGo('CompleteWorkspaceConnect', d.service, d.state, d.code)
          invalidateCache('ListWorkspaceConnections')
          await loadConnections()
          try { popup?.close?.() } catch { /* cross-origin close — harmless */ }
          resolve(conn)
        } catch (e) {
          error.value = humanizeCompleteError(service, e)
          reject(e)
        }
      }
      window.addEventListener('message', onMessage)
      const poll = setInterval(() => {
        if (done) return
        if (Date.now() - start > OAUTH_TIMEOUT_MS) {
          cleanup()
          error.value = 'Connection timed out — please try again. If the pop-up reached the provider, check that the OAuth redirect URI is registered for this service.'
          reject(new Error(error.value))
          return
        }
        if (popup && popup.closed) {
          cleanup()
          // popup.closed=true with no preceding message usually means
          // (a) user actually canceled, (b) cross-origin close detection
          // misfired. Surface a friendly message either way and let
          // the user retry.
          error.value = 'Sign-in window closed before the provider returned. Click Connect to try again.'
          reject(new Error(error.value))
        }
      }, POPUP_POLL_MS)
    })
  }

  // humanizeStartError keeps server-side error strings readable
  // — most "configuration missing" failures come back as plain
  // sentences from the backend and pass through unchanged. The
  // exception is bare "HTTP error! status: NNN" strings from the
  // bridge, which we expand into something a user can act on.
  function humanizeStartError(service, err) {
    const raw = err?.message || String(err || '')
    if (/status:\s*401/.test(raw)) {
      return `You must be signed in to connect ${service}. Sign out and back in, then try again.`
    }
    if (/status:\s*403/.test(raw)) {
      return `Argus refused to start the ${service} sign-in flow. Check that the workspace integration is enabled for your tier.`
    }
    if (/status:\s*5\d\d/.test(raw)) {
      return `Argus backend hit an error starting the ${service} sign-in flow. Check the backend logs for "workspace" entries.`
    }
    return raw || `Could not start the ${service} sign-in flow.`
  }

  // humanizeCompleteError is the same for the second leg — the
  // token-exchange and profile-fetch round-trip. Token-exchange
  // failures (mis-registered redirect URI, invalid_grant, scopes
  // mismatch) come back as descriptive backend errors and pass
  // through unchanged.
  function humanizeCompleteError(service, err) {
    const raw = err?.message || String(err || '')
    if (/invalid_grant|invalid_client/i.test(raw)) {
      return `The ${service} provider rejected the auth code: ${raw}. This usually means the OAuth client credentials or the redirect URI registered with the provider are out of date.`
    }
    return raw || `Could not finish the ${service} sign-in flow.`
  }

  async function disconnect(id) {
    error.value = null
    try {
      const token = getSessionTokenSync()
      await callGo('DeleteWorkspaceConnection', token, id)
      invalidateCache('ListWorkspaceConnections')
      await loadConnections()
    } catch (e) {
      error.value = e?.message || String(e)
      throw e
    }
  }

  async function loadSlackChannels(connectionID) {
    if (!connectionID) return
    slackLoading.value = true
    try {
      const token = getSessionTokenSync()
      const result = await cachedCallGo(
        'ListSlackChannels',
        [token, connectionID],
        FAST_TTL,
      )
      slackChannels.value = {
        ...slackChannels.value,
        [connectionID]: Array.isArray(result) ? result : [],
      }
    } catch (e) {
      error.value = e?.message || String(e)
    } finally {
      slackLoading.value = false
    }
  }

  function clearSlackSendStatus() {
    slackSendError.value = null
    slackSendStatus.value = null
    if (slackStatusTimer) {
      clearTimeout(slackStatusTimer)
      slackStatusTimer = null
    }
  }

  async function sendSlackMessage(connectionID, channelID, text) {
    // Clear any in-flight status so a fast retry doesn't compound timers.
    clearSlackSendStatus()
    try {
      const token = getSessionTokenSync()
      await callGo('SendSlackMessage', token, connectionID, channelID, text)
      slackSendStatus.value = { text, channelID, at: Date.now() }
      slackStatusTimer = setTimeout(() => {
        slackSendStatus.value = null
        slackStatusTimer = null
      }, 4000)
      return true
    } catch (e) {
      slackSendError.value = e?.message || String(e)
      slackStatusTimer = setTimeout(() => {
        slackSendError.value = null
        slackStatusTimer = null
      }, 4000)
      throw e
    }
  }

  // -------------------- Google Chat actions --------------------
  async function loadGChatSpaces(connectionID) {
    if (!connectionID) return
    gchatLoading.value = true
    try {
      const token = getSessionTokenSync()
      const result = await cachedCallGo(
        'ListGoogleChatSpaces',
        [token, connectionID],
        FAST_TTL,
      )
      gchatSpaces.value = {
        ...gchatSpaces.value,
        [connectionID]: Array.isArray(result) ? result : [],
      }
    } catch (e) {
      error.value = e?.message || String(e)
    } finally {
      gchatLoading.value = false
    }
  }

  function clearGChatSendStatus() {
    gchatSendError.value = null
    gchatSendStatus.value = null
    if (gchatStatusTimer) {
      clearTimeout(gchatStatusTimer)
      gchatStatusTimer = null
    }
  }

  async function sendGChatMessage(connectionID, spaceID, text) {
    clearGChatSendStatus()
    try {
      const token = getSessionTokenSync()
      await callGo('SendGoogleChatMessage', token, connectionID, spaceID, text)
      gchatSendStatus.value = { text, spaceID, at: Date.now() }
      gchatStatusTimer = setTimeout(() => {
        gchatSendStatus.value = null
        gchatStatusTimer = null
      }, 4000)
      return true
    } catch (e) {
      gchatSendError.value = e?.message || String(e)
      gchatStatusTimer = setTimeout(() => {
        gchatSendError.value = null
        gchatStatusTimer = null
      }, 4000)
      throw e
    }
  }

  // -------------------- Google: shared status helpers --------------------
  // Centralized so every google action shares one timer slot. Auto-clear
  // mirrors the Slack pattern so the UX feels uniform across panels.
  function clearGoogleStatus() {
    googleError.value = null
    googleStatus.value = null
    if (googleStatusTimer) {
      clearTimeout(googleStatusTimer)
      googleStatusTimer = null
    }
  }

  function _setGoogleStatus(op) {
    googleStatus.value = { op, at: Date.now() }
    if (googleStatusTimer) clearTimeout(googleStatusTimer)
    googleStatusTimer = setTimeout(() => {
      googleStatus.value = null
      googleStatusTimer = null
    }, 4000)
  }

  function _setGoogleError(e) {
    googleError.value = e?.message || String(e)
    if (googleStatusTimer) clearTimeout(googleStatusTimer)
    googleStatusTimer = setTimeout(() => {
      googleError.value = null
      googleStatusTimer = null
    }, 4000)
  }

  // Each action follows the same shape: flip loading, call, set status or
  // error, clear loading. Reads use cachedCallGo; writes hit callGo and
  // invalidate so the next read picks up the fresh data.
  async function _googleCall(method, args, opLabel, { cache = false } = {}) {
    googleLoading.value = true
    googleError.value = null
    try {
      const token = getSessionTokenSync()
      const fn = cache ? cachedCallGo : callGo
      const result = cache
        ? await fn(method, [token, ...args], FAST_TTL)
        : await fn(method, token, ...args)
      if (opLabel) _setGoogleStatus(opLabel)
      return result
    } catch (e) {
      _setGoogleError(e)
      throw e
    } finally {
      googleLoading.value = false
    }
  }

  // -------------------- Docs --------------------
  async function createDoc(connectionID, title, body) {
    const doc = await _googleCall('CreateGoogleDoc', [connectionID, title, body], 'doc-created')
    // Stash into the docs cache so a future "recent" strip can pick it up.
    const list = docs.value[connectionID] || []
    docs.value = { ...docs.value, [connectionID]: [doc, ...list].slice(0, 50) }
    return doc
  }
  async function readDoc(connectionID, docID) {
    return _googleCall('ReadGoogleDoc', [connectionID, docID], null)
  }
  async function appendDoc(connectionID, docID, text) {
    return _googleCall('AppendGoogleDoc', [connectionID, docID, text], 'doc-appended')
  }

  // -------------------- Sheets --------------------
  async function createSheet(connectionID, title) {
    const sheet = await _googleCall('CreateGoogleSheet', [connectionID, title], 'sheet-created')
    const list = sheets.value[connectionID] || []
    sheets.value = { ...sheets.value, [connectionID]: [sheet, ...list].slice(0, 50) }
    return sheet
  }
  async function getSheet(connectionID, sheetID) {
    return _googleCall('GetGoogleSheet', [connectionID, sheetID], null)
  }
  async function readSheetRange(connectionID, sheetID, a1Range) {
    return _googleCall('ReadGoogleSheetRange', [connectionID, sheetID, a1Range], null)
  }
  async function writeSheetRange(connectionID, sheetID, a1Range, rows) {
    return _googleCall('WriteGoogleSheetRange', [connectionID, sheetID, a1Range, rows], 'sheet-written')
  }

  // -------------------- Tasks --------------------
  async function loadTaskLists(connectionID) {
    if (!connectionID) return []
    const result = await _googleCall('ListGoogleTaskLists', [connectionID], null, { cache: true })
    const arr = Array.isArray(result) ? result : []
    taskLists.value = { ...taskLists.value, [connectionID]: arr }
    return arr
  }
  async function loadTasks(connectionID, listID) {
    if (!connectionID || !listID) return []
    const result = await _googleCall('ListGoogleTasks', [connectionID, listID], null, { cache: true })
    const arr = Array.isArray(result) ? result : []
    tasks.value = { ...tasks.value, [`${connectionID}:${listID}`]: arr }
    return arr
  }
  async function createTask(connectionID, listID, task) {
    const created = await _googleCall(
      'CreateGoogleTask', [connectionID, listID, task], 'task-created',
    )
    invalidateCache('ListGoogleTasks', getSessionTokenSync(), connectionID, listID)
    const key = `${connectionID}:${listID}`
    tasks.value = { ...tasks.value, [key]: [...(tasks.value[key] || []), created] }
    return created
  }
  async function updateTask(connectionID, listID, taskID, patch) {
    const updated = await _googleCall(
      'UpdateGoogleTask', [connectionID, listID, taskID, patch], 'task-updated',
    )
    invalidateCache('ListGoogleTasks', getSessionTokenSync(), connectionID, listID)
    const key = `${connectionID}:${listID}`
    const list = (tasks.value[key] || []).map((t) => (t.id === taskID ? { ...t, ...updated } : t))
    tasks.value = { ...tasks.value, [key]: list }
    return updated
  }
  async function deleteTask(connectionID, listID, taskID) {
    await _googleCall('DeleteGoogleTask', [connectionID, listID, taskID], 'task-deleted')
    invalidateCache('ListGoogleTasks', getSessionTokenSync(), connectionID, listID)
    const key = `${connectionID}:${listID}`
    tasks.value = {
      ...tasks.value,
      [key]: (tasks.value[key] || []).filter((t) => t.id !== taskID),
    }
  }

  // -------------------- Calendar -------------------------------------------
  function clearCalendarStatus() {
    calendarError.value = null
    calendarStatus.value = null
    if (calendarStatusTimer) {
      clearTimeout(calendarStatusTimer)
      calendarStatusTimer = null
    }
  }
  async function _calendarCall(method, args, opLabel) {
    calendarLoading.value = true
    calendarError.value = null
    try {
      const token = getSessionTokenSync()
      const result = await callGo(method, token, ...args)
      if (opLabel) {
        calendarStatus.value = { op: opLabel, at: Date.now() }
        if (calendarStatusTimer) clearTimeout(calendarStatusTimer)
        calendarStatusTimer = setTimeout(() => {
          calendarStatus.value = null
          calendarStatusTimer = null
        }, 4000)
      }
      return result
    } catch (e) {
      calendarError.value = e?.message || String(e)
      if (calendarStatusTimer) clearTimeout(calendarStatusTimer)
      calendarStatusTimer = setTimeout(() => {
        calendarError.value = null
        calendarStatusTimer = null
      }, 4000)
      throw e
    } finally {
      calendarLoading.value = false
    }
  }
  async function loadCalendarEvents(connectionID, start, end) {
    if (!connectionID) return []
    const result = await _calendarCall(
      'ListGoogleCalendarEvents', [connectionID, start || '', end || ''], null,
    )
    const arr = Array.isArray(result) ? result : []
    calendarEvents.value = { ...calendarEvents.value, [connectionID]: arr }
    return arr
  }
  async function createCalendarEvent(connectionID, ev) {
    const created = await _calendarCall(
      'CreateGoogleCalendarEvent', [connectionID, ev], 'event-created',
    )
    const list = calendarEvents.value[connectionID] || []
    calendarEvents.value = { ...calendarEvents.value, [connectionID]: [...list, created] }
    return created
  }
  async function updateCalendarEvent(connectionID, eventID, ev) {
    const updated = await _calendarCall(
      'UpdateGoogleCalendarEvent', [connectionID, eventID, ev], 'event-updated',
    )
    const list = (calendarEvents.value[connectionID] || []).map(
      (e) => (e.id === eventID ? { ...e, ...updated } : e),
    )
    calendarEvents.value = { ...calendarEvents.value, [connectionID]: list }
    return updated
  }
  async function deleteCalendarEvent(connectionID, eventID) {
    await _calendarCall(
      'DeleteGoogleCalendarEvent', [connectionID, eventID], 'event-deleted',
    )
    calendarEvents.value = {
      ...calendarEvents.value,
      [connectionID]: (calendarEvents.value[connectionID] || []).filter(
        (e) => e.id !== eventID,
      ),
    }
  }

  // -------------------- iCloud ---------------------------------------------
  const icloudConnections = computed(() =>
    connections.value.filter((c) => c.service === 'icloud'),
  )
  const icloudLoading = ref(false)
  const icloudError = ref(null)
  const icloudStatus = ref(null)
  let icloudStatusTimer = null
  const icloudNotes = ref([])
  const icloudReminders = ref([])

  function clearICloudStatus() {
    icloudError.value = null
    icloudStatus.value = null
    if (icloudStatusTimer) {
      clearTimeout(icloudStatusTimer)
      icloudStatusTimer = null
    }
  }
  async function connectICloud(appleID, appPassword) {
    icloudLoading.value = true
    icloudError.value = null
    try {
      const token = getSessionTokenSync()
      const conn = await callGo('ConnectICloud', token, appleID, appPassword)
      invalidateCache('ListWorkspaceConnections')
      await loadConnections()
      icloudStatus.value = { op: 'icloud-connected', at: Date.now() }
      icloudStatusTimer = setTimeout(() => {
        icloudStatus.value = null
        icloudStatusTimer = null
      }, 4000)
      return conn
    } catch (e) {
      icloudError.value = e?.message || String(e)
      throw e
    } finally {
      icloudLoading.value = false
    }
  }

  return {
    services,
    connections,
    loading,
    error,
    connectionsByService,
    slackChannels,
    slackLoading,
    slackSendError,
    slackSendStatus,
    slackConnections,
    loadServices,
    loadConnections,
    startConnect,
    disconnect,
    loadSlackChannels,
    sendSlackMessage,
    clearSlackSendStatus,
    // Phase 3 — Google Chat
    gchatSpaces,
    gchatLoading,
    gchatSendError,
    gchatSendStatus,
    loadGChatSpaces,
    sendGChatMessage,
    clearGChatSendStatus,
    // Phase 2 — Google
    docs,
    sheets,
    taskLists,
    tasks,
    googleLoading,
    googleError,
    googleStatus,
    googleConnections,
    createDoc,
    readDoc,
    appendDoc,
    createSheet,
    getSheet,
    readSheetRange,
    writeSheetRange,
    loadTaskLists,
    loadTasks,
    createTask,
    updateTask,
    deleteTask,
    clearGoogleStatus,
    // Phase 2 — Calendar
    calendarEvents,
    calendarLoading,
    calendarError,
    calendarStatus,
    loadCalendarEvents,
    createCalendarEvent,
    updateCalendarEvent,
    deleteCalendarEvent,
    clearCalendarStatus,
    // Phase 2 — iCloud
    icloudConnections,
    icloudLoading,
    icloudError,
    icloudStatus,
    icloudNotes,
    icloudReminders,
    connectICloud,
    clearICloudStatus,
  }
})
