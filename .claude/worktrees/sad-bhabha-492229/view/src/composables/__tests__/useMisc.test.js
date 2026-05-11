import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { useCodeBlock } from '../useMisc'

describe('useCodeBlock', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    if (window.go) delete window.go
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('returns isRunning, output, isAnalyzing, suggestion, runCode, getAiSuggestion', () => {
    const result = useCodeBlock()
    expect(result.isRunning.value).toBe(false)
    expect(result.output.value).toBe('')
    expect(result.isAnalyzing.value).toBe(false)
    expect(result.suggestion.value).toBe('')
    expect(typeof result.runCode).toBe('function')
    expect(typeof result.getAiSuggestion).toBe('function')
  })

  it('runCode calls RunCodeSandbox and sets output', async () => {
    const mockRunCode = vi.fn().mockResolvedValue('Hello World!')

    vi.stubGlobal('go', {
      pkg: {
        App: {
          RunCodeSandbox: mockRunCode,
        },
      },
    })

    const { output, isRunning, runCode } = useCodeBlock()
    await runCode('console.log("Hello World!")', 'javascript')

    expect(mockRunCode).toHaveBeenCalledWith('console.log("Hello World!")', 'javascript')
    expect(output.value).toBe('Hello World!')
    expect(isRunning.value).toBe(false)
  })

  it('runCode returns "No output" when result is empty', async () => {
    vi.stubGlobal('go', {
      pkg: {
        App: {
          RunCodeSandbox: vi.fn().mockResolvedValue(null),
        },
      },
    })

    const { output, runCode } = useCodeBlock()
    await runCode('echo test', 'bash')

    expect(output.value).toBe('No output')
  })

  it('runCode sets output to error message on failure', async () => {
    const errorMsg = 'SyntaxError: unexpected token'
    vi.stubGlobal('go', {
      pkg: {
        App: {
          RunCodeSandbox: vi.fn().mockRejectedValue(new Error(errorMsg)),
        },
      },
    })

    const { output, isRunning, runCode } = useCodeBlock()
    await runCode('bad code', 'javascript')

    expect(output.value).toBe(errorMsg)
    expect(isRunning.value).toBe(false)
  })

  it('runCode sets isRunning correctly', async () => {
    vi.stubGlobal('go', {
      pkg: {
        App: {
          RunCodeSandbox: vi.fn().mockImplementation(
            () => new Promise(r => setTimeout(() => r('done'), 10))
          ),
        },
      },
    })

    const { isRunning, runCode } = useCodeBlock()
    const promise = runCode('test', 'python')

    expect(isRunning.value).toBe(true)
    await promise
    expect(isRunning.value).toBe(false)
  })

  it('getAiSuggestion calls GetCodeSuggestion and sets suggestion', async () => {
    const suggestionText = 'Use async/await instead'
    const mockSuggest = vi.fn().mockResolvedValue(suggestionText)

    vi.stubGlobal('go', {
      pkg: {
        App: {
          GetCodeSuggestion: mockSuggest,
        },
      },
    })

    const { suggestion, isAnalyzing, getAiSuggestion } = useCodeBlock()
    await getAiSuggestion('function foo() { return bar; }', 'javascript')

    expect(mockSuggest).toHaveBeenCalledWith('function foo() { return bar; }', 'javascript')
    expect(suggestion.value).toBe(suggestionText)
    expect(isAnalyzing.value).toBe(false)
  })

  it('getAiSuggestion returns fallback when result is empty', async () => {
    vi.stubGlobal('go', {
      pkg: {
        App: {
          GetCodeSuggestion: vi.fn().mockResolvedValue(null),
        },
      },
    })

    const { suggestion, getAiSuggestion } = useCodeBlock()
    await getAiSuggestion('code', 'python')

    expect(suggestion.value).toBe('No suggestion available')
  })

  it('getAiSuggestion sets suggestion to error message on failure', async () => {
    const errorMsg = 'API rate limit exceeded'
    vi.stubGlobal('go', {
      pkg: {
        App: {
          GetCodeSuggestion: vi.fn().mockRejectedValue(new Error(errorMsg)),
        },
      },
    })

    const { suggestion, isAnalyzing, getAiSuggestion } = useCodeBlock()
    await getAiSuggestion('code', 'go')

    expect(suggestion.value).toBe(errorMsg)
    expect(isAnalyzing.value).toBe(false)
  })

  it('getAiSuggestion sets isAnalyzing correctly', async () => {
    vi.stubGlobal('go', {
      pkg: {
        App: {
          GetCodeSuggestion: vi.fn().mockImplementation(
            () => new Promise(r => setTimeout(() => r('suggestion'), 10))
          ),
        },
      },
    })

    const { isAnalyzing, getAiSuggestion } = useCodeBlock()
    const promise = getAiSuggestion('code', 'python')

    expect(isAnalyzing.value).toBe(true)
    await promise
    expect(isAnalyzing.value).toBe(false)
  })

  it('resets output to empty string before each runCode', async () => {
    const mockRunCode = vi.fn().mockResolvedValue('new output')

    vi.stubGlobal('go', {
      pkg: {
        App: {
          RunCodeSandbox: mockRunCode,
        },
      },
    })

    const { output, runCode } = useCodeBlock()
    output.value = 'old output'

    await runCode('new code', 'javascript')

    expect(output.value).toBe('new output')
  })

  it('resets suggestion to empty string before each getAiSuggestion', async () => {
    const mockSuggest = vi.fn().mockResolvedValue('new suggestion')

    vi.stubGlobal('go', {
      pkg: {
        App: {
          GetCodeSuggestion: mockSuggest,
        },
      },
    })

    const { suggestion, getAiSuggestion } = useCodeBlock()
    suggestion.value = 'old suggestion'

    await getAiSuggestion('new code', 'python')

    expect(suggestion.value).toBe('new suggestion')
  })
})
