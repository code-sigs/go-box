package registry_interface

import "context"

type Registry interface {
	Register(ctx context.Context, info *ServiceInfo) error
	Unregister(ctx context.Context, info *ServiceInfo) error
	Watch(ctx context.Context, serviceName string) (<-chan []*ServiceInstance, error)
	Name() string
	GetServiceInstances(ctx context.Context, serviceName string) ([]*ServiceInstance, error)
}

type ServiceInfo struct {
	Name     string
	Address  string
	Version  string
	Metadata map[string]string
	LeaseID  int64
}

type ServiceInstance struct {
	Address  string
	Metadata map[string]string
}
