import { describe, it, expect } from 'vitest'
import { useOutputCaptureStore, __test } from '../outputCapture'

describe('outputCapture.stripAnsi', () => {
  it('strips standard SGR color codes', () => {
    expect(__test.stripAnsi('\x1b[31mred\x1b[0m')).toBe('red')
    expect(__test.stripAnsi('\x1b[1;32mok\x1b[0m')).toBe('ok')
  })
  it('strips cursor and other CSI sequences', () => {
    expect(__test.stripAnsi('a\x1b[2Kb')).toBe('ab')
    expect(__test.stripAnsi('a\x1b[?25hb')).toBe('ab')
  })
  it('leaves plain text unchanged', () => {
    expect(__test.stripAnsi('plain text\nline 2')).toBe('plain text\nline 2')
  })
  it('handles empty/nullish', () => {
    expect(__test.stripAnsi('')).toBe('')
    expect(__test.stripAnsi(null)).toBe('')
    expect(__test.stripAnsi(undefined)).toBe('')
  })
})

describe('outputCapture store', () => {
  it('starts with no active block and an empty buffer map', () => {
    const s = useOutputCaptureStore()
    expect(s.activeBlockId).toBeNull()
    expect(s.buffers).toEqual({})
  })

  it('startCapture sets the active block and resets its buffer', () => {
    const s = useOutputCaptureStore()
    s.startCapture('block-1')
    expect(s.activeBlockId).toBe('block-1')
    expect(s.buffers['block-1']).toBe('')
    expect(s.isCapturing('block-1')).toBe(true)
  })

  it('appendOutput is a no-op when no block is capturing', () => {
    const s = useOutputCaptureStore()
    s.appendOutput('some output')
    expect(s.buffers).toEqual({})
  })

  it('appendOutput appends to the active block buffer with ANSI stripped', () => {
    const s = useOutputCaptureStore()
    s.startCapture('block-1')
    s.appendOutput('\x1b[32mhello\x1b[0m\n')
    s.appendOutput('world')
    expect(s.bufferFor('block-1')).toBe('hello\nworld')
  })

  it('switching active blocks preserves the previous buffer but routes new output elsewhere', () => {
    const s = useOutputCaptureStore()
    s.startCapture('block-A')
    s.appendOutput('output for A\n')

    s.startCapture('block-B')
    s.appendOutput('output for B\n')

    expect(s.bufferFor('block-A')).toBe('output for A\n')
    expect(s.bufferFor('block-B')).toBe('output for B\n')
    expect(s.activeBlockId).toBe('block-B')
  })

  it('startCapture twice on the same block resets the buffer', () => {
    const s = useOutputCaptureStore()
    s.startCapture('block-1')
    s.appendOutput('first run\n')
    s.startCapture('block-1')
    expect(s.bufferFor('block-1')).toBe('')
  })

  it('stopCapture clears active when called for the active block', () => {
    const s = useOutputCaptureStore()
    s.startCapture('block-1')
    s.stopCapture('block-1')
    expect(s.activeBlockId).toBeNull()

    s.startCapture('block-2')
    s.stopCapture('block-1')
    expect(s.activeBlockId).toBe('block-2')
  })

  it('clearBuffer wipes the buffer for a specific block', () => {
    const s = useOutputCaptureStore()
    s.startCapture('block-1')
    s.appendOutput('some data')
    s.clearBuffer('block-1')
    expect(s.bufferFor('block-1')).toBe('')
  })

  it('caps a single block buffer at the configured maximum', () => {
    const s = useOutputCaptureStore()
    s.startCapture('block-1')
    const chunk = 'x'.repeat(8192)
    for (let i = 0; i < 12; i++) s.appendOutput(chunk)
    expect(s.bufferFor('block-1').length).toBeLessThanOrEqual(64 * 1024)
  })
})
