<script setup>
import { ref, computed, watch, nextTick } from 'vue'
import { callGo } from '../../composables/useBridge'

// Debug-strategy picker. Opens when the user clicks "Debug" on a pod
// row in PodList. Replaces the older "always nicolaka/netshoot" flow
// with a preset chooser + customization fields so the user can:
//   - pick a different debug image (alpine for package install,
//     busybox for the tiniest shell, custom for an in-house image)
//   - override the entrypoint / args (e.g. `bash -c "sleep infinity"`
//     for an image that exits immediately)
//   - target a specific container in a multi-container pod (sharing
//     the right /proc/<pid> view)
//   - add extra env vars

const props = defineProps({
  pod: { type: Object, required: true }, // { name, namespace, containers: [{name,...}] }
  show: { type: Boolean, default: false },
})
const emit = defineEmits(['close', 'injected', 'error'])

// Built-in strategies. The user picks one as a starting point; image
// + command auto-fill, then any field is editable. "Custom" leaves
// the image blank so the user types in their own.
const STRATEGIES = [
  {
    id: 'netshoot',
    label: 'Network tools',
    image: 'nicolaka/netshoot:latest',
    command: [],
    desc: 'curl, dig, tcpdump, mtr, nmap — the SRE-grade network toolkit.',
  },
  {
    id: 'busybox',
    label: 'Busybox',
    image: 'busybox:latest',
    command: ['sh'],
    desc: 'Minimal POSIX shell + coreutils. Tiny image, no package manager.',
  },
  {
    id: 'alpine',
    label: 'Alpine',
    image: 'alpine:latest',
    command: ['sh'],
    desc: 'apk install whatever you need. Good when netshoot is overkill.',
  },
  {
    id: 'multitool',
    label: 'Network multitool',
    image: 'praqma/network-multitool:latest',
    command: [],
    desc: 'Alternative to netshoot — adds nginx, iperf3, more curl variants.',
  },
  {
    id: 'distroless-debug',
    label: 'Distroless debug',
    image: 'gcr.io/distroless/base-debian12:debug',
    command: ['/busybox/sh'],
    desc: 'For pods built from distroless — same base, with a shell.',
  },
  {
    id: 'custom',
    label: 'Custom',
    image: '',
    command: [],
    desc: 'Provide your own image + command. Useful for in-house debug tools.',
  },
]

const selectedId = ref('netshoot')
const image = ref(STRATEGIES[0].image)
const commandText = ref('') // user types as space-separated; we split on submit
const argsText = ref('')
const targetContainer = ref('')
const envText = ref('') // KEY=value per line

const dialogRef = ref(null)
const submitting = ref(false)
const errorMsg = ref('')

const containerOptions = computed(() => {
  return (props.pod?.containers || []).map((c) => ({ value: c.name, label: c.name }))
})

function selectStrategy(id) {
  selectedId.value = id
  const s = STRATEGIES.find((x) => x.id === id)
  if (!s) return
  // Only auto-fill image when switching to a preset. Don't clobber
  // user-typed values when they re-click the same chip.
  image.value = s.image
  commandText.value = (s.command || []).join(' ')
  argsText.value = ''
  errorMsg.value = ''
}

// Open the native <dialog> when `show` flips to true. Keeps the
// markup declarative while still using showModal() for proper focus
// trap + Escape handling.
watch(() => props.show, async (open) => {
  if (open) {
    // Reset transient state but keep the previously-chosen strategy
    // so re-opening the popup on another pod stays familiar.
    errorMsg.value = ''
    submitting.value = false
    // Pre-target the first container (matches backend default).
    targetContainer.value = props.pod?.containers?.[0]?.name || ''
    await nextTick()
    dialogRef.value?.showModal?.()
  } else {
    if (dialogRef.value?.open) dialogRef.value.close()
  }
})

function close() { emit('close') }

function onDialogClick(e) {
  if (e.target === dialogRef.value) close()
}

function parseEnv(text) {
  const out = {}
  for (const line of String(text).split('\n')) {
    const t = line.trim()
    if (!t || t.startsWith('#')) continue
    const eq = t.indexOf('=')
    if (eq <= 0) continue
    out[t.slice(0, eq).trim()] = t.slice(eq + 1).trim()
  }
  return out
}

function tokenize(text) {
  // Whitespace split is fine for our use — the user can also leave
  // command empty to use the image's ENTRYPOINT.
  const t = String(text).trim()
  if (!t) return []
  return t.split(/\s+/)
}

async function inject() {
  if (!image.value.trim()) {
    errorMsg.value = 'Image is required.'
    return
  }
  submitting.value = true
  errorMsg.value = ''
  try {
    const opts = {
      image: image.value.trim(),
      command: tokenize(commandText.value),
      args: tokenize(argsText.value),
      targetContainer: targetContainer.value || '',
      env: parseEnv(envText.value),
    }
    const res = await callGo('InjectDebugContainerWithOptions', props.pod.namespace, props.pod.name, opts)
    emit('injected', res)
    close()
  } catch (e) {
    errorMsg.value = e?.message || String(e)
    emit('error', errorMsg.value)
  } finally {
    submitting.value = false
  }
}
</script>

<template>
  <dialog
    ref="dialogRef"
    class="debug-modal"
    data-testid="debug-strategy-modal"
    :aria-label="`Debug ${pod?.name}`"
    @close="close"
    @click="onDialogClick"
  >
    <div v-if="show" class="modal-inner">
      <div class="modal-header">
        <div>
          <div class="modal-title">Inject debug container</div>
          <div class="modal-sub">
            <span class="font-mono">{{ pod?.namespace }}/{{ pod?.name }}</span>
          </div>
        </div>
        <button class="modal-close" aria-label="Close" @click="close">×</button>
      </div>

      <div class="modal-body">
        <div class="section-label">Strategy</div>
        <div class="strategy-grid">
          <button
            v-for="s in STRATEGIES"
            :key="s.id"
            type="button"
            class="strategy-card"
            :class="{ active: selectedId === s.id }"
            :data-testid="`debug-strategy-${s.id}`"
            @click="selectStrategy(s.id)"
          >
            <div class="strategy-name">{{ s.label }}</div>
            <div class="strategy-desc">{{ s.desc }}</div>
          </button>
        </div>

        <div class="section-label">Image</div>
        <input
          v-model="image"
          class="form-input font-mono"
          placeholder="e.g. nicolaka/netshoot:latest"
          data-testid="debug-image-input"
        />

        <div class="section-label">Command (overrides ENTRYPOINT — leave blank to keep image default)</div>
        <input
          v-model="commandText"
          class="form-input font-mono"
          placeholder="e.g. bash"
          data-testid="debug-command-input"
        />

        <div class="section-label">Args</div>
        <input
          v-model="argsText"
          class="form-input font-mono"
          placeholder="e.g. -c 'sleep infinity'"
          data-testid="debug-args-input"
        />

        <div class="section-label">Target container</div>
        <select
          v-model="targetContainer"
          class="form-input"
          data-testid="debug-target-container"
          :disabled="containerOptions.length === 0"
        >
          <option value="">(first container — default)</option>
          <option v-for="c in containerOptions" :key="c.value" :value="c.value">{{ c.label }}</option>
        </select>

        <div class="section-label">Environment (KEY=value, one per line)</div>
        <textarea
          v-model="envText"
          class="form-textarea font-mono"
          rows="3"
          placeholder="EXTRA=1&#10;LOG_LEVEL=debug"
          data-testid="debug-env-input"
        ></textarea>

        <div v-if="errorMsg" class="error-row" role="alert">
          <span class="error-dot"></span>{{ errorMsg }}
        </div>
      </div>

      <div class="modal-footer">
        <button type="button" class="btn-cancel" @click="close">Cancel</button>
        <button
          type="button"
          class="btn-inject"
          :disabled="submitting || !image.trim()"
          data-testid="debug-inject-submit"
          @click="inject"
        >{{ submitting ? 'Injecting…' : 'Inject debug container' }}</button>
      </div>
    </div>
  </dialog>
</template>

<style scoped>
.debug-modal {
  background: #1a1c1f;
  border: 1px solid rgba(255,255,255,0.1);
  border-radius: 8px;
  max-width: 600px;
  width: calc(100vw - 48px);
  max-height: 85vh;
  padding: 0;
  color: var(--text, #e8eaec);
  position: fixed;
  top: 50%; left: 50%;
  right: auto; bottom: auto;
  margin: 0;
  transform: translate(-50%, -50%);
}
.debug-modal::backdrop { background: rgba(0,0,0,0.6); }
.modal-inner { display: flex; flex-direction: column; max-height: calc(85vh - 2px); overflow: hidden; }

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

.strategy-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(160px, 1fr));
  gap: 8px;
}
.strategy-card {
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
.strategy-card:hover { background: #2a2e35; }
.strategy-card.active {
  border-color: #6d4ade;
  background: rgba(109, 74, 222, 0.18);
}
.strategy-name { font-size: 12px; font-weight: 600; margin-bottom: 3px; }
.strategy-desc { font-size: 10.5px; color: var(--text2, #b0b4ba); line-height: 1.35; }

.form-input, .form-textarea {
  background: var(--bg3, #23262a);
  border: 1px solid var(--border, #3d424c);
  color: var(--text, #e8eaec);
  padding: 7px 10px;
  border-radius: 5px;
  font: inherit;
  font-size: 12px;
  outline: none;
  resize: vertical;
}
.form-input:focus, .form-textarea:focus { border-color: #6d4ade; }
.font-mono { font-family: var(--mono, ui-monospace, SFMono-Regular, Menlo, monospace); }

.error-row {
  display: flex; align-items: center; gap: 8px;
  padding: 8px 10px;
  background: rgba(240,84,84,0.12);
  border: 1px solid rgba(240,84,84,0.45);
  border-radius: 5px;
  font-size: 12px;
  margin-top: 4px;
}
.error-dot { width: 6px; height: 6px; border-radius: 50%; background: #f05454; flex-shrink: 0; }

.modal-footer {
  display: flex; justify-content: flex-end; gap: 8px;
  padding: 12px 18px;
  border-top: 1px solid rgba(255,255,255,0.08);
}
.btn-cancel, .btn-inject {
  font: inherit;
  font-size: 12px;
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
.btn-inject {
  background: #6d4ade;
  border: 1px solid #6d4ade;
  color: #fff;
}
.btn-inject:hover:not(:disabled) { background: #5a3bc7; }
.btn-inject:disabled { opacity: 0.5; cursor: not-allowed; }
</style>
