package grpc

import (
	"context"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/code-sigs/go-box/internal/registry/registry"
	. "github.com/code-sigs/go-box/internal/resolver"
	"github.com/code-sigs/go-box/internal/rpc"
	"github.com/code-sigs/go-box/pkg/registry_factory"
	"google.golang.org/grpc"
	"google.golang.org/grpc/resolver"
)

type GRPC struct {
	registry registry.Registry
}

// New 创建一个新的 GRPC 实例
func New(opts ...registry_factory.RegistryOption) *GRPC {
	g := &GRPC{}
	var err error
	var opt *registry_factory.RegistryOption
	if len(opts) > 0 {
		opt = &opts[len(opts)-1]
	} else {
		opt = nil
	}
	g.registry, err = registry_factory.NewRegistry(opt)
	if err != nil {
		panic("failed to create registry: " + err.Error())
	}
	builder := &ServiceResolverBuilder{Registry: g.registry}
	resolver.Register(builder)
	return g
}

// GRPC 监听实现（支持优雅关闭）
func (g *GRPC) Listen(address string) error {
	lis, err := net.Listen("tcp", address)
	if err != nil {
		return err
	}
	server := rpc.NewGRPCServer()

	// 优雅关闭
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-quit
		server.GracefulStop()
	}()

	return server.Serve(lis)
}

// GRPC 注册及监听接口
// ListenAndServe 启动 GRPC 服务并监听指定地址
func (g *GRPC) ListenAndRegister(serviceName, registerAddress, listenAddress string) error {
	lis, err := net.Listen("tcp", listenAddress)
	if err != nil {
		return err
	}
	server := rpc.NewGRPCServer()

	// 注册到注册中心
	info := &registry.ServiceInfo{
		Name:    serviceName,
		Address: registerAddress,
		// 可扩展更多元数据
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := g.registry.Register(ctx, info); err != nil {
		return err
	}
	defer g.registry.Unregister(context.Background(), info)

	// 优雅关闭
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-quit
		server.GracefulStop()
	}()

	return server.Serve(lis)
}

// GetRPConnection 获取 GRPC 连接
func (g *GRPC) GetRPConnection(serviceName string) (*grpc.ClientConn, error) {
	return rpc.NewGRPCConn(context.Background(), serviceName, g.registry)

}
