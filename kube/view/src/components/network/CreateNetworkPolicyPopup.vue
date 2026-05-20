<script setup>
import { ref, watch, nextTick } from 'vue'
import { callGo } from '../../composables/useBridge'

// Create-NetworkPolicy popup. YAML editor pre-filled with one of
// four scaffolds the user picks; Apply hits the existing ApplyYaml
// backend method, surfaces inline success / error.
//
// Why YAML and not a form: NetworkPolicy spec is *expressive* (peer
// selectors, port ranges, namespace selectors, CIDR blocks). A real
// form would be either too restrictive (cover only the easy cases)
// or as complex as the YAML itself. Scaffolds give the user the
// shape they need; they edit from there.

const props = defineProps({
  show: { type: Boolean, default: false },
  // Pre-fill the metadata.namespace field. Optional — the user can
  // still edit it.
  defaultNamespace: { type: String, default: 'default' },
  // Pre-fill the editor body with a specific YAML (e.g. when the
  // popup is opened to apply a recommendation's suggestedYAML).
  initialYaml: { type: String, default: '' },
})
const emit = defineEmits(['close', 'applied', 'error'])

// Scaffold templates — keep all four trivially copy-pasteable. The
// names match common k8s NetworkPolicy patterns the user can grep
// for in docs.
const SCAFFOLDS = [
  {
    id: 'default-deny',
    label: 'Default deny (all)',
    desc: 'Block every ingress + egress for every pod in the namespace. Start here, then add allow-rules.',
    yaml: (ns) =>
      `apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: default-deny-all
  namespace: ${ns}
spec:
  podSelector: {}
  policyTypes:
    - Ingress
    - Egress
`,
  },
  {
    id: 'allow-same-ns',
    label: 'Allow same-namespace',
    desc: 'Allow ingress only from other pods in this namespace. Pair with default-deny for tight isolation.',
    yaml: (ns) =>
      `apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: allow-same-namespace
  namespace: ${ns}
spec:
  podSelector: {}
  policyTypes: [Ingress]
  ingress:
    - from:
        - podSelector: {}
`,
  },
  {
    id: 'allow-egress-dns',
    label: 'Allow egress to DNS',
    desc: 'Allow egress UDP/TCP 53 to kube-dns. Required when you have a default-deny egress policy.',
    yaml: (ns) =>
      `apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: allow-egress-dns
  namespace: ${ns}
spec:
  podSelector: {}
  policyTypes: [Egress]
  egress:
    - to:
        - namespaceSelector:
            matchLabels:
              kubernetes.io/metadata.name: kube-system
          podSelector:
            matchLabels:
              k8s-app: kube-dns
      ports:
        - port: 53
          protocol: UDP
        - port: 53
          protocol: TCP
`,
  },
  {
    id: 'blank',
    label: 'Blank',
    desc: 'Start from an empty spec — write your own selectors and rules.',
    yaml: (ns) =>
      `apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: my-policy
  namespace: ${ns}
spec:
  podSelector: {}
  policyTypes: [Ingress]
  ingress: []
`,
  },
]

const selectedId = ref('default-deny')
const namespace = ref(props.defaultNamespace || 'default')
const yamlText = ref('')

const applying = ref(false)
const errorMsg = ref('')
const successMsg = ref('')

const dialogRef = ref(null)

function applyScaffold(id) {
  selectedId.value = id
  const s = SCAFFOLDS.find(x => x.id === id)
  if (!s) return
  yamlText.value = s.yaml(namespace.value || 'default')
}

// When the user retypes the namespace, only re-scaffold if the
// editor still holds the unmodified template — don't clobber their
// in-progress edits.
watch(namespace, (next) => {
  const s = SCAFFOLDS.find(x => x.id === selectedId.value)
  if (!s) return
  // crude "is template" check — true when current YAML matches the
  // scaffold for the previous namespace value
  const looksTemplate = SCAFFOLDS.some(sc => sc.yaml(namespace.value) === yamlText.value || sc.yaml('').includes(yamlText.value.slice(0, 80)))
  if (looksTemplate) {
    yamlText.value = s.yaml(next || 'default')
  }
})

watch(() => props.show, async (open) => {
  if (open) {
    errorMsg.value = ''
    successMsg.value = ''
    namespace.value = props.defaultNamespace || 'default'
    // initialYaml wins if the parent supplied one (recommendation flow).
    if (props.initialYaml) {
      yamlText.value = props.initialYaml
      selectedId.value = 'blank'
    } else {
      applyScaffold(selectedId.value || 'default-deny')
    }
    await nextTick()
    dialogRef.value?.showModal?.()
  } else if (dialogRef.value?.open) {
    dialogRef.value.close()
  }
})

function close() { emit('close') }

function onDialogClick(e) {
  if (e.target === dialogRef.value) close()
}

async function applyYaml() {
  if (!yamlText.value.trim()) {
    errorMsg.value = 'Manifest is empty.'
    return
  }
  applying.value = true
  errorMsg.value = ''
  successMsg.value = ''
  try {
    const result = await callGo('ApplyYaml', yamlText.value)
    successMsg.value = result || 'NetworkPolicy applied.'
    emit('applied', { yaml: yamlText.value, result })
  } catch (e) {
    errorMsg.value = e?.message || String(e)
    emit('error', errorMsg.value)
  } finally {
    applying.value = false
  }
}
</script>

<template>
  <dialog
    ref="dialogRef"
    class="np-modal"
    data-testid="create-netpol-modal"
    aria-label="Create NetworkPolicy"
    @close="close"
    @click="onDialogClick"
  >
    <div v-if="show" class="modal-inner">
      <div class="modal-header">
        <div>
          <div class="modal-title">Create NetworkPolicy</div>
          <div class="modal-sub">
            Pick a scaffold, tweak the YAML, apply to the cluster.
          </div>
        </div>
        <button type="button" class="modal-close" aria-label="Close" @click="close">×</button>
      </div>

      <div class="modal-body">
        <div class="section-label">Scaffold</div>
        <div class="scaffold-grid">
          <button
            v-for="s in SCAFFOLDS"
            :key="s.id"
            type="button"
            class="scaffold-card"
            :class="{ active: selectedId === s.id }"
            :data-testid="`netpol-scaffold-${s.id}`"
            @click="applyScaffold(s.id)"
          >
            <div class="sc-label">{{ s.label }}</div>
            <div class="sc-desc">{{ s.desc }}</div>
          </button>
        </div>

        <div class="section-label">Namespace</div>
        <input
          v-model="namespace"
          class="form-input font-mono"
          placeholder="e.g. default"
          data-testid="netpol-namespace-input"
        />

        <div class="section-label">Manifest</div>
        <textarea
          v-model="yamlText"
          class="yaml-editor font-mono"
          rows="14"
          spellcheck="false"
          data-testid="netpol-yaml-editor"
        ></textarea>

        <div v-if="errorMsg" class="status-row err" role="alert">
          <span class="status-dot err"></span>{{ errorMsg }}
        </div>
        <div v-if="successMsg" class="status-row ok" role="status">
          <span class="status-dot ok"></span>{{ successMsg }}
        </div>
      </div>

      <div class="modal-footer">
        <button type="button" class="btn-cancel" @click="close">Cancel</button>
        <button
          type="button"
          class="btn-apply"
          :disabled="applying || !yamlText.trim()"
          data-testid="netpol-apply-submit"
          @click="applyYaml"
        >{{ applying ? 'Applying…' : 'Apply to cluster' }}</button>
      </div>
    </div>
  </dialog>
</template>

<style scoped>
.np-modal {
  background: #1a1c1f;
  border: 1px solid rgba(255,255,255,0.1);
  border-radius: 8px;
  max-width: 760px;
  width: calc(100vw - 48px);
  max-height: 88vh;
  padding: 0;
  color: var(--text, #e8eaec);
  position: fixed;
  top: 50%; left: 50%;
  right: auto; bottom: auto;
  margin: 0;
  transform: translate(-50%, -50%);
}
.np-modal::backdrop { background: rgba(0,0,0,0.6); }
.modal-inner { display: flex; flex-direction: column; max-height: calc(88vh - 2px); overflow: hidden; }

.modal-header {
  display: flex; justify-content: space-between; align-items: flex-start;
  gap: 16px;
  padding: 14px 18px;
  border-bottom: 1px solid rgba(255,255,255,0.08);
}
.modal-title { font-size: 15px; font-weight: 600; }
.modal-sub { margin-top: 4px; font-size: 12px; color: var(--text2, #b0b4ba); }
.modal-close {
  background: none; border: none; color: var(--text2, #b0b4ba);
  font-size: 22px; line-height: 1; cursor: pointer; padding: 4px 10px;
}
.modal-close:hover { color: var(--text, #fff); }

.modal-body {
  flex: 1; overflow-y: auto; padding: 14px 18px;
  display: flex; flex-direction: column; gap: 10px;
}
.section-label {
  font-size: 10.5px; text-transform: uppercase; letter-spacing: 0.06em;
  color: var(--text2, #b0b4ba); margin-top: 6px;
}

.scaffold-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(200px, 1fr));
  gap: 8px;
}
.scaffold-card {
  background: #23262a;
  border: 1px solid rgba(255,255,255,0.08);
  border-radius: 6px;
  padding: 8px 10px;
  text-align: left;
  cursor: pointer;
  color: var(--text, #e8eaec);
  font: inherit;
  transition: border-color 0.12s, background 0.12s;
}
.scaffold-card:hover { background: #2a2e35; }
.scaffold-card.active {
  border-color: #6d4ade;
  background: rgba(109,74,222,0.18);
}
.sc-label { font-size: 12px; font-weight: 600; margin-bottom: 4px; }
.sc-desc { font-size: 10.5px; color: var(--text2, #b0b4ba); line-height: 1.4; }

.form-input, .yaml-editor {
  background: #141517;
  border: 1px solid var(--border, #3d424c);
  color: var(--text, #e8eaec);
  padding: 8px 10px;
  border-radius: 5px;
  font: inherit;
  font-size: 12px;
  outline: none;
  resize: vertical;
  line-height: 1.5;
}
.form-input:focus, .yaml-editor:focus { border-color: #6d4ade; }
.yaml-editor { tab-size: 2; min-height: 240px; }

.status-row {
  display: flex; align-items: center; gap: 8px;
  padding: 8px 10px;
  border-radius: 5px;
  font-size: 12px;
  margin-top: 4px;
  word-break: break-word;
}
.status-row.err { background: #b8392f; color: #fff; }
.status-row.ok  { background: #1d6f49; color: #fff; }
.status-dot { width: 6px; height: 6px; border-radius: 50%; flex-shrink: 0; background: rgba(255,255,255,0.85); }

.modal-footer {
  display: flex; justify-content: flex-end; gap: 8px;
  padding: 12px 18px;
  border-top: 1px solid rgba(255,255,255,0.08);
}
.btn-cancel, .btn-apply {
  font: inherit; font-size: 12px;
  padding: 6px 14px;
  border-radius: 5px;
  cursor: pointer;
}
.btn-cancel {
  background: #2a2e35;
  border: 1px solid #3d424c;
  color: var(--text, #fff);
}
.btn-cancel:hover { background: #3a4049; }
.btn-apply {
  background: #6d4ade;
  border: 1px solid #6d4ade;
  color: #fff;
}
.btn-apply:hover:not(:disabled) { background: #5a3bc7; }
.btn-apply:disabled { opacity: 0.5; cursor: not-allowed; }

.font-mono { font-family: var(--mono, ui-monospace, SFMono-Regular, Menlo, monospace); }
</style>
