package api

import (
	"context"
)

type contextKey string

const (
	specIDKey       contextKey = "spec_id"
	endpointIDKey   contextKey = "endpoint_id"
	driftIDKey      contextKey = "drift_id"
	userIDKey       contextKey = "user_id"
	requestIDKey    contextKey = "request_id"
)

func GetSpecID(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(specIDKey).(string)
	return v, ok
}

func GetEndpointID(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(endpointIDKey).(string)
	return v, ok
}

func GetDriftID(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(driftIDKey).(string)
	return v, ok
}

func GetUserID(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(userIDKey).(string)
	return v, ok
}

func GetRequestID(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(requestIDKey).(string)
	return v, ok
}
