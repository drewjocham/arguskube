<template>
  <div class="pricing-page">
    <section class="pricing-hero">
      <div class="pricing-badge">{{ c.pricing.badge }}</div>
      <h1 class="pricing-title">{{ c.pricing.title }}</h1>
      <p class="pricing-subtitle" v-html="c.pricing.subtitle.replace(/\n/g, '<br />')" />
    </section>

    <section class="plans">
      <div class="plan-card">
        <div class="plan-header">
          <h2 class="plan-name">{{ c.pricing.planName }}</h2>
          <div class="plan-price">
            <span class="price-free">{{ c.pricing.price }}</span>
          </div>
          <p class="plan-period">{{ c.pricing.period }}</p>
        </div>

        <ul class="plan-features">
          <li v-for="f in c.pricing.features" :key="f" class="plan-feature">
            <span class="feat-check">&#10003;</span> {{ f }}
          </li>
        </ul>

        <div class="plan-action">
          <n-button type="primary" size="large" round block @click="goRegister">
            Sign up for the beta
          </n-button>
        </div>
      </div>
    </section>

    <section class="how-it-works">
      <h2 class="hiw-title">How it works</h2>
      <div class="hiw-steps">
        <div v-for="(step, i) in c.pricing.steps" :key="i" class="hiw-step">
          <div class="hiw-num">{{ step.num }}</div>
          <h4>{{ step.title }}</h4>
          <p>{{ step.desc }}</p>
        </div>
        <Icon icon="material-symbols:arrow-forward" class="hiw-arrow" />
      </div>
    </section>

    <section class="faq">
      <h2 class="faq-title">Frequently asked questions</h2>
      <div class="faq-list">
        <div v-for="(item, i) in c.pricing.faqs" :key="i" class="faq-item">
          <h4 class="faq-q">{{ item.q }}</h4>
          <p class="faq-a">{{ item.a }}</p>
        </div>
      </div>
    </section>
  </div>
</template>

<script setup lang="ts">
import { NButton } from 'naive-ui'
import { Icon } from '@iconify/vue'
import { useRouter } from 'vue-router'
import { useContentStore } from '../stores/content'

const router = useRouter()
const c = useContentStore()
const goRegister = () => router.push('/register')
</script>

<style scoped>
.pricing-page {
  max-width: 800px;
  margin: 0 auto;
  padding: 60px 24px 100px;
}

.pricing-hero {
  text-align: center;
  margin-bottom: 60px;
}

.pricing-badge {
  display: inline-block;
  font-size: 12px;
  font-weight: 600;
  letter-spacing: 2px;
  color: #e37400;
  background: #fef7e0;
  padding: 6px 14px;
  border-radius: 20px;
  margin-bottom: 20px;
}

.pricing-title {
  font-size: 40px;
  font-weight: 700;
  color: #202124;
  margin-bottom: 16px;
}

.pricing-subtitle {
  font-size: 16px;
  line-height: 1.7;
  color: #5f6368;
}

.plans {
  display: flex;
  justify-content: center;
  margin-bottom: 80px;
}

.plan-card {
  width: 100%;
  max-width: 420px;
  border: 2px solid #1a73e8;
  border-radius: 16px;
  padding: 40px 32px;
  background: #fff;
}

.plan-header {
  text-align: center;
  margin-bottom: 32px;
}

.plan-name {
  font-size: 22px;
  font-weight: 700;
  color: #202124;
  margin-bottom: 12px;
}

.price-free {
  font-size: 48px;
  font-weight: 700;
  color: #34a853;
}

.plan-period {
  font-size: 14px;
  color: #5f6368;
  margin-top: 8px;
}

.plan-features {
  list-style: none;
  margin-bottom: 32px;
}

.plan-feature {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 10px 0;
  font-size: 15px;
  color: #202124;
  border-bottom: 1px solid #f1f3f4;
}

.plan-feature:last-child {
  border-bottom: none;
}

.feat-check {
  color: #34a853;
  font-weight: 700;
  font-size: 16px;
}

.how-it-works {
  text-align: center;
  margin-bottom: 80px;
}

.hiw-title {
  font-size: 28px;
  font-weight: 700;
  color: #202124;
  margin-bottom: 40px;
}

.hiw-steps {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 24px;
}

.hiw-step {
  flex: 1;
  max-width: 200px;
}

.hiw-num {
  width: 36px;
  height: 36px;
  border-radius: 50%;
  background: #1a73e8;
  color: #fff;
  font-weight: 700;
  font-size: 16px;
  display: flex;
  align-items: center;
  justify-content: center;
  margin: 0 auto 12px;
}

.hiw-step h4 {
  font-size: 16px;
  font-weight: 600;
  color: #202124;
  margin-bottom: 6px;
}

.hiw-step p {
  font-size: 14px;
  color: #5f6368;
  line-height: 1.5;
}

.hiw-arrow {
  font-size: 28px;
  color: #dadce0;
  flex-shrink: 0;
}

.faq {
  max-width: 600px;
  margin: 0 auto;
}

.faq-title {
  font-size: 24px;
  font-weight: 700;
  color: #202124;
  text-align: center;
  margin-bottom: 32px;
}

.faq-list {
  display: flex;
  flex-direction: column;
  gap: 24px;
}

.faq-q {
  font-size: 16px;
  font-weight: 600;
  color: #202124;
  margin-bottom: 4px;
}

.faq-a {
  font-size: 14px;
  color: #5f6368;
  line-height: 1.6;
}

@media (max-width: 640px) {
  .pricing-title { font-size: 28px; }
  .hiw-steps { flex-direction: column; }
  .hiw-arrow { transform: rotate(90deg); }
  .plan-card { padding: 28px 20px; }
}
</style>
