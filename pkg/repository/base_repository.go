package repository

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
)

// BaseRepository 定义所有仓库实现应遵循的通用接口。
type BaseRepository[T any, K comparable] interface {
	Create(ctx context.Context, entity *T) error
	GetByID(ctx context.Context, id K) (*T, error)
	Update(ctx context.Context, entity *T) error
	Delete(ctx context.Context, id K) error
	List(ctx context.Context) ([]*T, error)
	Find(ctx context.Context, filter map[string]any, sort bson.D) ([]*T, error)
	Paginate(ctx context.Context, offset int, limit int, filter map[string]any, sort bson.D) ([]*T, int64, error)
	Count(ctx context.Context, filter map[string]any) (int64, error)
	WithTransaction(ctx context.Context, fn func(txCtx context.Context) error) error
}
