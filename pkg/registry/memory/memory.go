package memory

import (
	"context"
	"sync"

	registry "github.com/code-sigs/go-box/pkg/registry/registry_interface"
)

type MemoryRegistry struct {
	mu       sync.RWMutex
	services map[string]map[string]*registry.ServiceInfo // serviceName -> address -> info
	watchers map[string][]chan []*registry.ServiceInstance
}

func NewMemoryRegistry() *MemoryRegistry {
	return &MemoryRegistry{
		services: make(map[string]map[string]*registry.ServiceInfo),
		watchers: make(map[string][]chan []*registry.ServiceInstance),
	}
}

func (m *MemoryRegistry) Register(ctx context.Context, info *registry.ServiceInfo) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.services[info.Name] == nil {
		m.services[info.Name] = make(map[string]*registry.ServiceInfo)
	}
	m.services[info.Name][info.Address] = info
	m.notifyWatchers(info.Name)
	return nil
}

func (m *MemoryRegistry) Unregister(ctx context.Context, info *registry.ServiceInfo) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.services[info.Name] != nil {
		delete(m.services[info.Name], info.Address)
		if len(m.services[info.Name]) == 0 {
			delete(m.services, info.Name)
		}
	}
	m.notifyWatchers(info.Name)
	return nil
}

func (m *MemoryRegistry) Watch(ctx context.Context, serviceName string) (<-chan []*registry.ServiceInstance, error) {
	ch := make(chan []*registry.ServiceInstance, 1)
	m.mu.Lock()
	m.watchers[serviceName] = append(m.watchers[serviceName], ch)
	instances := m.getInstances(serviceName)
	m.mu.Unlock()

	// 首次推送当前实例列表
	ch <- instances

	// 监听 context 关闭，自动移除 watcher
	go func() {
		<-ctx.Done()
		m.mu.Lock()
		defer m.mu.Unlock()
		watchers := m.watchers[serviceName]
		for i, w := range watchers {
			if w == ch {
				m.watchers[serviceName] = append(watchers[:i], watchers[i+1:]...)
				break
			}
		}
		close(ch)
	}()
	return ch, nil
}

func (m *MemoryRegistry) Name() string {
	return "memory"
}

func (m *MemoryRegistry) getInstances(serviceName string) []*registry.ServiceInstance {
	var instances []*registry.ServiceInstance
	for _, info := range m.services[serviceName] {
		instances = append(instances, &registry.ServiceInstance{
			Address:  info.Address,
			Metadata: info.Metadata,
		})
	}
	return instances
}

func (m *MemoryRegistry) notifyWatchers(serviceName string) {
	instances := m.getInstances(serviceName)
	for _, ch := range m.watchers[serviceName] {
		// 非阻塞推送
		select {
		case ch <- instances:
		default:
		}
	}
}

func (m *MemoryRegistry) GetServiceInstances(ctx context.Context, serviceName string) ([]*registry.ServiceInstance, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.getInstances(serviceName), nil
}
