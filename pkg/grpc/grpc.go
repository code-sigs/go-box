package grpc

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/code-sigs/go-box/pkg/grpc/rpc"
	"github.com/code-sigs/go-box/pkg/registry/registry_interface"

	. "github.com/code-sigs/go-box/pkg/resolver"
	"google.golang.org/grpc"
	"google.golang.org/grpc/resolver"
)

type GRPC struct {
	registry registry_interface.Registry
}

// New 创建一个新的 GRPC 实例
func New(registry registry_interface.Registry) *GRPC {
	g := &GRPC{
		registry: registry,
	}
	builder := &ServiceResolverBuilder{Registry: g.registry}
	resolver.Register(builder)
	return g
}

// GRPC 监听实现（支持优雅关闭）
func (g *GRPC) Listen(address string, shutdown func()) error {
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
		if shutdown != nil {
			shutdown()
		}
		server.GracefulStop()
	}()

	return server.Serve(lis)
}

// GRPC 注册及监听接口
// ListenAndServe 启动 GRPC 服务并监听指定地址
func (g *GRPC) ListenAndRegister(serviceName, host string, port int, register func(*grpc.Server, string), shutdown func()) error {
	lis, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", port))
	if err != nil {
		return err
	}
	server := rpc.NewGRPCServer()
	add := lis.Addr().(*net.TCPAddr)
	if add.Port != port {
		port = add.Port
	}
	// 注册到注册中心
	info := &registry_interface.ServiceInfo{
		Name:    serviceName,
		Address: fmt.Sprintf("%s:%d", host, port),
		// 可扩展更多元数据
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := g.registry.Register(ctx, info); err != nil {
		return err
	}
	defer g.registry.Unregister(context.Background(), info)
	if register != nil {
		register(server, fmt.Sprintf("%s:%d", host, port))
	}
	// 优雅关闭
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-quit
		if shutdown != nil {
			shutdown()
		}
		server.GracefulStop()
	}()

	return server.Serve(lis)
}

// GetRPConnection 获取 GRPC 连接
func (g *GRPC) GetRPConnection(serviceName string) (*grpc.ClientConn, error) {
	return rpc.NewGRPCConn(context.Background(), serviceName, g.registry)

}
