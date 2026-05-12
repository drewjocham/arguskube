// secretRef — a tiny grammar for "where does this value come from?".
//
// Anywhere in the app where the user types a username, password, token, or
// other configurable env var, they can prefix the value with a short label
// to declare its source. The label is parsed here; the backend
// (internal/secretref/resolver.go) resolves the actual value at use time.
//
// Why labels and not separate UI fields per source: it's a common pattern
// (Kubernetes ENV mounts, Docker compose variable substitution, dotenv
// references) and it keeps the UI to a single text input that supports
// every source uniformly. The user sees an inline pill telling them what
// kind of source the label resolves to, plus a dropdown to switch source
// without retyping.
//
// Grammar:
//
//   <kind>:<value>           e.g. env:DATABASE_URL
//   <kind>:<value>#<key>     e.g. aws-secret:prod/db-creds#username
//
// Recognized kinds (anything else is treated as inline):
//
//   inline       — literal value (default if no prefix)
//   env          — process environment variable (env:NAME)
//   file         — absolute path on the host (file:/etc/foo/secret)
//   volume       — k8s volume name + path inside it (volume:my-vol/secret)
//   aws-secret   — AWS Secrets Manager (aws-secret:my-secret[#key])
//   gcp-secret   — GCP Secret Manager   (gcp-secret:projects/x/secrets/y[#version])
//   azure-vault  — Azure Key Vault      (azure-vault:vault-name/secret-name)
//   vault        — local credential vault entry (vault:entry-id[#key])
//
// All parsing is best-effort and case-sensitive. Whitespace is trimmed
// from the kind and value but preserved inside multi-segment values.

export const SECRET_REF_KINDS = Object.freeze([
  'inline',
  'env',
  'file',
  'volume',
  'aws-secret',
  'gcp-secret',
  'azure-vault',
  'vault',
])

const KIND_LOOKUP = new Set(SECRET_REF_KINDS)

// Human-readable label + a short hint shown in tooltips/badges.
export const SECRET_REF_META = Object.freeze({
  inline:        { label: 'Inline',           hint: 'Literal value typed directly. Stored as-is.' },
  env:           { label: 'Env var',          hint: 'Read from the host process environment.' },
  file:          { label: 'File',             hint: 'Read from an absolute path on the host filesystem.' },
  volume:        { label: 'Volume',           hint: 'Read from a path inside a Kubernetes volume mount.' },
  'aws-secret':  { label: 'AWS Secrets Mgr',  hint: 'Fetched from AWS Secrets Manager.' },
  'gcp-secret':  { label: 'GCP Secret Mgr',   hint: 'Fetched from Google Secret Manager.' },
  'azure-vault': { label: 'Azure Key Vault',  hint: 'Fetched from Azure Key Vault.' },
  vault:         { label: 'Argus Vault',      hint: "Read from this app's local encrypted credential vault." },
})

// parseSecretRef("aws-secret:prod/db#username")
//   → { kind: 'aws-secret', value: 'prod/db', key: 'username', raw: '…' }
//
// parseSecretRef("plain text")
//   → { kind: 'inline', value: 'plain text', key: '', raw: 'plain text' }
//
// Edge cases:
//   • Empty / null / undefined input → inline with empty value.
//   • Unknown prefix (e.g. "weird:value") → inline; the colon stays in value.
//   • Trailing whitespace in the kind is tolerated; leading whitespace in
//     the value is trimmed; whitespace inside the value is preserved so a
//     password like "p ass" still works inline.
export function parseSecretRef(input) {
  if (input === null || input === undefined) {
    return { kind: 'inline', value: '', key: '', raw: '' }
  }
  const raw = String(input)
  if (!raw) return { kind: 'inline', value: '', key: '', raw: '' }

  const colon = raw.indexOf(':')
  if (colon < 0) {
    return { kind: 'inline', value: raw, key: '', raw }
  }
  const kind = raw.slice(0, colon).trim().toLowerCase()
  if (!KIND_LOOKUP.has(kind) || kind === 'inline') {
    // Unknown prefix — fall back to inline with the whole string preserved.
    return {
      kind: 'inline',
      value: raw.startsWith('inline:') ? raw.slice('inline:'.length) : raw,
      key: '',
      raw,
    }
  }
  // Anything after "<kind>:" is the value, optionally split by "#key".
  let body = raw.slice(colon + 1)
  // Drop a single leading space so users can write "vault: my-id" naturally.
  if (body.startsWith(' ')) body = body.slice(1)

  // The key suffix is opt-in and only meaningful for kinds that can hold
  // structured values (aws-secret, gcp-secret, vault). We still parse it
  // generically — the resolver decides whether it makes sense for the kind.
  const hash = body.lastIndexOf('#')
  let value = body
  let key = ''
  if (hash >= 0) {
    value = body.slice(0, hash)
    key = body.slice(hash + 1)
  }
  return { kind, value, key, raw }
}

// formatSecretRef({kind, value, key}) → canonical "kind:value[#key]"
//
// Round-trips parseSecretRef. Used by the SecretRefInput when the user
// switches the source dropdown (we re-format the existing value with the
// new kind prefix so the input field reflects what the resolver sees).
export function formatSecretRef(ref) {
  if (!ref || typeof ref !== 'object') return ''
  const kind = ref.kind || 'inline'
  const value = ref.value == null ? '' : String(ref.value)
  if (kind === 'inline' || !KIND_LOOKUP.has(kind)) return value
  const k = ref.key ? `#${ref.key}` : ''
  return `${kind}:${value}${k}`
}

// isResolvable(ref) — does this reference need a backend round-trip to
// produce a value, or can the UI use it as-is? Inline values are always
// usable; everything else must be resolved server-side because the data
// either lives on disk, in a cloud vault, or in the host process.
export function isResolvable(ref) {
  return Boolean(ref) && ref.kind !== 'inline' && ref.kind !== ''
}

// describeSecretRef(ref) — short user-facing summary for tooltips.
//   parseSecretRef("aws-secret:prod/db#user") → "AWS Secrets Mgr · prod/db (user)"
//   parseSecretRef("just a value")            → "Inline value"
export function describeSecretRef(ref) {
  if (!ref) return ''
  const meta = SECRET_REF_META[ref.kind] || SECRET_REF_META.inline
  if (ref.kind === 'inline') {
    return ref.value ? 'Inline value' : 'Empty inline value'
  }
  const tail = ref.key ? `${ref.value} (${ref.key})` : ref.value
  return tail ? `${meta.label} · ${tail}` : meta.label
}

// secretRefIsValid — a minimal sanity check used to disable Save buttons.
// This is not a security check — it just catches typos like an empty path.
export function secretRefIsValid(ref) {
  if (!ref) return false
  switch (ref.kind) {
    case 'inline':
      return true // empty inline is allowed (nullable env vars)
    case 'env':
      return /^[A-Z_][A-Z0-9_]*$/i.test(ref.value || '')
    case 'file':
      return typeof ref.value === 'string' && ref.value.startsWith('/')
    case 'volume':
      // volume:<name>/<path-inside-volume>
      return /^[a-z0-9][a-z0-9-]*\/.+/i.test(ref.value || '')
    case 'aws-secret':
    case 'gcp-secret':
    case 'azure-vault':
    case 'vault':
      return Boolean(ref.value)
    default:
      return false
  }
}
