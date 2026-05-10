<script setup>
import { ref, onMounted, computed } from 'vue'
import { useAuthStore } from '../../stores/auth'

const auth = useAuthStore()

const mode = ref('login') // 'login' | 'signup'
const email = ref('')
const name = ref('')
const password = ref('')
const passwordConfirm = ref('')
const errorMsg = ref('')
const busy = ref(false)
const oauthInFlight = ref('') // provider name while a login flow is open in the browser

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

function openInBrowser(url) {
  // Wails desktop: send to the system browser so the OAuth provider
  // sees a real Chrome/Safari window (Google blocks embedded webviews).
  // Browser tab mode: open a new tab.
  if (typeof window !== 'undefined' && window.runtime?.BrowserOpenURL) {
    window.runtime.BrowserOpenURL(url)
  } else {
    window.open(url, '_blank', 'noopener,noreferrer')
  }
}

async function startOAuth(provider) {
  errorMsg.value = ''
  oauthInFlight.value = provider
  try {
    const { authUrl, state } = await auth.startOAuth(provider)
    openInBrowser(authUrl)
    // Poll the backend until the callback completes — long-poll style
    // with a soft cap of 5 minutes. The backend's pending row also
    // self-expires after 15 min as a hard upper bound.
    const deadline = Date.now() + 5 * 60 * 1000
    while (Date.now() < deadline) {
      await new Promise((r) => setTimeout(r, 1500))
      try {
        const res = await auth.pollOAuth(state)
        if (res.done) {
          oauthInFlight.value = ''
          return
        }
      } catch (err) {
        oauthInFlight.value = ''
        errorMsg.value = err.message || 'OAuth login failed'
        return
      }
    }
    oauthInFlight.value = ''
    errorMsg.value = 'OAuth sign-in timed out — please try again'
  } catch (err) {
    oauthInFlight.value = ''
    errorMsg.value = err.message || 'Could not start OAuth flow'
  }
}

function cancelOAuth() {
  oauthInFlight.value = ''
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

      <div v-if="oauthInFlight" class="oauth-pending">
        <div class="spinner" />
        <p>Finish signing in with <strong>{{ oauthInFlight }}</strong> in your browser.</p>
        <p class="hint">This page will continue automatically once the provider redirects back.</p>
        <button type="button" class="link" @click="cancelOAuth">Cancel</button>
      </div>

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
            <input
              type="password"
              v-model="password"
              :autocomplete="mode === 'login' ? 'current-password' : 'new-password'"
              required
              minlength="12"
              :placeholder="mode === 'signup' ? 'At least 12 characters' : ''"
            />
          </label>
          <label v-if="mode === 'signup'">
            <span>Confirm password</span>
            <input
              type="password"
              v-model="passwordConfirm"
              autocomplete="new-password"
              required
              minlength="12"
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

        <div v-if="auth.providers.length" class="oauth-section">
          <div class="divider"><span>or</span></div>
          <div class="oauth-buttons">
            <button
              v-for="p in auth.providers"
              :key="p.name"
              type="button"
              :class="['oauth-btn', `oauth-${p.name}`]"
              @click="startOAuth(p.name)"
              :disabled="busy"
            >
              <span class="provider-mark">{{ p.name === 'google' ? 'G' : '@' }}</span>
              Continue with {{ p.displayName }}
            </button>
          </div>
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
