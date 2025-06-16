package registry

import (
	"time"

	"github.com/code-sigs/go-box/pkg/registry/etcd"
	"github.com/code-sigs/go-box/pkg/registry/memory"
	"github.com/code-sigs/go-box/pkg/registry/registry_interface"
	"github.com/code-sigs/go-box/pkg/registry/zk"
)

// RegistryType 定义注册中心类型
type RegistryType string

const (
	MemoryType RegistryType = "memory"
	EtcdType   RegistryType = "etcd"
	ZkType     RegistryType = "zookeeper"
)

// RegistryOption 配置参数
type RegistryOption struct {
	Type      RegistryType
	Etcd      *EtcdOption
	Zookeeper *ZkOption
}

type EtcdOption struct {
	Endpoints   []string
	DialTimeout time.Duration
}

type ZkOption struct {
	Servers  []string
	RootPath string
	Timeout  time.Duration
}

// NewRegistry 根据 opt 创建注册中心，默认 memory
func NewRegistry(opt *RegistryOption) (registry_interface.Registry, error) {
	switch {
	case opt != nil && opt.Type == EtcdType && opt.Etcd != nil:
		return etcd.NewEtcdRegistry(opt.Etcd.Endpoints, opt.Etcd.DialTimeout)
	case opt != nil && opt.Type == ZkType && opt.Zookeeper != nil:
		return zk.NewZkRegistry(opt.Zookeeper.Servers, opt.Zookeeper.RootPath, opt.Zookeeper.Timeout)
	default:
		return memory.NewMemoryRegistry(), nil
	}
}
