import { defineStore } from 'pinia'
import { fetchOneEntry } from '@builder.io/sdk-vue'
import type { LandingContent, TabKubeContent, TabApiContent, TabDataContent, PricingContent } from '../types/content'

const BUILDER_API_KEY = import.meta.env.VITE_BUILDER_API_KEY as string | undefined

const staticKube: TabKubeContent = {
  hero: {
    title: 'Your observability,',
    highlight: 'your control.',
    subtitle: 'Every destructive tool you rely on — Prometheus, Grafana, Alertmanager —\nfinally has a first-class home. Argus Kube brings them together,\nso you monitor what matters, not what manages you.',
    ctas: [
      { label: 'Get Started', primary: true },
      { label: 'View on GitHub', primary: false },
    ],
    meta: ['Kubernetes Native', 'Open Source', 'Your Data Stays Yours'],
  },
  features: {
    badge: undefined,
    title: 'Built on Argus Kube',
    cards: [
      { icon: '\u26A1', title: 'Always Current', desc: 'Every metric, dashboard, and alert stays automatically in sync. No stale dashboards. No missed changes.' },
      { icon: '\uD83D\uDDC4\uFE0F', title: 'Native Feel', desc: 'Helm chart, CRDs, RBAC — it drops into your cluster like a first-class citizen. No sidecars, no hacks.' },
      { icon: '\uD83D\uDE80', title: 'Boosts Your Potential', desc: 'Stop gluing together dashboards. Start understanding what your data is telling you.' },
      { icon: '\uD83D\uDD14', title: 'Alerting Made Simple', desc: 'Silence, route, and escalate with plain intent. Alert fatigue ends here.' },
      { icon: '\u26EF', title: 'Unified Control Plane', desc: 'Prometheus, Grafana, Alertmanager — one streamlined interface instead of three silos.' },
      { icon: '\uD83D\uDEE1\uFE0F', title: 'Battle-Tested Stack', desc: 'Built on the ecosystem you trust, without the sprawl you hate.' },
    ],
  },
  promise: {
    badge: 'KUBERNETES NATIVE',
    title: 'Runs where you run.',
    cards: [],
  },
}

const staticApi: TabApiContent = {
  hero: {
    title: 'Interact through code.',
    highlight: 'Control through API.',
    subtitle: 'Argus API gives you programmatic access to every metric, alert, and dashboard.\nIntegrate with your pipelines, ChatOps, and automation without ever leaving your terminal.',
    ctas: [],
  },
  features: {
    badge: undefined,
    title: 'API Capabilities',
    cards: [
      { icon: '\uD83D\uDD0C', title: 'RESTful by Default', desc: 'Every resource — alerts, silences, dashboards, routes — exposed via clean REST endpoints.' },
      { icon: '\uD83D\uDD11', title: 'Token-Based Auth', desc: 'Scoped API tokens integrate with your existing secrets management and CI/CD pipelines.' },
      { icon: '\uD83E\uDD16', title: 'ChatOps Ready', desc: 'Acknowledge, silence, or escalate alerts directly from Slack, Teams, or your bot of choice.' },
      { icon: '\uD83D\uDCCA', title: 'Programmatic Dashboards', desc: 'Create, update, and version-control dashboards as code. No click-ops.' },
      { icon: '\u2699\uFE0F', title: 'Webhook Sinks', desc: 'Forward alerts to PagerDuty, OpsGenie, or custom webhooks with zero configuration drift.' },
      { icon: '\uD83D\uDEE0\uFE0F', title: 'Client Libraries', desc: 'Official Python and Go clients. For everything else, curl is all you need.' },
    ],
  },
  codeSample: {
    title: 'Simple by design.',
    code: '# Silence an alert by label\ncurl -X POST https://argus:8080/api/v1/silences \\\n  -H "Authorization: Bearer $ARGUS_TOKEN" \\\n  -d \'{ "matchers": ["severity=critical"], "duration": "1h" }\'\n\n# List all firing alerts\ncurl https://argus:8080/api/v1/alerts \\\n  -H "Authorization: Bearer $ARGUS_TOKEN"',
    caption: 'RESTful API. Any language. Any pipeline.',
  },
}

const staticData: TabDataContent = {
  hero: {
    title: 'Your data cannot be used on',
    highlight: '',
    subtitle: 'Argus Data is self-hosted in your own environment. Your metrics, your alerts, your dashboards —\nnone of it ever leaves your infrastructure. We don\'t see it. We can\'t see it.\nThere is no cloud backhaul, no telemetry, no "phone home."',
    ctas: [],
  },
  pledges: {
    badge: 'PRIVACY FIRST',
    title: '',
    cards: [
      { icon: 'Self-Hosted', title: 'Self-Hosted', desc: 'Runs in your cluster, on your cloud, or on-prem. Zero external dependencies.' },
      { icon: 'No Telemetry', title: 'No Telemetry', desc: 'No usage stats, no crash reports, no "optimization" data sent anywhere. Ever.' },
      { icon: 'Audit Trail', title: 'Audit Trail', desc: 'Every silence, route change, and escalation is logged. Know who did what and when.' },
      { icon: 'SSO & RBAC', title: 'SSO & RBAC', desc: 'Integrate with your identity provider. Map teams to roles. Lock it down.' },
      { icon: 'Encryption at Rest', title: 'Encryption at Rest', desc: 'All sensitive data encrypted. Your encryption keys, your control.' },
      { icon: 'Open Source', title: 'Open Source', desc: 'Audit the code. Build from source. Trust, but verify — because you can.' },
    ],
  },
  alerting: {
    title: 'Alerting operations, simplified.',
    desc: 'Silence with a reason. Route with a label. Escalate with a policy.\nEvery action is audited, every override is visible, and everything is in your control.',
    pillars: [
      { num: 1, title: 'Silence', desc: 'Mute alerts by matchers with a clear expiry and justification.' },
      { num: 2, title: 'Route', desc: 'Define routing trees by severity, namespace, or any label.' },
      { num: 3, title: 'Escalate', desc: 'Auto-escalate unacknowledged criticals to PagerDuty, Slack, or email.' },
    ],
  },
}

const staticPricing: PricingContent = {
  badge: 'BETA PROGRAM',
  title: 'Get Argus free during beta',
  subtitle: 'We\'re selecting a limited group of beta testers. Sign up early and if chosen,\nyou get full access to Argus Kube, Argus API, and Argus Data — completely free.',
  planName: 'Beta Tester',
  price: 'Free',
  period: 'For the duration of the beta program',
  features: [
    'Full access to Argus Kube',
    'Argus API with unlimited tokens',
    'Argus Data — self-hosted, zero telemetry',
    'Priority support via Discord',
    'No credit card required',
    'Influence the product roadmap',
  ],
  steps: [
    { num: 1, title: 'Register', desc: 'Create your account with GitHub or Google.' },
    { num: 2, title: 'Apply', desc: 'Tell us a bit about your environment and use case.' },
    { num: 3, title: 'Get Selected', desc: 'If chosen, you\'ll receive full access at no cost.' },
  ],
  faqs: [
    { q: 'How long will the beta last?', a: 'We expect the beta to run for 3-6 months. All beta testers will get ample notice before any changes.' },
    { q: 'Will I have to pay after the beta?', a: 'Beta testers will receive a special early-adopter discount. Pricing details will be shared before the beta ends.' },
    { q: 'How are beta testers selected?', a: 'We\'re looking for a diverse set of environments and use cases. The more variety, the better the product.' },
    { q: 'Can I run Argus in production during beta?', a: 'Yes. Argus is built on battle-tested components (Prometheus, Grafana, Alertmanager) and is production-ready from day one.' },
  ],
}

export const useContentStore = defineStore('content', {
  state: () => ({
    kube: { ...staticKube } as TabKubeContent,
    api: { ...staticApi } as TabApiContent,
    data: { ...staticData } as TabDataContent,
    pricing: { ...staticPricing } as PricingContent,
    loading: false,
    fromBuilder: false,
  }),

  actions: {
    async fetchAll() {
      if (!BUILDER_API_KEY) return

      this.loading = true
      try {
        const result = await fetchOneEntry({
          model: 'landing-page',
          apiKey: BUILDER_API_KEY,
          userAttributes: { urlPath: '/' },
        })

        if (result?.data) {
          const cms = result.data as LandingContent
          if (cms.kube) this.kube = { ...staticKube, ...cms.kube }
          if (cms.api) this.api = { ...staticApi, ...cms.api }
          if (cms.data) this.data = { ...staticData, ...cms.data }
          if (cms.pricing) this.pricing = { ...staticPricing, ...cms.pricing }
          this.fromBuilder = true
        }
      } finally {
        this.loading = false
      }
    },
  },
})
