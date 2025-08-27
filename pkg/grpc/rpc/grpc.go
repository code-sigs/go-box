package rpc

import (
	"context"

	"github.com/code-sigs/go-box/pkg/registry/registry_interface"
	"github.com/code-sigs/go-box/pkg/resolver"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// NewGRPCServer 创建带有拦截器的 gRPC 服务端
func NewGRPCServer() *grpc.Server {
	return grpc.NewServer(
		grpc.UnaryInterceptor(RPCServerInterceptor()), // 你的服务端拦截器
		grpc.MaxRecvMsgSize(1024*1024*100),            // 设置最大接收消息大小为 100MB
		grpc.MaxSendMsgSize(1024*1024*100),            // 设置最大发送消息大小为 100MB
		grpc.InitialWindowSize(1024*1024*10),          // 设置初始窗口大小为 10MB
		grpc.InitialConnWindowSize(1024*1024*10),      // 设置初始连接窗口大小为 10MB
	)
}

func NewGRPCConn(ctx context.Context, serviceName string, registry registry_interface.Registry) (*grpc.ClientConn, error) {
	client, err := grpc.NewClient(
		registry.Name()+":///"+serviceName,
		grpc.WithResolvers(resolver.NewBuilder(registry)),
		grpc.WithTransportCredentials(insecure.NewCredentials()),                            // 注意：生产环境中请使用安全连接
		grpc.WithDefaultCallOptions(grpc.MaxCallSendMsgSize(1024*1024*100)),                 // 设置最大发送消息大小为 100MB
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(1024*1024*100)),                 // 设置最大接收消息大小为 100MB
		grpc.WithUnaryInterceptor(RPCClientInterceptor([]string{"user-id", "platform-id"})), // 可以传入自定义的 header 列表
		grpc.WithDefaultServiceConfig(`{"loadBalancingPolicy":"round_robin"}`),
	)
	if err != nil {
		return nil, err
	}
	return client, nil
}

func NewGRPCConnsForAllInstances(
	ctx context.Context,
	serviceName string,
	registry registry_interface.Registry,
) ([]*grpc.ClientConn, error) {
	instances, err := registry.GetServiceInstances(ctx, serviceName)
	if err != nil {
		return nil, err
	}
	var conns []*grpc.ClientConn
	for _, inst := range instances {
		conn, err := grpc.NewClient(
			inst.Address,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithDefaultCallOptions(grpc.MaxCallSendMsgSize(1024*1024*100)),
			grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(1024*1024*100)),
			grpc.WithUnaryInterceptor(RPCClientInterceptor([]string{"userID", "platformID"})),
		)
		if err != nil {
			// 可选择跳过错误或直接返回
			continue
		}
		conns = append(conns, conn)
	}
	return conns, nil
}
