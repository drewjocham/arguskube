import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { useArgusScan, useVulnerabilities } from '../useMonitoring'
import { invalidateCachePrefix } from '../useBridge'

describe('useArgusScan', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    if (window.go) delete window.go
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('returns report, loading, error, runScan', () => {
    const result = useArgusScan()
    expect(result.report.value).toBe(null)
    expect(result.loading.value).toBe(false)
    expect(result.error.value).toBe(null)
    expect(typeof result.runScan).toBe('function')
  })

  it('runScan calls RunArgusScan and sets report', async () => {
    const reportData = {
      status: 'completed',
      findings: [{ severity: 'high', description: 'Open port 22' }],
    }

    vi.stubGlobal('go', {
      pkg: {
        App: {
          RunArgusScan: vi.fn().mockResolvedValue(reportData),
        },
      },
    })

    const { report, loading, runScan } = useArgusScan()
    expect(loading.value).toBe(false)

    await runScan()

    expect(report.value).toEqual(reportData)
    expect(loading.value).toBe(false)
  })

  it('runScan sets loading state correctly', async () => {
    vi.stubGlobal('go', {
      pkg: {
        App: {
          RunArgusScan: vi.fn().mockImplementation(
            () => new Promise(r => setTimeout(() => r({ results: [] }), 10))
          ),
        },
      },
    })

    const { loading, runScan } = useArgusScan()
    const promise = runScan()

    expect(loading.value).toBe(true)
    await promise
    expect(loading.value).toBe(false)
  })

  it('runScan handles errors gracefully', async () => {
    vi.stubGlobal('go', {
      pkg: {
        App: {
          RunArgusScan: vi.fn().mockRejectedValue(new Error('Scan failed')),
        },
      },
    })

    const { report, error, loading, runScan } = useArgusScan()
    await runScan()

    expect(report.value).toBe(null)
    expect(error.value).toBeTruthy()
    expect(loading.value).toBe(false)
  })
})

describe('useVulnerabilities', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    if (window.go) delete window.go
    invalidateCachePrefix('ListVulnerabilities')
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('returns images, loading, error, listVulnerabilities, scanImage, scanAllImages', () => {
    const result = useVulnerabilities()
    expect(result.images.value).toEqual([])
    expect(result.loading.value).toBe(false)
    expect(result.error.value).toBe(null)
    expect(typeof result.listVulnerabilities).toBe('function')
    expect(typeof result.scanImage).toBe('function')
    expect(typeof result.scanAllImages).toBe('function')
  })

  it('listVulnerabilities populates images from backend', async () => {
    const imageData = [
      { name: 'nginx:latest', vulnerabilities: 3 },
      { name: 'redis:alpine', vulnerabilities: 1 },
    ]

    vi.stubGlobal('go', {
      pkg: {
        App: {
          ListVulnerabilities: vi.fn().mockResolvedValue(imageData),
        },
      },
    })

    const { images, loading, error, listVulnerabilities } = useVulnerabilities()
    await listVulnerabilities()

    expect(images.value).toEqual(imageData)
    expect(error.value).toBe(null)
    expect(loading.value).toBe(false)
  })

  it('listVulnerabilities handles null result', async () => {
    vi.stubGlobal('go', {
      pkg: {
        App: {
          ListVulnerabilities: vi.fn().mockResolvedValue(null),
        },
      },
    })

    const { images, listVulnerabilities } = useVulnerabilities()
    await listVulnerabilities()

    expect(images.value).toEqual([])
  })

  it('listVulnerabilities handles errors gracefully', async () => {
    vi.stubGlobal('go', {
      pkg: {
        App: {
          ListVulnerabilities: vi.fn().mockRejectedValue(new Error('API error')),
        },
      },
    })

    const { images, error, listVulnerabilities } = useVulnerabilities()
    await listVulnerabilities()

    expect(images.value).toEqual([])
    expect(error.value).toBeTruthy()
  })

  it('scanImage calls ScanImage and refreshes list', async () => {
    const scanResult = { vulnerabilities: [], summary: 'clean' }
    const mockScan = vi.fn().mockResolvedValue(scanResult)
    const mockList = vi.fn().mockResolvedValue([])

    vi.stubGlobal('go', {
      pkg: {
        App: {
          ScanImage: mockScan,
          ListVulnerabilities: mockList,
        },
      },
    })

    const { scanImage } = useVulnerabilities()
    const result = await scanImage('nginx:latest', 'trivy')

    expect(mockScan).toHaveBeenCalledWith('nginx:latest', 'trivy')
    expect(result).toEqual(scanResult)
  })

  it('scanImage returns error string on failure', async () => {
    vi.stubGlobal('go', {
      pkg: {
        App: {
          ScanImage: vi.fn().mockRejectedValue(new Error('Scan error')),
          ListVulnerabilities: vi.fn().mockResolvedValue([]),
        },
      },
    })

    const { scanImage } = useVulnerabilities()
    const result = await scanImage('bad:image', 'trivy')

    expect(result).toBe('Scan failed')
  })

  it('scanAllImages calls ScanAllImages and refreshes list', async () => {
    const scanResults = [{ name: 'nginx:latest', vulnerabilities: 0 }]
    const mockScanAll = vi.fn().mockResolvedValue(scanResults)

    vi.stubGlobal('go', {
      pkg: {
        App: {
          ScanAllImages: mockScanAll,
          ListVulnerabilities: vi.fn().mockResolvedValue([]),
        },
      },
    })

    const { images, error, loading, scanAllImages } = useVulnerabilities()
    const result = await scanAllImages()

    expect(mockScanAll).toHaveBeenCalledWith('')
    expect(result).toEqual(scanResults)
    expect(images.value).toEqual(scanResults)
    expect(loading.value).toBe(false)
  })

  it('scanAllImages passes namespace', async () => {
    const mockScanAll = vi.fn().mockResolvedValue([])

    vi.stubGlobal('go', {
      pkg: {
        App: {
          ScanAllImages: mockScanAll,
          ListVulnerabilities: vi.fn().mockResolvedValue([]),
        },
      },
    })

    const { scanAllImages } = useVulnerabilities()
    await scanAllImages('kube-system')

    expect(mockScanAll).toHaveBeenCalledWith('kube-system')
  })

  it('scanAllImages handles errors gracefully', async () => {
    vi.stubGlobal('go', {
      pkg: {
        App: {
          ScanAllImages: vi.fn().mockRejectedValue(new Error('Scan all failed')),
        },
      },
    })

    const { images, error, loading, scanAllImages } = useVulnerabilities()
    const result = await scanAllImages()

    expect(result).toBe(null)
    expect(error.value).toBeTruthy()
    expect(loading.value).toBe(false)
  })
})
