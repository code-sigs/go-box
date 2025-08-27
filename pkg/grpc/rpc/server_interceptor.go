package rpc

import (
	"context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// RPCServerInterceptor 将 metadata 的所有键值对放入 context
func RPCServerInterceptor() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		md, ok := metadata.FromIncomingContext(ctx)
		if ok {
			for key, values := range md {
				if len(values) > 0 {
					ctx = context.WithValue(ctx, key, values[0])
				}
			}
		}
		return handler(ctx, req)
	}
}
