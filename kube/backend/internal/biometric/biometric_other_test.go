//go:build !darwin

package biometric

import (
	"strings"
	"testing"
)

func TestAvailable_FalseOffDarwin(t *testing.T) {
	if Available() {
		t.Fatal("Available must be false on non-darwin builds")
	}
}

func TestAuthenticate_NotSupportedError(t *testing.T) {
	err := Authenticate("Unlock Argus")
	if err == nil {
		t.Fatal("expected an error on non-darwin builds")
	}
	if !strings.Contains(err.Error(), "not supported") {
		t.Fatalf("error %q should mention 'not supported'", err)
	}
}
