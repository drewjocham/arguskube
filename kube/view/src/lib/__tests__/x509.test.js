import { describe, it, expect } from 'vitest'
import { parseCertificateValidity, pemToDer, expiryStatus } from '../x509'

// A real self-signed cert generated with:
//   openssl req -x509 -newkey rsa:2048 -nodes -days 365 \
//     -subj '/CN=argus.test' -out cert.pem
// Captured validity (UTC): 2026-05-13 → 2027-05-13.
// Using a fixed fixture means the test doesn't drift if the test
// machine's wall-clock is wrong.
const FIXTURE_CERT_PEM = `-----BEGIN CERTIFICATE-----
MIIDCzCCAfOgAwIBAgIUSRq7b8yLZilZ/dDqW/0MslLiEwswDQYJKoZIhvcNAQEL
BQAwFTETMBEGA1UEAwwKYXJndXMudGVzdDAeFw0yNjA1MTMwODExMjNaFw0yNzA1
MTMwODExMjNaMBUxEzARBgNVBAMMCmFyZ3VzLnRlc3QwggEiMA0GCSqGSIb3DQEB
AQUAA4IBDwAwggEKAoIBAQCZUWxo51qN+6gV0xfnXTmmnJN8cKZMEPSq+bZ/FQaR
s92gMFLurRGFe9cP/tBI7ouYTV/8ih8MlMLP0yqXWHjeXGn21AzRwLArnoaaSRMs
4zLDwguWvsmvLzlfG0iNX6n3B3TUgXYTTRTKQGRo5AgzTqX8jmXTenmMnSlOOZT7
NSMlQxbaK5gGMskp7WdK+INgr/L0IZRRzjbr8JB854dqF8Tvr6F8Go6PdY9c6o/B
hsngm5novkYiz7CaGhbNaZab3OQ4I5G4udC0y/RFPHGhATM9ZhMPNBE+48MzXMXO
IAY+KJPP5T+V5JPlHh+pm/4e49+UOiUmOfeoxkRSjnCvAgMBAAGjUzBRMB0GA1Ud
DgQWBBQx1Av74mpycuicCB9k+P2e4GfPPTAfBgNVHSMEGDAWgBQx1Av74mpycuic
CB9k+P2e4GfPPTAPBgNVHRMBAf8EBTADAQH/MA0GCSqGSIb3DQEBCwUAA4IBAQBl
ViRAvttsmC7D64q0TFya6Id4dN8aqO5aHJZay8GsPiUpzti8iwTk8CULe9JM2Y0V
gbE0BbDmlvzU094VHIWaOKqPHfV0OlflfLjYuPRgbCHTG5+MtBO5NRkLxT3NE2Ku
kkcXA2PzvC6BX3aM9yYN+dN+lWhGoSIhRXLHcyNMP1a1NNEYIx4hseb6ZRFs6Pkk
ZA2dZ4hY8SXoJRztH7/mSOc14G8rEuKjYe5Zzaw9fZREl10dDM3bzIrsoyharDTv
IL/bPXaoG1Pe71lFeZEC6iKU/iPa7QsiEakZqyPAIu65SVtnrWEzQQ02GHjJaA1/
R0iVT9mPhD+V3r4I3SPB
-----END CERTIFICATE-----`

describe('pemToDer', () => {
  it('decodes a PEM-armored cert to a Uint8Array', () => {
    const der = pemToDer(FIXTURE_CERT_PEM)
    expect(der).toBeInstanceOf(Uint8Array)
    expect(der.length).toBeGreaterThan(100)
    // First byte is ASN.1 SEQUENCE.
    expect(der[0]).toBe(0x30)
  })

  it('returns null on non-PEM input', () => {
    expect(pemToDer('hello world')).toBeNull()
    expect(pemToDer('')).toBeNull()
    expect(pemToDer(null)).toBeNull()
    expect(pemToDer(undefined)).toBeNull()
  })

  it('tolerates whitespace inside the base64 body', () => {
    const noisy = FIXTURE_CERT_PEM.replace(/\n/g, '\n   ')
    expect(pemToDer(noisy)).toBeInstanceOf(Uint8Array)
  })
})

describe('parseCertificateValidity', () => {
  it('extracts notBefore + notAfter from a real cert', () => {
    const v = parseCertificateValidity(FIXTURE_CERT_PEM)
    expect(v).not.toBeNull()
    // Fixture validity: notBefore 2026-05-13 08:11:23 UTC, notAfter
    // exactly one year later. Assert structurally so the test would
    // catch byte-shifts in the parser without depending on the exact
    // second.
    expect(v.notBefore.getUTCFullYear()).toBe(2026)
    expect(v.notBefore.getUTCMonth()).toBe(4) // May (0-indexed)
    expect(v.notBefore.getUTCDate()).toBe(13)
    expect(v.notAfter.getUTCFullYear()).toBe(2027)
    expect(v.notAfter.getUTCMonth()).toBe(4)
    expect(v.notAfter.getUTCDate()).toBe(13)
    // Sanity: validity window ≈ 365 days.
    const days = (v.notAfter - v.notBefore) / (24 * 3600 * 1000)
    expect(days).toBeGreaterThan(360)
    expect(days).toBeLessThan(370)
  })

  it('returns null on garbage input', () => {
    expect(parseCertificateValidity('garbage')).toBeNull()
    expect(parseCertificateValidity('')).toBeNull()
    expect(parseCertificateValidity(null)).toBeNull()
    expect(parseCertificateValidity(new Uint8Array([0, 1, 2]))).toBeNull()
  })

  it('returns null on a PEM that decodes to too-few bytes', () => {
    const tooShort = '-----BEGIN CERTIFICATE-----\nAAAA\n-----END CERTIFICATE-----'
    expect(parseCertificateValidity(tooShort)).toBeNull()
  })
})

describe('expiryStatus', () => {
  it('flags expired certs as expired', () => {
    const past = new Date('2020-01-01T00:00:00Z')
    const r = expiryStatus(past, new Date('2026-01-01T00:00:00Z'))
    expect(r.status).toBe('expired')
    expect(r.daysLeft).toBeLessThan(0)
  })
  it('flags certs expiring in ≤7 days as critical', () => {
    const soon = new Date('2026-01-05T00:00:00Z')
    const r = expiryStatus(soon, new Date('2026-01-01T00:00:00Z'))
    expect(r.status).toBe('critical')
    expect(r.daysLeft).toBe(4)
  })
  it('flags certs expiring in ≤30 days as warning', () => {
    const soonish = new Date('2026-01-25T00:00:00Z')
    const r = expiryStatus(soonish, new Date('2026-01-01T00:00:00Z'))
    expect(r.status).toBe('warning')
  })
  it('flags healthy certs as ok', () => {
    const farOut = new Date('2027-01-01T00:00:00Z')
    const r = expiryStatus(farOut, new Date('2026-01-01T00:00:00Z'))
    expect(r.status).toBe('ok')
    expect(r.daysLeft).toBeGreaterThan(360)
  })
  it('reports unknown when input is not a Date', () => {
    expect(expiryStatus(null).status).toBe('unknown')
    expect(expiryStatus('garbage').status).toBe('unknown')
    expect(expiryStatus(new Date(NaN)).status).toBe('unknown')
  })
})
