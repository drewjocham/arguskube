// useCredentialMonitor — registers every Vault credential the backend can
// live-probe as a watcher in the global watcherRegistry. The watcher
// engine drives the polling; the notificationGuard handles dedupe + spam.
//
// Before this rewrite the monitor was a per-feature setInterval that
// duplicated logic the rest of the app would soon need (cert expiry,
// license expiry, OAuth refresh token expiry). It now plugs into the
// generic registry: one 30 s tick, one guard, one source of truth.
//
// We still pre-fetch the Vault list once so we know which credentials to
// register, but the actual probes happen via the engine.

import { onMounted } from 'vue'
import { callGo } from './useBridge'
import { useCredentialAlertsStore } from '../stores/credentialAlerts'
import { useWatcherRegistryStore } from '../stores/watcherRegistry'

const PER_PROBE_INTERVAL_MS = 30 * 60 * 1000   // 30 min — overrideable per watcher

export function useCredentialMonitor() {
  const credAlerts = useCredentialAlertsStore()
  const registry = useWatcherRegistryStore()

  async function registerFromVault() {
    let entries = []
    try {
      entries = (await callGo('GetVaultStatus')) || []
    } catch (e) {
      console.debug('[credentialMonitor] GetVaultStatus failed:', e?.message || e)
      return
    }
    for (const e of entries) {
      if (!e || !e.probable || !e.configured) continue
      // Each credential becomes a watcher whose check() invokes the same
      // backend live-probe the manual Test button uses.
      registry.register({
        id: 'credential:' + e.id,
        label: e.label,
        kind: 'credential',
        intervalMs: PER_PROBE_INTERVAL_MS,
        configureAnchor: e.configureAnchor || '',
        async check() {
          let updated = null
          try {
            updated = await callGo('TestVaultProvider', e.id)
          } catch (err) {
            updated = { ...e, status: 'error', message: err?.message || String(err) }
          }
          // Keep the existing credentialAlerts store in sync so its
          // recovery-toast logic and seen-state for Argus context
          // remain authoritative for the credential domain.
          credAlerts.observe(updated || e)
          return {
            status: updated?.status || 'error',
            message: updated?.message || '',
            expiresAt: updated?.lastCheckedAt || '',
          }
        },
      })
    }
  }

  onMounted(() => {
    // Defer slightly so authentication / app-mode resolution happens first.
    setTimeout(() => { registerFromVault() }, 5_000)
  })

  return { registerFromVault }
}
