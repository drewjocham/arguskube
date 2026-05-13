<script setup>
import { ref } from 'vue'
import { useUserProfile } from '../../composables/useUserProfile'

// PrivacyControls — Settings → Privacy section. Today it surfaces a
// single user-initiated action (§6 promise: "A 'Forget my activity'
// button in Settings nukes the user_activity table"). Future controls
// (sync-across-machines toggle, mute-list management) slot in here.
//
// The Forget button is irreversible — we ask for confirmation inline
// via a two-stage button rather than a modal. The two-stage UI is the
// same affordance the existing destructive actions in Settings use
// (Vault: Reset, Notification Channels: Delete) so it feels familiar.

const profile = useUserProfile()

const confirming = ref(false)
const result = ref('')

function arm() {
  confirming.value = true
  result.value = ''
  setTimeout(() => { confirming.value = false }, 5000)
}

async function confirm() {
  const ok = await profile.clearActivity()
  confirming.value = false
  result.value = ok
    ? 'Activity cleared. Argus will start learning your patterns again from scratch.'
    : 'Could not clear activity. Please try again.'
}
</script>

<template>
  <section class="privacy-controls" data-testid="privacy-controls">
    <header>
      <h2>Privacy</h2>
      <p class="hint">
        Argus learns your navigation patterns to surface helpful
        suggestions. All observations stay on this Mac. Clear them
        any time — mute decisions on individual suggestions are
        preserved across a clear.
      </p>
    </header>

    <div class="action-row">
      <div class="action-text">
        <strong>Forget my activity</strong>
        <span class="hint-inline">Deletes every recorded navigation and suggestion log entry.</span>
      </div>
      <button
        v-if="!confirming"
        class="action-btn"
        data-testid="forget-activity-arm"
        @click="arm"
      >Forget my activity</button>
      <button
        v-else
        class="action-btn danger"
        data-testid="forget-activity-confirm"
        @click="confirm"
      >Click again to confirm</button>
    </div>

    <p v-if="result" class="result" data-testid="forget-activity-result">{{ result }}</p>
  </section>
</template>

<style scoped>
.privacy-controls {
  margin-bottom: 18px;
  padding: 14px;
  background: var(--bg, #141414);
  border: 1px solid var(--border, #2a2a2a);
  border-radius: 8px;
}

header h2 {
  margin: 0 0 6px;
  font-size: 14px;
  font-weight: 600;
  color: var(--text, #e5e5e5);
}
.hint {
  margin: 0 0 12px;
  font-size: 12px;
  color: var(--text2, #b0b0b0);
  line-height: 1.45;
}

.action-row {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 10px 12px;
  background: var(--bg2, #1a1a1a);
  border: 1px solid var(--border, #2a2a2a);
  border-radius: 6px;
}
.action-text {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 2px;
}
.action-text strong {
  font-size: 13px;
  color: var(--text, #e5e5e5);
}
.hint-inline {
  font-size: 11px;
  color: var(--text2, #b0b0b0);
}

.action-btn {
  flex-shrink: 0;
  padding: 4px 10px;
  font-size: 12px;
  background: var(--bg3, #222);
  color: var(--text, #e5e5e5);
  border: 1px solid var(--border, #2a2a2a);
  border-radius: 4px;
  cursor: pointer;
}
.action-btn:hover { background: var(--bg4, #2a2a2a); }
.action-btn.danger {
  background: #d05a5a;
  border-color: #d05a5a;
  color: #fff;
}
.action-btn.danger:hover { filter: brightness(1.1); }

.result {
  margin: 10px 0 0;
  font-size: 12px;
  color: var(--text2, #b0b0b0);
}
</style>
