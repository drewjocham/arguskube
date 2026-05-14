import { describe, it, expect, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import EndpointConfigModal from '../../components/loadtest/EndpointConfigModal.vue'

function blankEp() {
  return { method: 'POST', url: 'https://api/x', headers: {}, body: '', expect: null, chain: [] }
}
function mountIt(ep = blankEp()) {
  return mount(EndpointConfigModal, {
    props: { open: true, endpoint: ep },
    attachTo: document.body,
  })
}
function applyLast(w, prop) {
  const emits = w.emitted(prop)
  if (!emits) return null
  const last = emits[emits.length - 1][0]
  return last
}

describe('EndpointConfigModal.vue', () => {
  beforeEach(() => { document.body.innerHTML = '' })

  it('switches to expect tab and shows status + mode picker', async () => {
    const w = mountIt()
    await w.find('[data-testid="modal-tab-expect"]').trigger('click')
    expect(w.find('[data-testid="modal-status"]').exists()).toBe(true)
    expect(w.find('[data-testid="modal-mode-pick"]').exists()).toBe(true)
  })

  it('clicking a leaf in the tree picker adds an equals field check with the leaf value', async () => {
    const w = mountIt()
    await w.find('[data-testid="modal-tab-expect"]').trigger('click')
    // Load a sample with one leaf.
    const sample = JSON.stringify({ id: 42 })
    await w.find('[data-testid="modal-sample-input"]').setValue(sample)
    // Tree rows: row 0 = root object, row 1 = leaf "id".
    const addLeaf = w.find('[data-testid="tree-add-1"]')
    await addLeaf.trigger('click')
    const updated = applyLast(w, 'update:endpoint')
    expect(updated.expect.fieldChecks).toHaveLength(1)
    const fc = updated.expect.fieldChecks[0]
    expect(fc.path).toBe('id')
    expect(fc.kind).toBe('equals')
    expect(fc.value).toBe(42)
  })

  it('clicking the bracket icon on an array adds a length check', async () => {
    const w = mountIt()
    await w.find('[data-testid="modal-tab-expect"]').trigger('click')
    const sample = JSON.stringify({ items: [1, 2, 3] })
    await w.find('[data-testid="modal-sample-input"]').setValue(sample)
    // Rows: 0 root object, 1 array "items", 2..4 array leaves.
    await w.find('[data-testid="tree-add-1"]').trigger('click')
    const updated = applyLast(w, 'update:endpoint')
    const fc = updated.expect.fieldChecks[0]
    expect(fc.kind).toBe('length')
    expect(fc.path).toBe('items.#')
    expect(fc.value).toBe(3)
  })

  it('"Match whole response" mode populates bodyMatches and clears fieldChecks', async () => {
    const ep = { ...blankEp(), expect: { status: 0, bodyMatches: '', fieldChecks: [{ path: 'x', kind: 'exists' }] } }
    const w = mountIt(ep)
    await w.find('[data-testid="modal-tab-expect"]').trigger('click')
    await w.find('[data-testid="modal-mode-match"]').trigger('click')
    // setExpectMode('matchWhole') clears fieldChecks via patchExpect.
    const updated = applyLast(w, 'update:endpoint')
    expect(updated.expect.fieldChecks).toEqual([])
    // Now type into the bodyMatches textarea.
    await w.setProps({ endpoint: updated })
    const ta = w.find('[data-testid="modal-bodymatch"]')
    await ta.setValue('{"ok":true}')
    const upd2 = applyLast(w, 'update:endpoint')
    expect(upd2.expect.bodyMatches).toBe('{"ok":true}')
    // The visual JSON picker (sample input) should not be rendered in match mode.
    expect(w.find('[data-testid="modal-sample-input"]').exists()).toBe(false)
  })

  it('adds a chained call via the Chain tab', async () => {
    const w = mountIt()
    await w.find('[data-testid="modal-tab-chain"]').trigger('click')
    await w.find('[data-testid="modal-add-chain"]').trigger('click')
    const updated = applyLast(w, 'update:endpoint')
    expect(updated.chain).toHaveLength(1)
    expect(updated.chain[0].method).toBe('POST')
  })
})
