import { ref } from 'vue'
import { callGo } from './useBridge'

export function useManifestEdit() {
  const manifestPopup = ref(false)
  const editingManifest = ref(false)
  const manifestContent = ref('')
  const manifestKind = ref('')
  const manifestName = ref('')
  const manifestNamespace = ref('')
  const manifestResourceType = ref('')
  const manifestLoading = ref(false)
  const manifestApplying = ref(false)
  const manifestNotification = ref(null)

  let notifTimer = null
  function notify(message, ttl = 5000) {
    manifestNotification.value = message
    if (notifTimer) clearTimeout(notifTimer)
    notifTimer = setTimeout(() => { manifestNotification.value = null }, ttl)
  }

  async function openManifest({ resourceType, kind, namespace, name }) {
    manifestLoading.value = true
    manifestPopup.value = true
    manifestKind.value = kind
    manifestName.value = name
    manifestNamespace.value = namespace
    manifestResourceType.value = resourceType
    manifestContent.value = ''
    editingManifest.value = false
    try {
      const yaml = await callGo('GetResourceYaml', resourceType, namespace, name)
      manifestContent.value = yaml
    } catch (e) {
      manifestContent.value = `# Error fetching manifest: ${e?.message || e}`
    } finally {
      manifestLoading.value = false
    }
  }

  function closeManifest() {
    manifestPopup.value = false
    editingManifest.value = false
    manifestContent.value = ''
  }

  function toggleEditManifest() {
    editingManifest.value = !editingManifest.value
  }

  async function applyManifest(onSuccess) {
    if (!manifestContent.value.trim()) return null
    manifestApplying.value = true
    try {
      const result = await callGo('ApplyYaml', manifestContent.value)
      notify(`✓ ${result}`)
      closeManifest()
      if (typeof onSuccess === 'function') {
        await onSuccess()
      }
      return result
    } catch (e) {
      notify(`✗ Apply failed: ${e?.message || e}`, 8000)
      return null
    } finally {
      manifestApplying.value = false
    }
  }

  return {
    manifestPopup,
    editingManifest,
    manifestContent,
    manifestKind,
    manifestName,
    manifestNamespace,
    manifestResourceType,
    manifestLoading,
    manifestApplying,
    manifestNotification,
    openManifest,
    closeManifest,
    toggleEditManifest,
    applyManifest,
  }
}
