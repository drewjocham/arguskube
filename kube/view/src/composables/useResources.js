import { ref, onMounted } from 'vue'
import { callGo, cachedCallGo, invalidateCache, DEFAULT_TTL } from './useBridge'

/**
 * Composable for the resource browser.
 */
export function useResources() {
  const result = ref(null)
  const detail = ref(null)
  const namespaces = ref([])
  const loading = ref(false)
  const detailLoading = ref(false)
  const error = ref(null)

  async function listResources(kind, namespace, force = false) {
    const ns = namespace || '_all'
    loading.value = result.value === null
    error.value = null
    try {
      if (force) invalidateCache('ListResources', kind, ns)
      result.value = await cachedCallGo('ListResources', [kind, ns], DEFAULT_TTL)
    } catch (e) {
      error.value = e?.message || String(e)
    } finally {
      loading.value = false
    }
  }

  async function getResourceDetail(kind, namespace, name) {
    detailLoading.value = true
    try {
      detail.value = await cachedCallGo('GetResourceDetail', [kind, namespace || '', name], DEFAULT_TTL)
    } catch (e) {
      console.error('[resource-detail]', e)
    } finally {
      detailLoading.value = false
    }
  }

  async function listNamespaces() {
    try {
      const res = await cachedCallGo('ListAllNamespaces', [], DEFAULT_TTL)
      if (res) namespaces.value = res
    } catch (e) {
      console.error('[namespaces]', e)
    }
  }

  return { result, detail, namespaces, loading, detailLoading, error, listResources, getResourceDetail, listNamespaces }
}
