import { defineStore } from 'pinia'
import type { CaptureBuffers, CaptureTimestamps } from '../types/terminal'

const ANSI_RE = /\x1b\[[0-9;?]*[a-zA-Z]/g
const MAX_BYTES = 64 * 1024

function stripAnsi(s: unknown): string {
  if (!s) return ''
  return String(s).replace(ANSI_RE, '')
}

interface OutputCaptureState {
  activeBlockId: string | null
  buffers: CaptureBuffers
  capturedAt: CaptureTimestamps
}

export const useOutputCaptureStore = defineStore('outputCapture', {
  state: (): OutputCaptureState => ({
    activeBlockId: null,
    buffers: {},
    capturedAt: {},
  }),

  actions: {
    startCapture(blockId: string): void {
      if (!blockId) return
      this.activeBlockId = blockId
      this.buffers[blockId] = ''
      this.capturedAt[blockId] = Date.now()
    },

    stopCapture(blockId: string): void {
      if (this.activeBlockId === blockId) {
        this.activeBlockId = null
      }
    },

    appendOutput(chunk: string): void {
      if (!this.activeBlockId) return
      const id = this.activeBlockId
      const cleaned = stripAnsi(chunk)
      if (!cleaned) return
      const current = this.buffers[id] || ''
      let next = current + cleaned
      if (next.length > MAX_BYTES) {
        next = next.slice(next.length - MAX_BYTES)
      }
      this.buffers[id] = next
    },

    clearBuffer(blockId: string): void {
      if (!blockId) return
      this.buffers[blockId] = ''
    },

    isCapturing(blockId: string): boolean {
      return this.activeBlockId === blockId
    },

    bufferFor(blockId: string): string {
      return this.buffers[blockId] || ''
    },
  },
})

export const __test = { stripAnsi }
