package trace

import (
	"context"
	"fmt"
	"math/rand"
	"time"
)

const traceKey = "x-trace-id"

func GenerateTraceID() string {
	return fmt.Sprintf("%d-%06d", time.Now().UnixNano(), rand.Intn(1000000))
}

func WithNewTraceID(ctx context.Context) context.Context {
	return context.WithValue(ctx, traceKey, GenerateTraceID())
}

func GetTraceID(ctx context.Context) string {
	val := ctx.Value(traceKey)
	if val != nil {
		if id, ok := val.(string); ok {
			return id
		}
	}
	return ""
}
