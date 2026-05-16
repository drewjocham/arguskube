# Passkeys (WebAuthn) — operator guide

Passkeys are an optional sign-in method for Argus. When enabled, users
can sign in with Touch ID / Face ID / Windows Hello or a hardware
security key, with no password to phish.

## Enabling passkeys

Set these environment variables before starting Argus:

| Variable                  | Required | Default                  | What it is                                                          |
| ------------------------- | -------- | ------------------------ | ------------------------------------------------------------------- |
| `ARGUS_PASSKEY_ENABLED`   | yes      | `false`                  | Master switch. `true`, `1`, `yes`, `on` all enable.                 |
| `ARGUS_PASSKEY_RP_ID`     | yes      | `localhost`              | The Relying Party identifier. Must match the **effective domain**.  |
| `ARGUS_PASSKEY_RP_NAME`   | no       | `Argus`                  | Display name shown in the authenticator's prompt.                   |
| `ARGUS_PASSKEY_RP_ORIGIN` | yes      | `http://localhost:8080`  | Fully-qualified origin including scheme + port.                     |

After starting, `GET /auth/providers` will return `passkeyEnabled: true`
and the LoginView will surface the "Sign in with a passkey" button.

## The RP_ID gotcha

`ARGUS_PASSKEY_RP_ID` **must** be an exact eTLD+1 (or a parent domain)
of the host in `ARGUS_PASSKEY_RP_ORIGIN`. Some valid pairings:

| RP_ID                 | RP_ORIGIN                          | Valid? |
| --------------------- | ---------------------------------- | ------ |
| `localhost`           | `http://localhost:8080`            | yes    |
| `argus.example.com`   | `https://argus.example.com`        | yes    |
| `example.com`         | `https://argus.example.com`        | yes (parent) |
| `argus.example.com`   | `https://other.example.com`        | no     |
| `localhost:8080`      | `http://localhost:8080`            | no (port in RP_ID) |
| `https://localhost`   | `http://localhost:8080`            | no (scheme in RP_ID) |

If you get `InvalidStateError` or `SecurityError` in the browser
console during registration, the RP_ID is almost always the culprit.

## Dev-mode notes (`localhost`)

WebAuthn works on `http://localhost` without TLS — the browser treats
loopback as secure. On any other hostname you need real HTTPS; an
ngrok tunnel works fine for local QA. Self-signed certs will fail the
ceremony silently (Safari especially is strict).

## Registering a passkey

Once signed in (with any method):

1. Click the user avatar in the top-right.
2. Choose **Manage passkeys** from the menu.
3. Click **Add passkey**. The browser will prompt for biometric / PIN.
4. Optionally name the credential ("MacBook Touch ID", "YubiKey 5C").

To sign in:

1. On the login screen, click **Sign in with a passkey**.
2. Pick the credential in the browser's passkey picker.
3. Approve with biometric / PIN.

When the browser supports **conditional UI** (Safari 16+, Chrome 108+,
Firefox 119+) Argus also triggers an unattended ceremony on mount,
so the passkey appears in the email field's autofill drop-down.

## Browser support

| Browser       | Registration | Discoverable login | Conditional UI |
| ------------- | ------------ | ------------------ | -------------- |
| Safari 16+    | yes          | yes                | yes            |
| Chrome 108+   | yes          | yes                | yes            |
| Firefox 119+  | yes          | yes                | yes            |
| Edge 108+     | yes          | yes                | yes            |

Older browsers fall through gracefully — the button is shown but a
click results in an inline error rather than a broken modal.

## Data and revocation

Each passkey lives in the `passkey_credentials` table in
`~/.argus/argus.db`:

- One row per credential (a user may have several).
- `credential_id` is the raw bytes returned by the authenticator.
- `public_key` is the EC/RSA public key — no shared secret.
- `sign_count` is updated on every successful login; a regression
  here (lower than what we have stored) means a cloned authenticator,
  and the library will refuse the assertion.

Users revoke their own passkeys via the management UI. Admins can
also delete rows directly from sqlite if needed.

## Endpoints

All four ceremonies are exposed under `/auth/passkey/*`:

| Method | Path                              | Auth     | What it does                                |
| ------ | --------------------------------- | -------- | ------------------------------------------- |
| POST   | `/auth/passkey/register/begin`    | required | Returns `CredentialCreationOptions` + state |
| POST   | `/auth/passkey/register/finish`   | required | Verifies attestation, stores credential     |
| POST   | `/auth/passkey/login/begin`       | public   | Returns `CredentialRequestOptions` + state  |
| POST   | `/auth/passkey/login/finish`      | public   | Verifies assertion, issues session token    |
| GET    | `/auth/passkey/list`              | required | Lists the caller's credentials              |
| DELETE | `/auth/passkey/{id}`              | required | Revokes one credential by row id            |

State tokens are single-use and expire after 5 minutes. Expired rows
are swept by the existing auth janitor goroutine.
