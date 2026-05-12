import { onMounted, onUnmounted } from 'vue'
import type { TaskMap, TaskName, TaskPayload } from './tasks'

type Listener = (payload: any) => void

const APP_EVENTS = new Map<TaskName, Set<Listener>>()
const WAILS_CANCELS = new Map<string, () => void>()

let wailsReady = false
function ensureWails(): boolean {
  if (wailsReady) return true
  if (typeof window !== 'undefined' && window.runtime?.EventsOn) {
    wailsReady = true
    return true
  }
  return false
}

const bus = {

  /** Subscribe to an app-local event. Returns an unsubscribe fn. */
  on<K extends TaskName>(event: K, cb: (payload: TaskPayload<K>) => void): () => void {
    if (!APP_EVENTS.has(event)) APP_EVENTS.set(event, new Set())
    APP_EVENTS.get(event)!.add(cb as Listener)
    return () => { APP_EVENTS.get(event)?.delete(cb as Listener) }
  },

  /** Unsubscribe a specific listener. */
  off<K extends TaskName>(event: K, cb: (payload: TaskPayload<K>) => void): void {
    APP_EVENTS.get(event)?.delete(cb as Listener)
  },

  /** Emit an app-local event to all subscribers. */
  emit<K extends TaskName>(event: K, payload: TaskPayload<K>): void {
    APP_EVENTS.get(event)?.forEach((cb) => { try { cb(payload) } catch (e) { console.warn(`[bus] ${event} handler error:`, e) } })
  },

  /**
   * Subscribe to a Wails backend event (Go → Frontend push).
   * Registers via window.runtime.EventsOn. Returns an unsubscribe fn.
   */
  onWails<K extends TaskName>(event: K, cb: (payload: TaskPayload<K>) => void): () => void {
    if (!ensureWails()) {
      console.warn(`[bus] Wails runtime not available — ${event} listener not registered`)
      return () => {}
    }
    const cancel = window.runtime!.EventsOn(event, (raw: any) => {
      cb(raw as TaskPayload<K>)
    })
    const key = `${event}-${String(Math.random()).slice(2)}`
    WAILS_CANCELS.set(key, cancel)
    return () => {
      cancel()
      WAILS_CANCELS.delete(key)
    }
  },

  /**
   * Vue-lifecycle-safe Wails event subscription.
   * Automatically subscribes on mount, cleans up on unmount.
   */
  useWailsEvent<K extends TaskName>(event: K, cb: (payload: TaskPayload<K>) => void): void {
    let cancel: (() => void) | null = null
    onMounted(() => { cancel = bus.onWails(event, cb) })
    onUnmounted(() => { cancel?.() })
  },

  /**
   * Vue-lifecycle-safe app event subscription.
   * Use this inside setup() of any component/store.
   */
  useEvent<K extends TaskName>(event: K, cb: (payload: TaskPayload<K>) => void): void {
    onMounted(() => bus.on(event, cb))
    onUnmounted(() => bus.off(event, cb))
  },
}

if (typeof window !== 'undefined') {
  ;(window as any).__bus = bus
}

export { bus }
