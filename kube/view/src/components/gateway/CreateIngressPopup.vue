<script setup>
import { ref, watch, nextTick } from 'vue'
import { callGo } from '../../composables/useBridge'

// Create-Ingress popup. Mirrors CreateNetworkPolicyPopup: pick a
// scaffold, edit the YAML, Apply via ApplyYaml. Scaffolds cover the
// four common Ingress shapes (basic host+path, TLS, default-backend,
// blank). Recommendation cards on the list view can also pre-fill the
// editor via the initialYaml prop.

const props = defineProps({
  show: { type: Boolean, default: false },
  defaultNamespace: { type: String, default: 'default' },
  initialYaml: { type: String, default: '' },
})
const emit = defineEmits(['close', 'applied', 'error'])

const SCAFFOLDS = [
  {
    id: 'basic',
    label: 'Basic host + path',
    desc: 'A single host with one path forwarding to a Service. The everyday case.',
    yaml: (ns) =>
      `apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: my-ingress
  namespace: ${ns}
spec:
  ingressClassName: nginx
  rules:
    - host: example.com
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: my-service
                port:
                  number: 80
`,
  },
  {
    id: 'tls',
    label: 'HTTPS with TLS',
    desc: 'Same as basic but terminates TLS at the controller. Pair with a TLS Secret (cert-manager handles this).',
    yaml: (ns) =>
      `apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: my-ingress
  namespace: ${ns}
spec:
  ingressClassName: nginx
  tls:
    - hosts:
        - example.com
      secretName: my-ingress-tls
  rules:
    - host: example.com
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: my-service
                port:
                  number: 443
`,
  },
  {
    id: 'default-backend',
    label: 'Default backend',
    desc: 'No host rules — every request that misses other ingresses falls through to this Service. Useful as a catch-all 404 page.',
    yaml: (ns) =>
      `apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: catch-all
  namespace: ${ns}
spec:
  ingressClassName: nginx
  defaultBackend:
    service:
      name: fallback-service
      port:
        number: 80
`,
  },
  {
    id: 'blank',
    label: 'Blank',
    desc: 'Start from an empty spec — write your own rules.',
    yaml: (ns) =>
      `apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: my-ingress
  namespace: ${ns}
spec:
  ingressClassName: nginx
  rules: []
`,
  },
]

const selectedId = ref('basic')
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

// Re-template on namespace change only if the editor still looks like
// an unmodified scaffold; never clobber in-progress edits.
watch(namespace, (next) => {
  const s = SCAFFOLDS.find(x => x.id === selectedId.value)
  if (!s) return
  const looksTemplate = SCAFFOLDS.some(sc =>
    sc.yaml(namespace.value) === yamlText.value ||
    sc.yaml('').includes(yamlText.value.slice(0, 80))
  )
  if (looksTemplate) {
    yamlText.value = s.yaml(next || 'default')
  }
})

watch(() => props.show, async (open) => {
  if (open) {
    errorMsg.value = ''
    successMsg.value = ''
    namespace.value = props.defaultNamespace || 'default'
    if (props.initialYaml) {
      yamlText.value = props.initialYaml
      selectedId.value = 'blank'
    } else {
      applyScaffold(selectedId.value || 'basic')
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
    successMsg.value = result || 'Ingress applied.'
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
    class="ing-modal"
    data-testid="create-ingress-modal"
    aria-label="Create Ingress"
    @close="close"
    @click="onDialogClick"
  >
    <div v-if="show" class="modal-inner">
      <div class="modal-header">
        <div>
          <div class="modal-title">Create Ingress</div>
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
            :data-testid="`ingress-scaffold-${s.id}`"
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
          data-testid="ingress-namespace-input"
        />

        <div class="section-label">Manifest</div>
        <textarea
          v-model="yamlText"
          class="yaml-editor font-mono"
          rows="14"
          spellcheck="false"
          data-testid="ingress-yaml-editor"
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
          data-testid="ingress-apply-submit"
          @click="applyYaml"
        >{{ applying ? 'Applying…' : 'Apply to cluster' }}</button>
      </div>
    </div>
  </dialog>
</template>

<style scoped>
.ing-modal {
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
.ing-modal::backdrop { background: rgba(0,0,0,0.6); }
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
