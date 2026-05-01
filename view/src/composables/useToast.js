import { ref } from 'vue'

const toasts = ref([])

export function useToast() {
  function addToast(message, duration = 4000) {
    const id = Date.now() + Math.random()
    toasts.value.push({ id, message })
    setTimeout(() => {
      removeToast(id)
    }, duration)
  }

  function removeToast(id) {
    toasts.value = toasts.value.filter(t => t.id !== id)
  }

  return { toasts, addToast, removeToast }
}
