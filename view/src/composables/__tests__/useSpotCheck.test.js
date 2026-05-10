import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { useSpotCheck } from '../useSpotCheck'

const callGoMock = vi.hoisted(() => vi.fn())

vi.mock('../useBridge', () => ({
  callGo: callGoMock,
}))

vi.mock('../useEvents', () => ({
  useWailsEvent: vi.fn(),
  Events: {},
}))

describe('useSpotCheck', () => {
  let consoleWarnSpy

  beforeEach(() => {
    callGoMock.mockReset()
    consoleWarnSpy = vi.spyOn(console, 'warn').mockImplementation(() => {})
  })

  afterEach(() => {
    consoleWarnSpy.mockRestore()
  })

  it('returns active, runAll, and runOne', () => {
    const sc = useSpotCheck()
    expect(sc.active).toBeDefined()
    expect(typeof sc.runAll).toBe('function')
    expect(typeof sc.runOne).toBe('function')
  })

  it('runAll calls callGo with RunSpotChecks', async () => {
    callGoMock.mockResolvedValue(undefined)
    const sc = useSpotCheck()
    await sc.runAll()
    expect(callGoMock).toHaveBeenCalledWith('RunSpotChecks')
  })

  it('runOne calls callGo with RunSpotCheck and the name', async () => {
    callGoMock.mockResolvedValue(undefined)
    const sc = useSpotCheck()
    await sc.runOne('node-readiness')
    expect(callGoMock).toHaveBeenCalledWith('RunSpotCheck', 'node-readiness')
  })

  it('runOne returns early when name is empty', async () => {
    const sc = useSpotCheck()
    const result = await sc.runOne('')
    expect(result).toBeUndefined()
    expect(callGoMock).not.toHaveBeenCalled()
  })

  it('runAll catches errors and warns', async () => {
    callGoMock.mockRejectedValue(new Error('network error'))
    const sc = useSpotCheck()
    await sc.runAll()
    expect(consoleWarnSpy).toHaveBeenCalledWith('[spot-check] RunSpotChecks failed:', expect.any(Error))
  })

  it('runOne catches errors and warns', async () => {
    callGoMock.mockRejectedValue(new Error('timeout'))
    const sc = useSpotCheck()
    await sc.runOne('test-check')
    expect(consoleWarnSpy).toHaveBeenCalledWith('[spot-check] RunSpotCheck failed:', 'test-check', expect.any(Error))
  })
})
