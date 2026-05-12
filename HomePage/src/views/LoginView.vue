<template>
  <div class="auth-page">
    <div class="auth-card">
      <h1 class="auth-title">Welcome back</h1>
      <p class="auth-subtitle">Log in to your Argus account</p>

      <div class="auth-buttons">
        <AuthProviderBtn
          v-for="p in providers"
          :key="p"
          :provider="p"
          action="login"
          @click="handleLogin"
        />
      </div>

      <p class="auth-footer">
        Don't have an account?
        <router-link to="/register" class="auth-link">Register</router-link>
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

const handleLogin = (provider: AuthProvider) => {
  auth.login(provider)
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
