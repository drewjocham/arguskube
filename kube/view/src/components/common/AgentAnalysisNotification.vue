<script setup>
import { ref, onMounted, onUnmounted } from 'vue'
import { bus } from '../../lib/bus'
import { useAgentAnalysisStore } from '../../stores/agentAnalysis'

const store = useAgentAnalysisStore()

const state = ref('idle') // 'idle', 'analyzing', 'complete'
const lookingAt = ref('')
const resultText = ref('')
const isExpanded = ref(false)
const position = ref({ x: 20, y: 60 })

const newItemText = ref('')

let hideTimeout = null

bus.useWailsEvent('agent:analysis:start', (data) => {
  if (data && data.lookingAt) {
    lookingAt.value = data.lookingAt
    state.value = 'analyzing'
    isExpanded.value = false
    clearTimeout(hideTimeout)
    resetPosition()
  }
})

bus.useWailsEvent('agent:analysis:complete', (data) => {
  if (data && data.text) {
    resultText.value = data.text
    state.value = 'complete'
    startHideTimer()
  }
})

function startHideTimer() {
  clearTimeout(hideTimeout)
  if (!isExpanded.value) {
    hideTimeout = setTimeout(() => {
      dismiss()
    }, 10000)
  }
}

function dismiss() {
  state.value = 'idle'
  isExpanded.value = false
}

function toggleExpand() {
  if (state.value === 'analyzing') return
  isExpanded.value = !isExpanded.value
  if (isExpanded.value) {
    clearTimeout(hideTimeout) // Stop hide timer when expanded
  } else {
    startHideTimer()
  }
}

// Drag to dismiss
let isDragging = false
let startX = 0
let startY = 0
let currentX = 0
let currentY = 0

function resetPosition() {
  position.value = { x: 20, y: 60 }
  currentX = 0
  currentY = 0
}

function onMouseDown(e) {
  // Don't drag if clicking buttons or inputs inside expanded view
  if (isExpanded.value && (e.target.tagName === 'INPUT' || e.target.tagName === 'BUTTON' || e.target.closest('.checklist-area'))) {
    return
  }
  isDragging = true
  startX = e.clientX - currentX
  startY = e.clientY - currentY
  document.addEventListener('mousemove', onMouseMove)
  document.addEventListener('mouseup', onMouseUp)
}

function onMouseMove(e) {
  if (!isDragging) return
  currentX = e.clientX - startX
  currentY = e.clientY - startY
  position.value.x = 20 - currentX // Calculate from right
  position.value.y = 60 + currentY
}

function onMouseUp() {
  isDragging = false
  document.removeEventListener('mousemove', onMouseMove)
  document.removeEventListener('mouseup', onMouseUp)
  
  // If dragged far enough to the right (currentX > 150), dismiss
  if (currentX > 150) {
    dismiss()
  } else {
    // snap back
    resetPosition()
  }
}

// Checklist actions
function onAddItem() {
  if (newItemText.value) {
    store.addItem(newItemText.value)
    newItemText.value = ''
  }
}

function onRemoveItem(idx) {
  store.removeItem(idx)
}

onUnmounted(() => {
  clearTimeout(hideTimeout)
})
</script>

<template>
  <div v-if="state !== 'idle'" 
       class="agent-notification" 
       :class="[{ 'is-analyzing': state === 'analyzing', 'is-expanded': isExpanded }]"
       :style="{ right: position.x + 'px', top: position.y + 'px' }"
       @mousedown="onMouseDown">
    
    <div class="notif-header" @click="toggleExpand">
      <div class="agent-icon">🤖</div>
      <div class="notif-title">
        <span v-if="state === 'analyzing'">Analyzing: {{ lookingAt }}</span>
        <span v-else>Agent Analysis Complete</span>
      </div>
      <button class="close-btn" @click.stop="dismiss">×</button>
    </div>

    <div v-if="isExpanded && state === 'complete'" class="notif-body">
      <div class="agent-message">{{ resultText }}</div>
      
      <div class="checklist-area">
        <div class="checklist-title">Agent Checklist (Session Only)</div>
        <ul class="checklist">
          <li v-for="(item, idx) in store.checklist" :key="idx">
            <input type="text" :value="item" @change="e => store.updateItem(idx, e.target.value)" />
            <button class="del-btn" @click.stop="onRemoveItem(idx)">×</button>
          </li>
        </ul>
        <div class="add-item">
          <input type="text" v-model="newItemText" placeholder="Add checklist item..." @keyup.enter="onAddItem" />
          <button @click.stop="onAddItem">+</button>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.agent-notification {
  position: fixed;
  z-index: 9999;
  width: 300px;
  background: var(--bg2);
  border: 1px solid var(--border);
  border-radius: 8px;
  box-shadow: 0 4px 12px rgba(0,0,0,0.3);
  color: var(--text);
  transition: box-shadow 0.3s, transform 0.1s;
  cursor: grab;
  user-select: none;
}

.agent-notification:active {
  cursor: grabbing;
}

.agent-notification.is-expanded {
  width: 400px;
}

.notif-header {
  display: flex;
  align-items: center;
  padding: 10px 12px;
  gap: 8px;
}

.agent-icon {
  font-size: 16px;
}

.notif-title {
  flex: 1;
  font-size: 12px;
  font-weight: 500;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.close-btn {
  background: none;
  border: none;
  color: var(--text3);
  cursor: pointer;
  font-size: 16px;
}

.close-btn:hover {
  color: var(--text);
}

/* Pulsing effect during analysis */
.is-analyzing {
  background: rgba(79, 142, 247, 0.15);
  border-color: rgba(79, 142, 247, 0.4);
  animation: pulse 2s infinite alternate;
}

@keyframes pulse {
  from { box-shadow: 0 0 5px rgba(79, 142, 247, 0.2); }
  to { box-shadow: 0 0 20px rgba(79, 142, 247, 0.6); }
}

.notif-body {
  padding: 12px;
  border-top: 1px solid var(--border);
  background: var(--bg);
  cursor: default;
}

.agent-message {
  font-size: 12px;
  line-height: 1.4;
  margin-bottom: 12px;
  color: var(--text2);
}

.checklist-area {
  background: var(--bg3);
  padding: 8px;
  border-radius: 6px;
}

.checklist-title {
  font-size: 11px;
  font-weight: 600;
  color: var(--text3);
  margin-bottom: 8px;
}

.checklist {
  list-style: none;
  padding: 0;
  margin: 0 0 8px 0;
}

.checklist li {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-bottom: 4px;
}

.checklist input {
  flex: 1;
  background: var(--bg);
  border: 1px solid var(--border);
  color: var(--text);
  border-radius: 4px;
  padding: 4px;
  font-size: 11px;
}

.del-btn {
  background: none;
  border: none;
  color: var(--red);
  cursor: pointer;
}

.add-item {
  display: flex;
  gap: 6px;
}

.add-item input {
  flex: 1;
  background: var(--bg);
  border: 1px solid var(--border);
  color: var(--text);
  border-radius: 4px;
  padding: 4px;
  font-size: 11px;
}

.add-item button {
  background: var(--accent);
  color: white;
  border: none;
  border-radius: 4px;
  padding: 0 8px;
  cursor: pointer;
}
</style>
