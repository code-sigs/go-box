package repository

import (
	"context"
)

// BaseRepository 定义所有仓库实现应遵循的通用接口。
type BaseRepository[T any, K comparable] interface {
	CreateIndex(ctx context.Context, keys map[string]int, optionsMap map[string]any) (string, error)
	Create(ctx context.Context, entity *T) error
	CreateMany(ctx context.Context, entities []*T) error
	GetByID(ctx context.Context, id K) (*T, error)
	Update(ctx context.Context, entity *T) error
	Delete(ctx context.Context, id K) error
	UpdateFields(ctx context.Context, id K, updates map[string]any) error
	DeleteMany(ctx context.Context, id []K) error
	HardDelete(ctx context.Context, id K) error
	HardDeleteMany(ctx context.Context, id []K) error
	List(ctx context.Context) ([]*T, error)
	FindOne(ctx context.Context, filter map[string]any) (*T, error)
	Find(ctx context.Context, filter map[string]any, sort map[string]int) ([]*T, error)
	Paginate(ctx context.Context, offset int, limit int, filter map[string]any, sort map[string]int) ([]*T, int64, error)
	Count(ctx context.Context, filter map[string]any) (int64, error)
	WithTransaction(ctx context.Context, fn func(txCtx context.Context) error) error
}
