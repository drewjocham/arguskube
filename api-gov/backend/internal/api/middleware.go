package api

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
)

const (
	logKeyRequestID   = "request_id"
	logKeyRemoteAddr  = "remote_addr"
	logKeyDuration    = "duration"
	logMsgRequestStart = "request started"
	logMsgRequestDone = "request completed"
)

func (a *API) WithRequestContext(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqID := middleware.GetReqID(r.Context())
		if reqID == "" {
			reqID = uuid.New().String()
		}

		ctx := context.WithValue(r.Context(), requestIDKey, reqID)

		start := time.Now()

		a.Logger.LogAttrs(ctx, slog.LevelDebug, logMsgRequestStart,
			slog.String(logKeyRequestID, reqID),
			slog.String(logKeyMethod, r.Method),
			slog.String(logKeyPath, r.URL.Path),
			slog.String(logKeyRemoteAddr, r.RemoteAddr),
		)

		wrapped := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		next.ServeHTTP(wrapped, r.WithContext(ctx))

		a.Logger.LogAttrs(ctx, slog.LevelInfo, logMsgRequestDone,
			slog.String(logKeyRequestID, reqID),
			slog.String(logKeyMethod, r.Method),
			slog.String(logKeyPath, r.URL.Path),
			slog.Int("status", wrapped.Status()),
			slog.Duration(logKeyDuration, time.Since(start)),
		)
	})
}

func (a *API) WithCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Request-ID")
		w.Header().Set("Access-Control-Max-Age", "86400")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (a *API) WithContentTypeJSON(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		next.ServeHTTP(w, r)
	})
}

func (a *API) WithBodyLimit(maxBytes int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			next.ServeHTTP(w, r)
		})
	}
}

func (a *API) WithTimeout(timeout time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), timeout)
			defer cancel()
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
