import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { useSetup, useAnomaly } from '../useSetup'
import { invalidateCachePrefix } from '../useBridge'

describe('useSetup', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    if (window.go) delete window.go
    invalidateCachePrefix('CheckToolStatus')
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('returns tools, loading, actionLoading, checkTools, installArgusScan, deployAgent, undeployAgent', () => {
    const result = useSetup()
    expect(result.tools.value).toEqual([])
    expect(result.loading.value).toBe(false)
    expect(result.actionLoading.value).toBe(null)
    expect(typeof result.checkTools).toBe('function')
    expect(typeof result.installArgusScan).toBe('function')
    expect(typeof result.deployAgent).toBe('function')
    expect(typeof result.undeployAgent).toBe('function')
  })

  it('checkTools populates tools from backend', async () => {
    const toolData = [
      { name: 'argus-scan', installed: false },
      { name: 'kubewatcher-agent', installed: true },
    ]

    vi.stubGlobal('go', {
      pkg: {
        App: {
          CheckToolStatus: vi.fn().mockResolvedValue(toolData),
        },
      },
    })

    const { tools, loading, error, checkTools } = useSetup()
    // loading is false initially, set to true inside checkTools when tools array is empty
    await checkTools()

    expect(tools.value).toEqual(toolData)
    expect(loading.value).toBe(false)
  })

  it('checkTools handles null result', async () => {
    vi.stubGlobal('go', {
      pkg: {
        App: {
          CheckToolStatus: vi.fn().mockResolvedValue(null),
        },
      },
    })

    const { tools, checkTools } = useSetup()
    await checkTools()

    expect(tools.value).toEqual([])
  })

  it('checkTools handles errors gracefully', async () => {
    vi.stubGlobal('go', {
      pkg: {
        App: {
          CheckToolStatus: vi.fn().mockRejectedValue(new Error('Backend error')),
        },
      },
    })

    const { tools, checkTools } = useSetup()
    await checkTools()

    expect(tools.value).toEqual([])
  })

  it('installArgusScan calls InstallArgusScan and refreshes tools', async () => {
    const resultData = { success: true }
    const mockInstall = vi.fn().mockResolvedValue(resultData)
    const mockTools = vi.fn().mockResolvedValue([])

    vi.stubGlobal('go', {
      pkg: {
        App: {
          InstallArgusScan: mockInstall,
          CheckToolStatus: mockTools,
        },
      },
    })

    const { actionLoading, installArgusScan } = useSetup()
    const result = await installArgusScan()

    expect(mockInstall).toHaveBeenCalled()
    expect(result).toEqual(resultData)
    expect(actionLoading.value).toBe(null)
  })

  it('installArgusScan handles errors gracefully', async () => {
    vi.stubGlobal('go', {
      pkg: {
        App: {
          InstallArgusScan: vi.fn().mockRejectedValue(new Error('Install failed')),
          CheckToolStatus: vi.fn().mockResolvedValue([]),
        },
      },
    })

    const { actionLoading, installArgusScan } = useSetup()
    const result = await installArgusScan()

    expect(result.success).toBe(false)
    expect(result.message).toBeTruthy()
    expect(actionLoading.value).toBe(null)
  })

  it('deployAgent calls DeployAgent with namespace', async () => {
    const resultData = { success: true }
    const mockDeploy = vi.fn().mockResolvedValue(resultData)
    const mockTools = vi.fn().mockResolvedValue([])

    vi.stubGlobal('go', {
      pkg: {
        App: {
          DeployAgent: mockDeploy,
          CheckToolStatus: mockTools,
        },
      },
    })

    const { actionLoading, deployAgent } = useSetup()
    const result = await deployAgent('custom-ns')

    expect(mockDeploy).toHaveBeenCalledWith('custom-ns')
    expect(result).toEqual(resultData)
    expect(actionLoading.value).toBe(null)
  })

  it('deployAgent uses default namespace when none provided', async () => {
    const mockDeploy = vi.fn().mockResolvedValue({ success: true })
    vi.stubGlobal('go', {
      pkg: {
        App: {
          DeployAgent: mockDeploy,
          CheckToolStatus: vi.fn().mockResolvedValue([]),
        },
      },
    })

    const { deployAgent } = useSetup()
    await deployAgent()

    expect(mockDeploy).toHaveBeenCalledWith('kubewatcher')
  })

  it('deployAgent handles errors gracefully', async () => {
    vi.stubGlobal('go', {
      pkg: {
        App: {
          DeployAgent: vi.fn().mockRejectedValue(new Error('Deploy failed')),
          CheckToolStatus: vi.fn().mockResolvedValue([]),
        },
      },
    })

    const { deployAgent } = useSetup()
    const result = await deployAgent()

    expect(result.success).toBe(false)
    expect(result.message).toBeTruthy()
  })

  it('undeployAgent calls UndeployAgent with namespace', async () => {
    const resultData = { success: true }
    const mockUndeploy = vi.fn().mockResolvedValue(resultData)
    const mockTools = vi.fn().mockResolvedValue([])

    vi.stubGlobal('go', {
      pkg: {
        App: {
          UndeployAgent: mockUndeploy,
          CheckToolStatus: mockTools,
        },
      },
    })

    const { actionLoading, undeployAgent } = useSetup()
    const result = await undeployAgent('custom-ns')

    expect(mockUndeploy).toHaveBeenCalledWith('custom-ns')
    expect(result).toEqual(resultData)
    expect(actionLoading.value).toBe(null)
  })

  it('undeployAgent uses default namespace when none provided', async () => {
    const mockUndeploy = vi.fn().mockResolvedValue({ success: true })
    vi.stubGlobal('go', {
      pkg: {
        App: {
          UndeployAgent: mockUndeploy,
          CheckToolStatus: vi.fn().mockResolvedValue([]),
        },
      },
    })

    const { undeployAgent } = useSetup()
    await undeployAgent()

    expect(mockUndeploy).toHaveBeenCalledWith('kubewatcher')
  })

  it('undeployAgent handles errors gracefully', async () => {
    vi.stubGlobal('go', {
      pkg: {
        App: {
          UndeployAgent: vi.fn().mockRejectedValue(new Error('Undeploy failed')),
          CheckToolStatus: vi.fn().mockResolvedValue([]),
        },
      },
    })

    const { undeployAgent } = useSetup()
    const result = await undeployAgent()

    expect(result.success).toBe(false)
    expect(result.message).toBeTruthy()
  })

  it('checkTools sets loading=true initially when tools empty', async () => {
    vi.stubGlobal('go', {
      pkg: {
        App: {
          CheckToolStatus: vi.fn().mockResolvedValue([]),
        },
      },
    })

    // loading is set to true inside checkTools() when tools array is empty
    const { loading, checkTools } = useSetup()
    expect(loading.value).toBe(false)
    checkTools()
    expect(loading.value).toBe(true)
    await checkTools()
  })
})

describe('useAnomaly', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    if (window.go) delete window.go
    invalidateCachePrefix('GetAnomalySettings')
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('returns all expected refs and methods', () => {
    const result = useAnomaly()
    expect(result.anomalies.value).toEqual([])
    expect(result.settings.value).toBe(null)
    expect(result.rules.value).toEqual([])
    expect(result.jobs.value).toEqual([])
    expect(result.loading.value).toBe(false)
    expect(result.error.value).toBe(null)
    expect(typeof result.connectAgent).toBe('function')
    expect(typeof result.getSettings).toBe('function')
    expect(typeof result.saveSettings).toBe('function')
    expect(typeof result.getRules).toBe('function')
    expect(typeof result.saveRule).toBe('function')
    expect(typeof result.toggleRule).toBe('function')
    expect(typeof result.deleteRule).toBe('function')
    expect(typeof result.getJobs).toBe('function')
  })

  it('connectAgent calls ConnectToAgent and populates anomalies', async () => {
    const anomalyData = [
      { id: 'anom-1', type: 'cpu-spike', severity: 'critical' },
    ]

    vi.stubGlobal('go', {
      pkg: {
        App: {
          ConnectToAgent: vi.fn().mockResolvedValue(anomalyData),
        },
      },
    })

    const { anomalies, loading, connectAgent } = useAnomaly()
    await connectAgent()

    expect(anomalies.value).toEqual(anomalyData)
    expect(loading.value).toBe(false)
  })

  it('connectAgent passes namespace and handles null result', async () => {
    vi.stubGlobal('go', {
      pkg: {
        App: {
          ConnectToAgent: vi.fn().mockResolvedValue(null),
        },
      },
    })

    const { anomalies, connectAgent } = useAnomaly()
    await connectAgent('kube-system')

    expect(anomalies.value).toEqual([])
  })

  it('connectAgent handles errors gracefully', async () => {
    vi.stubGlobal('go', {
      pkg: {
        App: {
          ConnectToAgent: vi.fn().mockRejectedValue(new Error('Connection error')),
        },
      },
    })

    const { anomalies, error, loading, connectAgent } = useAnomaly()
    await connectAgent()

    expect(anomalies.value).toEqual([])
    expect(error.value).toBeTruthy()
    expect(loading.value).toBe(false)
  })

  it('getSettings fetches settings from backend', async () => {
    const settingsData = { enabled: true, interval: 60 }

    vi.stubGlobal('go', {
      pkg: {
        App: {
          GetAnomalySettings: vi.fn().mockResolvedValue(settingsData),
        },
      },
    })

    const { settings, getSettings } = useAnomaly()
    await getSettings()

    expect(settings.value).toEqual(settingsData)
  })

  it('getSettings handles errors gracefully', async () => {
    vi.stubGlobal('go', {
      pkg: {
        App: {
          GetAnomalySettings: vi.fn().mockRejectedValue(new Error('Settings error')),
        },
      },
    })

    const { settings, error, getSettings } = useAnomaly()
    await getSettings()

    // composable sets error but does NOT clear settings ref on error
    expect(error.value).toBeTruthy()
  })

  it('saveSettings calls SaveAnomalySettings and updates local ref', async () => {
    const newSettings = { enabled: false, interval: 120 }
    const mockSave = vi.fn().mockResolvedValue(undefined)

    vi.stubGlobal('go', {
      pkg: {
        App: {
          SaveAnomalySettings: mockSave,
        },
      },
    })

    const { settings, saveSettings } = useAnomaly()
    await saveSettings(newSettings)

    expect(mockSave).toHaveBeenCalledWith(newSettings)
    expect(settings.value).toEqual(newSettings)
  })

  it('saveSettings throws on error', async () => {
    vi.stubGlobal('go', {
      pkg: {
        App: {
          SaveAnomalySettings: vi.fn().mockRejectedValue(new Error('Save failed')),
        },
      },
    })

    const { saveSettings } = useAnomaly()
    await expect(saveSettings({})).rejects.toThrow('Save failed')
  })

  it('getRules fetches rules from backend', async () => {
    const rulesData = [{ id: 'rule-1', name: 'CPU spike', enabled: true }]

    vi.stubGlobal('go', {
      pkg: {
        App: {
          GetAnomalyRules: vi.fn().mockResolvedValue(rulesData),
        },
      },
    })

    const { rules, getRules } = useAnomaly()
    await getRules()

    expect(rules.value).toEqual(rulesData)
  })

  it('getRules handles errors gracefully', async () => {
    vi.stubGlobal('go', {
      pkg: {
        App: {
          GetAnomalyRules: vi.fn().mockRejectedValue(new Error('Rules error')),
        },
      },
    })

    const { rules, error, getRules } = useAnomaly()
    await getRules()

    expect(rules.value).toEqual([])
    expect(error.value).toBeTruthy()
  })

  it('saveRule calls SaveAnomalyRule and refreshes rules', async () => {
    const ruleData = { id: 'rule-1', name: 'CPU spike', enabled: true }
    const mockSave = vi.fn().mockResolvedValue(undefined)
    const mockGetRules = vi.fn().mockResolvedValue([ruleData])

    vi.stubGlobal('go', {
      pkg: {
        App: {
          SaveAnomalyRule: mockSave,
          GetAnomalyRules: mockGetRules,
        },
      },
    })

    const { rules, saveRule } = useAnomaly()
    await saveRule(ruleData)

    expect(mockSave).toHaveBeenCalledWith(ruleData)
    expect(mockGetRules).toHaveBeenCalled()
    expect(rules.value).toEqual([ruleData])
  })

  it('saveRule throws on error', async () => {
    vi.stubGlobal('go', {
      pkg: {
        App: {
          SaveAnomalyRule: vi.fn().mockRejectedValue(new Error('Save error')),
          GetAnomalyRules: vi.fn().mockResolvedValue([]),
        },
      },
    })

    const { saveRule } = useAnomaly()
    await expect(saveRule({})).rejects.toThrow('Save error')
  })

  it('toggleRule calls ToggleAnomalyRule and updates local state', async () => {
    const mockToggle = vi.fn().mockResolvedValue(true)

    vi.stubGlobal('go', {
      pkg: {
        App: {
          ToggleAnomalyRule: mockToggle,
        },
      },
    })

    const { rules, toggleRule } = useAnomaly()
    rules.value = [{ id: 'rule-1', name: 'CPU', enabled: false }]

    await toggleRule('rule-1')

    expect(mockToggle).toHaveBeenCalledWith('rule-1')
    expect(rules.value[0].enabled).toBe(true)
  })

  it('toggleRule handles errors gracefully', async () => {
    vi.stubGlobal('go', {
      pkg: {
        App: {
          ToggleAnomalyRule: vi.fn().mockRejectedValue(new Error('Toggle error')),
        },
      },
    })

    const { rules, error, toggleRule } = useAnomaly()
    rules.value = [{ id: 'rule-1', name: 'CPU', enabled: false }]

    await toggleRule('rule-1')

    expect(rules.value[0].enabled).toBe(false)
    expect(error.value).toBeTruthy()
  })

  it('deleteRule calls DeleteAnomalyRule and removes from local list', async () => {
    const mockDelete = vi.fn().mockResolvedValue(undefined)

    vi.stubGlobal('go', {
      pkg: {
        App: {
          DeleteAnomalyRule: mockDelete,
        },
      },
    })

    const { rules, deleteRule } = useAnomaly()
    rules.value = [{ id: 'rule-1' }, { id: 'rule-2' }]

    await deleteRule('rule-1')

    expect(mockDelete).toHaveBeenCalledWith('rule-1')
    expect(rules.value).toEqual([{ id: 'rule-2' }])
  })

  it('deleteRule handles errors gracefully', async () => {
    vi.stubGlobal('go', {
      pkg: {
        App: {
          DeleteAnomalyRule: vi.fn().mockRejectedValue(new Error('Delete error')),
        },
      },
    })

    const { rules, deleteRule } = useAnomaly()
    rules.value = [{ id: 'rule-1' }]

    await deleteRule('rule-1')

    // composable removes rule from list only on success; on error it sets error ref
    expect(rules.value).toEqual([{ id: 'rule-1' }])
  })

  it('getJobs fetches jobs from backend', async () => {
    const jobsData = [{ id: 'job-1', name: 'Daily scan', status: 'active' }]

    vi.stubGlobal('go', {
      pkg: {
        App: {
          GetAnomalyJobs: vi.fn().mockResolvedValue(jobsData),
        },
      },
    })

    const { jobs, getJobs } = useAnomaly()
    await getJobs()

    expect(jobs.value).toEqual(jobsData)
  })

  it('getJobs handles errors gracefully', async () => {
    vi.stubGlobal('go', {
      pkg: {
        App: {
          GetAnomalyJobs: vi.fn().mockRejectedValue(new Error('Jobs error')),
        },
      },
    })

    const { jobs, getJobs } = useAnomaly()
    await getJobs()

    expect(jobs.value).toEqual([])
  })
})
