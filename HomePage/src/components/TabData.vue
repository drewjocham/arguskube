<template>
  <div>
    <section class="hero">
      <div class="hero-badge">{{ c.pledges.badge }}</div>
      <h1 class="hero-title">
        {{ c.hero.title }}<br />
        <span class="strike">DrewOmniCloud.com</span>
      </h1>
      <p class="hero-subtitle" v-html="c.hero.subtitle.replace(/\n/g, '<br />')" />
    </section>

    <section class="pledges">
      <div class="pledges-grid">
        <div v-for="p in c.pledges.cards" :key="p.title" class="pledge-card">
          <div class="pledge-check">&#10003;</div>
          <h3 class="pledge-title">{{ p.title }}</h3>
          <p class="pledge-desc">{{ p.desc }}</p>
        </div>
      </div>
    </section>

    <section class="alerting">
      <div class="alerting-inner">
        <h2 class="alerting-title">{{ c.alerting.title }}</h2>
        <p class="alerting-desc" v-html="c.alerting.desc.replace(/\n/g, '<br />')" />
        <div class="alerting-pillars">
          <div v-for="p in c.alerting.pillars" :key="p.num" class="pillar">
            <div class="pillar-num">{{ p.num }}</div>
            <h4>{{ p.title }}</h4>
            <p>{{ p.desc }}</p>
          </div>
        </div>
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

.hero-badge {
  display: inline-block;
  font-size: 12px;
  font-weight: 600;
  letter-spacing: 2px;
  color: #1a73e8;
  background: #e8f0fe;
  padding: 6px 14px;
  border-radius: 20px;
  margin-bottom: 20px;
}

.hero-title {
  font-size: 44px;
  font-weight: 700;
  line-height: 1.2;
  color: #202124;
  margin-bottom: 20px;
}

.strike {
  text-decoration: line-through;
  color: #ea4335;
}

.hero-subtitle {
  font-size: 16px;
  line-height: 1.7;
  color: #5f6368;
}

.pledges {
  padding: 20px 48px 80px;
  max-width: 1000px;
  margin: 0 auto;
}

.pledges-grid {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 24px;
}

.pledge-card {
  padding: 28px 24px;
  border-radius: 12px;
  background: #f8f9fa;
}

.pledge-check {
  color: #34a853;
  font-weight: 700;
  font-size: 24px;
  margin-bottom: 12px;
}

.pledge-title { font-size: 17px; font-weight: 600; color: #202124; margin-bottom: 6px; }
.pledge-desc { font-size: 14px; line-height: 1.6; color: #5f6368; }

.alerting {
  padding: 80px 48px;
  background: #f8f9fa;
}

.alerting-inner {
  max-width: 800px;
  margin: 0 auto;
  text-align: center;
}

.alerting-title {
  font-size: 32px;
  font-weight: 700;
  color: #202124;
  margin-bottom: 12px;
}

.alerting-desc {
  font-size: 16px;
  line-height: 1.7;
  color: #5f6368;
  margin-bottom: 48px;
}

.alerting-pillars {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 32px;
  text-align: center;
}

.pillar-num {
  width: 40px;
  height: 40px;
  border-radius: 50%;
  background: #1a73e8;
  color: #fff;
  font-weight: 700;
  font-size: 18px;
  display: flex;
  align-items: center;
  justify-content: center;
  margin: 0 auto 12px;
}

.pillar h4 {
  font-size: 18px;
  font-weight: 600;
  color: #202124;
  margin-bottom: 8px;
}

.pillar p {
  font-size: 14px;
  line-height: 1.6;
  color: #5f6368;
}

@media (max-width: 640px) {
  .hero { padding: 60px 24px 40px; }
  .hero-title { font-size: 28px; }
  .pledges { padding: 20px 24px 60px; }
  .pledges-grid { grid-template-columns: 1fr; }
  .alerting { padding: 60px 24px; }
  .alerting-pillars { grid-template-columns: 1fr; gap: 24px; }
}
</style>
