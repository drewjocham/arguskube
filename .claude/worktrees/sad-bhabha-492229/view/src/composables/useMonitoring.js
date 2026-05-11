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
 */
export function useVulnerabilities() {
  const images = ref([])
  const loading = ref(false)
  const error = ref(null)

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
    try {
      const result = await callGo('ScanAllImages', namespace)
      invalidateCachePrefix('ListVulnerabilities')
      images.value = result || []
      return result
    } catch (e) {
      error.value = e?.message || String(e)
      return null
    } finally {
      loading.value = false
    }
  }

  return { images, loading, error, listVulnerabilities, scanImage, scanAllImages }
}
