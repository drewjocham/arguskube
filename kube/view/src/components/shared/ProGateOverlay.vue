<script setup>
defineProps({
  feature: { type: String, required: true },
  title: { type: String, default: 'Pro Feature' },
  description: { type: String, default: 'Upgrade to Argus Pro to unlock this feature.' }
})
</script>

<template>
  <div class="pro-gate-overlay">
    <div class="pro-gate-card">
      <div class="pro-gate-icon">
        <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5">
          <rect x="3" y="11" width="18" height="11" rx="2" ry="2"></rect>
          <path d="M7 11V7a5 5 0 0 1 10 0v4"></path>
        </svg>
      </div>
      <div class="pro-gate-title">{{ title }}</div>
      <div class="pro-gate-desc">{{ description }}</div>
      <button class="pro-gate-btn">Upgrade to Pro</button>
    </div>
  </div>
</template>

<style scoped>
.pro-gate-overlay {
  position: absolute;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(0, 0, 0, 0.6);
  backdrop-filter: blur(6px);
  z-index: 50;
  border-radius: 8px;
}

.pro-gate-card {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 10px;
  padding: 32px 40px;
  background: var(--bg3, #1e2023);
  border: 1px solid rgba(167, 139, 250, 0.2);
  border-radius: 12px;
  text-align: center;
  max-width: 320px;
}

.pro-gate-icon {
  width: 48px;
  height: 48px;
  border-radius: 12px;
  background: rgba(167, 139, 250, 0.1);
  display: flex;
  align-items: center;
  justify-content: center;
  color: #a78bfa;
}

.pro-gate-title {
  font-size: 16px;
  font-weight: 600;
  color: var(--text, #e8eaec);
}

.pro-gate-desc {
  font-size: 13px;
  color: var(--text3, #8b8f96);
  line-height: 1.5;
}

.pro-gate-btn {
  margin-top: 6px;
  padding: 8px 24px;
  border-radius: 8px;
  font-size: 13px;
  font-weight: 600;
  border: none;
  cursor: pointer;
  color: white;
  background: linear-gradient(135deg, #a78bfa 0%, #818cf8 100%);
  transition: opacity 0.15s;
}
.pro-gate-btn:hover {
  opacity: 0.9;
}
</style>
