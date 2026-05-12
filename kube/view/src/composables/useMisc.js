import { ref } from 'vue'
import { callGo } from './useBridge'

/**
 * Composable for code blocks and sandboxing.
 */
export function useCodeBlock() {
  const isRunning = ref(false)
  const output = ref('')
  const isAnalyzing = ref(false)
  const suggestion = ref('')

  async function runCode(code, language) {
    isRunning.value = true
    output.value = ''
    try {
      const result = await callGo('RunCodeSandbox', code, language)
      output.value = result || 'No output'
    } catch (e) {
      output.value = e?.message || String(e)
    } finally {
      isRunning.value = false
    }
  }

  async function getAiSuggestion(code, language) {
    isAnalyzing.value = true
    suggestion.value = ''
    try {
      const result = await callGo('GetCodeSuggestion', code, language)
      suggestion.value = result || 'No suggestion available'
    } catch (e) {
      suggestion.value = e?.message || String(e)
    } finally {
      isAnalyzing.value = false
    }
  }

  return { isRunning, output, isAnalyzing, suggestion, runCode, getAiSuggestion }
}
