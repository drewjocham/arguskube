package apperrors

import (
	"errors"
	"testing"
)

func TestMark(t *testing.T) {
	tests := []struct {
		name string
		err  error
		disp Disposition
		want func(error) bool
	}{
		{
			name: "wraps error with disposition",
			err:  ErrBadRequest,
			disp: BadRequest,
			want: func(got error) bool {
				d, ok := GetDisposition(got)
				return ok && d == BadRequest
			},
		},
		{
			name: "returns nil for nil input",
			err:  nil,
			disp: BadRequest,
			want: func(got error) bool {
				return got == nil
			},
		},
		{
			name: "supports errors.Is unwrapping",
			err:  ErrValidation,
			disp: BadRequest,
			want: func(got error) bool {
				return errors.Is(got, ErrValidation)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Mark(tt.err, tt.disp)
			if !tt.want(got) {
				t.Errorf("Mark() = %v, want disposition %v", got, tt.disp)
			}
		})
	}
}

func TestGetDisposition(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		wantDisp Disposition
		wantOK   bool
	}{
		{
			name:     "finds disposition from wrapped error",
			err:      Mark(ErrValidation, BadRequest),
			wantDisp: BadRequest,
			wantOK:   true,
		},
		{
			name:     "returns false for unwrapped error",
			err:      ErrValidation,
			wantDisp: "",
			wantOK:   false,
		},
		{
			name:     "returns false for nil",
			err:      nil,
			wantDisp: "",
			wantOK:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := GetDisposition(tt.err)
			if ok != tt.wantOK || got != tt.wantDisp {
				t.Errorf("GetDisposition() = (%v, %v), want (%v, %v)", got, ok, tt.wantDisp, tt.wantOK)
			}
		})
	}
}

func TestDisposition_Error(t *testing.T) {
	tests := []struct {
		name string
		disp Disposition
		want string
	}{
		{name: "ack", disp: Ack, want: "ACK"},
		{name: "bad request", disp: BadRequest, want: "BAD_REQUEST"},
		{name: "retry", disp: Retry, want: "RETRY"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.disp.Error(); got != tt.want {
				t.Errorf("Disposition.Error() = %q, want %q", got, tt.want)
			}
		})
	}
}
