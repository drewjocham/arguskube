// Tiny X.509 parser — extracts just the validity window (notBefore +
// notAfter) from a DER-encoded certificate. Pure JS, no dependencies.
//
// Why hand-roll: the Argus frontend already bundles ~1.2 MB; adding
// `node-forge` or `pkijs` would balloon that by another ~80 KB just to
// read two dates. ASN.1 has scary edges, but Certificate's tbsCertificate
// preamble is fixed-shape — version → serial → sig → issuer → validity
// — so a 50-line linear walker is enough.
//
// What this does NOT do (on purpose):
//   * verify signatures, chain trust, or CRL/OCSP
//   * parse extensions, SAN, subject CN, etc.
//   * handle weird ASN.1 (indefinite-length, BER-only encodings)
//
// What it WILL do:
//   * accept a PEM-armored "-----BEGIN CERTIFICATE-----…" string
//   * accept a raw DER Uint8Array
//   * return { notBefore: Date, notAfter: Date } or null on parse error

const TAG_SEQUENCE = 0x30
const TAG_INTEGER = 0x02
const TAG_UTCTIME = 0x17
const TAG_GENERALIZEDTIME = 0x18
const TAG_CONTEXT_0 = 0xa0 // EXPLICIT [0] — version wrapper

/**
 * Read one ASN.1 TLV (tag-length-value) starting at `pos`.
 * Returns { tag, valueStart, valueEnd, contentLen } or null on error.
 *
 * DER length encoding:
 *   single byte if < 0x80 → length is that value
 *   high-bit set (0x80…0xfe) → low 7 bits give the number of subsequent
 *   big-endian length bytes (so 0x82 0x01 0x23 = length 0x0123 = 291).
 *   0x80 (indefinite length) is a BER-only feature and is rejected here.
 */
function readTLV(buf, pos) {
  if (pos >= buf.length - 1) return null
  const tag = buf[pos]
  pos++
  let lenByte = buf[pos]
  pos++
  let len
  if (lenByte < 0x80) {
    len = lenByte
  } else if (lenByte === 0x80) {
    // Indefinite-length encoding — never emitted by certificate signers.
    return null
  } else {
    const nBytes = lenByte & 0x7f
    if (nBytes > 4 || pos + nBytes > buf.length) return null
    len = 0
    for (let i = 0; i < nBytes; i++) {
      len = (len << 8) | buf[pos++]
    }
  }
  if (pos + len > buf.length) return null
  return { tag, contentLen: len, valueStart: pos, valueEnd: pos + len }
}

/**
 * Convert a PEM-armored cert string to its DER bytes. Tolerates
 * leading/trailing whitespace and the optional preamble text some
 * tools add before the BEGIN line.
 */
export function pemToDer(pem) {
  if (typeof pem !== 'string') return null
  const m = pem.match(/-----BEGIN CERTIFICATE-----\s*([A-Za-z0-9+/=\s]+?)\s*-----END CERTIFICATE-----/)
  if (!m) return null
  const b64 = m[1].replace(/\s+/g, '')
  try {
    const bin = atob(b64)
    const out = new Uint8Array(bin.length)
    for (let i = 0; i < bin.length; i++) out[i] = bin.charCodeAt(i)
    return out
  } catch {
    return null
  }
}

/**
 * Parse an ASN.1 UTCTime or GeneralizedTime from the slice
 * `der[start..start+len]` into a Date (UTC). Returns null on
 * unrecognised tags or malformed digits.
 *
 * UTCTime is "YYMMDDHHMMSSZ" (2-digit year — RFC 5280 says years
 * 50…99 → 1950…1999, 00…49 → 2000…2049).
 * GeneralizedTime is "YYYYMMDDHHMMSSZ".
 */
function parseAsn1Time(tag, der, start, len) {
  let s = ''
  for (let i = 0; i < len; i++) s += String.fromCharCode(der[start + i])
  if (tag === TAG_UTCTIME) {
    if (s.length < 13 || !s.endsWith('Z')) return null
    const yy = parseInt(s.slice(0, 2), 10)
    if (Number.isNaN(yy)) return null
    const year = yy >= 50 ? 1900 + yy : 2000 + yy
    const month = parseInt(s.slice(2, 4), 10) - 1
    const day = parseInt(s.slice(4, 6), 10)
    const hh = parseInt(s.slice(6, 8), 10)
    const mm = parseInt(s.slice(8, 10), 10)
    const ss = parseInt(s.slice(10, 12), 10)
    const ts = Date.UTC(year, month, day, hh, mm, ss)
    return Number.isNaN(ts) ? null : new Date(ts)
  }
  if (tag === TAG_GENERALIZEDTIME) {
    if (s.length < 15 || !s.endsWith('Z')) return null
    const year = parseInt(s.slice(0, 4), 10)
    const month = parseInt(s.slice(4, 6), 10) - 1
    const day = parseInt(s.slice(6, 8), 10)
    const hh = parseInt(s.slice(8, 10), 10)
    const mm = parseInt(s.slice(10, 12), 10)
    const ss = parseInt(s.slice(12, 14), 10)
    const ts = Date.UTC(year, month, day, hh, mm, ss)
    return Number.isNaN(ts) ? null : new Date(ts)
  }
  return null
}

/**
 * Extract { notBefore, notAfter } from a certificate. Accepts a PEM
 * string OR a Uint8Array of DER bytes. Returns null on any parse
 * failure — callers should treat null as "not a valid cert" and show
 * an honest empty state, not a fake "expires never" placeholder.
 *
 * Structure walked (from RFC 5280):
 *
 *   Certificate ::= SEQUENCE {
 *     tbsCertificate       TBSCertificate,
 *     signatureAlgorithm   AlgorithmIdentifier,
 *     signatureValue       BIT STRING
 *   }
 *   TBSCertificate ::= SEQUENCE {
 *     version              [0] EXPLICIT Version DEFAULT v1,
 *     serialNumber         INTEGER,
 *     signature            AlgorithmIdentifier (SEQUENCE),
 *     issuer               Name (SEQUENCE),
 *     validity             SEQUENCE { notBefore Time, notAfter Time },
 *     ...
 *   }
 */
export function parseCertificateValidity(input) {
  const der = typeof input === 'string' ? pemToDer(input) : input
  if (!(der instanceof Uint8Array) || der.length < 32) return null

  const cert = readTLV(der, 0)
  if (!cert || cert.tag !== TAG_SEQUENCE) return null

  const tbs = readTLV(der, cert.valueStart)
  if (!tbs || tbs.tag !== TAG_SEQUENCE) return null

  let p = tbs.valueStart
  // Optional version [0] EXPLICIT.
  if (der[p] === TAG_CONTEXT_0) {
    const v = readTLV(der, p)
    if (!v) return null
    p = v.valueEnd
  }
  // serialNumber INTEGER
  const sn = readTLV(der, p)
  if (!sn || sn.tag !== TAG_INTEGER) return null
  p = sn.valueEnd
  // signature AlgorithmIdentifier (SEQUENCE)
  const sig = readTLV(der, p)
  if (!sig || sig.tag !== TAG_SEQUENCE) return null
  p = sig.valueEnd
  // issuer Name (SEQUENCE)
  const issuer = readTLV(der, p)
  if (!issuer || issuer.tag !== TAG_SEQUENCE) return null
  p = issuer.valueEnd
  // validity SEQUENCE
  const validity = readTLV(der, p)
  if (!validity || validity.tag !== TAG_SEQUENCE) return null

  // Two times inside.
  let q = validity.valueStart
  const nb = readTLV(der, q)
  if (!nb) return null
  q = nb.valueEnd
  const na = readTLV(der, q)
  if (!na) return null

  const notBefore = parseAsn1Time(nb.tag, der, nb.valueStart, nb.contentLen)
  const notAfter = parseAsn1Time(na.tag, der, na.valueStart, na.contentLen)
  if (!notBefore || !notAfter) return null
  return { notBefore, notAfter }
}

/**
 * Convenience helper for the TLS tab: compute days-until-expiry +
 * a coarse status bucket. `now` is injectable so tests don't depend
 * on wall-clock.
 */
export function expiryStatus(notAfter, now = new Date()) {
  if (!(notAfter instanceof Date) || Number.isNaN(notAfter.getTime())) {
    return { daysLeft: null, status: 'unknown' }
  }
  const ms = notAfter.getTime() - now.getTime()
  const daysLeft = Math.floor(ms / (1000 * 60 * 60 * 24))
  let status
  if (daysLeft < 0) status = 'expired'
  else if (daysLeft <= 7) status = 'critical'
  else if (daysLeft <= 30) status = 'warning'
  else status = 'ok'
  return { daysLeft, status }
}
