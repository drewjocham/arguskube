<script setup>
import { ref, onMounted, computed, inject } from 'vue'
import { useAuthStore } from '../../stores/auth'
import OAuthLoginButton from './OAuthLoginButton.vue'
import RevealableInput from '../common/RevealableInput.vue'

const auth = useAuthStore()

// App.vue provides the live biometric-unlock state. When the Touch ID
// dialog is in flight we hide the form and show a small hint so the
// user knows where the focus is. Falls back to a stub if mounted
// standalone (e.g. in tests) so the component still renders.
const bio = inject('argus:biometric', ref({ available: false, prompting: false, err: '' }))

const mode = ref('login') // 'login' | 'signup'
const email = ref('')
const name = ref('')
const password = ref('')
const passwordConfirm = ref('')
const errorMsg = ref('')
const busy = ref(false)
// When true, the "full UI" (state B) renders even if lastUsedMethod is
// set. Lets a returning user pick a different account / provider
// without losing their affinity until they actually sign in.
const expanded = ref(false)

onMounted(async () => {
  await auth.loadProviders()
  // If a token survived a reload, validate it before rendering. The
  // App.vue gate already checks isAuthenticated, but this catches the
  // "DB wiped, token now bogus" case.
  if (auth.token) {
    await auth.restoreSession()
  }
  // If the recorded provider is no longer offered (operator disabled
  // it), don't lie to the user — fall back to state B and leave a
  // breadcrumb for support.
  if (
    auth.lastUsedMethod &&
    auth.lastUsedMethod.kind !== 'local' &&
    auth.lastUsedMethod.kind !== 'passkey' &&
    auth.lastUsedMethod.kind !== 'apple' &&
    !providerByName.value(auth.lastUsedMethod.provider)
  ) {
    console.info(
      '[auth] last-used provider no longer configured; falling back to full login UI',
      { provider: auth.lastUsedMethod.provider }
    )
  }
  // Prefill the email field for the local-account fast path so the
  // returning user only types their password.
  if (auth.lastUsedMethod?.kind === 'local' && auth.lastUsedMethod.email) {
    email.value = auth.lastUsedMethod.email
  }
  // Kick off the conditional-mediation passkey ceremony. The browser
  // will surface the passkey in the email field's autocomplete pop-up
  // when one is available; if none is, the call resolves with no
  // assertion and we just fall back to the form. We swallow errors
  // here because conditional UI is best-effort — any failure should
  // not block the login screen rendering.
  if (auth.passkeyEnabled && typeof window !== 'undefined' && window.PublicKeyCredential) {
    try {
      const supported = await window.PublicKeyCredential
        .isConditionalMediationAvailable?.()
      if (supported) {
        auth.loginWithPasskey({ mediation: 'conditional' }).catch(() => {})
      }
    } catch {
      // ignore — feature detection only
    }
  }
})

// Provider lookup by name — returns the full provider record (with
// displayName), or undefined if it's not currently offered.
const providerByName = computed(() => {
  return (n) => auth.providers.find((p) => p.name === n)
})

// Does the recorded affinity still match a currently-offered provider
// (or is it the always-available local-password path)?
const hasValidAffinity = computed(() => {
  const m = auth.lastUsedMethod
  if (!m) return false
  if (m.kind === 'local') return true
  // Passkey affinity is rendered as a state-B one-tap CTA (showPasskeyOneTap)
  // rather than the state-A return path; keep state-A scoped to local /
  // OAuth / Apple so we don't have to double-implement the button.
  if (m.kind === 'passkey') return false
  return Boolean(providerByName.value(m.provider))
})

// State A renders when we have a valid affinity AND the user hasn't
// explicitly asked for the full list.
const showOneTap = computed(() => hasValidAffinity.value && !expanded.value)

// For state A's button label, prefer the live displayName from the
// providers payload — operators can rename "oidc" → "Acme SSO" and the
// affinity record's "oidc" name shouldn't leak through.
const oneTapLabel = computed(() => {
  const m = auth.lastUsedMethod
  if (!m) return ''
  if (m.kind === 'local') return `Continue as ${m.email || 'returning user'}`
  const p = providerByName.value(m.provider)
  const display = p?.displayName || m.provider || 'provider'
  return `Continue with ${display}`
})

async function signInWithPasskey() {
  errorMsg.value = ''
  busy.value = true
  try {
    await auth.loginWithPasskey()
  } catch (err) {
    errorMsg.value = err?.message || 'Passkey sign-in failed'
  } finally {
    busy.value = false
  }
}

const showPasskeyOneTap = computed(
  () => auth.passkeyEnabled && auth.lastUsedMethod?.kind === 'passkey',
)

const canSubmit = computed(() => {
  if (busy.value) return false
  if (!email.value || !password.value) return false
  if (mode.value === 'signup' && password.value !== passwordConfirm.value) return false
  if (mode.value === 'signup' && password.value.length < 12) return false
  return true
})

async function submit() {
  errorMsg.value = ''
  if (!canSubmit.value) return
  busy.value = true
  try {
    if (mode.value === 'login') {
      await auth.login(email.value, password.value)
    } else {
      await auth.register(email.value, name.value || email.value, password.value)
    }
  } catch (err) {
    errorMsg.value = err.message || 'Sign-in failed'
  } finally {
    busy.value = false
  }
}

function expandFullUI() {
  expanded.value = true
  // Clear any prefilled email when the user explicitly chose to use a
  // different account — they probably want a blank form.
  if (auth.lastUsedMethod?.kind === 'local') {
    email.value = ''
  }
}

// OAuth flow is handled inside OAuthLoginButton; we only surface its
// outcome here so the LoginView's own error banner stays in sync.
function onOAuthLogin(_user) {
  errorMsg.value = ''
}
function onOAuthError(msg) {
  errorMsg.value = msg || 'OAuth sign-in failed'
}

// Trigger the one-tap OAuth flow. We forward to the existing
// OAuthLoginButton via a ref so we don't duplicate the polling logic.
const oneTapBtn = ref(null)
function triggerOneTapOAuth() {
  const m = auth.lastUsedMethod
  if (!m || m.kind === 'local') return
  oneTapBtn.value?.startFlow?.(m.provider)
}
</script>

<template>
  <div class="login-shell">
    <div class="login-card">
      <header class="brand">
        <div class="logo">⌬</div>
        <h1>Argus</h1>
        <p class="subtitle">SRE Console for Kubernetes</p>
      </header>

      <!-- Touch ID in flight: render a hint instead of the form so the
           user knows the system dialog is the active surface. App.vue
           takes the user to the dashboard automatically on success. -->
      <div v-if="bio.prompting" class="bio-hint" data-testid="bio-prompting">
        <div class="spinner" aria-hidden="true"></div>
        <p>Touch ID requested…</p>
        <p class="hint">Use your fingerprint to unlock Argus.</p>
      </div>

      <!-- State A: one-tap return path. The detect-&-default UX kicks
           in only when we have a valid recorded affinity AND the user
           hasn't explicitly asked to switch accounts. -->
      <section v-else-if="showOneTap" class="one-tap" data-testid="one-tap">
        <p class="welcome-back">Welcome back</p>

        <!-- Local-password fast path: email is prefilled, user only
             types their password. -->
        <form
          v-if="auth.lastUsedMethod.kind === 'local'"
          class="form one-tap-form"
          autocomplete="on"
          @submit.prevent="submit"
        >
          <label>
            <span>Email</span>
            <input
              type="email"
              v-model="email"
              autocomplete="email"
              required
              data-testid="one-tap-email"
            />
          </label>
          <label>
            <span>Password</span>
            <RevealableInput
              v-model="password"
              autocomplete="current-password"
              :required="true"
              :minlength="12"
              aria-label="Password"
            />
          </label>
          <button type="submit" class="primary big" :disabled="!canSubmit">
            <span v-if="!busy">{{ oneTapLabel }}</span>
            <span v-else>Working…</span>
          </button>
        </form>

        <!-- OAuth / Apple fast path: render a single big provider
             button. We mount OAuthLoginButton with a hidden trigger
             and drive it via ref so branding stays consistent. -->
        <div v-else class="one-tap-oauth">
          <button
            type="button"
            class="primary big one-tap-provider"
            :data-provider="auth.lastUsedMethod.provider"
            data-testid="one-tap-provider"
            @click="triggerOneTapOAuth"
          >
            <span class="provider-mark" :data-provider="auth.lastUsedMethod.provider">
              {{ (providerByName(auth.lastUsedMethod.provider)?.displayName || auth.lastUsedMethod.provider || '?').charAt(0).toUpperCase() }}
            </span>
            <span>{{ oneTapLabel }}</span>
          </button>
          <!-- Hidden delegate: reuses the existing polling logic so we
               don't duplicate startOAuth/pollOAuth here. -->
          <div class="hidden-delegate" aria-hidden="true">
            <OAuthLoginButton
              ref="oneTapBtn"
              @login="onOAuthLogin"
              @error="onOAuthError"
              @cancel="errorMsg = ''"
            />
          </div>
        </div>

        <p v-if="errorMsg" class="error">{{ errorMsg }}</p>

        <button
          type="button"
          class="link different-account"
          data-testid="expand-different-account"
          @click="expandFullUI"
        >Sign in with a different account</button>
      </section>

      <!-- State B: full login form + provider list. -->
      <template v-else>
        <div class="tab-row">
          <button
            type="button"
            :class="['tab', { active: mode === 'login' }]"
            @click="mode = 'login'"
          >Sign in</button>
          <button
            v-if="auth.allowSignup"
            type="button"
            :class="['tab', { active: mode === 'signup' }]"
            @click="mode = 'signup'"
          >Create account</button>
        </div>

        <form @submit.prevent="submit" class="form" autocomplete="on">
          <label>
            <span>Email</span>
            <input
              type="email"
              v-model="email"
              :autocomplete="auth.passkeyEnabled ? 'username webauthn' : 'email'"
              required
              :placeholder="mode === 'login' ? 'you@company.com' : 'work email'"
            />
          </label>
          <label v-if="mode === 'signup'">
            <span>Name</span>
            <input
              type="text"
              v-model="name"
              autocomplete="name"
              placeholder="Display name (optional)"
            />
          </label>
          <label>
            <span>Password</span>
            <RevealableInput
              v-model="password"
              :autocomplete="mode === 'login' ? 'current-password' : 'new-password'"
              :required="true"
              :minlength="12"
              :placeholder="mode === 'signup' ? 'At least 12 characters' : ''"
              aria-label="Password"
            />
          </label>
          <label v-if="mode === 'signup'">
            <span>Confirm password</span>
            <RevealableInput
              v-model="passwordConfirm"
              autocomplete="new-password"
              :required="true"
              :minlength="12"
              aria-label="Confirm password"
            />
            <small v-if="passwordConfirm && password !== passwordConfirm" class="warn">
              Passwords don't match
            </small>
          </label>

          <button type="submit" class="primary" :disabled="!canSubmit">
            <span v-if="!busy">{{ mode === 'login' ? 'Sign in' : 'Create account' }}</span>
            <span v-else>Working…</span>
          </button>
        </form>

        <!-- One-tap "Continue with passkey" CTA. When the user's last
             successful sign-in was a passkey we promote it above the OAuth
             button — the form below is still available via the "use a
             different method" link. -->
        <button
          v-if="showPasskeyOneTap"
          type="button"
          class="primary passkey-cta"
          data-test="passkey-one-tap"
          :disabled="busy"
          @click="signInWithPasskey"
        >
          <span class="passkey-icon" aria-hidden="true">🔑</span>
          Continue with passkey
        </button>

        <!-- Standard "Sign in with a passkey" button (only shown when the
             feature is enabled). Lives above the OAuth divider so it's
             visually the first non-password option. -->
        <button
          v-if="auth.passkeyEnabled && !showPasskeyOneTap"
          type="button"
          class="oauth-btn passkey-btn"
          data-test="passkey-button"
          :disabled="busy"
          @click="signInWithPasskey"
        >
          <span class="provider-mark" aria-hidden="true">🔑</span>
          Sign in with a passkey
        </button>

        <!-- Single unified OAuth button. Picks one provider directly when
             only one is configured, otherwise opens a menu. -->
        <div v-if="auth.providers.length" class="oauth-section">
          <div class="divider"><span>or</span></div>
          <OAuthLoginButton
            @login="onOAuthLogin"
            @error="onOAuthError"
            @cancel="errorMsg = ''"
          />
        </div>

        <p v-if="errorMsg" class="error">{{ errorMsg }}</p>

        <p v-if="!auth.allowSignup && mode === 'login'" class="footer-note">
          Self-registration is disabled. Ask your administrator for an account.
        </p>
      </template>
    </div>
  </div>
</template>

<style scoped>
.login-shell {
  min-height: 100vh;
  display: grid;
  place-items: center;
  background: radial-gradient(ellipse at top, #20242a 0%, #16181a 60%, #0e1012 100%);
  padding: 2rem;
  font-family: var(--font);
  color: var(--text);
}
.login-card {
  width: 100%;
  max-width: 28rem;
  background: var(--bg2);
  border: 1px solid var(--border);
  border-radius: var(--r);
  padding: 2rem;
  box-shadow: var(--shadow2);
}
.brand { text-align: center; margin-bottom: 1.5rem; }
.brand .logo { font-size: 2.25rem; color: var(--accent); margin-bottom: .25rem; }
.brand h1 { font-size: 1.5rem; letter-spacing: .02em; }
.brand .subtitle { font-size: .85rem; color: var(--text2); margin-top: .25rem; }

.tab-row {
  display: flex;
  gap: .25rem;
  margin-bottom: 1.25rem;
  border-bottom: 1px solid var(--border);
}
.tab {
  flex: 1;
  background: transparent;
  border: 0;
  color: var(--text2);
  padding: .6rem .25rem;
  font-size: .9rem;
  font-family: inherit;
  cursor: pointer;
  border-bottom: 2px solid transparent;
  transition: color .15s var(--ease), border-color .15s var(--ease);
}
.tab.active { color: var(--text); border-bottom-color: var(--accent); }
.tab:hover:not(.active) { color: var(--text); }

.form { display: flex; flex-direction: column; gap: .85rem; }
.form label { display: flex; flex-direction: column; gap: .3rem; }
.form label span { font-size: .8rem; color: var(--text2); }
.form input {
  background: var(--bg3);
  border: 1px solid var(--border2);
  border-radius: var(--r2);
  padding: .6rem .75rem;
  color: var(--text);
  font: inherit;
  outline: none;
  transition: border-color .15s var(--ease);
}
.form input:focus { border-color: var(--accent); }
.form .warn { color: var(--amber); font-size: .75rem; }

.primary {
  margin-top: .5rem;
  background: var(--accent);
  color: #fff;
  border: 0;
  border-radius: var(--r2);
  padding: .7rem 1rem;
  font: inherit;
  font-weight: 500;
  cursor: pointer;
  transition: filter .15s var(--ease), opacity .15s var(--ease);
}
.primary:hover:not(:disabled) { filter: brightness(1.1); }
.primary:disabled { opacity: .55; cursor: not-allowed; }
.primary.big {
  padding: .85rem 1rem;
  font-size: 1rem;
  font-weight: 600;
  width: 100%;
}

.one-tap { display: flex; flex-direction: column; gap: .9rem; }
.one-tap .welcome-back {
  font-size: .85rem;
  color: var(--text2);
  text-align: center;
  margin: 0 0 .25rem;
}
.one-tap-form { gap: .75rem; }
.one-tap-oauth { display: flex; flex-direction: column; gap: .5rem; }
.one-tap-provider {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: .65rem;
}
.one-tap-provider .provider-mark {
  width: 1.5rem; height: 1.5rem;
  display: inline-flex; align-items: center; justify-content: center;
  background: rgba(255,255,255,.18);
  border-radius: 50%;
  font-weight: 700; font-size: .85rem;
  color: #fff;
}
/* Apple branding — match the system convention: black bg, white text. */
.one-tap-provider[data-provider="apple"] {
  background: #000;
  color: #fff;
}
.one-tap-provider[data-provider="apple"]:hover:not(:disabled) {
  filter: brightness(1.2);
}
.hidden-delegate {
  position: absolute;
  width: 1px; height: 1px;
  overflow: hidden;
  clip: rect(0 0 0 0);
  pointer-events: none;
  opacity: 0;
}
.different-account {
  align-self: center;
  margin-top: .25rem;
}

.passkey-cta { margin-top: 1rem; display: flex; align-items: center; justify-content: center; gap: .5rem; }
.passkey-icon { font-size: 1.1rem; }
.passkey-btn { margin-top: 1rem; width: 100%; justify-content: center; }
.oauth-section { margin-top: 1.5rem; }
.divider {
  position: relative;
  text-align: center;
  margin: 1rem 0;
}
.divider::before {
  content: '';
  position: absolute;
  top: 50%; left: 0; right: 0; height: 1px;
  background: var(--border);
}
.divider span {
  position: relative;
  background: var(--bg2);
  padding: 0 .75rem;
  font-size: .75rem;
  color: var(--text3);
  text-transform: uppercase;
  letter-spacing: .08em;
}

.error {
  margin-top: 1rem;
  background: rgba(240,84,84,.1);
  border: 1px solid rgba(240,84,84,.3);
  color: var(--red2);
  padding: .55rem .75rem;
  border-radius: var(--r2);
  font-size: .85rem;
}
.footer-note {
  margin-top: 1rem;
  font-size: .8rem;
  color: var(--text3);
  text-align: center;
}

.oauth-pending {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: .75rem;
  padding: 1.5rem 0;
  text-align: center;
}
.oauth-pending p { color: var(--text2); font-size: .9rem; max-width: 22rem; }
.oauth-pending .hint { color: var(--text3); font-size: .8rem; }
.spinner {
  width: 28px; height: 28px;
  border: 2px solid var(--border2);
  border-top-color: var(--accent);
  border-radius: 50%;
  animation: spin 1s linear infinite;
}
@keyframes spin { to { transform: rotate(360deg); } }

.bio-hint {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: .6rem;
  padding: 1.5rem 0;
  text-align: center;
}
.bio-hint p { color: var(--text2); font-size: .9rem; margin: 0; }
.bio-hint .hint { color: var(--text3); font-size: .8rem; }

.link {
  background: transparent; border: 0; color: var(--text3);
  font: inherit; font-size: .85rem; cursor: pointer; text-decoration: underline;
}
.link:hover { color: var(--text); }
</style>
