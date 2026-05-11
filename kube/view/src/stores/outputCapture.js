import { defineStore } from 'pinia'

// At any moment at most one CodeBlock instance is the "active capture target"
// for terminal output. When the user clicks Run on a code block, that block
// becomes the active target and any subsequent terminal output is appended
// to its buffer (with ANSI escapes stripped). Clicking Run on another block
// switches the target — the previous block's buffer is preserved but no
// longer accepting new lines.
//
// This is one Pinia store rather than per-block local state because the
// "active target" is genuinely cross-component and only-one-at-a-time.

const ANSI_RE = /\x1b\[[0-9;?]*[a-zA-Z]/g
const MAX_BYTES = 64 * 1024 // cap buffer at 64KB per block

function stripAnsi(s) {
  if (!s) return ''
  return String(s).replace(ANSI_RE, '')
}

export const useOutputCaptureStore = defineStore('outputCapture', {
  state: () => ({
    activeBlockId: null, // string | null — id of the CodeBlock that owns the next output chunks
    buffers: {},         // { [blockId]: string } accumulated output per block
    // capturedAt[blockId] = ms timestamp the capture started; used for UX timing
    capturedAt: {},
  }),

  actions: {
    startCapture(blockId) {
      if (!blockId) return
      this.activeBlockId = blockId
      // Reset the buffer for this block so each Run shows fresh output.
      this.buffers[blockId] = ''
      this.capturedAt[blockId] = Date.now()
    },

    stopCapture(blockId) {
      if (this.activeBlockId === blockId) {
        this.activeBlockId = null
      }
    },

    appendOutput(chunk) {
      if (!this.activeBlockId) return
      const id = this.activeBlockId
      const cleaned = stripAnsi(chunk)
      if (!cleaned) return
      const current = this.buffers[id] || ''
      let next = current + cleaned
      if (next.length > MAX_BYTES) {
        // Keep the tail — most recent output is the most useful.
        next = next.slice(next.length - MAX_BYTES)
      }
      this.buffers[id] = next
    },

    clearBuffer(blockId) {
      if (!blockId) return
      this.buffers[blockId] = ''
    },

    isCapturing(blockId) {
      return this.activeBlockId === blockId
    },

    bufferFor(blockId) {
      return this.buffers[blockId] || ''
    },
  },
})

// Test-only export for the ANSI stripper.
export const __test = { stripAnsi }
