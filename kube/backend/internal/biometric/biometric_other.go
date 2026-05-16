//go:build !darwin

// Stub implementation for non-Darwin platforms. Touch ID is macOS-only;
// on Linux and Windows builds the feature is simply absent and the FE
// falls through to the regular login flow.
package biometric

import "errors"

// Available always returns false off macOS — there's no Touch ID
// hardware path on other platforms.
func Available() bool { return false }

// Authenticate returns a clear error so any caller that ignores
// Available() still gets a sensible failure mode rather than a panic.
func Authenticate(reason string) error {
	_ = reason
	return errors.New("biometric: not supported on this platform")
}
