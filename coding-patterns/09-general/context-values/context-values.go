package main

import (
	"context"
	"fmt"
)

type contextKey string

const (
	UserIDKey    contextKey = "user_id"
	RequestIDKey contextKey = "request_id"
	TraceIDKey   contextKey = "trace_id"
)

func WithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, UserIDKey, userID)
}

func WithRequestID(ctx context.Context, reqID string) context.Context {
	return context.WithValue(ctx, RequestIDKey, reqID)
}

func UserIDFrom(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(UserIDKey).(string)
	return v, ok
}

func RequestIDFrom(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(RequestIDKey).(string)
	return v, ok
}

type LogContext struct {
	RequestID string
	UserID    string
	TraceID   string
}

func ExtractLogContext(ctx context.Context) LogContext {
	return LogContext{
		RequestID: must(RequestIDFrom(ctx)),
		UserID:    must(UserIDFrom(ctx)),
	}
}

func must(s string, ok bool) string {
	if !ok {
		return "unknown"
	}
	return s
}

func handleRequest(ctx context.Context) {
	lc := ExtractLogContext(ctx)
	fmt.Printf("[req=%s user=%s] processing\n", lc.RequestID, lc.UserID)
}

func main() {
	ctx := context.Background()
	ctx = WithRequestID(ctx, "req-abc-123")
	ctx = WithUserID(ctx, "user-42")
	handleRequest(ctx)
}
