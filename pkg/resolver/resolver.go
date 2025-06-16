// File: resolver/etcd_resolver.go
package resolver

import (
	"context"
	"fmt"
	"sync"

	"github.com/code-sigs/go-box/pkg/registry/registry_interface"
	"google.golang.org/grpc/resolver"
)

type serviceResolver struct {
	cc       resolver.ClientConn
	registry registry_interface.Registry
	ctx      context.Context
	cancel   context.CancelFunc
	ch       <-chan []*registry_interface.ServiceInstance
	sync.Mutex
	addrs []resolver.Address
}

type ServiceResolverBuilder struct {
	Registry registry_interface.Registry
}

func NewBuilder(reg registry_interface.Registry) resolver.Builder {
	return &ServiceResolverBuilder{Registry: reg}
}

func (b *ServiceResolverBuilder) Build(target resolver.Target, cc resolver.ClientConn, opts resolver.BuildOptions) (resolver.Resolver, error) {
	serviceName := target.Endpoint() // for target like go-box:///user-service

	ctx, cancel := context.WithCancel(context.Background())
	ch, err := b.Registry.Watch(ctx, serviceName)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to watch service [%s]: %w", serviceName, err)
	}

	r := &serviceResolver{
		cc:       cc,
		registry: b.Registry,
		ctx:      ctx,
		cancel:   cancel,
		ch:       ch,
	}
	go r.watch()
	return r, nil
}

func (b *ServiceResolverBuilder) Scheme() string {
	return b.Registry.Name()
}

func (r *serviceResolver) watch() {
	for {
		select {
		case <-r.ctx.Done():
			return
		case instances, ok := <-r.ch:
			if !ok {
				return
			}

			var addrs []resolver.Address
			for _, ins := range instances {
				addrs = append(addrs, resolver.Address{Addr: ins.Address})
			}

			r.Lock()
			r.addrs = addrs
			r.Unlock()

			r.cc.UpdateState(resolver.State{Addresses: addrs})
		}
	}
}

func (r *serviceResolver) ResolveNow(resolver.ResolveNowOptions) {
	// no-op: we use long polling Watch mechanism
}

func (r *serviceResolver) Close() {
	r.cancel()
}
