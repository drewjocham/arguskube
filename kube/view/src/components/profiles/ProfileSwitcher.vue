<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useProfilesStore } from '../../stores/profiles'
import { useAppNavStore } from '../../stores/appNav'

const profiles = useProfilesStore()
const appNav = useAppNavStore()

const isOpen = ref(false)

const currentLabel = computed(() => {
  const v = profiles.activeVariant
  const g = profiles.activeGroup
  if (v && g) return `${g.name} › ${v.name} v${v.version}`
  if (v) return `${v.name} v${v.version}`
  return 'No Profile'
})

const hasProfiles = computed(() => profiles.groups.length > 0)

function toggle() {
  isOpen.value = !isOpen.value
}

function select(groupId: string, variantId: string) {
  profiles.applyVariant(variantId)
  isOpen.value = false
}

function capture() {
  const g = profiles.activeGroup
  const v = profiles.activeVariant
  if (g && v) {
    profiles.captureToVariant(g.id, v.id)
  }
  isOpen.value = false
}

function openSettings() {
  isOpen.value = false
  appNav.requestNav({ navId: 'settings', anchor: 'profile-groups' })
}

function onDocClick(e: MouseEvent) {
  if (isOpen.value && !(e.target as Element)?.closest?.('.profile-switcher')) {
    isOpen.value = false
  }
}

onMounted(() => document.addEventListener('click', onDocClick))
onUnmounted(() => document.removeEventListener('click', onDocClick))
</script>

<template>
  <div class="profile-switcher">
    <button
      class="ps-trigger"
      :class="{ active: isOpen }"
      @click.stop="toggle"
      title="Switch profile"
    >
      <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
        <path d="M4 4h16c1.1 0 2 .9 2 2v12c0 1.1-.9 2-2 2H4c-1.1 0-2-.9-2-2V6c0-1.1.9-2 2-2z"/>
        <polyline points="22,6 12,13 2,6"/>
      </svg>
      <span class="ps-label">{{ currentLabel }}</span>
      <svg width="10" height="10" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="ps-chevron">
        <polyline points="6 9 12 15 18 9"/>
      </svg>
    </button>

    <div v-if="isOpen" class="ps-dropdown" @click.stop>
      <div class="ps-header">Profiles</div>

      <div v-if="!hasProfiles" class="ps-empty">
        <div class="ps-empty-text">No profiles yet</div>
        <button class="ps-action-btn" @click="openSettings">Create your first profile</button>
      </div>

      <template v-else>
        <div v-for="group in profiles.groups" :key="group.id" class="ps-group">
          <div class="ps-group-label">{{ group.name }}</div>
          <div
            v-for="v in group.variants"
            :key="v.id"
            class="ps-variant"
            :class="{ active: v.id === profiles.activeVariantId && group.id === profiles.activeGroupId }"
            @click="select(group.id, v.id)"
          >
            <span class="ps-v-name">{{ v.name }}</span>
            <span class="ps-v-version">v{{ v.version }}</span>
          </div>
        </div>
      </template>

      <div v-if="hasProfiles" class="ps-divider" />

      <div class="ps-footer">
        <button class="ps-footer-btn" @click="capture">
          <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <path d="M23 19a2 2 0 0 1-2 2H3a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h4l2-3h6l2 3h4a2 2 0 0 1 2 2z"/>
            <circle cx="12" cy="13" r="4"/>
          </svg>
          Capture Current
        </button>
        <button class="ps-footer-btn" @click="openSettings">Manage Profiles</button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.profile-switcher {
  position: relative;
  display: inline-flex;
}

.ps-trigger {
  display: flex;
  align-items: center;
  gap: 5px;
  height: 28px;
  padding: 0 8px;
  background: var(--bg2);
  border: 1px solid var(--border);
  border-radius: 6px;
  color: var(--text2);
  cursor: pointer;
  font-size: 11px;
  transition: all 0.15s;
  white-space: nowrap;
}
.ps-trigger:hover { background: var(--bg3); color: var(--text); border-color: var(--border2); }
.ps-trigger.active { background: var(--bg3); border-color: var(--accent); color: var(--text); }

.ps-label {
  max-width: 140px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.ps-chevron {
  transition: transform 0.15s;
}
.ps-trigger.active .ps-chevron {
  transform: rotate(180deg);
}

.ps-dropdown {
  position: absolute;
  top: calc(100% + 6px);
  left: 0;
  min-width: 220px;
  background: var(--bg3);
  border: 1px solid var(--border2);
  border-radius: var(--r2);
  box-shadow: 0 8px 24px rgba(0,0,0,0.4);
  z-index: 1000;
  padding: 6px;
  animation: ps-slide 0.15s ease-out;
}
@keyframes ps-slide { from { opacity: 0; transform: translateY(-4px); } to { opacity: 1; transform: translateY(0); } }

.ps-header {
  font-size: 10.5px;
  font-weight: 600;
  color: var(--text2);
  text-transform: uppercase;
  letter-spacing: 0.04em;
  padding: 4px 6px 6px;
}

.ps-empty {
  padding: 12px 6px;
  text-align: center;
}
.ps-empty-text {
  font-size: 12px;
  color: var(--text3);
  margin-bottom: 8px;
}
.ps-action-btn {
  background: var(--accent);
  color: #fff;
  border: none;
  border-radius: 4px;
  padding: 5px 12px;
  font-size: 11px;
  cursor: pointer;
}
.ps-action-btn:hover { opacity: 0.9; }

.ps-group {
  margin-bottom: 4px;
}
.ps-group-label {
  font-size: 10px;
  font-weight: 600;
  color: var(--text3);
  text-transform: uppercase;
  letter-spacing: 0.03em;
  padding: 4px 6px 2px;
}

.ps-variant {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 5px 8px;
  border-radius: 4px;
  cursor: pointer;
  transition: background 0.12s;
  margin: 1px 0;
}
.ps-variant:hover { background: var(--bg4); }
.ps-variant.active { background: rgba(79,142,247,0.12); }

.ps-v-name {
  flex: 1;
  font-size: 12px;
  color: var(--text);
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.ps-v-version {
  font-size: 10px;
  font-family: var(--mono);
  color: var(--text3);
  padding: 1px 5px;
  border-radius: 3px;
  background: var(--bg2);
  flex-shrink: 0;
}

.ps-divider {
  height: 1px;
  background: var(--border);
  margin: 4px 0;
}

.ps-footer {
  display: flex;
  flex-direction: column;
  gap: 2px;
  padding: 2px 0;
}
.ps-footer-btn {
  display: flex;
  align-items: center;
  gap: 6px;
  width: 100%;
  background: none;
  border: none;
  color: var(--text2);
  padding: 5px 8px;
  font-size: 11px;
  cursor: pointer;
  border-radius: 4px;
  text-align: left;
}
.ps-footer-btn:hover { background: var(--bg4); color: var(--text); }
</style>
