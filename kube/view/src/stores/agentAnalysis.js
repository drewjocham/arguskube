import { defineStore } from 'pinia'
import { ref } from 'vue'

export const useAgentAnalysisStore = defineStore('agentAnalysis', () => {
  const checklist = ref([
    'Node Health',
    'Pod Restarts',
    'Network Latency',
    'Pending Alerts',
    'Recent Incidents'
  ])

  function addItem(item) {
    if (item && !checklist.value.includes(item)) {
      checklist.value.push(item)
    }
  }

  function removeItem(index) {
    checklist.value.splice(index, 1)
  }

  function updateItem(index, val) {
    if (val && val.trim()) {
      checklist.value[index] = val.trim()
    }
  }

  return { checklist, addItem, removeItem, updateItem }
})
