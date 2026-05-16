# Sign in with Apple — Operator Setup

Argus supports Apple as a first-class auth provider alongside Google and
generic OIDC. Apple's flow is a few non-standard ways different from
every other OIDC provider, which is why setup needs more care.

## Prerequisites

- An Apple Developer Program enrollment ($99/yr) — Sign in with Apple
  requires it; the free Apple ID is not enough.
- The Argus backend must be reachable from the public internet over
  HTTPS. Apple rejects `http://` and `localhost` return URLs.

## Apple Developer portal steps

1. **App ID** — go to *Certificates, Identifiers & Profiles → Identifiers
   → "+"* → *App IDs*. Create one (or reuse). Under capabilities, tick
   **Sign in with Apple**.
2. **Services ID** — back in *Identifiers → "+"*, pick *Services IDs*.
   This becomes the OAuth client_id (e.g. `com.argus.signin`). Enable
   *Sign in with Apple*, click *Configure*, set:
   - **Primary App ID**: the one from step 1.
   - **Return URLs**: `<PublicBaseURL>/auth/apple/callback`. Apple is
     strict about exact-match — include the scheme, host, and full path.
3. **Key** — *Keys → "+"*, name it (e.g. "Argus Sign-in"), enable
   *Sign in with Apple*, click *Configure* and pick the Primary App ID.
   *Continue → Register → Download*. You get a one-time `.p8` file.
   The portal also displays a **Key ID** (10 chars) — copy it.
4. **Team ID** — top-right of the Developer portal, under your account
   name. 10 chars.

## Argus env vars

```
ARGUS_APPLE_SERVICES_ID=com.argus.signin
ARGUS_APPLE_TEAM_ID=ABC1234567
ARGUS_APPLE_KEY_ID=DEF1234567
ARGUS_APPLE_PRIVATE_KEY_FILE=/etc/argus/AuthKey_DEF1234567.p8
# Or inline:
# ARGUS_APPLE_PRIVATE_KEY="-----BEGIN PRIVATE KEY-----\n…\n-----END PRIVATE KEY-----\n"
ARGUS_APPLE_DISPLAY_NAME=Apple   # optional; shown on the login button
ARGUS_AUTH_BASE_URL=https://argus.example.com   # must match the Services-ID return URL
```

The backend logs `Sign in with Apple registered` at boot when all four
required fields parse. If you see no log line, double-check the four env
vars are non-empty and the `.p8` file is readable.

## Local development

Apple requires HTTPS + a public host for the return URL, so the usual
`http://127.0.0.1:8080` won't work. Two practical options:

1. **ngrok / cloudflared tunnel** — point a temporary public HTTPS host
   at your local backend, register that URL in the Services ID, then
   set `ARGUS_AUTH_BASE_URL` to the tunnel URL.
2. **Test in a deployed environment only** — wire Apple in staging and
   leave it off locally.

## Behavior notes

- The user's display name and email are returned **only on the first
  authorization** for a given Apple ID. Argus persists them server-side
  under the `(provider, provider_subject)` key. Subsequent sign-ins
  re-use the stored row.
- The Apple `email` claim may be a private-relay address
  (`*.privaterelay.appleid.com`). That's expected and forwards to the
  user's real address.
- The client_secret JWT is rotated automatically every 50 minutes.
  Apple permits up to 6 months; the shorter window keeps a leaked
  secret useless quickly.
