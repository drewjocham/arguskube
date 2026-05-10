<script setup>
import { ref, computed } from 'vue'
import { storeToRefs } from 'pinia'
import { isWails } from '../../composables/useWails'
import { useNotificationsStore } from '../../stores/notifications'
import { useAuthStore } from '../../stores/auth'
import { useNavSearchStore } from '../../stores/navSearch'
import { useSpotCheck } from '../../composables/useSpotCheck'
import NotificationsPanel from '../notifications/NotificationsPanel.vue'
import EnvironmentSelector from './EnvironmentSelector.vue'

const { active: spotCheckActive, runAll: runSpotChecks } = useSpotCheck()

// Sidebar search — typed here, consumed in the sidebar via the
// navSearch store (no prop drilling through App.vue).
const navSearch = useNavSearchStore()
const { query: navQuery } = storeToRefs(navSearch)

defineProps({
  clusterInfo: { type: Object, default: null },
  terminalOpen: { type: Boolean, default: false },
})

const emit = defineEmits(['toggle-terminal', 'pop-out'])

const notifications = useNotificationsStore()
const { unreadCount, panelOpen: notifPanelOpen } = storeToRefs(notifications)

const auth = useAuthStore()
const userMenuOpen = ref(false)
const userInitial = computed(() => {
  const u = auth.user
  if (!u) return '?'
  const src = u.name || u.email || '?'
  return src.trim().charAt(0).toUpperCase()
})

function openDeepLink(path) {
  window.location.href = `kubewatcher://${path}`
}

function toggleNotifications() {
  notifications.togglePanel()
}

function toggleUserMenu() {
  userMenuOpen.value = !userMenuOpen.value
}

async function signOut() {
  userMenuOpen.value = false
  await auth.logout()
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

    <!-- Sidebar search. Typed here, applied to the navTree by the
         Sidebar component via the shared navSearch store. The input
         needs `--wails-draggable: no-drag` so the OS doesn't try to
         drag the window when the user clicks on it. -->
    <div class="nav-search" style="--wails-draggable: no-drag">
      <svg class="nav-search-icon" width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
        <circle cx="11" cy="11" r="7"></circle>
        <line x1="21" y1="21" x2="16.65" y2="16.65"></line>
      </svg>
      <input
        type="text"
        class="nav-search-input"
        v-model="navQuery"
        placeholder="Search menu…"
        spellcheck="false"
        autocomplete="off"
        @keydown.escape="navSearch.clear()"
      />
      <button
        v-if="navQuery"
        class="nav-search-clear"
        @click="navSearch.clear()"
        title="Clear (esc)"
      >×</button>
    </div>

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
      <EnvironmentSelector style="--wails-draggable: no-drag" />
      <!-- Spot-check activity pill: visible only while a probe is
           running. Click runs all checks now (manual trigger). -->
      <button
        v-if="spotCheckActive"
        class="spotcheck-pill"
        @click="runSpotChecks"
        :title="`Argus is ${spotCheckActive.description.toLowerCase()} Click to run all checks now.`"
        style="--wails-draggable: no-drag"
      >
        <span class="spotcheck-dot"></span>
        <span class="spotcheck-desc">{{ spotCheckActive.description }}</span>
      </button>
      <button
        class="tb-bell"
        :class="{ active: notifPanelOpen }"
        @click="toggleNotifications"
        title="Argus notifications"
        style="--wails-draggable: no-drag"
      >
        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <path d="M18 8A6 6 0 0 0 6 8c0 7-3 9-3 9h18s-3-2-3-9"/>
          <path d="M13.73 21a2 2 0 0 1-3.46 0"/>
        </svg>
        <span v-if="unreadCount > 0" class="tb-bell-badge">{{ unreadCount > 99 ? '99+' : unreadCount }}</span>
      </button>
      <div class="health-dot"></div>
      <div
        v-if="auth.authDisabled"
        class="dev-badge"
        title="KUBEWATCHER_AUTH_DISABLED is set — every API request is unauthenticated. Local development only."
      >DEV · NO AUTH</div>
      <div v-if="auth.user" class="user-chip" style="--wails-draggable: no-drag">
        <button
          class="user-avatar"
          :class="{ active: userMenuOpen }"
          @click="toggleUserMenu"
          :title="auth.user.email"
        >{{ userInitial }}</button>
        <div v-if="userMenuOpen" class="user-menu">
          <div class="user-meta">
            <div class="u-name">{{ auth.user.name || auth.user.email }}</div>
            <div class="u-email">{{ auth.user.email }}</div>
            <div class="u-prov">via {{ auth.user.provider }}</div>
          </div>
          <button class="u-action" @click="signOut">Sign out</button>
        </div>
      </div>
    </div>
    <NotificationsPanel />
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

.nav-search {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 0 8px;
  height: 24px;
  width: 220px;
  margin-right: 12px;
  background: var(--bg3);
  border: 1px solid var(--border);
  border-radius: 6px;
  flex-shrink: 0;
  transition: border-color 0.15s, background 0.15s;
}
.nav-search:focus-within {
  border-color: var(--accent);
  background: var(--bg2);
}
.nav-search-icon { color: var(--text3); flex-shrink: 0; }
.nav-search:focus-within .nav-search-icon { color: var(--accent2); }
.nav-search-input {
  flex: 1;
  min-width: 0;
  background: transparent;
  border: 0;
  outline: 0;
  color: var(--text);
  font: inherit;
  font-size: 12px;
  padding: 0;
}
.nav-search-input::placeholder { color: var(--text3); }
.nav-search-clear {
  flex-shrink: 0;
  width: 16px; height: 16px;
  background: transparent;
  border: 0;
  color: var(--text3);
  font-size: 14px;
  line-height: 1;
  cursor: pointer;
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 0;
}
.nav-search-clear:hover { background: var(--bg4); color: var(--text); }

.titlebar-title {
  flex: 1;
  text-align: center;
  font-size: 12.5px;
  font-weight: 500;
  color: var(--text2);
  letter-spacing: 0.01em;
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

/* Notifications bell — same shape as tb-btn but with a small badge for the
   unread count. Hooked to the global notifications store. */
.tb-bell {
  position: relative;
  width: 28px; height: 28px;
  display: inline-flex; align-items: center; justify-content: center;
  background: var(--bg2);
  border: 1px solid var(--border);
  border-radius: 6px;
  color: var(--text2);
  cursor: pointer;
  transition: all 0.15s;
  margin-right: 4px;
}
.tb-bell:hover { background: var(--bg3); color: var(--text); border-color: var(--border2); }
.tb-bell.active { background: rgba(167,139,250,0.12); color: #a78bfa; border-color: rgba(167,139,250,0.25); }
.tb-bell-badge {
  position: absolute;
  top: -3px; right: -3px;
  min-width: 14px; height: 14px;
  padding: 0 4px;
  background: #f05454;
  border-radius: 8px;
  color: #fff;
  font-size: 9px;
  font-weight: 700;
  display: inline-flex; align-items: center; justify-content: center;
  border: 1.5px solid var(--bg2);
}

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

.spotcheck-pill {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 3px 10px;
  margin-right: 8px;
  border-radius: 999px;
  background: rgba(167, 139, 250, 0.12);
  border: 1px solid rgba(167, 139, 250, 0.4);
  color: var(--purple);
  font-size: 10.5px;
  font-weight: 500;
  cursor: pointer;
  font: inherit;
  font-size: 10.5px;
  font-weight: 500;
  letter-spacing: 0.01em;
  max-width: 280px;
  overflow: hidden;
  white-space: nowrap;
  text-overflow: ellipsis;
  transition: background 0.15s, border-color 0.15s;
}
.spotcheck-pill:hover { background: rgba(167, 139, 250, 0.22); border-color: rgba(167, 139, 250, 0.7); }
.spotcheck-dot {
  width: 6px; height: 6px; border-radius: 50%;
  background: var(--purple);
  box-shadow: 0 0 6px var(--purple);
  animation: pulse 1.4s ease-in-out infinite;
}
.spotcheck-desc { overflow: hidden; text-overflow: ellipsis; }

.dev-badge {
  display: inline-flex;
  align-items: center;
  padding: 2px 8px;
  margin-left: 8px;
  border-radius: 999px;
  background: rgba(245,166,35,0.15);
  border: 1px solid rgba(245,166,35,0.5);
  color: var(--amber2);
  font-size: 9.5px;
  font-weight: 700;
  letter-spacing: 0.05em;
  text-transform: uppercase;
  cursor: help;
  user-select: none;
}

.user-chip { position: relative; margin-left: 8px; }
.user-avatar {
  width: 26px;
  height: 26px;
  border-radius: 50%;
  background: linear-gradient(135deg, var(--accent) 0%, var(--purple) 100%);
  color: #fff;
  border: 0;
  font: inherit;
  font-weight: 600;
  font-size: 11px;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: filter .15s var(--ease);
}
.user-avatar:hover, .user-avatar.active { filter: brightness(1.15); }

.user-menu {
  position: absolute;
  top: calc(100% + 6px);
  right: 0;
  min-width: 220px;
  background: var(--bg2);
  border: 1px solid var(--border2);
  border-radius: var(--r2);
  box-shadow: var(--shadow2);
  padding: .65rem;
  z-index: 50;
}
.user-meta { padding: .25rem .35rem .55rem; border-bottom: 1px solid var(--border); margin-bottom: .55rem; }
.user-meta .u-name { color: var(--text); font-size: .9rem; font-weight: 500; }
.user-meta .u-email { color: var(--text2); font-size: .75rem; margin-top: .15rem; }
.user-meta .u-prov { color: var(--text3); font-size: .7rem; margin-top: .15rem; text-transform: uppercase; letter-spacing: .06em; }
.u-action {
  display: block;
  width: 100%;
  text-align: left;
  background: transparent;
  border: 0;
  color: var(--text);
  font: inherit;
  font-size: .85rem;
  padding: .4rem .5rem;
  border-radius: var(--r2);
  cursor: pointer;
}
.u-action:hover { background: var(--bg3); }
</style>
