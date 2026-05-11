// externalSecrets — first-class secrets-encryption tooling config.
//
// Two things live in this store:
//
//   1. Local CLI tooling state — kubeseal, sops, gpg, age. Whether the
//      user has marked each as enabled (so the UI doesn't badger them
//      about a missing kubeseal binary they didn't intend to use), plus
//      a small handful of options (default key id, controller namespace,
//      key file path).
//
//   2. External Secrets Operator backend config — AWS Secrets Manager,
//      GCP Secret Manager, Azure Key Vault, HashiCorp Vault. ESO itself
//      runs in-cluster; this store just records which backends the user
//      *wants* the operator wired to so the UI can render the matching
//      SecretStore CR templates and credential references.
//
// Persisted to localStorage. Probe results (binary path / version) live
// transiently in this store — they're refreshed by SettingsPanel calling
// callGo('TestSecretsTool', name) and don't need to survive reloads.

import { defineStore } from 'pinia'
import { ref, computed } from 'vue'

const STORAGE_KEY = 'kw-external-secrets/v1'

const DEFAULTS = {
  // Local CLI tooling.
  kubeseal: { enabled: false, controllerNamespace: 'kube-system', controllerName: 'sealed-secrets-controller' },
  sops:     { enabled: false, defaultKeyFile: '' },
  gpg:      { enabled: false, defaultKeyId: '' },
  age:      { enabled: false, defaultKeyFile: '' },

  // External Secrets Operator backends. Enabled means "tell me about
  // SecretStore CRs that point at this backend"; the actual credentials
  // live in cluster Secrets, never in this store.
  esoAws:    { enabled: false, region: 'us-east-1', authRef: '' },
  esoGcp:    { enabled: false, projectId: '', authRef: '' },
  esoAzure:  { enabled: false, vaultUrl: '', tenantId: '', authRef: '' },
  esoVault:  { enabled: false, address: '', path: 'secret/data', authMethod: 'kubernetes' },
}

function loadInitial() {
  try {
    const raw = localStorage.getItem(STORAGE_KEY)
    if (!raw) return structuredClone(DEFAULTS)
    const parsed = JSON.parse(raw)
    if (!parsed || typeof parsed !== 'object') return structuredClone(DEFAULTS)
    // Shallow-merge per top-level key so adding a new tool to DEFAULTS
    // doesn't lose the user's existing entries on next load.
    const out = structuredClone(DEFAULTS)
    for (const k of Object.keys(out)) {
      if (parsed[k] && typeof parsed[k] === 'object') {
        out[k] = { ...out[k], ...parsed[k] }
      }
    }
    return out
  } catch {
    return structuredClone(DEFAULTS)
  }
}

function persist(value) {
  try { localStorage.setItem(STORAGE_KEY, JSON.stringify(value)) } catch { /* ignore */ }
}

export const useExternalSecretsStore = defineStore('externalSecrets', () => {
  const config = ref(loadInitial())

  // Transient probe results: { kubeseal: { found, version, path, error } }
  const probes = ref({})
  // Whether a probe is currently in flight per tool — drives the spinner.
  const probing = ref({})

  function setSection(key, patch) {
    if (!(key in config.value)) return
    config.value = {
      ...config.value,
      [key]: { ...config.value[key], ...patch },
    }
    persist(config.value)
  }

  function setEnabled(key, on) {
    setSection(key, { enabled: Boolean(on) })
  }

  function setProbeResult(tool, result) {
    probes.value = { ...probes.value, [tool]: result }
  }

  function setProbing(tool, on) {
    probing.value = { ...probing.value, [tool]: Boolean(on) }
  }

  // Aggregate signal for the Settings header chip — "3/4 tools detected".
  const localToolCount = computed(() => ['kubeseal', 'sops', 'gpg', 'age'].length)
  const localToolFound = computed(() =>
    ['kubeseal', 'sops', 'gpg', 'age']
      .filter((t) => probes.value[t]?.found).length
  )

  return {
    config, probes, probing,
    setSection, setEnabled, setProbeResult, setProbing,
    localToolCount, localToolFound,
  }
})
