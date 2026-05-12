import { ref } from 'vue'
import { bus } from '../lib/bus'

const toasts = ref([])

let showHandler = null
let dismissHandler = null

function registerHandlers() {
  if (showHandler) return
  showHandler = (msg) => {
    const id = Date.now() + Math.random()
    toasts.value = [...toasts.value, { id, message: msg.message, kind: msg.kind || 'info' }]
    const duration = msg.duration ?? 4000
    if (duration > 0) {
      setTimeout(() => { bus.emit('toast:dismiss', { id: String(id) }) }, duration)
    }
  }
  dismissHandler = ({ id }) => {
    toasts.value = toasts.value.filter(t => String(t.id) !== String(id))
  }
  bus.on('toast:show', showHandler)
  bus.on('toast:dismiss', dismissHandler)
}

registerHandlers()

export function _resetToasts() {
  toasts.value = []
}

export function useToast() {
  function addToast(message, duration = 4000) {
    bus.emit('toast:show', { message, duration })
  }

  function removeToast(id) {
    bus.emit('toast:dismiss', { id: String(id) })
  }

  return { toasts, addToast, removeToast }
}
