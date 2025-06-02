package zk

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/code-sigs/go-box/internal/registry/registry"
	"github.com/go-zookeeper/zk"
)

type ZkRegistry struct {
	conn     *zk.Conn
	rootPath string
	mu       sync.Mutex
	cache    map[string][]*registry.ServiceInstance
	cacheMu  sync.RWMutex
}

func NewZkRegistry(servers []string, rootPath string, timeout time.Duration) (*ZkRegistry, error) {
	conn, _, err := zk.Connect(servers, timeout)
	if err != nil {
		return nil, err
	}

	reg := &ZkRegistry{
		conn:     conn,
		rootPath: rootPath,
		cache:    make(map[string][]*registry.ServiceInstance),
	}

	// 初始化根路径
	if exists, _, _ := conn.Exists(rootPath); !exists {
		_, err := conn.Create(rootPath, nil, 0, zk.WorldACL(zk.PermAll))
		if err != nil && err != zk.ErrNodeExists {
			return nil, err
		}
	}

	return reg, nil
}

func (z *ZkRegistry) Name() string {
	return "go-box-zookeeper"
}

func (z *ZkRegistry) Register(ctx context.Context, info *registry.ServiceInfo) error {
	path := fmt.Sprintf("%s/%s", z.servicePath(info.Name), info.Address)
	data := []byte(info.Address)

	exists, _, err := z.conn.Exists(path)
	if err != nil {
		return err
	}
	if exists {
		_ = z.conn.Delete(path, -1)
	}

	_, err = z.conn.Create(path, data, zk.FlagEphemeral, zk.WorldACL(zk.PermAll))
	return err
}

func (z *ZkRegistry) Unregister(ctx context.Context, info *registry.ServiceInfo) error {
	path := fmt.Sprintf("%s/%s", z.servicePath(info.Name), info.Address)
	return z.conn.Delete(path, -1)
}

func (z *ZkRegistry) Watch(ctx context.Context, serviceName string) (<-chan []*registry.ServiceInstance, error) {
	ch := make(chan []*registry.ServiceInstance)

	go func() {
		defer close(ch)
		for {
			children, _, events, err := z.conn.ChildrenW(z.servicePath(serviceName))
			if err != nil {
				time.Sleep(time.Second * 2)
				continue
			}

			instances := []*registry.ServiceInstance{}
			for _, child := range children {
				fullPath := fmt.Sprintf("%s/%s", z.servicePath(serviceName), child)
				data, _, err := z.conn.Get(fullPath)
				if err == nil {
					instances = append(instances, &registry.ServiceInstance{
						Address: string(data),
					})
				}
			}
			// 更新本地缓存
			z.cacheMu.Lock()
			z.cache[serviceName] = instances
			z.cacheMu.Unlock()

			select {
			case ch <- instances:
			case <-ctx.Done():
				return
			}

			select {
			case <-events:
				continue
			case <-ctx.Done():
				return
			}
		}
	}()

	return ch, nil
}

func (z *ZkRegistry) servicePath(service string) string {
	return fmt.Sprintf("%s/%s", z.rootPath, strings.Trim(service, "/"))
}

func (z *ZkRegistry) GetServiceInstances(ctx context.Context, serviceName string) ([]*registry.ServiceInstance, error) {
	z.cacheMu.RLock()
	defer z.cacheMu.RUnlock()
	instances := z.cache[serviceName]
	// 返回副本，防止外部修改
	result := make([]*registry.ServiceInstance, len(instances))
	copy(result, instances)
	return result, nil
}
