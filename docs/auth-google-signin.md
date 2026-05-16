# Google / OIDC Sign-In via the system browser

Argus uses the user's **system browser** (Safari, Chrome, Firefox — whatever
the OS reports as default) for the Google and generic-OIDC OAuth flows, not
an in-webview popup. The mechanics:

## How it works (loopback callback)

1. User clicks **Continue with Google** in the Argus login screen.
2. The frontend calls `auth.startOAuth(provider)` — the backend mints a
   `state` + PKCE verifier, stashes them in the local `oauth_pending` row,
   and returns the upstream authorization URL.
3. The frontend hands the URL to `runtime.BrowserOpenURL` (Wails) — the
   OS launches the user's default browser. In `npm run dev` (no Wails
   runtime) the component falls back to `window.open(url, '_blank')`.
4. The user signs in with their already-saved Google credentials. The
   provider redirects to `http://127.0.0.1:8080/auth/google/callback?
   state=...&code=...` — this is Argus's own HTTP server, listening on
   loopback (configured via `ARGUS_AUTH_BASE_URL`, default
   `http://127.0.0.1:8080`).
5. The callback handler validates state, exchanges the code for tokens,
   persists the session, marks the `oauth_pending` row complete, and
   renders a tiny "You can close this tab and return to Argus" page.
6. Meanwhile, the Argus UI has been polling `/auth/oauth/poll?state=...`
   every 1.5s. The next poll returns `done: true` and the login resolves.

This is the same pattern `gcloud auth login`, `aws sso login`, and
`firebase login` use. It works on every desktop OS without any
system-level configuration because **the redirect target is a normal HTTP
URL on localhost** — the OS doesn't need to know anything about Argus.

## Why not a custom URL scheme (argus://)?

A future enhancement is to register `argus://oauth/callback` as a custom
URL scheme so the browser tab closes itself by handing the URL straight
to the running Argus app via macOS's `open-url` event (and Linux/Windows
equivalents). This would eliminate the "you can close this tab" page.

The macOS `CFBundleURLTypes` entry is already declared in
`kube/backend/build/darwin/Info.plist` and `Info.dev.plist` so a future
patch can wire a Wails `OnSecondInstanceLaunch` (or `Mac.OnUrlOpen` once
that lands in stable) handler without re-architecting the bundle. We
haven't shipped that path because Wails v2.12 doesn't expose a clean
URL-open hook, and the loopback pattern is good enough — the "close
this tab" page is the only friction, and most browsers auto-close
windows opened via `window.open` once a callback page calls
`window.close()`.

## Apple is a special case

Sign in with Apple uses `response_mode=form_post` — Apple POSTs the
callback form-encoded to the backend's `/auth/apple/callback`, not to a
URL the browser can redirect to. The current Apple flow stays as-is and
is **not** affected by this change. See `kube/backend/api/pkg/auth_handler.go`
(`handleAppleStart` / `handleAppleCallback`).

## Environment

| Var                    | Default                  | Purpose                                                                   |
|------------------------|--------------------------|---------------------------------------------------------------------------|
| `ARGUS_AUTH_BASE_URL`  | `http://127.0.0.1:8080`  | Public callback base. Must match the Authorized redirect URI in Google Cloud Console / OIDC client config. |
| `ARGUS_GOOGLE_CLIENT_ID`     | _(empty)_          | Google OAuth client ID. Provider only enabled when both ID + secret set.   |
| `ARGUS_GOOGLE_CLIENT_SECRET` | _(empty)_          | Google OAuth client secret.                                                |

Add `http://127.0.0.1:8080/auth/google/callback` as an **Authorized
redirect URI** in Google Cloud Console. Google explicitly allows loopback
URIs for installed-application clients — this is the OAuth 2.0 for
Native Apps (RFC 8252) recommended pattern.
