package apperrors

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
)

type Error string

func (e Error) Error() string { return string(e) }

const (
	ErrClusterUnreachable Error = "cluster unreachable"
	ErrAlertNotFound      Error = "alert not found"
	ErrContextAssembly    Error = "context assembly failed"
	ErrAnomstackTimeout   Error = "anomstack request timed out"
	ErrAnomstackFailed    Error = "anomstack request failed"
	ErrFlinkTimeout       Error = "flink request timed out"
	ErrFlinkFailed        Error = "flink request failed"
	ErrFeatureGated       Error = "feature requires pro tier"
	ErrDecisionLogParse   Error = "decision log parse error"
	ErrDiagnosisFailed    Error = "diagnosis generation failed"
	ErrUnknown            Error = "unknown error"
)

type Disposition string

const (
	OK         Disposition = "OK"
	BadRequest Disposition = "BAD_REQUEST"
	NotFound   Disposition = "NOT_FOUND"
	Forbidden  Disposition = "FORBIDDEN"
	Retry      Disposition = "RETRY"
)

type dispositionError struct {
	err  error
	disp Disposition
}

func (d dispositionError) Error() string { return d.err.Error() }
func (d dispositionError) Unwrap() error { return d.err }

func Mark(err error, disp Disposition) error {
	return dispositionError{err: err, disp: disp}
}

func GetDisposition(err error) Disposition {
	if err == nil {
		return OK
	}
	if de, ok := err.(dispositionError); ok {
		return de.disp
	}
	return Retry
}

var dispositionToHTTP = map[Disposition]int{
	OK:         http.StatusOK,
	BadRequest: http.StatusBadRequest,
	NotFound:   http.StatusNotFound,
	Forbidden:  http.StatusForbidden,
	Retry:      http.StatusInternalServerError,
}

// WriteHTTPResponse maps a (possibly nil) error to the correct HTTP status and logs it.
func WriteHTTPResponse(ctx context.Context, w http.ResponseWriter, logger *slog.Logger, err error) {
	if err == nil {
		w.WriteHeader(http.StatusOK)
		return
	}

	disp := GetDisposition(err)
	status := dispositionToHTTP[disp]
	if status == 0 {
		status = http.StatusInternalServerError
	}

	switch disp {
	case BadRequest, NotFound:
		logger.WarnContext(ctx, "request error",
			slog.String("disposition", string(disp)),
			slog.String(logKeyError, err.Error()),
		)
	case Forbidden:
		logger.WarnContext(ctx, "access denied",
			slog.String("disposition", string(disp)),
			slog.String(logKeyError, err.Error()),
		)
	default:
		logger.ErrorContext(ctx, "internal error",
			slog.String("disposition", string(disp)),
			slog.String(logKeyError, err.Error()),
		)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error":       err.Error(),
		"disposition": string(disp),
	})
}

const logKeyError = "error"
