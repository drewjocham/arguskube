//go:build darwin

package biometric

import "testing"

// We can't simulate Touch ID hardware in CI — the LAContext call
// returns whatever the host machine reports. The only thing worth
// asserting is that the call path doesn't panic, doesn't deadlock,
// and the returned bool is a real bool. The value is logged for the
// developer running the test locally.
func TestAvailable_NoPanic(t *testing.T) {
	got := Available()
	t.Logf("biometric.Available() on this host = %v", got)
}
