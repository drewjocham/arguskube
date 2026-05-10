import { ref } from 'vue'
import { callGo, cachedCallGo, invalidateCachePrefix, DEFAULT_TTL } from './useBridge'

/**
 * Composable for Argus Scan cluster scan.
 */
export function useArgusScan() {
  const report = ref(null)
  const loading = ref(false)
  const error = ref(null)

  async function runScan() {
    loading.value = true
    error.value = null
    try {
      report.value = await callGo('RunArgusScan')
    } catch (e) {
      error.value = e?.message || String(e)
      report.value = null
    } finally {
      loading.value = false
    }
  }

  return { report, loading, error, runScan }
}

/**
 * Composable for vulnerabilities.
 *
 * Wails doesn't expose an AbortSignal for callGo, so a long-running scan
 * (e.g. ScanAllImages can take many seconds) cannot truly be cancelled
 * mid-flight on the backend. What we CAN do is stop honoring the response
 * once the caller has signalled cancellation — that prevents stale results
 * from a previous mount overwriting fresh state when the user navigates
 * away and comes back. `cancel()` marks the in-flight scan as abandoned;
 * the backend's eventual response is dropped on the floor.
 */
export function useVulnerabilities() {
  const images = ref([])
  const loading = ref(false)
  const error = ref(null)
  let scanGeneration = 0

  async function listVulnerabilities() {
    loading.value = images.value.length === 0
    error.value = null
    try {
      const result = await cachedCallGo('ListVulnerabilities', [], DEFAULT_TTL)
      images.value = result || []
    } catch (e) {
      error.value = e?.message || String(e)
      images.value = []
    } finally {
      loading.value = false
    }
  }

  async function scanImage(image, engine) {
    try {
      const result = await callGo('ScanImage', image, engine)
      invalidateCachePrefix('ListVulnerabilities')
      await listVulnerabilities()
      return result
    } catch (e) {
      console.error('[vulnerabilities] scan:', e)
      return 'Scan failed'
    }
  }

  async function scanAllImages(namespace = '') {
    loading.value = true
    error.value = null
    const myGeneration = ++scanGeneration
    try {
      const result = await callGo('ScanAllImages', namespace)
      // Drop the result if the caller cancelled (component unmounted, user
      // navigated away, or another scan was started in the meantime).
      if (myGeneration !== scanGeneration) {
        return null
      }
      invalidateCachePrefix('ListVulnerabilities')
      images.value = result || []
      return result
    } catch (e) {
      if (myGeneration !== scanGeneration) return null
      error.value = e?.message || String(e)
      return null
    } finally {
      if (myGeneration === scanGeneration) {
        loading.value = false
      }
    }
  }

  function cancel() {
    // Bumping the generation invalidates any in-flight scanAllImages call so
    // its eventual response won't mutate state on remount.
    scanGeneration++
    loading.value = false
  }

  return { images, loading, error, listVulnerabilities, scanImage, scanAllImages, cancel }
}
