<template>
  <n-button
    size="large"
    class="auth-btn"
    :class="`provider-${provider}`"
    @click="$emit('click', provider)"
  >
    <span class="btn-icon">
      <Icon :icon="iconMap[provider]" width="20" height="20" />
    </span>
    {{ label }}
  </n-button>
</template>

<script setup lang="ts">
import { NButton } from 'naive-ui'
import { Icon } from '@iconify/vue'
import type { AuthProvider } from '../types/auth'

const props = defineProps<{
  provider: AuthProvider
  action: 'login' | 'register'
}>()

defineEmits<{
  (e: 'click', provider: AuthProvider): void
}>()

const iconMap: Record<AuthProvider, string> = {
  github: 'mdi:github',
  google: 'flat-color-icons:google',
}

const label = `${props.action === 'login' ? 'Continue' : 'Register'} with ${props.provider === 'github' ? 'GitHub' : 'Google'}`
</script>

<style scoped>
.auth-btn {
  width: 100% !important;
  display: flex !important;
  align-items: center;
  justify-content: center;
  gap: 10px;
  height: 48px !important;
  font-size: 15px !important;
  border-radius: 8px !important;
  border: 1px solid #dadce0 !important;
  background: #fff !important;
  color: #202124 !important;
}

.auth-btn:hover {
  background: #f8f9fa !important;
}

.btn-icon {
  display: inline-flex;
  align-items: center;
}
</style>
