package apperrors_test

import (
	"errors"
	"testing"

	"github.com/argues/kube-watcher/internal/apperrors"
)

func TestErrorConstants(t *testing.T) {
	tests := []struct {
		name string
		err  apperrors.Error
		want string
	}{
		{"ErrClusterUnreachable", apperrors.ErrClusterUnreachable, "cluster unreachable"},
		{"ErrAlertNotFound", apperrors.ErrAlertNotFound, "alert not found"},
		{"ErrContextAssembly", apperrors.ErrContextAssembly, "context assembly failed"},
		{"ErrAnomstackTimeout", apperrors.ErrAnomstackTimeout, "anomstack request timed out"},
		{"ErrAnomstackFailed", apperrors.ErrAnomstackFailed, "anomstack request failed"},
		{"ErrFeatureGated", apperrors.ErrFeatureGated, "feature requires pro tier"},
		{"ErrDecisionLogParse", apperrors.ErrDecisionLogParse, "decision log parse error"},
		{"ErrDiagnosisFailed", apperrors.ErrDiagnosisFailed, "diagnosis generation failed"},
		{"ErrUnknown", apperrors.ErrUnknown, "unknown error"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.want {
				t.Errorf("Error() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestMarkAndGetDisposition(t *testing.T) {
	base := errors.New("something broke")
	marked := apperrors.Mark(base, apperrors.BadRequest)
	if marked == nil {
		t.Fatal("Mark returned nil")
	}
	if !errors.Is(marked, base) {
		t.Error("Marked error should wrap the original")
	}
	if d := apperrors.GetDisposition(marked); d != apperrors.BadRequest {
		t.Errorf("GetDisposition = %q, want %q", d, apperrors.BadRequest)
	}
}

func TestGetDisposition(t *testing.T) {
	if d := apperrors.GetDisposition(nil); d != apperrors.OK {
		t.Errorf("GetDisposition(nil) = %q, want %q", d, apperrors.OK)
	}
	if d := apperrors.GetDisposition(errors.New("plain")); d != apperrors.Retry {
		t.Errorf("GetDisposition(plain) = %q, want %q", d, apperrors.Retry)
	}
}

func TestDispositionConstants(t *testing.T) {
	tests := []struct {
		disp apperrors.Disposition
		want string
	}{
		{apperrors.OK, "OK"},
		{apperrors.BadRequest, "BAD_REQUEST"},
		{apperrors.NotFound, "NOT_FOUND"},
		{apperrors.Forbidden, "FORBIDDEN"},
		{apperrors.Retry, "RETRY"},
	}
	for _, tt := range tests {
		t.Run(string(tt.disp), func(t *testing.T) {
			if got := string(tt.disp); got != tt.want {
				t.Errorf("Disposition = %q, want %q", got, tt.want)
			}
		})
	}
}
