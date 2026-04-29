<script setup>
defineProps({
  clusterInfo: { type: Object, default: null },
  terminalOpen: { type: Boolean, default: false },
})

const emit = defineEmits(['toggle-terminal'])
</script>

<template>
  <div class="titlebar" style="--wails-draggable: drag">
    <div class="traffic-lights">
      <div class="tl tl-r"></div>
      <div class="tl tl-y"></div>
      <div class="tl tl-g"></div>
    </div>
    <div class="titlebar-title">
      <span>KubeWatcher</span> — SRE Console
    </div>
    <div class="titlebar-right">
      <button class="tb-btn" :class="{ active: terminalOpen }" @click="emit('toggle-terminal')" title="Toggle Terminal" style="--wails-draggable: no-drag">
        <svg width="14" height="14" viewBox="0 0 14 14" fill="none">
          <rect x="1.5" y="2.5" width="11" height="9" rx="1.5" stroke="currentColor" stroke-width="1.2"/>
          <path d="M4 6l2 1.5L4 9" stroke="currentColor" stroke-width="1.2" stroke-linecap="round" stroke-linejoin="round"/>
          <line x1="7.5" y1="9" x2="10" y2="9" stroke="currentColor" stroke-width="1.2" stroke-linecap="round"/>
        </svg>
      </button>
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

.health-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: var(--green);
  box-shadow: 0 0 6px var(--green);
  animation: pulse 2s ease-in-out infinite;
}
</style>
