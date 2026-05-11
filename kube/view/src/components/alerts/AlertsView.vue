<script setup>
import { ref, computed, onMounted, watch, nextTick } from 'vue'
import { useNotificationGuardStore } from '../../stores/notificationGuard'
import { useWatcherRegistryStore } from '../../stores/watcherRegistry'
import { useNotificationsStore } from '../../stores/notifications'
import { useAppNavStore } from '../../stores/appNav'
import { runDueNow as watcherRunDueNow, runWatcherById } from '../../composables/useWatcherEngine'

// AlertsView — the single dashboard for everything notifications-related.
//
// Why this view exists separately from Settings → Watchers & Notifications:
// the user wanted "alerts views found in the left main menu panel" so they
// can manage silences/registered watchers without diving into Settings. The
// underlying state is the same — `notificationGuard` for silences and
// `watcherRegistry` for the registered probes — but this view is action-
// focused (silence, unsilence, acknowledge, re-check) rather than
// configuration-focused. Heavy threshold/window tuning still lives in
// Settings (linked from here).
//
// Three sections, in priority order:
//   1. Active silences (with un-acknowledged spam silences pulsing)
//   2. Registered watchers (status + Re-check + Silence)
//   3. Recent alerts log (read-only, mirror of the bell)
//
// Deep-link anchors:
//   silence:<source>   — scroll & flash the silence row for that source
//   watcher:<id>       — scroll & flash the registered-watcher row for that id

const guard = useNotificationGuardStore()
const registry = useWatcherRegistryStore()
const notif = useNotificationsStore()
const appNav = useAppNavStore()

const watchersRunning = ref(false)
const watcherRunningId = ref('')
const focusedAnchor = ref('') // anchor we're currently flashing, e.g. "silence:credential:github"

async function runAllWatchersNow() {
  watchersRunning.value = true
  try { await watcherRunDueNow({ force: true }) }
  finally { watchersRunning.value = false }
}

async function runOneWatcher(id) {
  watcherRunningId.value = id
  try { await runWatcherById(id) }
  finally { watcherRunningId.value = '' }
}

function watcherStatusOf(w) {
  return registry.results[w.id]?.status || ''
}

function fmtLocal(epochMs) {
  if (!epochMs) return ''
  try { return new Date(epochMs).toLocaleString() } catch { return String(epochMs) }
}
function fmtRelative(epochMs) {
  if (!epochMs) return ''
  const diff = Date.now() - epochMs
  if (diff < 60_000) return 'just now'
  if (diff < 3_600_000) return Math.round(diff / 60_000) + ' min ago'
  if (diff < 86_400_000) return Math.round(diff / 3_600_000) + ' h ago'
  return fmtLocal(epochMs)
}

// Two-way deep link to Settings → Watchers & Notifications. Clicking
// "Configure" remembers we came from Alerts so the SettingsPanel can
// render a "Go back to Alerts" affordance.
function jumpToWatcherSettings(source) {
  appNav.requestNav({
    navId: 'settings',
    anchor: 'watchers-notifications',
    returnTo: {
      navId: 'alerts',
      anchor: source ? 'silence:' + source : '',
      label: 'Alerts',
    },
  })
}

// On mount + when appNav posts a fresh pending nav for us, consume an
// anchor like "silence:<src>" or "watcher:<id>" and scroll/flash the row.
function applyPendingAnchor() {
  const req = appNav.pending
  if (!req || req.navId !== 'alerts' || !req.anchor) return
  focusedAnchor.value = req.anchor
  appNav.consumeNav()
  nextTick(() => {
    const el = document.getElementById('alerts-anchor:' + focusedAnchor.value)
    if (el && typeof el.scrollIntoView === 'function') {
      el.scrollIntoView({ behavior: 'smooth', block: 'center' })
    }
    setTimeout(() => { focusedAnchor.value = '' }, 4000)
  })
}

onMounted(applyPendingAnchor)
watch(() => appNav.pending, applyPendingAnchor)

// Recent alerts log — read-only mirror of the notifications store filtered
// to watcher-driven entries. We don't try to be exhaustive; the bell panel
// is the canonical history.
// notifications store uses sortedItems (newest-first) + createdAt ISO. We
// only surface the most recent slice; the bell panel is the canonical
// history.
const recentAlerts = computed(() =>
  (notif.sortedItems || []).slice(0, 25).map((n) => ({
    id: n.id,
    kind: n.kind,
    title: n.title,
    body: n.body,
    timestamp: n.createdAt ? Date.parse(n.createdAt) : 0,
  }))
)

// Manual alert creation
const showManualAlert = ref(false)
const manualAlertForm = ref({ title: '', severity: 'warning', message: '' })

function openManualAlert() {
  manualAlertForm.value = { title: '', severity: 'warning', message: '' }
  showManualAlert.value = true
}

function submitManualAlert() {
  const { title, severity, message } = manualAlertForm.value
  if (!title.trim()) return
  notif.add({
    kind: severity === 'critical' ? 'error' : severity === 'warning' ? 'warn' : 'info',
    title: title.trim(),
    body: message.trim() || 'Manual alert',
  })
  showManualAlert.value = false
}

const stats = computed(() => {
  const watchers = registry.list
  const okCount = watchers.filter((w) => {
    const s = watcherStatusOf(w)
    return s === 'ok' || s === 'valid' || s === 'present' || s === 'healthy'
  }).length
  const failingCount = watchers.length - okCount
  const silenceCount = guard.activeSilences.length
  const pendingAckCount = guard.pendingAcks.length
  return { total: watchers.length, okCount, failingCount, silenceCount, pendingAckCount }
})

// Manual silence flow for the "Silence…" button row. Default duration
// equals guard.settings.defaultSilenceMs; the +/- buttons step in 15-min
// increments and stay clamped to [1 min, 24 h].
const silenceDraft = ref({}) // { [watcherId]: durationMs }
function defaultSilenceFor(id) {
  return silenceDraft.value[id] ?? guard.settings.defaultSilenceMs
}
function nudgeSilence(id, deltaMs) {
  const cur = defaultSilenceFor(id)
  const next = Math.max(60_000, Math.min(24 * 60 * 60_000, cur + deltaMs))
  silenceDraft.value = { ...silenceDraft.value, [id]: next }
}
function silenceManually(w) {
  guard.silence(w.id, defaultSilenceFor(w.id), {
    label: w.label,
    anchor: w.configureAnchor || 'watchers-notifications',
    reason: 'manual',
  })
}

function fmtDuration(ms) {
  if (!Number.isFinite(ms) || ms <= 0) return '—'
  if (ms < 60_000) return Math.round(ms / 1000) + 's'
  if (ms < 60 * 60_000) return Math.round(ms / 60_000) + 'm'
  if (ms < 24 * 60 * 60_000) {
    const h = Math.floor(ms / 3_600_000)
    const m = Math.round((ms % 3_600_000) / 60_000)
    return m === 0 ? h + 'h' : h + 'h ' + m + 'm'
  }
  return Math.round(ms / 86_400_000) + 'd'
}
</script>

<template>
  <div class="alerts-view">
    <div class="alerts-header">
      <div class="alerts-title-row">
        <h2 class="alerts-title">Alerts &amp; Silences</h2>
        <div class="alerts-actions">
          <button class="alerts-btn" @click="jumpToWatcherSettings('')">
            Configure thresholds →
          </button>
          <button class="alerts-btn" @click="openManualAlert">
            + Create Alert
          </button>
          <button class="alerts-btn primary" @click="runAllWatchersNow" :disabled="watchersRunning">
            {{ watchersRunning ? 'Re-checking…' : 'Re-check all watchers' }}
          </button>
        </div>
      </div>
      <div class="alerts-summary">
        <div class="summary-pill" data-tone="ok">
          <div class="summary-num">{{ stats.okCount }}</div>
          <div class="summary-lbl">healthy</div>
        </div>
        <div class="summary-pill" :data-tone="stats.failingCount ? 'fail' : 'idle'">
          <div class="summary-num">{{ stats.failingCount }}</div>
          <div class="summary-lbl">failing</div>
        </div>
        <div class="summary-pill" :data-tone="stats.silenceCount ? 'warn' : 'idle'">
          <div class="summary-num">{{ stats.silenceCount }}</div>
          <div class="summary-lbl">silenced</div>
        </div>
        <div class="summary-pill" :data-tone="stats.pendingAckCount ? 'unack' : 'idle'">
          <div class="summary-num">{{ stats.pendingAckCount }}</div>
          <div class="summary-lbl">need ack</div>
        </div>
      </div>
    </div>

    <div class="alerts-scroll">
      <!-- Active silences ----------------------------------------------- -->
      <div class="alerts-section">
        <div class="section-bar">
          <h3 class="section-h">Active silences</h3>
          <span class="section-count" v-if="guard.activeSilences.length">
            {{ guard.activeSilences.length }}
          </span>
        </div>

        <div v-if="!guard.activeSilences.length" class="empty-card">
          No silences. Anything noisy that auto-silences (or that you silence
          manually) will appear here.
        </div>

        <div v-else class="silence-list">
          <div
            v-for="s in guard.activeSilences"
            :key="s.source"
            :id="'alerts-anchor:silence:' + s.source"
            class="silence-card"
            :class="{
              unack: !s.acknowledged,
              focused: focusedAnchor === 'silence:' + s.source,
            }"
            :data-reason="s.reason"
          >
            <div class="silence-card-head">
              <div class="silence-card-title">
                <span class="silence-reason-badge" :data-reason="s.reason">{{ s.reason }}</span>
                <span class="silence-card-label">{{ s.label }}</span>
              </div>
              <div class="silence-card-meta mono">
                resumes <span class="meta-strong">{{ fmtLocal(s.until) }}</span>
                <template v-if="s.pendingCount"> · {{ s.pendingCount }} suppressed</template>
              </div>
            </div>
            <div class="silence-card-actions">
              <button v-if="!s.acknowledged" class="alerts-btn primary"
                @click="guard.acknowledge(s.source)">Acknowledge</button>
              <button class="alerts-btn"
                @click="guard.unsilence(s.source)">Unsilence</button>
              <button class="alerts-btn link"
                @click="jumpToWatcherSettings(s.source)">Configure →</button>
            </div>
          </div>
        </div>
      </div>

      <!-- Registered watchers ------------------------------------------ -->
      <div class="alerts-section">
        <div class="section-bar">
          <h3 class="section-h">Watchers</h3>
          <span class="section-count" v-if="registry.list.length">
            {{ registry.list.length }}
          </span>
        </div>

        <div v-if="!registry.list.length" class="empty-card">
          No watchers registered yet. Each feature that has something
          expirable (credentials, certs, license keys, …) registers itself
          on first load.
        </div>

        <div v-else class="watcher-list">
          <div
            v-for="w in registry.list"
            :key="w.id"
            :id="'alerts-anchor:watcher:' + w.id"
            class="watcher-card"
            :data-status="watcherStatusOf(w) || 'pending'"
            :class="{
              focused: focusedAnchor === 'watcher:' + w.id,
              disabled: !w.enabled,
            }"
          >
            <div class="watcher-card-main">
              <div class="watcher-card-title">
                <span class="watcher-kind-pill">{{ w.kind }}</span>
                <span class="watcher-card-label">{{ w.label }}</span>
                <span class="watcher-status-pill" :data-status="watcherStatusOf(w) || 'pending'">
                  {{ watcherStatusOf(w) || 'pending' }}
                </span>
              </div>
              <div class="watcher-card-msg">
                <template v-if="registry.results[w.id]?.message">{{ registry.results[w.id].message }}</template>
                <template v-else>No probe result yet.</template>
              </div>
              <div class="watcher-card-foot mono">
                every {{ fmtDuration(w.intervalMs) }}
                <template v-if="registry.lastCheckedAt[w.id]"> · last check {{ fmtRelative(registry.lastCheckedAt[w.id]) }}</template>
                <template v-else> · never checked</template>
              </div>
            </div>
            <div class="watcher-card-actions">
              <button class="alerts-btn"
                @click="runOneWatcher(w.id)"
                :disabled="!w.enabled || watcherRunningId === w.id">
                {{ watcherRunningId === w.id ? 'Running…' : 'Re-check' }}
              </button>
              <div v-if="!guard.silences[w.id]" class="silence-stepper">
                <button class="step-btn" @click="nudgeSilence(w.id, -15 * 60_000)" title="−15m">−</button>
                <span class="step-val mono">{{ fmtDuration(defaultSilenceFor(w.id)) }}</span>
                <button class="step-btn" @click="nudgeSilence(w.id, 15 * 60_000)" title="+15m">+</button>
                <button class="alerts-btn warn" @click="silenceManually(w)">Silence</button>
              </div>
              <button v-else class="alerts-btn primary" @click="guard.unsilence(w.id)">Unsilence</button>
            </div>
          </div>
        </div>
      </div>

      <!-- Recent alerts log -------------------------------------------- -->
      <div class="alerts-section">
        <div class="section-bar">
          <h3 class="section-h">Recent alerts</h3>
          <span class="section-count" v-if="recentAlerts.length">
            {{ recentAlerts.length }}
          </span>
        </div>

        <div v-if="!recentAlerts.length" class="empty-card">
          No alerts yet. The bell shows the canonical history; this list
          mirrors the most recent entries.
        </div>

        <div v-else class="recent-list">
          <div
            v-for="n in recentAlerts"
            :key="n.id"
            class="recent-row"
            :data-kind="n.kind"
          >
            <div class="recent-dot" :data-kind="n.kind"></div>
            <div class="recent-body">
              <div class="recent-title">{{ n.title }}</div>
              <div v-if="n.body" class="recent-sub">{{ n.body }}</div>
            </div>
            <div class="recent-time mono" v-if="n.timestamp">{{ fmtRelative(n.timestamp) }}</div>
          </div>
        </div>
      </div>
    </div>

    <!-- Manual Alert Modal -->
    <div v-if="showManualAlert" class="alerts-modal-backdrop" @click.self="showManualAlert = false">
      <div class="alerts-modal">
        <div class="alerts-modal-header">
          <h3>Create Manual Alert</h3>
          <button class="modal-close" @click="showManualAlert = false">×</button>
        </div>
        <div class="alerts-modal-body">
          <label class="manual-field">
            <span>Title</span>
            <input v-model="manualAlertForm.title" placeholder="e.g. Production incident" class="ctrl-input" />
          </label>
          <label class="manual-field">
            <span>Severity</span>
            <select v-model="manualAlertForm.severity" class="ctrl-input">
              <option value="info">Info</option>
              <option value="warning">Warning</option>
              <option value="critical">Critical</option>
            </select>
          </label>
          <label class="manual-field">
            <span>Message</span>
            <textarea v-model="manualAlertForm.message" placeholder="Optional details…" class="ctrl-input ctrl-textarea" rows="3"></textarea>
          </label>
        </div>
        <div class="alerts-modal-footer">
          <button class="alerts-btn" @click="showManualAlert = false">Cancel</button>
          <button class="alerts-btn primary" @click="submitManualAlert" :disabled="!manualAlertForm.title.trim()">
            Create Alert
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.alerts-view {
  display: flex;
  flex-direction: column;
  height: 100%;
  min-height: 0;
}

.alerts-header {
  flex-shrink: 0;
  padding: 16px 20px 12px;
  border-bottom: 1px solid var(--border);
  background: var(--bg2);
}
.alerts-title-row { display: flex; align-items: center; gap: 12px; }
.alerts-title {
  font-size: 14px; font-weight: 600;
  color: var(--text);
  margin: 0;
  flex: 1;
}
.alerts-actions { display: flex; gap: 6px; }

.alerts-summary {
  display: flex; gap: 8px; margin-top: 12px; flex-wrap: wrap;
}
.summary-pill {
  display: flex; flex-direction: column; align-items: center;
  min-width: 64px;
  padding: 6px 12px;
  border-radius: 8px;
  background: var(--bg3);
  border: 1px solid var(--border2);
}
.summary-pill[data-tone="ok"]    { border-color: rgba(62,207,142,0.4); }
.summary-pill[data-tone="warn"]  { border-color: rgba(245,166,35,0.4); }
.summary-pill[data-tone="unack"] { border-color: rgba(245,166,35,0.7); animation: silence-flash 2.4s ease-in-out infinite; }
.summary-pill[data-tone="fail"]  { border-color: rgba(240,84,84,0.5); }
.summary-num { font-size: 18px; font-weight: 600; color: var(--text); font-family: var(--mono); }
.summary-lbl { font-size: 9.5px; text-transform: uppercase; letter-spacing: 0.06em; color: var(--text3); margin-top: 2px; }

.alerts-scroll {
  flex: 1; min-height: 0;
  overflow-y: auto;
  padding: 16px 20px 24px;
}

.alerts-section { margin-bottom: 22px; }
.section-bar {
  display: flex; align-items: center; gap: 8px;
  margin-bottom: 8px;
}
.section-h {
  font-size: 11px; font-weight: 600;
  text-transform: uppercase; letter-spacing: 0.07em;
  color: var(--text3);
  margin: 0;
}
.section-count {
  font-size: 10px; padding: 1px 6px;
  border-radius: 8px;
  background: var(--bg3);
  border: 1px solid var(--border2);
  color: var(--text2);
  font-family: var(--mono);
}

.empty-card {
  padding: 14px;
  border-radius: 8px;
  background: var(--bg3);
  border: 1px dashed var(--border2);
  font-size: 11.5px;
  color: var(--text3);
}

/* Silence cards */
.silence-list { display: flex; flex-direction: column; gap: 8px; }
.silence-card {
  padding: 10px 12px;
  border-radius: 8px;
  background: var(--bg3);
  border: 1px solid var(--border2);
  border-left: 3px solid rgba(245,166,35,0.5);
  display: flex; align-items: center; gap: 12px;
  transition: border-color 0.15s, background 0.15s;
}
.silence-card[data-reason="manual"] { border-left-color: rgba(79,142,247,0.6); }
.silence-card[data-reason="argus"]  { border-left-color: rgba(167,139,250,0.6); }
.silence-card.unack {
  background: rgba(40,32,14,0.55);
  border-left-color: rgba(245,166,35,0.95);
  animation: silence-flash 2.4s ease-in-out infinite;
}
.silence-card.focused {
  outline: 2px solid var(--accent);
  outline-offset: 2px;
}
@keyframes silence-flash {
  0%, 100% { box-shadow: 0 0 0 0 rgba(245,166,35,0); }
  50%      { box-shadow: 0 0 0 4px rgba(245,166,35,0.18); }
}
.silence-card-head { flex: 1; min-width: 0; }
.silence-card-title {
  display: flex; align-items: center; gap: 6px;
  margin-bottom: 4px;
}
.silence-card-label {
  font-size: 12.5px; font-weight: 500; color: var(--text);
  word-break: break-word;
}
.silence-reason-badge {
  font-size: 9px; font-weight: 600;
  text-transform: uppercase; letter-spacing: 0.07em;
  padding: 1px 6px; border-radius: 4px;
  background: rgba(245,166,35,0.15); color: var(--amber);
}
.silence-reason-badge[data-reason="manual"] { background: rgba(79,142,247,0.15); color: var(--accent2); }
.silence-reason-badge[data-reason="argus"]  { background: rgba(167,139,250,0.15); color: var(--purple); }
.silence-card-meta {
  font-size: 11px; color: var(--text3);
}
.meta-strong { color: var(--text2); }
.silence-card-actions { display: flex; gap: 6px; flex-wrap: wrap; }

/* Watcher cards */
.watcher-list { display: flex; flex-direction: column; gap: 8px; }
.watcher-card {
  padding: 10px 12px;
  border-radius: 8px;
  background: var(--bg3);
  border: 1px solid var(--border2);
  border-left: 3px solid var(--text3);
  display: flex; gap: 12px; align-items: flex-start;
  transition: border-color 0.15s;
}
.watcher-card[data-status="ok"],
.watcher-card[data-status="valid"],
.watcher-card[data-status="present"],
.watcher-card[data-status="healthy"] { border-left-color: var(--green); }
.watcher-card[data-status="warn"]                { border-left-color: var(--amber); }
.watcher-card[data-status="error"],
.watcher-card[data-status="expired"],
.watcher-card[data-status="invalid"]             { border-left-color: var(--red); }
.watcher-card.disabled { opacity: 0.55; }
.watcher-card.focused {
  outline: 2px solid var(--accent);
  outline-offset: 2px;
}
.watcher-card-main { flex: 1; min-width: 0; }
.watcher-card-title {
  display: flex; align-items: center; gap: 8px;
  margin-bottom: 4px;
  flex-wrap: wrap;
}
.watcher-kind-pill {
  font-size: 9px; font-weight: 600;
  text-transform: uppercase; letter-spacing: 0.07em;
  padding: 1px 6px; border-radius: 4px;
  background: var(--bg4); color: var(--text3);
}
.watcher-card-label {
  font-size: 12.5px; font-weight: 500; color: var(--text);
  word-break: break-word;
}
.watcher-status-pill {
  font-size: 9.5px; font-weight: 600;
  text-transform: uppercase; letter-spacing: 0.06em;
  padding: 1px 6px; border-radius: 4px;
  background: var(--bg4); color: var(--text3);
  margin-left: auto;
}
.watcher-status-pill[data-status="ok"],
.watcher-status-pill[data-status="valid"],
.watcher-status-pill[data-status="present"],
.watcher-status-pill[data-status="healthy"] { color: var(--green); background: rgba(62,207,142,0.12); }
.watcher-status-pill[data-status="warn"]                { color: var(--amber); background: rgba(245,166,35,0.12); }
.watcher-status-pill[data-status="error"],
.watcher-status-pill[data-status="expired"],
.watcher-status-pill[data-status="invalid"]             { color: var(--red); background: rgba(240,84,84,0.12); }
.watcher-card-msg {
  font-size: 11.5px; color: var(--text2);
  margin-bottom: 4px;
  word-break: break-word;
}
.watcher-card-foot {
  font-size: 10.5px; color: var(--text3);
}
.watcher-card-actions {
  display: flex; flex-direction: column; gap: 6px; align-items: flex-end;
  flex-shrink: 0;
}
.silence-stepper {
  display: flex; align-items: center; gap: 4px;
  background: var(--bg2);
  border: 1px solid var(--border2);
  border-radius: 6px;
  padding: 2px;
}
.step-btn {
  background: transparent; border: 0; color: var(--text2);
  width: 18px; height: 18px;
  font-size: 12px; line-height: 1;
  cursor: pointer; border-radius: 3px;
  transition: background 0.1s;
}
.step-btn:hover { background: var(--bg4); color: var(--text); }
.step-val {
  font-size: 10.5px; color: var(--text2);
  min-width: 36px; text-align: center;
}

/* Buttons (shared) */
.alerts-btn {
  background: transparent;
  border: 1px solid var(--border2);
  color: var(--text2);
  padding: 4px 10px;
  border-radius: 5px;
  font-size: 11px;
  cursor: pointer;
  transition: border-color 0.12s, color 0.12s, background 0.12s;
}
.alerts-btn:hover:not(:disabled) {
  border-color: var(--accent);
  color: var(--text);
  background: rgba(79,142,247,0.06);
}
.alerts-btn:disabled { opacity: 0.45; cursor: not-allowed; }
.alerts-btn.primary {
  background: rgba(79,142,247,0.18);
  border-color: rgba(79,142,247,0.55);
  color: var(--accent2);
}
.alerts-btn.primary:hover:not(:disabled) {
  background: rgba(79,142,247,0.3);
  color: #fff;
}
.alerts-btn.warn {
  background: rgba(245,166,35,0.18);
  border-color: rgba(245,166,35,0.55);
  color: var(--amber);
}
.alerts-btn.warn:hover:not(:disabled) {
  background: rgba(245,166,35,0.3);
  color: #fff;
}
.alerts-btn.link {
  border-color: transparent;
  color: var(--accent2);
  text-decoration: underline;
  padding: 4px 4px;
}
.alerts-btn.link:hover:not(:disabled) {
  background: transparent;
  color: var(--accent);
}

/* Recent alerts */
.recent-list { display: flex; flex-direction: column; }
.recent-row {
  display: flex; align-items: flex-start; gap: 10px;
  padding: 8px 4px;
  border-bottom: 1px solid var(--border);
}
.recent-row:last-child { border-bottom: 0; }
.recent-dot {
  width: 6px; height: 6px; margin-top: 6px;
  border-radius: 50%;
  background: var(--text3);
  flex-shrink: 0;
}
.recent-dot[data-kind="error"] { background: var(--red); }
.recent-dot[data-kind="warn"]  { background: var(--amber); }
.recent-dot[data-kind="info"]  { background: var(--accent); }
.recent-body { flex: 1; min-width: 0; }
.recent-title {
  font-size: 12px; color: var(--text); font-weight: 500;
  word-break: break-word;
}
.recent-sub {
  font-size: 11px; color: var(--text3); margin-top: 1px;
  word-break: break-word;
}
.recent-time {
  font-size: 10px; color: var(--text3); flex-shrink: 0;
}

/* Manual Alert Modal */
.alerts-modal-backdrop {
  position: fixed; inset: 0; z-index: 100;
  background: rgba(0,0,0,0.5);
  display: flex; align-items: center; justify-content: center;
  padding: 24px;
}
.alerts-modal {
  background: var(--bg2); border: 1px solid var(--border); border-radius: 8px;
  width: min(480px, 100%); max-height: 88vh;
  display: flex; flex-direction: column;
  box-shadow: 0 16px 40px rgba(0,0,0,0.45);
}
.alerts-modal-header {
  display: flex; justify-content: space-between; align-items: center;
  padding: 12px 16px; border-bottom: 1px solid var(--border);
}
.alerts-modal-header h3 { font-size: 14px; font-weight: 600; color: var(--text); margin: 0; }
.modal-close {
  background: transparent; border: 0; color: var(--text2);
  width: 24px; height: 24px; cursor: pointer; font-size: 18px;
}
.modal-close:hover { color: var(--text); }
.alerts-modal-body {
  padding: 16px; display: flex; flex-direction: column; gap: 12px;
}
.manual-field { display: flex; flex-direction: column; gap: 4px; font-size: 12px; color: var(--text2); }
.ctrl-input { background: var(--bg); border: 1px solid var(--border); color: var(--text); padding: 7px 10px; border-radius: 4px; font-size: 12px; outline: none; }
.ctrl-input:focus { border-color: var(--accent); }
.ctrl-textarea { resize: vertical; font-family: inherit; }
.alerts-modal-footer {
  display: flex; gap: 8px; justify-content: flex-end;
  padding: 10px 16px; border-top: 1px solid var(--border);
}
</style>
