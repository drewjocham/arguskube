<script setup>
import { ref, provide, onMounted, onUnmounted, watch } from 'vue'
import { storeToRefs } from 'pinia'
import { callGo, useAppMode, useClusterInfo, useMetrics, useAlerts, useDiagnostics, useFeatures } from './composables/useWails'
import { useWailsEvent, Events } from './composables/useEvents'
import { bus } from './lib/bus'
import { useTerminalDispatch } from './composables/useTerminalDispatch'
import { useUIPrefsStore } from './stores/uiPrefs'
import { useNotificationsStore } from './stores/notifications'
import { useAuthStore } from './stores/auth'
import { useAppNavStore } from './stores/appNav'
import { useSectionTabsStore } from './stores/sectionTabs'
import { SECTIONS, sectionForTab } from './lib/sectionTabs'
import { useNavVisibilityProbes } from './composables/useNavVisibilityProbes'
import { useCredentialMonitor } from './composables/useCredentialMonitor'
import { useWatcherEngine } from './composables/useWatcherEngine'
import { useArgusAlertContext } from './composables/useArgusAlertContext'
import { useAutoContext } from './composables/useAutoContext'
import { useAutoContextProbe } from './composables/useAutoContextProbe'
import { useEnvProbeWatcher } from './composables/useEnvProbeWatcher'
import { useSecretStore } from './composables/useSecretStore'
import { useUserProfile } from './composables/useUserProfile'
import ArgusSuggestionCard from './components/ai/ArgusSuggestionCard.vue'
import LoginView from './components/auth/LoginView.vue'
import ChatPopOut from './components/ai/ChatPopOut.vue'
import ToastContainer from './components/ToastContainer.vue'
import SaveToastStack from './components/SaveToastStack.vue'
import Titlebar from './components/titlebar/Titlebar.vue'
import Sidebar from './components/sidebar/Sidebar.vue'
import CenterPanel from './components/center/CenterPanel.vue'
import DiagnosticsPanel from './components/diagnostics/DiagnosticsPanel.vue'
import AgentAnalysisNotification from './components/common/AgentAnalysisNotification.vue'
import FeaturePanel from './features/FeaturePanel.vue'
import ProDesktopApp from './components/desktop/ProDesktopApp.vue'
import StatusRibbon from './components/status/StatusRibbon.vue'
import { useStatusFeedStore } from './stores/statusFeed'

// Auth gate — no session means the user only sees LoginView. Once
// signed in (or when the backend reports auth is disabled for local
// dev), isAuthenticated flips to true and the dashboard renders.
const auth = useAuthStore()
const authReady = ref(false)
// macOS Keychain bridge — used by onMounted to migrate any pre-existing
// localStorage session token onto the OS-native secret store.
const secretStore = useSecretStore()

// Smart-defaults probes — reveal optional sidebar sections when their
// matching subsystem is configured (S3 bucket → Knowledge, cluster
// has PVCs → Storage). Fire-and-forget: a slow probe never blocks
// auth from settling, and the UI starts with the 5 core sections.
const navProbes = useNavVisibilityProbes()
onMounted(async () => {
  // /auth/providers tells us whether dev-mode bypass is on, in parallel
  // with restoring any persisted token. Both have to land before we
  // decide which gate to show — otherwise we'd flash LoginView for one
  // frame even when auth is disabled.
  //
  // On Wails desktop we also one-time-migrate any pre-existing
  // localStorage token into the OS Keychain. Idempotent + fire-and-
  // forget — never blocks the gate from settling.
  secretStore.migrateLegacyToken().catch(() => {})
  const tasks = [auth.loadProviders()]
  if (auth.token) tasks.push(auth.restoreSession())
  await Promise.all(tasks)
  authReady.value = true
  // Kick off the probes after auth has settled so they don't fight
  // for the bridge with the auth restore call.
  navProbes.run()
})

// /api/* fetches dispatch this when they get a 401 — the bridge clears
// localStorage so the next render must walk through LoginView again.
function onSessionExpired() {
  auth.logout()
}
let unsubSessionExpired = null
onMounted(() => { unsubSessionExpired = bus.on('argus:session-expired', onSessionExpired) })
onUnmounted(() => { unsubSessionExpired?.() })

const { info: clusterInfo, refresh: refreshClusterInfo } = useClusterInfo()
const { metrics, refresh: refreshMetrics } = useMetrics()
const { alerts, refresh: refreshAlerts } = useAlerts()
const { bundle, loading: diagLoading, error: diagError, diagnose } = useDiagnostics()
const { tier, isAllowed } = useFeatures()
const { mode } = useAppMode()

const logLines = ref([])
const selectedAlert = ref(null)
// activeNav is now a SECTION id (one of the 9 SECTIONS keys). The
// Sidebar emits section ids; CenterPanel routes by section. Legacy
// callers that hand in tab ids (e.g. 'pods') are translated below.
const activeNav = ref('monitoring')

// Translate any incoming nav target into a (section, tab) pair. Returns
// the section id to set on activeNav; if the incoming id was a tab,
// also writes the tab into the sectionTabs store so CenterPanel opens
// directly on that tab.
const sectionTabsStore = useSectionTabsStore()
function navigateTo(navId) {
  if (!navId) return
  if (navId in SECTIONS) {
    activeNav.value = navId
    return
  }
  const sectionId = sectionForTab(navId)
  if (sectionId) {
    sectionTabsStore.setTab(sectionId, navId)
    activeNav.value = sectionId
  }
}

// appNav lets components deep inside the center panel push the user to a
// different sidebar nav (e.g. a "settings" link in a tooltip jumps to
// Settings → Notification Channels). The destination view consumes the
// pending record on its own onMounted.
const appNav = useAppNavStore()
watch(() => appNav.pending, (req) => {
  if (!req || !req.navId) return
  navigateTo(req.navId)
}, { immediate: false })

// One global tick drives every registered watcher (credentials today,
// certs / licenses / refresh tokens later). Each watcher's results route
// through the notificationGuard so dedupe + spam protection apply
// uniformly. useCredentialMonitor populates the registry; useWatcherEngine
// runs the loop.
useCredentialMonitor()
useWatcherEngine()
// Exposes window.argusAlertContext so the AI chat can read live watcher
// + silence state and silence/unsilence things on the user's behalf.
useArgusAlertContext()

// Auto-context resolver. Once per session, ask the backend to probe every
// kubeconfig context, pick the best one, and switch the live client to it.
// The backend emits status events directly into the bottom ribbon — this
// composable just kicks the flow off and refreshes the cluster panel when
// the chosen context is reachable.
const { resolve: resolveAutoContext } = useAutoContext()
// Bridges that resolution into the Settings checklist as a single row.
useAutoContextProbe()
// Subscribes to "argus:envprobe" events (DNS / TLS / clock skew) and
// upserts checklist rows. The backend's StartEnvProbeLoop pushes
// updates every 60s so corp-network changes mid-session are caught.
useEnvProbeWatcher()

// Learning agents (§6): record every navigation into the user-profile
// activity log. The suggester reads from that log and surfaces at most
// one card at a time via <ArgusSuggestionCard>. Pure observation — no
// network round-trip blocks the nav.
const userProfile = useUserProfile()
watch(activeNav, (next) => {
  if (!next) return
  userProfile.recordView(next, clusterInfo.value?.name || '', '')
}, { immediate: true })
watch(() => auth.isAuthenticated, async (isAuth, wasAuth) => {
  if (!isAuth || wasAuth) return
  const r = await resolveAutoContext()
  if (r && r.chosen && r.reachableCount > 0) {
    refreshClusterInfo()
    refreshMetrics()
    refreshAlerts()
  }
}, { immediate: true })

const terminalOpen = ref(false)
const terminalHeight = ref(220)
const popOutOpen = ref(false)
const diagCollapsed = ref(false)

function toggleDiag() {
  diagCollapsed.value = !diagCollapsed.value
}

useWailsEvent(Events.ALERT_UPDATE, (data) => {
  if (data) alerts.value = data
})

useWailsEvent(Events.METRICS_UPDATE, (data) => {
  if (data) metrics.value = data
})

useWailsEvent(Events.LOG_LINE, (data) => {
  if (data) {
    logLines.value.push(data)
    if (logLines.value.length > 200) {
      logLines.value = logLines.value.slice(-200)
    }
  }
})

function onAlertSelect(alert) {
  selectedAlert.value = alert
  if (isAllowed('ai_diagnostics')) {
    diagnose(alert.id)
  }
}

// "Diagnose All" — CenterPanel emits this per-alert in a loop. We
// fire each diagnosis without awaiting (the agent queue handles
// concurrency) but DON'T flip selectedAlert, so the UI doesn't
// rapidly switch right-rail context.
function onDiagnoseAlert(alert) {
  if (!alert || !alert.id) return
  if (!isAllowed('ai_diagnostics')) return
  diagnose(alert.id)
}

function onContextSwitched() {
  refreshClusterInfo()
  refreshMetrics()
  refreshAlerts()
}

function toggleTerminal() {
  // Close pop-out if open — only one PTY at a time.
  if (popOutOpen.value) {
    popOutOpen.value = false
  }
  terminalOpen.value = !terminalOpen.value
}

async function openPopOut() {
  // Spawn a real OS-level second window via a fresh process of this same
  // binary in terminal mode. Falls back to the in-app overlay if the
  // backend method isn't available (e.g. SaaS/web mode where exec doesn't
  // make sense).
  try {
    await callGo('LaunchPopOutTerminal')
    // The dashboard's embedded terminal stays running — the new window has
    // its own PTY, so they don't collide.
  } catch (e) {
    console.warn('[popout] backend launch failed, falling back to overlay:', e)
    terminalOpen.value = false
    popOutOpen.value = true
  }
}

function closePopOut() {
  popOutOpen.value = false
}

// Terminal resize drag.
let dragging = false
let startY = 0
let startHeight = 0

function onDragStart(e) {
  dragging = true
  startY = e.clientY
  startHeight = terminalHeight.value
  document.addEventListener('mousemove', onDragMove)
  document.addEventListener('mouseup', onDragEnd)
  e.preventDefault()
}

function onDragMove(e) {
  if (!dragging) return
  const delta = startY - e.clientY
  terminalHeight.value = Math.max(100, Math.min(500, startHeight + delta))
}

function onDragEnd() {
  dragging = false
  document.removeEventListener('mousemove', onDragMove)
  document.removeEventListener('mouseup', onDragEnd)
}

// Pause background polling when the window is hidden/minimized.
function onVisibilityChange() {
  callGo('SetPaused', document.hidden).catch(() => {})
}
onMounted(() => document.addEventListener('visibilitychange', onVisibilityChange))
onUnmounted(() => document.removeEventListener('visibilitychange', onVisibilityChange))

provide('tier', tier)
provide('isAllowed', isAllowed)

// When another view (e.g. Argus AI chat) requests a command be sent to the
// terminal, make sure the terminal panel is visible so the user can see the
// command land. The actual writing happens inside TerminalView.
const { requestOpen: terminalOpenRequests } = useTerminalDispatch()
watch(terminalOpenRequests, () => {
  if (popOutOpen.value) popOutOpen.value = false
  terminalOpen.value = true
})

const uiPrefs = useUIPrefsStore()
const { chatPopOutOpen } = storeToRefs(uiPrefs)

// Argus notifications: a single Wails event channel so the backend can
// surface spot-check findings, scan results, and async warnings into the
// titlebar bell + dropdown without each emitter needing its own bridge.
//
// Backend usage (from Go):
//     runtime.EventsEmit(ctx, "argus:notification", map[string]any{
//         "kind": "spot-check", "title": "...", "body": "...",
//         "rerunnable": true, "rerunPayload": "...",
//     })
const notifications = useNotificationsStore()
useWailsEvent('argus:notification', (data) => {
  if (!data || typeof data !== 'object') return
  notifications.add(data)
})

// Status feed: backend emits "argus:status" events for envprobe, k8s
// refresh, ai diagnosis, popeye, agent connect, etc. They land in the
// statusFeed ring buffer and surface in the bottom <StatusRibbon>.
const statusFeed = useStatusFeedStore()
useWailsEvent('argus:status', (data) => {
  if (!data || typeof data !== 'object') return
  statusFeed.push(data)
})
</script>

<template>
  <!-- Auth gate. Until restoreSession() finishes we render nothing to
       avoid a flash of either screen; afterwards we either show the
       login view or the real app. The terminal pop-out window is
       allowed through unauthenticated because it's spawned as a
       child of an already-authenticated parent process. -->
  <template v-if="!authReady && mode !== 'terminal'">
    <div class="auth-bootstrap"></div>
  </template>

  <template v-else-if="!auth.isAuthenticated && mode !== 'terminal'">
    <LoginView />
  </template>

  <template v-else-if="mode === 'terminal'">
    <ProDesktopApp standalone @close="window.close()" />
  </template>

  <template v-else>
    <Titlebar :clusterInfo="clusterInfo" @toggle-terminal="toggleTerminal" @pop-out="openPopOut" :terminalOpen="terminalOpen" />
    <div class="app-shell">
    <div class="main">
      <Sidebar
        :clusterInfo="clusterInfo"
        :alerts="alerts"
        :activeNav="activeNav"
        @update:activeNav="navigateTo($event)"
        @context-switched="onContextSwitched"
      />
      <div class="center-area">
        <div class="center-content">
          <CenterPanel
            :metrics="metrics"
            :alerts="alerts"
            :selectedAlert="selectedAlert"
            :logLines="logLines"
            :activeNav="activeNav"
            @select-alert="onAlertSelect"
            @diagnose-all="onDiagnoseAlert"
          />
          <div class="diag-collapse-bar" @click="toggleDiag" :title="diagCollapsed ? 'Show AI panel' : 'Hide AI panel'">
            <svg :class="{ flipped: diagCollapsed }" width="10" height="10" viewBox="0 0 10 10">
              <polyline points="3 2 7 5 3 8" fill="none" stroke="currentColor" stroke-width="1.4" stroke-linecap="round" stroke-linejoin="round"/>
            </svg>
          </div>
          <div v-show="!diagCollapsed" class="diag-stack">
            <ArgusSuggestionCard :activeView="activeNav" />
            <DiagnosticsPanel
              :selectedAlert="selectedAlert"
              :bundle="bundle"
              :loading="diagLoading"
              :error="diagError"
              @diagnose="onAlertSelect"
            />
          </div>
        </div>

        <!-- Terminal panel -->
        <template v-if="terminalOpen">
          <div class="terminal-divider" @mousedown="onDragStart">
            <div class="divider-handle"></div>
          </div>
          <div class="terminal-panel" :style="{ height: terminalHeight + 'px' }">
            <div class="terminal-header">
              <div class="terminal-tabs">
                <div class="terminal-tab active">Terminal</div>
              </div>
              <button class="terminal-close" @click="toggleTerminal" title="Close Terminal">
                <svg width="10" height="10" viewBox="0 0 10 10">
                  <path d="M2 2l6 6M8 2l-6 6" stroke="currentColor" stroke-width="1.3" stroke-linecap="round"/>
                </svg>
              </button>
            </div>
            <FeaturePanel id="terminal" :visible="terminalOpen" />
          </div>
        </template>
      </div>
    </div>
      <StatusRibbon />
    </div>

    <ToastContainer />
    <SaveToastStack />
    <AgentAnalysisNotification />
    <ProDesktopApp v-if="popOutOpen" @close="closePopOut" />
    <ChatPopOut v-if="chatPopOutOpen" />
  </template>
</template>

<style scoped>
.app-shell {
  flex: 1;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.main {
  flex: 1;
  display: flex;
  overflow: hidden;
}

.center-area {
  flex: 1;
  /* min-height: 0 lets this flex item shrink below its content's
     intrinsic height. Without it, a tall child (e.g. Argus AI chat
     with many messages) makes this row grow instead of clipping, and
     the descendant scroll-region's overflow-y never engages. */
  min-height: 0;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.center-content {
  flex: 1;
  /* Same reason — see .center-area. The chat panel's scroll relies on
     this constraint to propagate from the window all the way down. */
  min-height: 0;
  display: flex;
  overflow: hidden;
}

/* Right-rail stack: the suggestion card sits above the diagnostics
   panel so the user sees both without the card pushing diagnostics
   off-screen. DiagnosticsPanel keeps its own height, the card is
   intrinsic height (~110 px when visible). */
.diag-stack {
  display: flex;
  flex-direction: column;
  min-width: 320px;
  max-width: 420px;
  border-left: 1px solid var(--border, #2a2a2a);
  background: var(--bg, #141414);
}
.diag-stack > :deep(.diagnostics-panel) {
  flex: 1;
  min-height: 0;
  border-left: none;
}

/* Terminal panel */
.terminal-divider {
  height: 4px;
  background: var(--border);
  cursor: row-resize;
  flex-shrink: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: background 0.15s;
}
.terminal-divider:hover { background: var(--accent); }
.divider-handle {
  width: 30px;
  height: 2px;
  border-radius: 1px;
  background: var(--text3);
  opacity: 0.4;
}

.terminal-panel {
  display: flex;
  flex-direction: column;
  background: var(--bg);
  flex-shrink: 0;
  overflow: hidden;
}

.terminal-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  height: 30px;
  background: var(--bg2);
  border-top: 1px solid var(--border);
  border-bottom: 1px solid var(--border);
  padding: 0 10px;
  flex-shrink: 0;
}

.terminal-tabs { display: flex; gap: 0; }

.terminal-tab {
  font-size: 11px;
  font-weight: 500;
  color: var(--text2);
  padding: 4px 10px;
  cursor: pointer;
}
.terminal-tab.active { color: var(--text); }

.terminal-close {
  background: none;
  border: none;
  color: var(--text3);
  cursor: pointer;
  padding: 3px;
  border-radius: 3px;
  display: flex;
  transition: all 0.15s;
}
.terminal-close:hover { background: var(--bg4); color: var(--text); }

/* Collapsible AI panel toggle */
.diag-collapse-bar {
  width: 14px;
  flex-shrink: 0;
  background: var(--bg2);
  border-left: 1px solid var(--border);
  border-right: 1px solid var(--border);
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--text3);
  transition: background 0.15s, color 0.15s;
}
.diag-collapse-bar:hover {
  background: var(--bg3);
  color: var(--text);
}
.diag-collapse-bar svg {
  transition: transform 0.2s ease;
}
.diag-collapse-bar svg.flipped {
  transform: rotate(180deg);
}

.auth-bootstrap {
  position: fixed;
  inset: 0;
  background: var(--bg);
}
</style>
