<script setup>
import { computed, ref, onMounted, onUnmounted, watch, nextTick } from 'vue'
import { storeToRefs } from 'pinia'
import { useStatusFeedStore } from '../../stores/statusFeed'
import { useNotificationsStore } from '../../stores/notifications'

// Status ribbon — pinned to the bottom of the main window. Renders the
// live statusFeed as a right-to-left scrolling strip with a slowly
// pulsing border. Hovering pauses the scroll for 3s. Clicking expands
// the existing NotificationsPanel for scroll-back. The component is
// stateless beyond what's needed to drive the scroll animation —
// every input comes from the statusFeed store.

const HOVER_PAUSE_MS = 3000
const SCROLL_SPEED_PX_PER_S = 40
const SEP = '   ·   '

const feed = useStatusFeedStore()
const notifications = useNotificationsStore()
const { scrollItems, pausedUntil } = storeToRefs(feed)

const trackRef = ref(null)
const offsetPx = ref(0)
const trackWidthPx = ref(0)
const containerWidthPx = ref(0)
const lastFrameTs = ref(0)
const rafHandle = ref(0)

// `paused` follows pausedUntil so a producer-side pause (e.g. for tests
// or future "wait until user reads this" flows) works without DOM events.
const paused = computed(() => Number(pausedUntil.value) > Date.now())

// rendered text is the joined scrollItems message list with a separator.
// We keep a single concatenated string so the marquee animation is one
// element, not N — saves layout work when items churn.
const renderedText = computed(() => {
  if (!scrollItems.value.length) return ''
  return scrollItems.value.map(e => e.message).join(SEP)
})

const latest = computed(() => feed.latest)

// Severity of the most-recently-pushed event drives the 4px left strip.
const severityClass = computed(() => {
  if (!latest.value) return 'sev-idle'
  return `sev-${latest.value.severity}`
})

// aria-live snapshot: screen readers get the newest message as a single
// announcement instead of a sliding text fragment.
const announcement = computed(() => {
  if (!latest.value) return ''
  return `${latest.value.source}: ${latest.value.message}`
})

// Animation loop. We drive offsetPx with requestAnimationFrame so:
//  - prefers-reduced-motion can disable it without leaking timers
//  - paused state takes effect on the next frame
//  - the loop self-stops when there's nothing to scroll
function tick(ts) {
  if (!lastFrameTs.value) lastFrameTs.value = ts
  const dt = (ts - lastFrameTs.value) / 1000
  lastFrameTs.value = ts

  if (!paused.value && trackWidthPx.value > 0) {
    offsetPx.value -= SCROLL_SPEED_PX_PER_S * dt
    // Loop: once the text has fully scrolled off, jump back to the right
    // edge. The track contains the text twice (see template) so the
    // visual wrap is seamless.
    const loopWidth = trackWidthPx.value / 2
    if (loopWidth > 0 && -offsetPx.value >= loopWidth) {
      offsetPx.value += loopWidth
    }
  }
  rafHandle.value = requestAnimationFrame(tick)
}

function startLoop() {
  if (rafHandle.value) return
  if (prefersReducedMotion()) return
  lastFrameTs.value = 0
  rafHandle.value = requestAnimationFrame(tick)
}

function stopLoop() {
  if (rafHandle.value) {
    cancelAnimationFrame(rafHandle.value)
    rafHandle.value = 0
  }
}

function prefersReducedMotion() {
  if (typeof window === 'undefined' || !window.matchMedia) return false
  return window.matchMedia('(prefers-reduced-motion: reduce)').matches
}

// Re-measure widths whenever the text changes so the loop length stays
// honest. Use nextTick + getBoundingClientRect so we read post-layout.
async function measure() {
  await nextTick()
  const track = trackRef.value
  if (!track) return
  // The track holds the message twice. Half its width is one full loop.
  trackWidthPx.value = track.scrollWidth
  containerWidthPx.value = track.parentElement?.clientWidth || 0
}

watch(renderedText, () => { measure() }, { flush: 'post' })

// Hover behavior, per spec: pause for 3s on mouseenter, resume on
// mouseleave (or when the 3s window elapses).
function onMouseEnter() { feed.pauseFor(HOVER_PAUSE_MS) }
function onMouseLeave() { feed.resume() }

// Click anywhere on the ribbon opens the existing notifications panel so
// the user can scroll back. We also mirror new ribbon events into the
// notifications store with kind='status' so the scroll-back shows them.
function onClick() {
  notifications.openPanel()
  feed.setExpanded(true)
}

// Keyboard: Enter opens the panel. Esc closes it.
function onKeydown(e) {
  if (e.key === 'Enter' || e.key === ' ') {
    e.preventDefault()
    onClick()
  } else if (e.key === 'Escape') {
    notifications.closePanel()
    feed.setExpanded(false)
  }
}

// When ribbon events come in, persist them into the notifications store
// too so the panel's scroll-back is comprehensive. We only mirror the
// item that was just pushed, not the whole feed, to avoid duplicates.
watch(latest, (e) => {
  if (!e) return
  notifications.add({
    kind: 'status',
    title: e.source,
    body: e.message,
    meta: { detail: e.detail, severity: e.severity, statusEventId: e.id },
  })
}, { flush: 'post' })

onMounted(() => {
  measure()
  startLoop()
})

onUnmounted(() => {
  stopLoop()
})

// If the OS toggles reduced-motion mid-session we honour it live.
let reducedMotionMQ = null
function onReducedMotionChange(e) {
  if (e.matches) stopLoop()
  else startLoop()
}
onMounted(() => {
  if (typeof window === 'undefined' || !window.matchMedia) return
  reducedMotionMQ = window.matchMedia('(prefers-reduced-motion: reduce)')
  if (reducedMotionMQ.addEventListener) {
    reducedMotionMQ.addEventListener('change', onReducedMotionChange)
  }
})
onUnmounted(() => {
  if (reducedMotionMQ?.removeEventListener) {
    reducedMotionMQ.removeEventListener('change', onReducedMotionChange)
  }
})

const trackStyle = computed(() => ({
  transform: `translateX(${offsetPx.value}px)`,
}))
</script>

<template>
  <div
    class="status-ribbon"
    :class="[severityClass, { 'is-empty': !latest, 'is-paused': paused }]"
    role="status"
    aria-live="polite"
    aria-atomic="false"
    tabindex="0"
    :aria-label="announcement || 'Argus status'"
    data-testid="status-ribbon"
    @mouseenter="onMouseEnter"
    @mouseleave="onMouseLeave"
    @focus="onMouseEnter"
    @blur="onMouseLeave"
    @click="onClick"
    @keydown="onKeydown"
  >
    <span class="sev-strip" aria-hidden="true"></span>

    <div class="viewport">
      <div
        v-if="renderedText"
        ref="trackRef"
        class="track"
        :style="trackStyle"
        aria-hidden="true"
      >
        <!-- The text is duplicated so the right-to-left wrap is seamless. -->
        <span class="text">{{ renderedText }}</span>
        <span class="text">{{ renderedText }}</span>
      </div>
      <div v-else class="idle" aria-hidden="true">Argus is idle</div>
    </div>

    <!-- Screen-reader snapshot of the latest event. We render this in a
         visually-hidden node so AT users get one announcement per event
         instead of mid-scroll fragments. -->
    <span class="sr-only">{{ announcement }}</span>
  </div>
</template>

<style scoped>
.status-ribbon {
  position: relative;
  height: 28px;
  display: flex;
  align-items: center;
  background: var(--bg2, #1a1a1a);
  border-top: 1px solid var(--border, #2a2a2a);
  color: var(--text2, #b8b8b8);
  font-size: 12px;
  user-select: none;
  cursor: pointer;
  outline: none;
  overflow: hidden;
  /* Slow opacity pulse on the border via a box-shadow that simulates a
     1px inset border we can fade independently of the static one above.
     2s cycle, stops at 0.4 and 1.0. */
  animation: ribbon-pulse 2s ease-in-out infinite;
}
.status-ribbon:focus-visible {
  box-shadow: inset 0 0 0 2px var(--accent, #4a9eff);
}

@keyframes ribbon-pulse {
  0%   { box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.06); }
  50%  { box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.18); }
  100% { box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.06); }
}

/* Reduced motion: no pulse, no scroll. The text is rendered statically. */
@media (prefers-reduced-motion: reduce) {
  .status-ribbon { animation: none; }
  .status-ribbon .track { transform: none !important; }
}

.sev-strip {
  width: 4px;
  align-self: stretch;
  flex-shrink: 0;
  background: var(--text3, #5a5a5a);
}
.sev-idle .sev-strip  { background: var(--text3, #5a5a5a); }
.sev-info .sev-strip  { background: var(--text3, #5a5a5a); }
.sev-warn .sev-strip  { background: #d4a256; }
.sev-error .sev-strip { background: #d05a5a; }

.viewport {
  flex: 1;
  position: relative;
  overflow: hidden;
  padding: 0 10px;
  white-space: nowrap;
}

.track {
  display: inline-flex;
  gap: 0;
  will-change: transform;
}
.track .text {
  display: inline-block;
  padding-right: 40px;
  letter-spacing: 0.01em;
}

.idle {
  color: var(--text3, #5a5a5a);
  font-style: italic;
}

.sr-only {
  position: absolute;
  width: 1px;
  height: 1px;
  padding: 0;
  margin: -1px;
  overflow: hidden;
  clip: rect(0, 0, 0, 0);
  white-space: nowrap;
  border: 0;
}
</style>
