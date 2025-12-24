package etcd

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	registry "github.com/code-sigs/go-box/pkg/registry/registry_interface"
	clientv3 "go.etcd.io/etcd/client/v3"
)

type EtcdRegistry struct {
	cli     *clientv3.Client
	cache   map[string][]*registry.ServiceInstance
	cacheMu sync.RWMutex
}

func NewEtcdRegistry(endpoints []string, dialTimeout time.Duration) (*EtcdRegistry, error) {
	cli, err := clientv3.New(clientv3.Config{
		Endpoints: endpoints,
		//DialTimeout: dialTimeout,
	})
	if err != nil {
		return nil, err
	}
	return &EtcdRegistry{
		cli:   cli,
		cache: make(map[string][]*registry.ServiceInstance),
	}, nil
}

func (e *EtcdRegistry) Register(ctx context.Context, info *registry.ServiceInfo) error {
	key := "/go-box-services/" + info.Name + "/" + info.Address

	valBytes, err := json.Marshal(info)
	if err != nil {
		return err
	}
	val := string(valBytes)

	leaseResp, err := e.cli.Grant(ctx, 600)
	if err != nil {
		return err
	}
	info.LeaseID = int64(leaseResp.ID)

	_, err = e.cli.Put(ctx, key, val, clientv3.WithLease(leaseResp.ID))
	if err != nil {
		_, _ = e.cli.Revoke(context.Background(), leaseResp.ID)
		return err
	}

	ch, kaerr := e.cli.KeepAlive(ctx, leaseResp.ID)
	if kaerr != nil {
		_, _ = e.cli.Delete(context.Background(), key)
		_, _ = e.cli.Revoke(context.Background(), leaseResp.ID)
		return kaerr
	}

	go func() {
		for {
			select {
			case _, ok := <-ch:
				if !ok {
					_, _ = e.cli.Delete(context.Background(), key)
					_, _ = e.cli.Revoke(context.Background(), leaseResp.ID)
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	return nil
}

func (e *EtcdRegistry) Unregister(ctx context.Context, info *registry.ServiceInfo) error {
	key := "/go-box-services/" + info.Name + "/" + info.Address
	_, err := e.cli.Delete(ctx, key)
	if info.LeaseID != 0 {
		_, _ = e.cli.Revoke(ctx, clientv3.LeaseID(info.LeaseID))
	}
	return err
}

func (e *EtcdRegistry) Watch(ctx context.Context, serviceName string) (<-chan []*registry.ServiceInstance, error) {
	prefix := "/go-box-services/" + serviceName + "/"
	out := make(chan []*registry.ServiceInstance, 10) // 缓冲防止阻塞

	go func() {
		defer close(out)

		loadInstances := func() ([]*registry.ServiceInstance, error) {
			resp, err := e.cli.Get(ctx, prefix, clientv3.WithPrefix())
			if err != nil {
				return nil, err
			}
			addrSet := make(map[string]struct{})
			var instances []*registry.ServiceInstance
			for _, kv := range resp.Kvs {
				var inst registry.ServiceInfo
				if err := json.Unmarshal(kv.Value, &inst); err != nil {
					continue // ignore bad data
				}
				if _, ok := addrSet[inst.Address]; ok {
					continue
				}
				addrSet[inst.Address] = struct{}{}
				instances = append(instances, &registry.ServiceInstance{
					Address: inst.Address,
					Metadata: map[string]string{
						"version": inst.Version,
					},
				})
			}
			return instances, nil
		}

		sendInstances := func(insts []*registry.ServiceInstance) {
			select {
			case out <- insts:
			case <-ctx.Done():
			default:
				// 丢弃防止阻塞
			}
		}

		instances, err := loadInstances()
		if err != nil {
			return
		}
		// 更新本地缓存
		e.cacheMu.Lock()
		e.cache[serviceName] = instances
		e.cacheMu.Unlock()
		sendInstances(instances)

		backoff := time.Second

		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			watchChan := e.cli.Watch(ctx, prefix, clientv3.WithPrefix(), clientv3.WithPrevKV())
			for watchResp := range watchChan {
				if watchResp.Err() != nil {
					break
				}
				instances, err := loadInstances()
				if err != nil {
					break
				}
				// 更新本地缓存
				e.cacheMu.Lock()
				e.cache[serviceName] = instances
				e.cacheMu.Unlock()
				sendInstances(instances)
			}

			time.Sleep(backoff)
			backoff *= 2
			if backoff > 30*time.Second {
				backoff = 30 * time.Second
			}
		}
	}()

	return out, nil
}

func (e *EtcdRegistry) Name() string {
	return "go-box-etcd"
}

// 优化后的 GetServiceInstances：直接读本地缓存
func (e *EtcdRegistry) GetServiceInstances(ctx context.Context, serviceName string) ([]*registry.ServiceInstance, error) {
	e.cacheMu.RLock()
	defer e.cacheMu.RUnlock()
	instances := e.cache[serviceName]
	// 返回副本，防止外部修改
	result := make([]*registry.ServiceInstance, len(instances))
	copy(result, instances)
	return result, nil
}
