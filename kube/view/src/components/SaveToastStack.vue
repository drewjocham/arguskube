<script setup>
import { ref, computed, watch, onMounted, onBeforeUnmount } from 'vue'
import { bus } from '../lib/bus'
import { useSaveActivityStore } from '../stores/saveActivity'
import { useNotificationGuardStore } from '../stores/notificationGuard'
import { useAppNavStore } from '../stores/appNav'

// Top-right transparent toast stack for save outcomes. Listens to the
// saveActivity store (which itself relays from the global `argus:save`
// window event the bridge dispatches). Each toast self-dismisses after
// AUTO_DISMISS_MS; clicking it dismisses early.
//
// Sticky silence banners: when notificationGuard detects spam from a
// source, it fires `watcher:silenced` and we render a *non*-auto-
// dismissing banner the user must acknowledge. The banner has a deep
// link straight to Settings → Watchers & Notifications so they can
// review/adjust before clearing it.
//
// "Findable" half: every event also lives in the persistent
// notifications store, so anything that scrolled off the toast stack is
// still visible in the bell panel.

const AUTO_DISMISS_MS = 4000

const store = useSaveActivityStore()
const guard = useNotificationGuardStore()
const appNav = useAppNavStore()
onMounted(() => store.attach())

// Local visible list — mirrors store.events but with per-toast timers we
// own. Matched by id so we can dismiss without disturbing the store
// (which keeps its own ring for in-session debugging).
const visible = ref([])
const timers = new Map()

function show(entry) {
  if (!entry) return
  // Avoid double-renders if the same id arrives via multiple paths.
  if (visible.value.some((v) => v.id === entry.id)) return
  visible.value = [entry, ...visible.value].slice(0, 6)

  const t = setTimeout(() => dismiss(entry.id), AUTO_DISMISS_MS)
  timers.set(entry.id, t)
}

function dismiss(id) {
  visible.value = visible.value.filter((v) => v.id !== id)
  const t = timers.get(id)
  if (t) {
    clearTimeout(t)
    timers.delete(id)
  }
}

// React to new entries arriving in the store. We only show the freshest
// one per change — the store is newest-first, so events.value[0].
watch(
  () => store.events[0]?.id,
  (id) => {
    if (!id) return
    show(store.events[0])
  },
)

// Sticky silence banners — survive AUTO_DISMISS_MS, must be clicked to
// acknowledge. Source maps 1:1 to a guard.silences[source] entry.
const silenceBanners = ref([])

let unsubSilenced = null
onMounted(() => {
  unsubSilenced = bus.on('watcher:silenced', (detail) => {
    if (!detail || !detail.source) return
    if (silenceBanners.value.some((b) => b.source === detail.source)) return
    silenceBanners.value = [{ ...detail }, ...silenceBanners.value].slice(0, 4)
  })
})

function silenceMeta(source) {
  return guard.silences[source] || null
}

function ackSilence(source) {
  guard.acknowledge(source)
  silenceBanners.value = silenceBanners.value.filter((b) => b.source !== source)
}

function unsilenceFromBanner(source) {
  guard.unsilence(source)
  silenceBanners.value = silenceBanners.value.filter((b) => b.source !== source)
}

// Two-way deep-link: jump the user to Settings → Watchers & Notifications.
// Pass a returnTo so wherever they came from gets a Go-back banner.
function configureSilence(source) {
  appNav.requestNav({
    navId: 'settings',
    anchor: 'watchers-notifications',
    returnTo: { navId: 'alerts', anchor: 'silence:' + source, label: 'Alerts' },
  })
}

function fmtUntil(iso) {
  if (!iso) return ''
  try { return new Date(iso).toLocaleString() } catch { return iso }
}

onBeforeUnmount(() => {
  for (const t of timers.values()) clearTimeout(t)
  timers.clear()
  unsubSilenced?.()
})

function fmtDuration(ms) {
  if (!Number.isFinite(ms) || ms <= 0) return ''
  if (ms < 1000) return ms + ' ms'
  return (ms / 1000).toFixed(ms < 10000 ? 2 : 1) + ' s'
}
</script>

<template>
  <div class="save-toast-stack" aria-live="polite" aria-relevant="additions">
    <!-- Sticky silence banners. Pinned above regular toasts and never
         auto-dismiss — the user must click Acknowledge or Unsilence. -->
    <transition-group name="save-toast">
      <div
        v-for="b in silenceBanners"
        :key="'silence:' + b.source"
        class="save-toast silence-banner"
        role="alert"
      >
        <span class="save-toast-icon" aria-hidden="true">
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.4" stroke-linecap="round" stroke-linejoin="round">
            <path d="M12 2a7 7 0 0 0-7 7v4l-2 4h18l-2-4V9a7 7 0 0 0-7-7z"></path>
            <line x1="4" y1="4" x2="20" y2="20"></line>
          </svg>
        </span>
        <div class="save-toast-body">
          <div class="save-toast-title">
            {{ silenceMeta(b.source)?.label || b.source }} silenced
          </div>
          <div class="save-toast-sub">
            {{ b.count }} notification{{ b.count === 1 ? '' : 's' }} suppressed.
            Resumes at <span class="mono">{{ fmtUntil(b.silencedUntil) }}</span>.
          </div>
          <div class="silence-actions">
            <button class="silence-btn primary" @click="ackSilence(b.source)">Acknowledge</button>
            <button class="silence-btn" @click="unsilenceFromBanner(b.source)">Unsilence</button>
            <button class="silence-btn link" @click="configureSilence(b.source)">Settings →</button>
          </div>
        </div>
      </div>
    </transition-group>

    <transition-group name="save-toast">
      <div
        v-for="t in visible"
        :key="t.id"
        class="save-toast"
        :class="['kind-' + t.status]"
        role="status"
        @click="dismiss(t.id)"
        :title="t.error || 'Click to dismiss'"
      >
        <span class="save-toast-icon" aria-hidden="true">
          <svg v-if="t.status === 'ok'" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.6" stroke-linecap="round" stroke-linejoin="round">
            <polyline points="20 6 9 17 4 12"></polyline>
          </svg>
          <svg v-else width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.4" stroke-linecap="round" stroke-linejoin="round">
            <circle cx="12" cy="12" r="10"></circle>
            <line x1="12" y1="8" x2="12" y2="13"></line>
            <line x1="12" y1="16.5" x2="12" y2="17"></line>
          </svg>
        </span>
        <div class="save-toast-body">
          <div class="save-toast-title">{{ t.label }}</div>
          <div v-if="t.status === 'error' && t.error" class="save-toast-sub">{{ t.error }}</div>
          <div v-else-if="t.durationMs" class="save-toast-sub">{{ fmtDuration(t.durationMs) }}</div>
        </div>
      </div>
    </transition-group>
  </div>
</template>

<style scoped>
.save-toast-stack {
  position: fixed;
  top: 56px;          /* clears the titlebar/header */
  right: 16px;
  z-index: 1000;
  display: flex;
  flex-direction: column;
  gap: 8px;
  pointer-events: none;   /* let clicks pass through gaps between toasts */
  max-width: 360px;
}

.save-toast {
  pointer-events: auto;
  display: flex;
  align-items: flex-start;
  gap: 9px;
  padding: 9px 12px;
  border-radius: 8px;
  font-size: 12.5px;
  line-height: 1.4;
  cursor: pointer;
  /* Transparent translucent fill — sits over content without obscuring it. */
  backdrop-filter: blur(8px) saturate(140%);
  -webkit-backdrop-filter: blur(8px) saturate(140%);
  background: rgba(20, 22, 28, 0.55);
  color: var(--text);
  box-shadow:
    0 6px 20px rgba(0, 0, 0, 0.28),
    0 0 0 1px rgba(255, 255, 255, 0.06) inset;
}

/* Success: thin green left rim + green check icon. */
.save-toast.kind-ok {
  border-left: 3px solid rgba(62, 207, 142, 0.85);
}
.save-toast.kind-ok .save-toast-icon { color: #3ecf8e; }

/* Error: thin red rim + red ! icon. */
.save-toast.kind-error {
  border-left: 3px solid rgba(240, 84, 84, 0.85);
}
.save-toast.kind-error .save-toast-icon { color: #f05454; }

.save-toast-icon {
  flex-shrink: 0;
  margin-top: 1px;
  width: 14px; height: 14px;
  display: inline-flex; align-items: center; justify-content: center;
}

.save-toast-body { min-width: 0; }
.save-toast-title {
  font-weight: 500;
  color: rgba(255, 255, 255, 0.92);
  word-break: break-word;
}
.save-toast-sub {
  font-size: 11px;
  color: rgba(255, 255, 255, 0.55);
  margin-top: 2px;
  word-break: break-word;
}

/* Slide-in from right; fade-out in place. */
.save-toast-enter-active,
.save-toast-leave-active {
  transition: transform 0.25s cubic-bezier(0.4, 0, 0.2, 1),
              opacity 0.25s ease;
}
.save-toast-enter-from {
  opacity: 0;
  transform: translateX(20px);
}
.save-toast-leave-to {
  opacity: 0;
  transform: translateX(8px);
}
.save-toast-move { transition: transform 0.25s ease; }

/* Silence banner — sticky, must be acknowledged. Amber rim + slow
   pulse so it's distinguishable from the regular save toasts and
   doesn't blend into the page background. */
.save-toast.silence-banner {
  cursor: default;
  border-left: 3px solid rgba(245, 166, 35, 0.85);
  background: rgba(40, 32, 14, 0.65);
  animation: silence-banner-pulse 2.4s ease-in-out infinite;
}
.save-toast.silence-banner .save-toast-icon { color: #f5a623; }
@keyframes silence-banner-pulse {
  0%, 100% { box-shadow: 0 6px 20px rgba(0,0,0,0.28), 0 0 0 0 rgba(245,166,35,0); }
  50%      { box-shadow: 0 6px 20px rgba(0,0,0,0.28), 0 0 0 4px rgba(245,166,35,0.2); }
}

.silence-actions {
  display: flex; gap: 6px; margin-top: 8px; flex-wrap: wrap;
}
.silence-btn {
  background: transparent; border: 1px solid rgba(255,255,255,0.18);
  color: rgba(255,255,255,0.85); padding: 3px 9px; border-radius: 4px;
  font-size: 10.5px; cursor: pointer;
  transition: border-color 0.12s, color 0.12s, background 0.12s;
}
.silence-btn:hover { border-color: #f5a623; color: #fff; background: rgba(245,166,35,0.1); }
.silence-btn.primary {
  background: rgba(245,166,35,0.2); border-color: rgba(245,166,35,0.55); color: #f5a623;
}
.silence-btn.primary:hover { background: rgba(245,166,35,0.3); color: #fff; }
.silence-btn.link {
  border-color: transparent; padding: 3px 4px;
  color: #4f8ef7; text-decoration: underline;
}
.silence-btn.link:hover { background: transparent; color: #6ea7fb; }
</style>
