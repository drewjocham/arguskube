package apperrors_test

import (
	"errors"
	"testing"

	"github.com/argues/kube-watcher/internal/apperrors"
)

func TestNewError(t *testing.T) {
	err := apperrors.NewError("TEST-001", "something went wrong", nil)
	if err == nil {
		t.Fatal("NewError() returned nil")
	}
}

func TestErrorCode(t *testing.T) {
	err := apperrors.NewError("TEST-001", "something went wrong", nil)
	var ae *apperrors.Error
	if !errors.As(err, &ae) {
		t.Fatal("expected *apperrors.Error type")
	}
	if ae.Code != "TEST-001" {
		t.Errorf("expected Code 'TEST-001', got %q", ae.Code)
	}
}

func TestErrorMessage(t *testing.T) {
	err := apperrors.NewError("TEST-001", "something went wrong", nil)
	var ae *apperrors.Error
	if !errors.As(err, &ae) {
		t.Fatal("expected *apperrors.Error type")
	}
	if ae.Message != "something went wrong" {
		t.Errorf("expected Message 'something went wrong', got %q", ae.Message)
	}
}

func TestErrorWrapping(t *testing.T) {
	inner := errors.New("inner error")
	err := apperrors.NewError("TEST-002", "outer error", inner)

	var ae *apperrors.Error
	if !errors.As(err, &ae) {
		t.Fatal("expected *apperrors.Error type")
	}
	if ae.Code != "TEST-002" {
		t.Errorf("expected Code 'TEST-002', got %q", ae.Code)
	}

	// Verify unwrapping works.
	if !errors.Is(err, inner) {
		t.Error("expected errors.Is to match inner error")
	}
}

func TestErrorStringContainsMessage(t *testing.T) {
	err := apperrors.NewError("ERR-001", "disk full", nil)
	errStr := err.Error()
	if len(errStr) == 0 {
		t.Fatal("expected non-empty error string")
	}
	if !contains(errStr, "disk full") {
		t.Errorf("expected error string to contain 'disk full', got %q", errStr)
	}
}

func TestErrorStringContainsCode(t *testing.T) {
	err := apperrors.NewError("ERR-001", "disk full", nil)
	errStr := err.Error()
	if !contains(errStr, "ERR-001") {
		t.Errorf("expected error string to contain 'ERR-001', got %q", errStr)
	}
}

func TestErrorNestedWrapping(t *testing.T) {
	innermost := errors.New("root cause")
	middle := apperrors.NewError("MID-001", "middle error", innermost)
	outer := apperrors.NewError("OUT-001", "outer error", middle)

	// Should be able to reach root cause through errors.Is.
	if !errors.Is(outer, innermost) {
		t.Error("expected errors.Is to reach innermost error")
	}

	// Should match middle error via errors.As.
	var ae *apperrors.Error
	if !errors.As(outer, &ae) {
		t.Fatal("expected *apperrors.Error type")
	}
}

func TestNewErrorWithNilMessage(t *testing.T) {
	err := apperrors.NewError("NIL-001", "", nil)
	if err == nil {
		t.Fatal("expected non-nil error even with empty message")
	}
}

func TestSentinelErrors(t *testing.T) {
	// Verify exported sentinel error variables exist and are non-nil.
	sentinels := []struct {
		name string
		err  error
	}{
		{"ErrNotFound", apperrors.ErrNotFound},
		{"ErrInvalidInput", apperrors.ErrInvalidInput},
		{"ErrUnauthorized", apperrors.ErrUnauthorized},
		{"ErrInternal", apperrors.ErrInternal},
		{"ErrTimeout", apperrors.ErrTimeout},
		{"ErrConflict", apperrors.ErrConflict},
	}

	for _, s := range sentinels {
		t.Run(s.name, func(t *testing.T) {
			if s.err == nil {
				t.Errorf("%s is nil", s.name)
			}
		})
	}
}

func TestSentinelErrorMatching(t *testing.T) {
	// Verify sentinel errors can be matched with errors.Is.
	err := apperrors.NewError("404", "not found", apperrors.ErrNotFound)
	if !errors.Is(err, apperrors.ErrNotFound) {
		t.Error("expected errors.Is to match ErrNotFound")
	}
}

func TestSentinelErrorWrapping(t *testing.T) {
	cases := []struct {
		name       string
		sentinel   error
		code       string
		message    string
	}{
		{"not found", apperrors.ErrNotFound, "NOT_FOUND", "resource not found"},
		{"invalid input", apperrors.ErrInvalidInput, "INVALID_INPUT", "bad request"},
		{"unauthorized", apperrors.ErrUnauthorized, "UNAUTHORIZED", "access denied"},
		{"internal", apperrors.ErrInternal, "INTERNAL", "internal server error"},
		{"timeout", apperrors.ErrTimeout, "TIMEOUT", "request timed out"},
		{"conflict", apperrors.ErrConflict, "CONFLICT", "resource conflict"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := apperrors.NewError(tc.code, tc.message, tc.sentinel)
			if !errors.Is(err, tc.sentinel) {
				t.Errorf("expected errors.Is to match %s", tc.name)
			}
		})
	}
}

func TestErrorAs(t *testing.T) {
	err := apperrors.NewError("ERR-001", "test error", nil)

	var ae *apperrors.Error
	if !errors.As(err, &ae) {
		t.Fatal("expected *apperrors.Error type")
	}
}

func TestErrorFormat(t *testing.T) {
	err := apperrors.NewError("FMT-001", "formatted error", nil)

	// Verify the Error() interface method works.
	msg := err.Error()
	if len(msg) == 0 {
		t.Fatal("expected non-empty Error() string")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && containsStr(s, substr)
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
