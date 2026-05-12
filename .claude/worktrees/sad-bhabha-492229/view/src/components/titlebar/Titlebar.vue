<script setup>
import { isWails } from '../../composables/useWails'

defineProps({
  clusterInfo: { type: Object, default: null },
  terminalOpen: { type: Boolean, default: false },
})

const emit = defineEmits(['toggle-terminal', 'pop-out'])

function openDeepLink(path) {
  window.location.href = `kubewatcher://${path}`
}
</script>

<template>
  <div class="titlebar" style="--wails-draggable: drag">
    <div v-if="!isWails()" class="traffic-lights">
      <div class="tl tl-r"></div>
      <div class="tl tl-y"></div>
      <div class="tl tl-g"></div>
    </div>
    <div v-else class="traffic-spacer"></div>
    <div class="titlebar-title">
      <span>KubeWatcher</span> — SRE Console
    </div>
    <div class="titlebar-right">
      <template v-if="!isWails()">
        <button class="tb-saas-btn" @click="openDeepLink('app')" title="Open Native Desktop App">
          Desktop App
        </button>
        <button class="tb-saas-btn primary" @click="openDeepLink('terminal')" title="Launch Native Warp Terminal">
          Native Terminal
        </button>
      </template>
      <template v-else>
        <button class="tb-btn" :class="{ active: terminalOpen }" @click="emit('toggle-terminal')" title="Toggle Terminal" style="--wails-draggable: no-drag">
          <svg width="14" height="14" viewBox="0 0 14 14" fill="none">
            <rect x="1.5" y="2.5" width="11" height="9" rx="1.5" stroke="currentColor" stroke-width="1.2"/>
            <path d="M4 6l2 1.5L4 9" stroke="currentColor" stroke-width="1.2" stroke-linecap="round" stroke-linejoin="round"/>
            <line x1="7.5" y1="9" x2="10" y2="9" stroke="currentColor" stroke-width="1.2" stroke-linecap="round"/>
          </svg>
        </button>
        <button class="tb-btn" @click="emit('pop-out')" title="Pop-out Desktop Environment" style="--wails-draggable: no-drag; margin-right: 8px;">
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6"></path><polyline points="15 3 21 3 21 9"></polyline><line x1="10" y1="14" x2="21" y2="3"></line></svg>
        </button>
      </template>
      <div class="env-badge env-prod">PROD</div>
      <div class="env-badge env-qa">QA</div>
      <div class="health-dot"></div>
    </div>
  </div>
</template>

<style scoped>
.titlebar {
  height: 44px;
  background: var(--bg2);
  border-bottom: 1px solid var(--border);
  display: flex;
  align-items: center;
  padding: 0 16px;
  gap: 12px;
  flex-shrink: 0;
  user-select: none;
}

.traffic-lights {
  display: flex;
  gap: 7px;
  align-items: center;
}

.tl {
  width: 12px;
  height: 12px;
  border-radius: 50%;
  cursor: default;
}
.tl-r { background: #ff5f57; box-shadow: 0 0 0 0.5px rgba(0,0,0,0.3); }
.tl-y { background: #febc2e; box-shadow: 0 0 0 0.5px rgba(0,0,0,0.3); }
.tl-g { background: #28c840; box-shadow: 0 0 0 0.5px rgba(0,0,0,0.3); }

.traffic-spacer {
  width: 68px;
  flex-shrink: 0;
}

.titlebar-title {
  flex: 1;
  text-align: center;
  font-size: 12.5px;
  font-weight: 500;
  color: var(--text2);
  letter-spacing: 0.01em;
  margin-left: -84px;
}
.titlebar-title span { color: var(--text); }

.titlebar-right {
  display: flex;
  gap: 6px;
  align-items: center;
}

.env-badge {
  padding: 2px 8px;
  border-radius: 20px;
  font-size: 10.5px;
  font-weight: 500;
  letter-spacing: 0.04em;
  font-family: var(--mono);
}
.env-prod { background: rgba(240,84,84,0.15); color: var(--red2); border: 1px solid rgba(240,84,84,0.25); }
.env-qa { background: rgba(245,166,35,0.12); color: var(--amber2); border: 1px solid rgba(245,166,35,0.2); }

.tb-btn {
  background: none;
  border: 1px solid transparent;
  color: var(--text3);
  cursor: pointer;
  padding: 3px 5px;
  border-radius: 5px;
  display: flex;
  align-items: center;
  transition: all 0.15s;
}
.tb-btn:hover { background: var(--bg3); color: var(--text); border-color: var(--border2); }
.tb-btn.active { background: rgba(79,142,247,0.12); color: var(--accent2); border-color: rgba(79,142,247,0.25); }

.tb-saas-btn {
  background: rgba(255,255,255,0.05);
  border: 1px solid var(--border2);
  color: var(--text2);
  cursor: pointer;
  padding: 4px 10px;
  border-radius: 6px;
  font-size: 11px;
  font-weight: 500;
  display: flex;
  align-items: center;
  transition: all 0.15s;
  margin-right: 4px;
}
.tb-saas-btn:hover { background: var(--bg3); color: var(--text); }
.tb-saas-btn.primary {
  background: linear-gradient(135deg, rgba(79,142,247,0.15) 0%, rgba(162,119,255,0.15) 100%);
  border-color: rgba(79,142,247,0.3);
  color: var(--accent2);
}
.tb-saas-btn.primary:hover {
  background: linear-gradient(135deg, rgba(79,142,247,0.25) 0%, rgba(162,119,255,0.25) 100%);
  color: #fff;
  border-color: rgba(79,142,247,0.5);
}

.health-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: var(--green);
  box-shadow: 0 0 6px var(--green);
  animation: pulse 2s ease-in-out infinite;
}
</style>
