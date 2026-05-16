package pkg

import "github.com/argues/argus/internal/biometric"

// Biometric unlock — Wails bindings that gate access to the
// Keychain-stored session JWT behind a Touch ID prompt. The session
// token itself is NEVER returned through these methods; the frontend
// reads the token via the existing GetSessionToken binding only AFTER
// AuthenticateWithBiometrics returns success. That split keeps the
// "tap a finger" UX without giving the JS side a way to extract the
// token without prompting the user.
//
// Neither method is in httpExposedMethods. Biometric prompts are
// inherently bound to the physical operator at this machine, so
// exposing them over HTTP would be both incoherent (no Touch ID on
// the remote browser) and a real footgun (a SaaS caller cannot
// validate user presence at all). See server.go.

// IsBiometricAvailable reports whether the host can prompt for
// Touch ID. Returns false on non-macOS, on Macs without the hardware,
// and on Macs where no fingerprint is enrolled.
func (a *App) IsBiometricAvailable() bool {
	return biometric.Available()
}

// AuthenticateWithBiometrics shows the system Touch ID prompt. Blocks
// until the user resolves the prompt. Returns nil on success, or an
// error describing the failure mode (user cancelled, retry limit, no
// biometric enrolled, etc.).
//
// Wails routes each binding call on its own goroutine, so the block
// here doesn't stall the runtime — but the FE should still treat this
// as a long-running call and show a "Touch ID requested…" hint.
func (a *App) AuthenticateWithBiometrics(reason string) error {
	if reason == "" {
		reason = "Unlock Argus"
	}
	return biometric.Authenticate(reason)
}
