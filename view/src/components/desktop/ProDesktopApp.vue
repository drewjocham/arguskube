<script setup>
import { ref, inject } from 'vue'

const emit = defineEmits(['close'])
const isAllowed = inject('isAllowed', () => true)

const terminalOutput = ref([
  { type: 'system', text: 'Initializing Argus Agent Workspace...' },
  { type: 'system', text: 'Connected to local Kubernetes context: production-cluster-01' },
  { type: 'system', text: 'Agent is ready. Waiting for tasks.' }
])

const inputText = ref('')

function runCommand() {
  if (!inputText.value.trim()) return
  const cmd = inputText.value
  terminalOutput.value.push({ type: 'user', text: `$ ${cmd}` })
  inputText.value = ''
  
  setTimeout(() => {
    if (cmd.includes('workflow')) {
      terminalOutput.value.push({ type: 'agent', text: 'Executing workflow: ' + cmd })
      terminalOutput.value.push({ type: 'agent', text: 'Workflow completed successfully.' })
    } else {
      terminalOutput.value.push({ type: 'agent', text: 'Command executed.' })
    }
  }, 600)
}
</script>

<template>
  <div class="pro-desktop-overlay">
    <div class="pro-desktop-window">
      <!-- Titlebar -->
      <div class="pro-window-titlebar" style="--wails-draggable: drag">
        <div class="traffic-lights">
          <div class="tl tl-r" @click="emit('close')"></div>
          <div class="tl tl-y"></div>
          <div class="tl tl-g"></div>
        </div>
        <div class="window-title">
          <span>Argus Agent</span> — Isolated Desktop Environment (Pro)
        </div>
        <div class="window-right">
          <div class="badge-pro">PRO</div>
        </div>
      </div>
      
      <!-- Content -->
      <div class="pro-window-content">
        <div class="sidebar-mini">
          <div class="nav-item active">
            <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="4 17 10 11 4 5"></polyline><line x1="12" y1="19" x2="20" y2="19"></line></svg>
            Terminal
          </div>
          <div class="nav-item">
            <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="3" y="3" width="18" height="18" rx="2" ry="2"></rect><line x1="3" y1="9" x2="21" y2="9"></line><line x1="9" y1="21" x2="9" y2="9"></line></svg>
            Workflows
          </div>
          <div class="nav-item">
            <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"></path><polyline points="14 2 14 8 20 8"></polyline><line x1="16" y1="13" x2="8" y2="13"></line><line x1="16" y1="17" x2="8" y2="17"></line><polyline points="10 9 9 9 8 9"></polyline></svg>
            Files
          </div>
        </div>
        
        <div class="main-terminal">
          <div class="terminal-output">
            <div v-for="(line, idx) in terminalOutput" :key="idx" class="term-line" :class="line.type">
              {{ line.text }}
            </div>
          </div>
          <div class="terminal-input">
            <span class="prompt">❯</span>
            <input v-model="inputText" @keydown.enter="runCommand" type="text" placeholder="Type a command or ask the agent to run a workflow..." spellcheck="false" />
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.pro-desktop-overlay {
  position: fixed;
  top: 0; left: 0; right: 0; bottom: 0;
  background: rgba(0,0,0,0.6);
  backdrop-filter: blur(8px);
  z-index: 10000;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 40px;
}

.pro-desktop-window {
  width: 100%;
  max-width: 1200px;
  height: 100%;
  max-height: 800px;
  background: #111214;
  border-radius: 12px;
  border: 1px solid rgba(255,255,255,0.1);
  box-shadow: 0 24px 60px rgba(0,0,0,0.5), 0 0 0 1px rgba(255,255,255,0.05) inset;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  animation: pop-in 0.3s cubic-bezier(0.16, 1, 0.3, 1);
}

@keyframes pop-in {
  from { opacity: 0; transform: scale(0.95) translateY(10px); }
  to { opacity: 1; transform: scale(1) translateY(0); }
}

.pro-window-titlebar {
  height: 44px;
  background: #1a1c1f;
  border-bottom: 1px solid rgba(255,255,255,0.05);
  display: flex;
  align-items: center;
  padding: 0 16px;
  justify-content: space-between;
}

.traffic-lights {
  display: flex;
  gap: 7px;
  align-items: center;
}
.tl { width: 12px; height: 12px; border-radius: 50%; cursor: pointer; }
.tl-r { background: #ff5f57; box-shadow: 0 0 0 0.5px rgba(0,0,0,0.3); }
.tl-y { background: #febc2e; box-shadow: 0 0 0 0.5px rgba(0,0,0,0.3); }
.tl-g { background: #28c840; box-shadow: 0 0 0 0.5px rgba(0,0,0,0.3); }

.window-title {
  font-size: 13px;
  font-weight: 500;
  color: #8b8f96;
}
.window-title span {
  color: #fff;
}

.badge-pro {
  background: linear-gradient(135deg, #a78bfa, #c084fc);
  color: #000;
  font-size: 10px;
  font-weight: 800;
  padding: 3px 8px;
  border-radius: 12px;
  letter-spacing: 0.5px;
}

.pro-window-content {
  flex: 1;
  display: flex;
  overflow: hidden;
}

.sidebar-mini {
  width: 200px;
  background: #141517;
  border-right: 1px solid rgba(255,255,255,0.05);
  padding: 16px 8px;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.nav-item {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 10px 12px;
  border-radius: 6px;
  color: #8b8f96;
  font-size: 13px;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.2s;
}
.nav-item:hover {
  background: rgba(255,255,255,0.05);
  color: #e8eaec;
}
.nav-item.active {
  background: rgba(167, 139, 250, 0.1);
  color: #c084fc;
}

.main-terminal {
  flex: 1;
  display: flex;
  flex-direction: column;
  background: #0d0d0d;
}

.terminal-output {
  flex: 1;
  padding: 24px;
  overflow-y: auto;
  display: flex;
  flex-direction: column;
  gap: 8px;
  font-family: 'SF Mono', Consolas, monospace;
  font-size: 13px;
  line-height: 1.6;
}

.term-line.system { color: #8b8f96; }
.term-line.user { color: #e8eaec; }
.term-line.agent { color: #a78bfa; border-left: 2px solid #a78bfa; padding-left: 12px; }

.terminal-input {
  display: flex;
  align-items: center;
  padding: 16px 24px;
  background: #111214;
  border-top: 1px solid rgba(255,255,255,0.05);
  font-family: 'SF Mono', Consolas, monospace;
}

.prompt {
  color: #c084fc;
  margin-right: 12px;
  font-size: 14px;
  font-weight: bold;
}

.terminal-input input {
  flex: 1;
  background: transparent;
  border: none;
  outline: none;
  color: #e8eaec;
  font-family: inherit;
  font-size: 13px;
}
.terminal-input input::placeholder {
  color: #4a4d54;
}
</style>
