export interface FeatureCard {
  icon: string
  title: string
  desc: string
}

export interface CtaBlock {
  label: string
  url?: string
  primary: boolean
}

export interface HeroSection {
  title: string
  highlight: string
  subtitle: string
  ctas: CtaBlock[]
  meta?: string[]
}

export interface FeatureSection {
  badge?: string
  title: string
  cards: FeatureCard[]
}

export interface CodeSample {
  title: string
  code: string
  caption: string
}

export interface Pillar {
  num: number
  title: string
  desc: string
}

export interface TabKubeContent {
  hero: HeroSection
  features: FeatureSection
  promise: FeatureSection
}

export interface TabApiContent {
  hero: HeroSection
  features: FeatureSection
  codeSample: CodeSample
}

export interface TabDataContent {
  hero: HeroSection
  pledges: FeatureSection
  alerting: {
    title: string
    desc: string
    pillars: Pillar[]
  }
}

export interface PricingContent {
  badge: string
  title: string
  subtitle: string
  planName: string
  price: string
  period: string
  features: string[]
  steps: { num: number; title: string; desc: string }[]
  faqs: { q: string; a: string }[]
}

export interface LandingContent {
  kube: TabKubeContent
  api: TabApiContent
  data: TabDataContent
  pricing: PricingContent
}
