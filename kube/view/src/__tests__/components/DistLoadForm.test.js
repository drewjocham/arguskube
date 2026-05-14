import { describe, it, expect } from 'vitest'
import { isPrivateOrLoopback } from '../../lib/destinationValidation'

// The audit's UX-polish finding: the previous destination validation
// only matched the literal string "localhost". A user pasting a
// private IP ("10.0.0.5", "192.168.1.1") sailed past validation and
// the cloud VMs failed at runtime. This pins the full RFC1918 +
// loopback + link-local + IPv6 check.

describe('isPrivateOrLoopback', () => {
  const privateCases = [
    // hostnames
    ['localhost', 'localhost / *.local'],
    ['localhost:9092', 'localhost / *.local'],
    ['my-host.local', 'localhost / *.local'],
    ['my-host.localdomain', 'localhost / *.local'],
    // IPv4 loopback + unspecified
    ['127.0.0.1', 'IPv4 loopback (127.0.0.0/8)'],
    ['127.0.0.1:8080', 'IPv4 loopback (127.0.0.0/8)'],
    ['127.5.5.5', 'IPv4 loopback (127.0.0.0/8)'],
    ['0.0.0.0', 'IPv4 unspecified (0.0.0.0)'],
    // IPv4 link-local
    ['169.254.169.254', 'IPv4 link-local (169.254.0.0/16)'],
    // IPv4 RFC1918
    ['10.0.0.5', 'IPv4 private (10.0.0.0/8)'],
    ['10.255.255.254', 'IPv4 private (10.0.0.0/8)'],
    ['192.168.1.1', 'IPv4 private (192.168.0.0/16)'],
    ['172.16.0.1', 'IPv4 private (172.16.0.0/12)'],
    ['172.20.5.5', 'IPv4 private (172.16.0.0/12)'],
    ['172.31.255.255', 'IPv4 private (172.16.0.0/12)'],
    // IPv6
    ['::1', 'IPv6 loopback (::1)'],
    ['fe80::1', 'IPv6 link-local (fe80::/10)'],
    ['fc00::1', 'IPv6 unique-local (fc00::/7)'],
    ['fd12:3456:7890::1', 'IPv6 unique-local (fc00::/7)'],
    // URL forms
    ['kafka://10.0.0.5:9092', 'IPv4 private (10.0.0.0/8)'],
    ['https://192.168.1.4', 'IPv4 private (192.168.0.0/16)'],
    ['https://192.168.1.4/topic', 'IPv4 private (192.168.0.0/16)'],
    // IPv6 bracket form
    ['[::1]:9092', 'IPv6 loopback (::1)'],
  ]

  const publicCases = [
    'broker.example.com',
    'kafka.prod.example.com:9092',
    'pubsub.googleapis.com',
    '8.8.8.8',
    '1.1.1.1',
    // Adjacent-but-not-private addresses — pinning the boundaries
    // is the cheapest insurance against off-by-one regressions.
    '172.15.0.1',   // just BELOW the 172.16 private range
    '172.32.0.1',   // just ABOVE the 172.31 private range
    '169.253.0.1',  // adjacent to link-local but public
    '11.0.0.1',     // adjacent to 10/8 but public
    '193.168.1.1',  // looks like 192.168 but isn't
    // Documentation range — public-routable for our purposes
    '2001:db8::1',
    // Empty / nullish guards
    '',
    null,
    undefined,
  ]

  for (const [input, expected] of privateCases) {
    it(`rejects: ${input}`, () => {
      expect(isPrivateOrLoopback(input)).toBe(expected)
    })
  }

  for (const input of publicCases) {
    it(`accepts: ${JSON.stringify(input)}`, () => {
      expect(isPrivateOrLoopback(input)).toBeNull()
    })
  }
})
