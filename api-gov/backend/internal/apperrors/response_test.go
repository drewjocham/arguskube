package apperrors

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWriteHTTPResponse(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	tests := []struct {
		name       string
		err        error
		wantStatus int
	}{
		{
			name:       "nil error returns 200",
			err:        nil,
			wantStatus: http.StatusOK,
		},
		{
			name:       "bad request returns 400",
			err:        Mark(ErrBadRequest, BadRequest),
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "validation error returns 400",
			err:        Mark(ErrValidation, BadRequest),
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "unknown error returns 500",
			err:        ErrUnknown,
			wantStatus: http.StatusInternalServerError,
		},
		{
			name:       "unmarked error defaults to retry (500)",
			err:        ErrSpecNotFound,
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			WriteHTTPResponse(context.Background(), w, logger, tt.err)

			if w.Code != tt.wantStatus {
				t.Errorf("WriteHTTPResponse() status = %d, want %d", w.Code, tt.wantStatus)
			}
		})
	}
}
