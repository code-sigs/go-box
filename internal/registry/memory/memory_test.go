package memory

import (
	"context"
	"testing"
	"time"

	"github.com/code-sigs/go-box/internal/registry/registry"
	"github.com/stretchr/testify/assert"
)

func TestMemoryRegistry_Register_Unregister_Watch(t *testing.T) {
	reg := NewMemoryRegistry()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	info := &registry.ServiceInfo{
		Name:    "test-service",
		Address: "127.0.0.1:8080",
		Metadata: map[string]string{
			"version": "v1",
		},
	}

	// Watch
	watchCh, err := reg.Watch(ctx, "test-service")
	assert.NoError(t, err)
	assert.NotNil(t, watchCh)

	// 消费首次推送的空实例列表
	select {
	case instances := <-watchCh:
		assert.Len(t, instances, 0)
	case <-time.After(time.Second):
		t.Fatal("watch did not receive initial event")
	}

	// 注册服务
	err = reg.Register(ctx, info)
	assert.NoError(t, err)

	// 检查 Watch 能收到注册事件
	select {
	case instances := <-watchCh:
		assert.Len(t, instances, 1)
		assert.Equal(t, "127.0.0.1:8080", instances[0].Address)
		assert.Equal(t, "v1", instances[0].Metadata["version"])
	case <-time.After(time.Second):
		t.Fatal("watch did not receive register event")
	}

	// 注销服务
	err = reg.Unregister(ctx, info)
	assert.NoError(t, err)

	// 检查 Watch 能收到注销事件
	select {
	case instances := <-watchCh:
		assert.Len(t, instances, 0)
	case <-time.After(time.Second):
		t.Fatal("watch did not receive unregister event")
	}
}

func TestMemoryRegistry_Name(t *testing.T) {
	reg := NewMemoryRegistry()
	assert.Equal(t, "memory", reg.Name())
}
