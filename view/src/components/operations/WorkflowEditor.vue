<script setup>
import { ref } from 'vue'

const steps = ref([
  { id: 1, type: 'trigger', name: 'Input', icon: '⚡' },
  { id: 2, type: 'action', name: 'Get 3 best stories', icon: '🐍', actionType: 'python' },
  { id: 3, type: 'action', name: 'Send Message to Channel (slack)', icon: '#', actionType: 'slack' }
])

const selectedStep = ref(null)

function selectStep(step) {
  selectedStep.value = step
}

function addStep(index) {
  const newId = Math.max(...steps.value.map(s => s.id), 0) + 1
  steps.value.splice(index + 1, 0, {
    id: newId,
    type: 'action',
    name: 'New Step',
    icon: '⚙️',
    actionType: 'custom'
  })
}

function removeStep(id) {
  steps.value = steps.value.filter(s => s.id !== id)
  if (selectedStep.value?.id === id) {
    selectedStep.value = null
  }
}
</script>

<template>
  <div class="workflow-editor">
    <div class="editor-canvas">
      
      <!-- Top header for the canvas -->
      <div class="canvas-header">
        <input type="text" class="workflow-title" value="Gets the top 3 HackerNews stories and send them" />
        <div class="canvas-actions">
          <button class="action-btn">Undo</button>
          <button class="action-btn">Redo</button>
          <button class="action-btn primary">Save & Deploy</button>
        </div>
      </div>

      <!-- Flow container -->
      <div class="flow-container">
        <template v-for="(step, index) in steps" :key="step.id">
          
          <!-- Step Card -->
          <div class="step-card" :class="{ selected: selectedStep?.id === step.id, trigger: step.type === 'trigger' }" @click="selectStep(step)">
            <div class="step-icon">{{ step.icon }}</div>
            <div class="step-name">{{ step.name }}</div>
            <div v-if="step.type !== 'trigger'" class="step-delete" @click.stop="removeStep(step.id)">×</div>
          </div>

          <!-- Connecting line & Add Button -->
          <div class="connector">
            <div class="line"></div>
            <div class="add-btn-wrapper">
              <button class="add-btn" @click="addStep(index)">+</button>
            </div>
            <div class="line"></div>
          </div>

        </template>

        <!-- Final Result Node -->
        <div class="step-card result-node">
          <div class="step-name" style="text-align: center; width: 100%;">Result</div>
        </div>
      </div>
    </div>

    <!-- Right Settings Panel -->
    <div class="settings-panel">
      <div class="panel-header">
        Settings
      </div>
      <div v-if="selectedStep" class="panel-content">
        <div class="form-group">
          <label>Step Name</label>
          <input type="text" v-model="selectedStep.name" class="settings-input" />
        </div>
        
        <div v-if="selectedStep.type !== 'trigger'" class="form-group">
          <label>Action Type</label>
          <select v-model="selectedStep.actionType" class="settings-select">
            <option value="python">Python Script</option>
            <option value="slack">Slack Integration</option>
            <option value="custom">Custom Action</option>
            <option value="k8s">Kubernetes Operation</option>
          </select>
        </div>

        <div v-if="selectedStep.actionType === 'python'" class="form-group">
          <label>Script Source</label>
          <textarea class="settings-textarea" rows="8">def main():
    return "Hello world!"</textarea>
        </div>

        <div v-if="selectedStep.actionType === 'slack'" class="form-group">
          <label>Channel</label>
          <input type="text" class="settings-input" value="#alerts" />
          
          <label style="margin-top: 10px;">Message Template</label>
          <textarea class="settings-textarea" rows="4" v-pre>{{ prev.result }}</textarea>
        </div>
      </div>
      <div v-else class="panel-empty">
        Select a node to configure
      </div>
    </div>
  </div>
</template>

<style scoped>
.workflow-editor {
  display: flex;
  height: 100%;
  background: var(--bg);
  border-radius: var(--r);
  border: 1px solid var(--border);
  overflow: hidden;
}

.editor-canvas {
  flex: 1;
  display: flex;
  flex-direction: column;
  background: radial-gradient(circle at center, var(--bg3) 1px, transparent 1px);
  background-size: 20px 20px;
  background-color: var(--bg2);
  overflow: hidden;
}

.canvas-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 12px 16px;
  background: var(--bg2);
  border-bottom: 1px solid var(--border);
}

.workflow-title {
  background: transparent;
  border: 1px solid transparent;
  color: var(--text);
  font-size: 14px;
  font-weight: 500;
  padding: 4px 8px;
  border-radius: 4px;
  width: 300px;
  outline: none;
}
.workflow-title:focus {
  border-color: var(--accent);
  background: var(--bg3);
}

.canvas-actions {
  display: flex;
  gap: 8px;
}

.action-btn {
  background: var(--bg3);
  border: 1px solid var(--border);
  color: var(--text2);
  padding: 5px 12px;
  border-radius: 4px;
  font-size: 12px;
  cursor: pointer;
  transition: all 0.2s;
}
.action-btn:hover { background: var(--bg4); color: var(--text); }
.action-btn.primary { background: rgba(79,142,247,0.15); color: var(--accent2); border-color: rgba(79,142,247,0.3); }
.action-btn.primary:hover { background: rgba(79,142,247,0.25); }

.flow-container {
  flex: 1;
  overflow-y: auto;
  padding: 40px;
  display: flex;
  flex-direction: column;
  align-items: center;
}

.step-card {
  display: flex;
  align-items: center;
  width: 280px;
  background: var(--bg3);
  border: 1px solid var(--border);
  border-radius: 6px;
  padding: 12px 16px;
  cursor: pointer;
  box-shadow: 0 4px 6px rgba(0,0,0,0.1);
  transition: all 0.2s;
  position: relative;
}
.step-card:hover {
  border-color: var(--border2);
  transform: translateY(-1px);
  box-shadow: 0 6px 12px rgba(0,0,0,0.15);
}
.step-card.selected {
  border-color: var(--accent);
  box-shadow: 0 0 0 1px var(--accent);
}
.step-card.trigger {
  background: rgba(79,142,247,0.05);
}
.result-node {
  background: var(--bg);
  border-style: dashed;
  color: var(--text3);
  cursor: default;
}
.result-node:hover {
  transform: none;
  box-shadow: 0 4px 6px rgba(0,0,0,0.1);
  border-color: var(--border);
}

.step-icon {
  width: 24px;
  height: 24px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: var(--bg4);
  border-radius: 4px;
  margin-right: 12px;
  font-size: 14px;
}

.step-name {
  flex: 1;
  font-size: 13px;
  color: var(--text);
  font-weight: 500;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.step-delete {
  position: absolute;
  top: -8px;
  right: -8px;
  width: 18px;
  height: 18px;
  background: var(--red);
  color: white;
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 12px;
  opacity: 0;
  transition: opacity 0.2s;
}
.step-card:hover .step-delete {
  opacity: 1;
}

.connector {
  display: flex;
  flex-direction: column;
  align-items: center;
  width: 100%;
}

.line {
  width: 1px;
  height: 20px;
  background: var(--border2);
}

.add-btn-wrapper {
  padding: 4px;
  background: var(--bg2);
  border-radius: 50%;
  z-index: 1;
}

.add-btn {
  width: 20px;
  height: 20px;
  border-radius: 50%;
  background: var(--bg3);
  border: 1px solid var(--border);
  color: var(--text2);
  display: flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  transition: all 0.2s;
}
.add-btn:hover {
  background: var(--accent);
  color: white;
  border-color: var(--accent);
  transform: scale(1.1);
}

.settings-panel {
  width: 300px;
  background: var(--bg2);
  border-left: 1px solid var(--border);
  display: flex;
  flex-direction: column;
}

.panel-header {
  padding: 12px 16px;
  font-size: 13px;
  font-weight: 500;
  color: var(--text);
  border-bottom: 1px solid var(--border);
  background: var(--bg3);
}

.panel-content {
  flex: 1;
  padding: 16px;
  overflow-y: auto;
}

.panel-empty {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--text3);
  font-size: 12px;
}

.form-group {
  margin-bottom: 16px;
}
.form-group label {
  display: block;
  font-size: 11px;
  font-weight: 500;
  color: var(--text3);
  text-transform: uppercase;
  letter-spacing: 0.05em;
  margin-bottom: 6px;
}

.settings-input, .settings-select, .settings-textarea {
  width: 100%;
  background: var(--bg);
  border: 1px solid var(--border);
  border-radius: 4px;
  padding: 8px 10px;
  color: var(--text);
  font-size: 12.5px;
  font-family: var(--font);
  outline: none;
  transition: border-color 0.2s;
}
.settings-input:focus, .settings-select:focus, .settings-textarea:focus {
  border-color: var(--accent);
}
.settings-textarea {
  resize: vertical;
  font-family: var(--mono);
}
</style>
