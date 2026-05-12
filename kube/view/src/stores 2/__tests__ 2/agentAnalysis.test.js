import { describe, it, expect, beforeEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { useAgentAnalysisStore } from '../agentAnalysis'

describe('agentAnalysis store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('starts with 5 default checklist items', () => {
    const s = useAgentAnalysisStore()
    expect(s.checklist).toEqual([
      'Node Health',
      'Pod Restarts',
      'Network Latency',
      'Pending Alerts',
      'Recent Incidents',
    ])
  })

  it('addItem appends a new unique item', () => {
    const s = useAgentAnalysisStore()
    s.addItem('CPU Usage')
    expect(s.checklist).toContain('CPU Usage')
    expect(s.checklist).toHaveLength(6)
  })

  it('addItem rejects duplicates', () => {
    const s = useAgentAnalysisStore()
    s.addItem('Node Health')
    expect(s.checklist).toHaveLength(5)
  })

  it('addItem rejects null input', () => {
    const s = useAgentAnalysisStore()
    s.addItem(null)
    expect(s.checklist).toHaveLength(5)
  })

  it('addItem rejects empty string', () => {
    const s = useAgentAnalysisStore()
    s.addItem('')
    expect(s.checklist).toHaveLength(5)
  })

  it('removeItem removes at valid index', () => {
    const s = useAgentAnalysisStore()
    s.removeItem(0)
    expect(s.checklist).not.toContain('Node Health')
    expect(s.checklist).toHaveLength(4)
  })

  it('updateItem updates at valid index with trimmed value', () => {
    const s = useAgentAnalysisStore()
    s.updateItem(0, '  Cluster Health  ')
    expect(s.checklist[0]).toBe('Cluster Health')
  })

  it('updateItem rejects empty value', () => {
    const s = useAgentAnalysisStore()
    s.updateItem(0, '')
    expect(s.checklist[0]).toBe('Node Health')
  })

  it('updateItem rejects whitespace-only value', () => {
    const s = useAgentAnalysisStore()
    s.updateItem(0, '   ')
    expect(s.checklist[0]).toBe('Node Health')
  })
})
