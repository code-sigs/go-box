package rpc

import (
	"context"
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
		//logger.Infof(ctx, "RPCClientInterceptor clientIP: %s", ctx.Value("clientip"))
		md := metadata.New(nil)
		client := ctx.Value("clientip")
		if client != nil {
			md.Append("clientip", client.(string))
		}
		traceID := trace.GetTraceID(ctx)
		if traceID == "" {
			ctx = trace.WithNewTraceID(ctx)
		}
		if len(proxyHeader) != 0 {
			for _, key := range proxyHeader {
				ctxValue := ctx.Value(key)
				if ctxValue != nil {
					if value, ok := ctxValue.(string); ok && value != "" {
						md.Append(key, value)
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
