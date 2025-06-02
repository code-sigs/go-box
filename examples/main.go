package main

import (
	"context"
	"fmt"
	"time"

	"github.com/code-sigs/go-box/internal/registry"
	"github.com/code-sigs/go-box/internal/registry/etcd"
	. "github.com/code-sigs/go-box/internal/resolver"
	"google.golang.org/grpc/resolver"
)

func main() {
	etcdRegistry, _ := etcd.NewEtcdRegistry([]string{"localhost:2379"}, 5*time.Second)

	_ = etcdRegistry.Register(context.Background(), &registry.ServiceInfo{
		Name:    "demo",
		Address: "127.0.0.1:50051",
	})

	builder := &ServiceResolverBuilder{Registry: etcdRegistry}
	resolver.Register(builder) // ← 使用 grpc 官方 resolver.Register

	conn, err := client.NewGRPCConn(context.Background(), "demo", "etcd")
	if err != nil {
		panic(err)
	}

	fmt.Println("gRPC connected to:", conn.Target())
}
