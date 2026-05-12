<template>
  <div>
    <section class="hero">
      <h1 class="hero-title">
        {{ c.hero.title }}<br />
        <span class="highlight">{{ c.hero.highlight }}</span>
      </h1>
      <p class="hero-subtitle" v-html="c.hero.subtitle.replace(/\n/g, '<br />')" />
    </section>

    <section class="api-cards">
      <div class="api-grid">
        <div v-for="item in c.features.cards" :key="item.title" class="api-card">
          <div class="api-icon">{{ item.icon }}</div>
          <h3 class="api-title">{{ item.title }}</h3>
          <p class="api-desc">{{ item.desc }}</p>
        </div>
      </div>
    </section>

    <section class="code-sample">
      <div class="code-inner">
        <h2 class="code-title">{{ c.codeSample.title }}</h2>
        <div class="code-block">
          <pre>{{ c.codeSample.code }}</pre>
        </div>
        <p class="code-caption">{{ c.codeSample.caption }}</p>
      </div>
    </section>
  </div>
</template>

<script setup lang="ts">
import { useContentStore } from '../stores/content'

const c = useContentStore()
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
}

.api-cards {
  padding: 20px 48px 80px;
  max-width: 1000px;
  margin: 0 auto;
}

.api-grid {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 24px;
}

.api-card {
  padding: 28px 24px;
  border-radius: 12px;
  background: #f8f9fa;
}

.api-icon { font-size: 28px; margin-bottom: 12px; }
.api-title { font-size: 17px; font-weight: 600; color: #202124; margin-bottom: 6px; }
.api-desc { font-size: 14px; line-height: 1.6; color: #5f6368; }

.code-sample {
  padding: 80px 48px;
  background: #f8f9fa;
}

.code-inner {
  max-width: 680px;
  margin: 0 auto;
  text-align: center;
}

.code-title {
  font-size: 28px;
  font-weight: 700;
  color: #202124;
  margin-bottom: 24px;
}

.code-block {
  background: #1e1e1e;
  border-radius: 10px;
  padding: 24px;
  text-align: left;
  overflow-x: auto;
}

.code-block pre {
  font-family: 'SF Mono', 'Fira Code', monospace;
  font-size: 13px;
  line-height: 1.7;
  color: #d4d4d4;
  white-space: pre;
}

.code-caption {
  margin-top: 16px;
  font-size: 14px;
  color: #5f6368;
}

@media (max-width: 640px) {
  .hero { padding: 60px 24px 40px; }
  .hero-title { font-size: 32px; }
  .api-cards { padding: 20px 24px 60px; }
  .api-grid { grid-template-columns: 1fr; }
  .code-sample { padding: 60px 24px; }
  .code-block pre { font-size: 12px; }
}
</style>
