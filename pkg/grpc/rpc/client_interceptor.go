package rpc

import (
	"context"
	"strings"

	"github.com/code-sigs/go-box/pkg/logger"
	"github.com/code-sigs/go-box/pkg/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// RPCClientInterceptor 将 http.Header 注入到 gRPC metadata
func RPCClientInterceptor(proxyHeader []string) grpc.UnaryClientInterceptor {
	return func(
		ctx context.Context,
		method string,
		req, reply interface{},
		cc *grpc.ClientConn,
		invoker grpc.UnaryInvoker,
		opts ...grpc.CallOption,
	) error {
		logger.Infof(ctx, "RPCClientInterceptor clientIP: %s", ctx.Value("clientip"))
		md := metadata.New(nil)
		md.Append("clientip", ctx.Value("clientip").(string))
		traceID := trace.GetTraceID(ctx)
		if traceID == "" {
			ctx = trace.WithNewTraceID(ctx)
		}
		if len(proxyHeader) != 0 {
			for _, key := range proxyHeader {
				ctxValue := ctx.Value(strings.ToLower(key))
				if ctxValue != nil {
					if value, ok := ctxValue.(string); ok && value != "" {
						md.Append(strings.ToLower(key), value)
					}
				}
			}
		}
		if len(md) > 0 {
			ctx = metadata.NewOutgoingContext(ctx, md)
		}
		return invoker(ctx, method, req, reply, cc, opts...)
	}
}
