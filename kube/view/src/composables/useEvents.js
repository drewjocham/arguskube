import { onMounted, onUnmounted } from 'vue'

export function useWailsEvent(eventName, callback) {
  let cancelFn = null

  onMounted(() => {
    if (typeof window !== 'undefined' && window.runtime?.EventsOn) {
      cancelFn = window.runtime.EventsOn(eventName, callback)
    }
  })

  onUnmounted(() => {
    if (cancelFn) cancelFn()
  })
}

export const Events = {
  ALERT_UPDATE: 'alert:update',
  LOG_LINE: 'log:line',
  METRICS_UPDATE: 'metrics:update',
  AUTO_SUMMARY: 'agent:auto-summary',
  AGENT_EVENT: 'agent:event',
  TERMINAL_OUTPUT: 'terminal:output',
  DEEP_LINK: 'deep-link',
}
