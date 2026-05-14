import { describe, it, expect, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import ScenarioBuilder from '../../components/loadtest/ScenarioBuilder.vue'

// ScenarioBuilder owns the multi-step REST scenario form. The
// canonical shape lives in the backend's DistLoadScenario; these tests
// pin the v-model contract and the cap rule.

function blankEp() {
  return { method: 'POST', url: '', headers: {}, body: '', expect: null, chain: [] }
}

function mountIt(initial) {
  return mount(ScenarioBuilder, {
    props: { modelValue: initial },
    attachTo: document.body,
  })
}

describe('ScenarioBuilder.vue', () => {
  beforeEach(() => { document.body.innerHTML = '' })

  it('renders the initial endpoint card', () => {
    const w = mountIt({ auth: { mode: 'none' }, endpoints: [blankEp()] })
    expect(w.find('[data-testid="scenario-builder"]').exists()).toBe(true)
    expect(w.find('[data-testid="scenario-add-endpoint"]').exists()).toBe(true)
  })

  it('adds endpoints up to the cap of 5, then disables the add button', async () => {
    const w = mountIt({ auth: { mode: 'none' }, endpoints: [blankEp()] })
    // Simulate parent updating modelValue after each emit (real v-model).
    w.vm.$emit = (...args) => {
      if (args[0] === 'update:modelValue') w.setProps({ modelValue: args[1] })
    }
    // Add four more via clicks; each click emits update:modelValue.
    for (let i = 0; i < 4; i++) {
      const addBtn = w.find('[data-testid="scenario-add-endpoint"]')
      await addBtn.trigger('click')
      const emitted = w.emitted('update:modelValue')
      const last = emitted[emitted.length - 1][0]
      await w.setProps({ modelValue: last })
    }
    const finalAdd = w.find('[data-testid="scenario-add-endpoint"]')
    expect(finalAdd.attributes('disabled')).toBeDefined()
    // 6th click should not produce another update beyond the 4 we did
    const before = w.emitted('update:modelValue')?.length || 0
    await finalAdd.trigger('click')
    const after = w.emitted('update:modelValue')?.length || 0
    expect(after).toBe(before)
  })

  it('switches auth mode and reveals the bearer fields', async () => {
    const w = mountIt({ auth: { mode: 'none' }, endpoints: [blankEp()] })
    // Auth section starts collapsed when mode === 'none'; expand it.
    await w.find('.collapser').trigger('click')
    await w.find('[data-testid="scenario-auth-bearer"]').trigger('click')
    const emit = w.emitted('update:modelValue')
    const last = emit[emit.length - 1][0]
    expect(last.auth.mode).toBe('bearer')
    // Bearer defaults: method POST, tokenPath access_token.
    expect(last.auth.bearerMethod).toBe('POST')
    expect(last.auth.bearerTokenPath).toBe('access_token')
  })

  it('switches auth to apiKey and reveals header inputs', async () => {
    const w = mountIt({ auth: { mode: 'none' }, endpoints: [blankEp()] })
    await w.find('.collapser').trigger('click')
    await w.find('[data-testid="scenario-auth-apikey"]').trigger('click')
    await w.setProps({ modelValue: w.emitted('update:modelValue').at(-1)[0] })
    expect(w.find('[data-testid="scenario-apikey-header"]').exists()).toBe(true)
    expect(w.find('[data-testid="scenario-apikey-value"]').exists()).toBe(true)
  })

  it('emits a scenario-shaped object on URL edit', async () => {
    const w = mountIt({ auth: { mode: 'none' }, endpoints: [blankEp()] })
    const urlInput = w.find('[data-testid="scenario-ep-url-0"]')
    await urlInput.setValue('https://api.example.com/users')
    const emit = w.emitted('update:modelValue')
    const last = emit[emit.length - 1][0]
    expect(last.endpoints[0].url).toBe('https://api.example.com/users')
    expect(last.endpoints[0].method).toBe('POST')
  })
})
