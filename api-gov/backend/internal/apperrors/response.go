package apperrors

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
)

const (
	processMessageBody          = "failed to process request"
	logKeyDisposition           = "disposition"
	logKeyHTTPStatus            = "http_status"
	logKeyError                 = "error"
	logMsgInternalServerFailure = "internal server failure"
	logMsgRequestRejected       = "request rejected"
	logMsgRequestAcknowledged   = "request acknowledged"
	responseBodyBadRequest      = "bad request: %s"
)

var dispositionToHTTP = map[error]int{
	Ack:        http.StatusOK,
	BadRequest: http.StatusBadRequest,
	Retry:      http.StatusInternalServerError,
}

func WriteHTTPResponse(ctx context.Context, w http.ResponseWriter, logger *slog.Logger, err error) {
	if err == nil {
		w.WriteHeader(http.StatusOK)
		return
	}

	disp, ok := GetDisposition(err)
	if !ok {
		disp = Retry
	}

	status, ok := dispositionToHTTP[disp]
	if !ok {
		status = http.StatusInternalServerError
	}

	var responseBody string
	switch {
	case errors.Is(disp, Ack):
		responseBody = ""
	case errors.Is(disp, BadRequest):
		responseBody = fmt.Sprintf(responseBodyBadRequest, err.Error())
	default:
		responseBody = processMessageBody
	}

	logAttr := []any{
		slog.String(logKeyDisposition, fmt.Sprintf("%v", disp)),
		slog.Int(logKeyHTTPStatus, status),
		slog.Any(logKeyError, err),
	}

	switch {
	case status >= http.StatusInternalServerError:
		logger.ErrorContext(ctx, logMsgInternalServerFailure, logAttr...)
	case errors.Is(disp, Ack):
		logger.DebugContext(ctx, logMsgRequestAcknowledged, logAttr...)
	default:
		logger.WarnContext(ctx, logMsgRequestRejected, logAttr...)
	}

	w.WriteHeader(status)
	if responseBody != "" {
		_, _ = fmt.Fprint(w, responseBody)
	}
}
