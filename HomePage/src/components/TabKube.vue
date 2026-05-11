<template>
  <div>
    <section class="hero">
      <h1 class="hero-title">
        {{ c.hero.title }}<br />
        <span class="highlight">{{ c.hero.highlight }}</span>
      </h1>
      <p class="hero-subtitle" v-html="c.hero.subtitle.replace(/\n/g, '<br />')" />
      <div v-if="c.hero.ctas.length" class="hero-actions">
        <n-button
          v-for="(cta, i) in c.hero.ctas"
          :key="i"
          :type="cta.primary ? 'primary' : 'default'"
          size="large"
          round
          @click="handleCta(cta)"
        >
          {{ cta.label }}
        </n-button>
      </div>
    </section>

    <section class="features">
      <div class="features-grid">
        <div v-for="f in c.features.cards" :key="f.title" class="feature-card">
          <div class="card-icon">{{ f.icon }}</div>
          <h3 class="card-title">{{ f.title }}</h3>
          <p class="card-desc">{{ f.desc }}</p>
        </div>
      </div>
    </section>

    <section class="promise">
      <div class="promise-inner">
        <div class="promise-badge">{{ c.promise.badge }}</div>
        <h2 class="promise-title">{{ c.promise.title }}</h2>
        <p class="promise-desc">
          Helm install. CRD-driven configuration. Native RBAC.<br />
          Argus Kube feels like part of your cluster — because it is.
        </p>
      </div>
    </section>
  </div>
</template>

<script setup lang="ts">
import { NButton } from 'naive-ui'
import { useContentStore } from '../stores/content'
import type { CtaBlock } from '../types/content'

const c = useContentStore()

const handleCta = (cta: CtaBlock) => {
  window.open(cta.url || 'https://github.com', '_blank')
}
</script>

<style scoped>
.hero {
  text-align: center;
  padding: 100px 48px 60px;
  max-width: 720px;
  margin: 0 auto;
}

.hero-title {
  font-size: 52px;
  font-weight: 700;
  line-height: 1.15;
  letter-spacing: -1.5px;
  color: #202124;
  margin-bottom: 20px;
}

.highlight { color: #1a73e8; }

.hero-subtitle {
  font-size: 18px;
  line-height: 1.7;
  color: #5f6368;
  margin-bottom: 36px;
}

.hero-actions {
  display: flex;
  gap: 12px;
  justify-content: center;
}

.features {
  padding: 60px 48px 100px;
  max-width: 1000px;
  margin: 0 auto;
}

.features-grid {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 24px;
}

.feature-card {
  padding: 28px 24px;
  border-radius: 12px;
  background: #f8f9fa;
  transition: box-shadow 0.2s;
}

.feature-card:hover {
  box-shadow: 0 4px 12px rgba(0,0,0,0.08);
}

.card-icon { font-size: 28px; margin-bottom: 12px; }
.card-title { font-size: 17px; font-weight: 600; color: #202124; margin-bottom: 6px; }
.card-desc { font-size: 14px; line-height: 1.6; color: #5f6368; }

.promise {
  padding: 80px 48px;
  background: #f8f9fa;
}

.promise-inner {
  max-width: 640px;
  margin: 0 auto;
  text-align: center;
}

.promise-badge {
  display: inline-block;
  font-size: 12px;
  font-weight: 600;
  letter-spacing: 2px;
  color: #1a73e8;
  background: #e8f0fe;
  padding: 6px 14px;
  border-radius: 20px;
  margin-bottom: 16px;
}

.promise-title {
  font-size: 36px;
  font-weight: 700;
  color: #202124;
  margin-bottom: 12px;
}

.promise-desc {
  font-size: 16px;
  line-height: 1.7;
  color: #5f6368;
}

@media (max-width: 640px) {
  .hero { padding: 60px 24px 40px; }
  .hero-title { font-size: 32px; }
  .hero-subtitle { font-size: 16px; }
  .hero-actions { flex-direction: column; align-items: center; }
  .features { padding: 40px 24px 60px; }
  .features-grid { grid-template-columns: 1fr; }
  .promise { padding: 60px 24px; }
  .promise-title { font-size: 28px; }
}
</style>
