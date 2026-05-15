<script setup>
import { ref, onMounted, computed } from 'vue'
import { useAuthStore } from '../../stores/auth'
import OAuthLoginButton from './OAuthLoginButton.vue'
import RevealableInput from '../common/RevealableInput.vue'

const auth = useAuthStore()

// Diagnostic: if LoginView is rendering, the auth gate decided
// !isAuthenticated. The most common silent cause is loadProviders
// failing (network, port, CORS) so authDisabled never flips true even
// when the backend has dev-mode enabled. Surfacing the state inline
// lets an operator running `make no-auth-run` see at a glance what
// happened — without needing the webview's dev tools, which some
// build configurations hide.
const debugInfo = computed(() => {
  return {
    authDisabled: auth.authDisabled,
    hasToken: !!auth.token,
    hasUser: !!auth.user,
    isAuthenticated: auth.isAuthenticated,
    providers: (auth.providers || []).map((p) => p.name).join(','),
  }
})

const mode = ref('login') // 'login' | 'signup'
const email = ref('')
const name = ref('')
const password = ref('')
const passwordConfirm = ref('')
const errorMsg = ref('')
const busy = ref(false)

onMounted(async () => {
  await auth.loadProviders()
  // If a token survived a reload, validate it before rendering. The
  // App.vue gate already checks isAuthenticated, but this catches the
  // "DB wiped, token now bogus" case.
  if (auth.token) {
    await auth.restoreSession()
  }
})

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

// OAuth flow is handled inside OAuthLoginButton; we only surface its
// outcome here so the LoginView's own error banner stays in sync.
function onOAuthLogin(_user) {
  errorMsg.value = ''
}
function onOAuthError(msg) {
  errorMsg.value = msg || 'OAuth sign-in failed'
}
</script>

<template>
  <div class="login-shell">
    <div class="login-card">
      <!-- Diagnostic banner: only useful while make no-auth-run isn't
           bypassing the gate. Remove (or hide) once the root cause is
           identified. -->
      <div class="diag-banner" data-testid="login-diag">
        <div>
          <strong>auth state:</strong>
          authDisabled={{ String(debugInfo.authDisabled) }} ·
          hasToken={{ String(debugInfo.hasToken) }} ·
          hasUser={{ String(debugInfo.hasUser) }} ·
          isAuthenticated={{ String(debugInfo.isAuthenticated) }} ·
          providers=[{{ debugInfo.providers || 'none' }}]
        </div>
        <div v-if="auth.providersError" class="diag-err">
          <strong>/auth/providers error:</strong> {{ auth.providersError }}
        </div>
      </div>
      <header class="brand">
        <div class="logo">⌬</div>
        <h1>Argus</h1>
        <p class="subtitle">SRE Console for Kubernetes</p>
      </header>

      <template>
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
              autocomplete="email"
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

        <!-- Single unified OAuth button. Picks one provider directly when
             only one is configured, otherwise opens a menu. We forward the
             same in-flight state to the rest of the form so manual
             email/password login is disabled while OAuth is mid-flight. -->
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
.diag-banner {
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 11px; line-height: 1.4;
  background: rgba(251, 191, 36, 0.12); color: #fde68a;
  border: 1px solid #f59e0b; border-radius: 4px;
  padding: 0.5rem 0.75rem; margin: 0 0 1rem;
  word-break: break-all;
}
.diag-err {
  margin-top: 0.5rem; padding-top: 0.4rem;
  border-top: 1px dashed rgba(251, 191, 36, 0.5);
  color: #fca5a5;
}
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
.oauth-buttons { display: flex; flex-direction: column; gap: .55rem; }
.oauth-btn {
  display: flex;
  align-items: center;
  gap: .65rem;
  background: var(--bg3);
  border: 1px solid var(--border2);
  color: var(--text);
  padding: .55rem .8rem;
  border-radius: var(--r2);
  font: inherit;
  cursor: pointer;
  transition: background .15s var(--ease);
}
.oauth-btn:hover:not(:disabled) { background: var(--bg4); }
.oauth-btn:disabled { opacity: .5; cursor: not-allowed; }
.provider-mark {
  width: 1.4rem; height: 1.4rem;
  display: inline-flex; align-items: center; justify-content: center;
  background: var(--bg);
  border-radius: 50%;
  font-weight: 600; font-size: .8rem;
}
.oauth-google .provider-mark { color: #4285F4; }
.oauth-oidc .provider-mark { color: var(--purple); }

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

.link {
  background: transparent; border: 0; color: var(--text3);
  font: inherit; font-size: .85rem; cursor: pointer; text-decoration: underline;
}
.link:hover { color: var(--text); }
</style>
