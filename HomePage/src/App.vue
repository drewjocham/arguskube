<template>
  <n-config-provider :theme-overrides="themeOverrides">
    <n-message-provider>
      <div class="app">
        <header class="topbar">
          <router-link to="/" class="logo">ARGUS</router-link>

          <nav class="topbar-center">
            <router-link
              v-for="t in tabs"
              :key="t.label"
              :to="t.to"
              class="tab-link"
              :class="{ active: isActive(t) }"
            >
              {{ t.label }}
            </router-link>
          </nav>

          <div class="topbar-right">
            <router-link to="/pricing" class="topbar-action">Pricing</router-link>
            <n-button size="small" quaternary @click="goLogin">Log in</n-button>
            <n-button size="small" type="primary" @click="goRegister">Register</n-button>
          </div>
        </header>

        <router-view class="content" />

        <TerminalBanner />
      </div>
    </n-message-provider>
  </n-config-provider>
</template>

<script setup lang="ts">
import { useRoute, useRouter } from 'vue-router'
import { NConfigProvider, NMessageProvider, NButton } from 'naive-ui'
import TerminalBanner from './components/TerminalBanner.vue'

interface Tab {
  label: string
  to: string | { path: string; query: Record<string, string> }
  tabKey: string
}

const tabs: Tab[] = [
  { label: 'ArgusKube', to: '/', tabKey: 'kube' },
  { label: 'Argus API', to: { path: '/', query: { tab: 'api' } }, tabKey: 'api' },
  { label: 'Argus Data', to: { path: '/', query: { tab: 'data' } }, tabKey: 'data' },
]

const route = useRoute()
const router = useRouter()

const isActive = (tab: Tab): boolean => {
  if (route.path !== '/') return false
  const currentTab = route.query.tab as string | undefined
  if (tab.tabKey === 'kube') {
    return !currentTab || currentTab === 'kube'
  }
  return currentTab === tab.tabKey
}

const goLogin = () => router.push('/login')
const goRegister = () => router.push('/register')

const themeOverrides = {
  common: {
    fontFamily: 'Inter, -apple-system, BlinkMacSystemFont, sans-serif',
  },
}
</script>

<style>
* { margin: 0; padding: 0; box-sizing: border-box; }
html { scroll-behavior: smooth; }
body {
  font-family: 'Inter', -apple-system, BlinkMacSystemFont, sans-serif;
  -webkit-font-smoothing: antialiased;
  background: #fff;
  color: #202124;
}

.app {
  min-height: 100vh;
  display: flex;
  flex-direction: column;
  padding-bottom: 52px;
}

.topbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 12px 48px;
  border-bottom: 1px solid #e8eaed;
  position: sticky;
  top: 0;
  background: #fff;
  z-index: 10;
}

.logo {
  font-weight: 700;
  font-size: 18px;
  letter-spacing: 3px;
  color: #1a73e8;
  text-decoration: none;
  white-space: nowrap;
}

.topbar-center {
  display: flex;
  gap: 24px;
}

.tab-link {
  font-size: 14px;
  font-weight: 500;
  color: #5f6368;
  text-decoration: none;
  padding: 4px 0;
  border-bottom: 2px solid transparent;
  transition: color 0.15s, border-color 0.15s;
}

.tab-link:hover,
.tab-link.active {
  color: #1a73e8;
  border-bottom-color: #1a73e8;
}

.topbar-right {
  display: flex;
  align-items: center;
  gap: 8px;
}

.topbar-action {
  font-size: 14px;
  color: #5f6368;
  text-decoration: none;
  margin-right: 8px;
}

.topbar-action:hover {
  color: #1a73e8;
}

.content {
  flex: 1;
}

@media (max-width: 640px) {
  .topbar { padding: 12px 16px; flex-wrap: wrap; gap: 8px; }
  .topbar-center { order: 3; width: 100%; justify-content: center; }
  .topbar-right { gap: 4px; }
  .topbar-action { display: none; }
}
</style>
