<template>
  <div class="auth-page">
    <div class="auth-card">
      <h1 class="auth-title">Create your account</h1>
      <p class="auth-subtitle">Start with Argus in under a minute</p>

      <div class="auth-buttons">
        <AuthProviderBtn
          v-for="p in providers"
          :key="p"
          :provider="p"
          action="register"
          @click="handleRegister"
        />
      </div>

      <p class="auth-footer">
        Already have an account?
        <router-link to="/login" class="auth-link">Log in</router-link>
      </p>
    </div>
  </div>
</template>

<script setup lang="ts">
import AuthProviderBtn from '../components/AuthProviderBtn.vue'
import { useAuthStore } from '../stores/auth'
import type { AuthProvider } from '../types/auth'

const providers: AuthProvider[] = ['github', 'google']
const auth = useAuthStore()

const handleRegister = (provider: AuthProvider) => {
  auth.register(provider)
}
</script>

<style scoped>
.auth-page {
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 80px 24px;
  min-height: calc(100vh - 120px);
}

.auth-card {
  width: 100%;
  max-width: 380px;
  text-align: center;
}

.auth-title {
  font-size: 28px;
  font-weight: 700;
  color: #202124;
  margin-bottom: 8px;
}

.auth-subtitle {
  font-size: 15px;
  color: #5f6368;
  margin-bottom: 36px;
}

.auth-buttons {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.auth-footer {
  margin-top: 28px;
  font-size: 14px;
  color: #5f6368;
}

.auth-link {
  color: #1a73e8;
  text-decoration: none;
  font-weight: 500;
}

.auth-link:hover {
  text-decoration: underline;
}
</style>
