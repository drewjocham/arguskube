<script setup>
import { ref, computed, watch, onMounted, onUnmounted } from 'vue'
import { useContexts } from '../../composables/useWails'

const emit = defineEmits(['close'])

const { contexts } = useContexts()

const isOpen = ref(false)
const selectedEnv = ref('stage')
const customLabel = ref('')
const precreatedEnvs = ['stage', 'prod', 'work', 'pre-prod', 'sandbox']

// mapping of context name to env label
const envMapping = ref({})

function loadMapping() {
  try {
    const data = localStorage.getItem('kubewatcher.envs')
    if (data) {
      envMapping.value = JSON.parse(data)
    }
  } catch(e){}
}

function saveMapping() {
  localStorage.setItem('kubewatcher.envs', JSON.stringify(envMapping.value))
}

const activeContextName = computed(() => {
  const active = contexts.value.find(c => c.active)
  return active ? active.name : null
})

const currentLabel = computed(() => {
  if (!activeContextName.value) return null
  return envMapping.value[activeContextName.value] || null
})

watch(activeContextName, (newVal) => {
  if (newVal) {
    const label = envMapping.value[newVal]
    if (label) {
      if (precreatedEnvs.includes(label)) {
        selectedEnv.value = label
        customLabel.value = ''
      } else {
        selectedEnv.value = 'custom'
        customLabel.value = label
      }
    } else {
      selectedEnv.value = ''
      customLabel.value = ''
    }
  }
})

function toggleDropdown() {
  isOpen.value = !isOpen.value
}

function setLabel(label) {
  if (!activeContextName.value) return
  if (label === 'custom') {
    selectedEnv.value = 'custom'
    return
  }
  selectedEnv.value = label
  envMapping.value[activeContextName.value] = label
  saveMapping()
  isOpen.value = false
}

function saveCustom() {
  if (!activeContextName.value || !customLabel.value.trim()) return
  envMapping.value[activeContextName.value] = customLabel.value.trim()
  saveMapping()
  isOpen.value = false
}

function clearLabel() {
  if (!activeContextName.value) return
  delete envMapping.value[activeContextName.value]
  selectedEnv.value = ''
  customLabel.value = ''
  saveMapping()
  isOpen.value = false
}

function onDocClick(e) {
  if (isOpen.value && !e.target.closest('.env-selector')) {
    isOpen.value = false
  }
}

onMounted(() => {
  loadMapping()
  document.addEventListener('click', onDocClick)
})
onUnmounted(() => document.removeEventListener('click', onDocClick))
</script>

<template>
  <div class="env-selector">
    <!-- Trigger Button -->
    <button class="tb-btn env-btn" @click.stop="toggleDropdown" title="Environment Label">
      <template v-if="currentLabel">
        <span class="env-badge" :class="'env-' + currentLabel.toLowerCase()">{{ currentLabel.toUpperCase() }}</span>
      </template>
      <template v-else>
        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
          <path d="M12 2v20M17 5H9.5a3.5 3.5 0 0 0 0 7h5a3.5 3.5 0 0 1 0 7H6"/>
        </svg>
      </template>
    </button>

    <!-- Dropdown -->
    <div v-if="isOpen" class="env-dropdown" @click.stop>
      <div class="env-header">Set Environment</div>
      <div class="env-context" v-if="activeContextName">
        Context: <span>{{ activeContextName }}</span>
      </div>
      <div class="env-list">
        <div 
          v-for="env in precreatedEnvs" 
          :key="env"
          class="env-item"
          :class="{ active: selectedEnv === env }"
          @click="setLabel(env)"
        >
          <div class="env-badge-preview" :class="'env-' + env">{{ env.toUpperCase() }}</div>
        </div>
        <div class="env-item" :class="{ active: selectedEnv === 'custom' }" @click="setLabel('custom')">
          <div class="env-badge-preview env-custom">CUSTOM</div>
        </div>
      </div>
      <div v-if="selectedEnv === 'custom'" class="env-custom-input">
        <input type="text" v-model="customLabel" placeholder="Custom label..." @keyup.enter="saveCustom" />
        <button @click="saveCustom">Save</button>
      </div>
      <div class="env-footer" v-if="currentLabel">
        <button class="clear-btn" @click="clearLabel">Clear Label</button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.env-selector {
  position: relative;
  display: inline-flex;
}

.env-btn {
  background: none;
  border: 1px solid transparent;
  color: var(--text3);
  cursor: pointer;
  padding: 3px 5px;
  border-radius: 5px;
  display: flex;
  align-items: center;
  transition: all 0.15s;
  height: 28px;
}
.env-btn:hover { background: var(--bg3); color: var(--text); border-color: var(--border2); }

.env-badge {
  padding: 2px 6px;
  border-radius: 4px;
  font-size: 10px;
  font-weight: 600;
  letter-spacing: 0.04em;
  font-family: var(--mono);
}

.env-badge-preview {
  padding: 2px 6px;
  border-radius: 4px;
  font-size: 10px;
  font-weight: 600;
  letter-spacing: 0.04em;
  font-family: var(--mono);
  display: inline-block;
}

.env-prod { background: rgba(240,84,84,0.15); color: var(--red2); border: 1px solid rgba(240,84,84,0.25); }
.env-qa, .env-stage, .env-pre-prod { background: rgba(245,166,35,0.12); color: var(--amber2); border: 1px solid rgba(245,166,35,0.2); }
.env-work, .env-sandbox, .env-custom { background: rgba(79,142,247,0.12); color: var(--accent2); border: 1px solid rgba(79,142,247,0.25); }

.env-dropdown {
  position: absolute;
  top: calc(100% + 6px);
  right: 0;
  width: 200px;
  background: var(--bg3);
  border: 1px solid var(--border2);
  border-radius: var(--r2);
  box-shadow: 0 8px 24px rgba(0,0,0,0.4);
  z-index: 1000;
  padding: 8px;
  animation: env-slide 0.15s ease-out;
}
@keyframes env-slide { from { opacity: 0; transform: translateY(-4px); } to { opacity: 1; transform: translateY(0); } }

.env-header {
  font-size: 11px;
  font-weight: 600;
  color: var(--text2);
  margin-bottom: 6px;
  padding: 0 4px;
}

.env-context {
  font-size: 10px;
  color: var(--text3);
  margin-bottom: 8px;
  padding: 0 4px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.env-context span {
  color: var(--text);
  font-weight: 500;
}

.env-list {
  display: flex;
  flex-direction: column;
  gap: 2px;
  margin-bottom: 8px;
}

.env-item {
  padding: 6px 8px;
  border-radius: 4px;
  cursor: pointer;
  transition: background 0.15s;
  display: flex;
  align-items: center;
}
.env-item:hover { background: var(--bg4); }
.env-item.active { background: rgba(255,255,255,0.05); }

.env-custom-input {
  display: flex;
  gap: 4px;
  margin-bottom: 8px;
  padding: 0 4px;
}
.env-custom-input input {
  flex: 1;
  background: var(--bg2);
  border: 1px solid var(--border);
  color: var(--text);
  border-radius: 4px;
  padding: 4px 6px;
  font-size: 11px;
}
.env-custom-input button {
  background: var(--accent);
  color: #fff;
  border: none;
  border-radius: 4px;
  padding: 4px 8px;
  font-size: 11px;
  cursor: pointer;
}
.env-custom-input button:hover { opacity: 0.9; }

.env-footer {
  border-top: 1px solid var(--border);
  padding-top: 6px;
  display: flex;
  justify-content: flex-end;
}
.clear-btn {
  background: none;
  border: none;
  color: var(--text3);
  font-size: 10px;
  cursor: pointer;
}
.clear-btn:hover { color: var(--red); }
</style>
