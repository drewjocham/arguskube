// Destination validation helpers for the distributed load tester.
//
// Pulled out of DistLoadForm.vue so the predicate can be unit-tested
// directly (no Vue mount needed) and reused if other forms ever need
// the same check.
//
// Cloud VMs running the load test can't reach the operator's laptop
// or anything on a private VPC the VMs aren't peered with. The audit
// flagged the previous "matches the literal string 'localhost'" check
// as too narrow — operators typed IPs and got past it. This catches
// the full set of "won't be reachable from the public internet"
// patterns:
//
//   - localhost, *.local, *.localdomain (mDNS / hostfile)
//   - IPv4 loopback 127.0.0.0/8
//   - IPv4 unspecified 0.0.0.0
//   - IPv4 link-local 169.254.0.0/16
//   - IPv4 RFC1918 private (10/8, 172.16/12, 192.168/16)
//   - IPv6 loopback ::1
//   - IPv6 link-local fe80::/10
//   - IPv6 ULA fc00::/7

/**
 * Returns a human-readable reason string when the input destination
 * resolves to a private / loopback host, or null when it appears
 * reachable from the public internet.
 *
 * Accepts bare hostnames ("localhost"), host:port ("kafka:9092"),
 * full URLs ("https://192.168.1.4:9092/topic"), and IPv6 bracket
 * forms ("[::1]:9092"). Returns the matched range name so the
 * caller can render a precise error.
 *
 * @param {string|null|undefined} input
 * @returns {string|null}
 */
export function isPrivateOrLoopback(input) {
  if (!input) return null
  let host = String(input).trim().toLowerCase()

  // Strip URL parts.
  host = host.replace(/^[a-z][a-z0-9+.-]*:\/\//, '')   // scheme://
  host = host.split('/')[0]                             // path

  // Strip port. IPv6 needs bracket-form handling; bare IPv6 with no
  // bracket can't carry a port, so the single-colon check is safe
  // for IPv4 + hostname only.
  if (host.startsWith('[')) {
    const idx = host.indexOf(']')
    if (idx > 0) host = host.slice(1, idx)
  } else if (host.includes(':') && (host.match(/:/g) || []).length === 1) {
    host = host.split(':')[0]
  }
  if (!host) return null

  if (host === 'localhost' || host.endsWith('.local') || host.endsWith('.localdomain')) {
    return 'localhost / *.local'
  }
  // IPv4
  const v4 = host.match(/^(\d{1,3})\.(\d{1,3})\.(\d{1,3})\.(\d{1,3})$/)
  if (v4) {
    const o = v4.slice(1, 5).map(Number)
    if (o[0] === 127) return 'IPv4 loopback (127.0.0.0/8)'
    if (o[0] === 0) return 'IPv4 unspecified (0.0.0.0)'
    if (o[0] === 169 && o[1] === 254) return 'IPv4 link-local (169.254.0.0/16)'
    if (o[0] === 10) return 'IPv4 private (10.0.0.0/8)'
    if (o[0] === 192 && o[1] === 168) return 'IPv4 private (192.168.0.0/16)'
    if (o[0] === 172 && o[1] >= 16 && o[1] <= 31) return 'IPv4 private (172.16.0.0/12)'
  }
  // IPv6
  if (host === '::1') return 'IPv6 loopback (::1)'
  if (host.startsWith('fe80:') || host.startsWith('fe80::')) return 'IPv6 link-local (fe80::/10)'
  if (host.startsWith('fc') || host.startsWith('fd')) return 'IPv6 unique-local (fc00::/7)'
  return null
}
