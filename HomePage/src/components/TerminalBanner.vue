<template>
  <div v-if="visible" class="banner">
    <div class="banner-inner">
      <Icon icon="material-symbols:terminal" width="20" height="20" class="banner-icon" />
      <div class="banner-text">
        <strong>Argus Terminal</strong> — Manage alerts, silence, and routes right from your CLI.
      </div>
      <n-button text size="small" class="banner-link" @click="learnMore">
        Learn more &rarr;
      </n-button>
      <button class="banner-close" @click="dismiss" aria-label="Dismiss">
        &#10005;
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { NButton } from 'naive-ui'
import { Icon } from '@iconify/vue'

const visible = ref(true)

const dismiss = () => { visible.value = false }
const learnMore = () => { window.open('https://github.com', '_blank') }
</script>

<style scoped>
.banner {
  position: fixed;
  bottom: 0;
  left: 0;
  right: 0;
  background: #1e1e1e;
  color: #d4d4d4;
  font-size: 14px;
  z-index: 100;
  border-top: 1px solid #333;
}

.banner-inner {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 12px;
  padding: 12px 48px;
  max-width: 1000px;
  margin: 0 auto;
}

.banner-icon {
  flex-shrink: 0;
}

.banner-text {
  flex: 1;
  text-align: center;
}

.banner-link {
  color: #1a73e8 !important;
  white-space: nowrap;
  flex-shrink: 0;
}

.banner-close {
  background: none;
  border: none;
  color: #80868b;
  font-size: 14px;
  cursor: pointer;
  padding: 4px;
  line-height: 1;
  flex-shrink: 0;
}

.banner-close:hover {
  color: #fff;
}

@media (max-width: 640px) {
  .banner-inner {
    padding: 10px 16px;
    flex-wrap: wrap;
    gap: 6px;
  }
  .banner-text { font-size: 13px; }
}
</style>
