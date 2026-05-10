import { describe, it, expect, beforeEach, vi } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'

const STORAGE_KEY = 'kubewatcher.uiPrefs.v1'

// jsdom in some setups exposes localStorage as a stub without setItem; install
// a tiny in-memory replacement so tests can seed/inspect it directly.
const memoryStore = {}
const fakeLocalStorage = {
  getItem: (k) => (k in memoryStore ? memoryStore[k] : null),
  setItem: (k, v) => { memoryStore[k] = String(v) },
  removeItem: (k) => { delete memoryStore[k] },
  clear: () => { for (const k of Object.keys(memoryStore)) delete memoryStore[k] },
}
Object.defineProperty(window, 'localStorage', {
  value: fakeLocalStorage,
  writable: true,
  configurable: true,
})

describe('uiPrefs store', () => {
  beforeEach(() => {
    fakeLocalStorage.clear()
    vi.resetModules()
    setActivePinia(createPinia())
  })

  async function loadFreshStore() {
    // resetModules() forces a re-import so the store re-reads localStorage
    // during state() initialization. Combined with a fresh Pinia in the
    // beforeEach above, each load is a clean simulated "restart".
    setActivePinia(createPinia())
    const mod = await import('../uiPrefs.js')
    return mod.useUIPrefsStore()
  }

  it('falls back to the default panel width when no localStorage entry exists', async () => {
    const s = await loadFreshStore()
    expect(s.rightPanelWidth).toBe(340)
    expect(s.rightPanelMin).toBe(280)
    expect(s.rightPanelMax).toBe(720)
  })

  it('clamps invalid widths to the default on init', async () => {
    window.localStorage.setItem('kubewatcher.uiPrefs.v1', JSON.stringify({ rightPanelWidth: 'not a number' }))
    const s = await loadFreshStore()
    expect(s.rightPanelWidth).toBe(340)
  })

  it('setRightPanelWidth clamps below the minimum and above the maximum', async () => {
    const s = await loadFreshStore()
    s.setRightPanelWidth(50)
    expect(s.rightPanelWidth).toBe(280)
    s.setRightPanelWidth(2000)
    expect(s.rightPanelWidth).toBe(720)
  })

  it('setRightPanelWidth persists the value to localStorage', async () => {
    const s = await loadFreshStore()
    s.setRightPanelWidth(420)
    const persisted = JSON.parse(window.localStorage.getItem('kubewatcher.uiPrefs.v1'))
    expect(persisted.rightPanelWidth).toBe(420)
  })

  it('persisted width survives a "restart" (re-instantiating the store)', async () => {
    const a = await loadFreshStore()
    a.setRightPanelWidth(500)
    const b = await loadFreshStore()
    expect(b.rightPanelWidth).toBe(500)
  })

  it('chatPopOutOpen defaults to false and is NOT persisted', async () => {
    const a = await loadFreshStore()
    a.openChatPopOut()
    expect(a.chatPopOutOpen).toBe(true)
    const b = await loadFreshStore()
    expect(b.chatPopOutOpen).toBe(false)
  })

  it('openChatPopOut and closeChatPopOut toggle the flag', async () => {
    const s = await loadFreshStore()
    s.openChatPopOut()
    expect(s.chatPopOutOpen).toBe(true)
    s.closeChatPopOut()
    expect(s.chatPopOutOpen).toBe(false)
  })
})
