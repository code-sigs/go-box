package mongo

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// MongoRepository 是 MongoDB 实现的通用仓库结构。
type MongoRepository[T any, K comparable] struct {
	collection *mongo.Collection
	idField    string
}

// NewMongoRepository 创建新的 MongoRepository，自动推导集合名。
func NewMongoRepository[T any, K comparable](db *mongo.Database) *MongoRepository[T, K] {
	var entity T
	typeName := reflect.TypeOf(entity).String()
	typeName = strings.TrimPrefix(typeName, "*")
	collectionName := toSnakeCase(typeName)
	collection := db.Collection(collectionName)
	return &MongoRepository[T, K]{
		collection: collection,
		idField:    "_id",
	}
}

func (r *MongoRepository[T, K]) Create(ctx context.Context, entity *T) error {
	setTimestamps(entity, true)
	_, err := r.collection.InsertOne(ctx, entity)
	return err
}

func (r *MongoRepository[T, K]) GetByID(ctx context.Context, id K) (*T, error) {
	filter := bson.M{r.idField: id, "deleted_at": bson.M{"$exists": false}}
	var result T
	err := r.collection.FindOne(ctx, filter).Decode(&result)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, nil
	}
	return &result, err
}

func (r *MongoRepository[T, K]) Update(ctx context.Context, entity *T) error {
	v := reflect.ValueOf(entity).Elem()
	t := v.Type()
	var id any

	for i := range t.NumField() {
		field := t.Field(i)
		bsonTag := field.Tag.Get("bson")
		if bsonTag == r.idField || field.Name == "ID" {
			id = v.Field(i).Interface()
			break
		}
	}
	if id == nil {
		return errors.New("missing ID field")
	}
	setTimestamps(entity, false)
	filter := bson.M{r.idField: id}
	_, err := r.collection.ReplaceOne(ctx, filter, entity)
	return err
}

func (r *MongoRepository[T, K]) Delete(ctx context.Context, id K) error {
	filter := bson.M{r.idField: id}
	update := bson.M{"$set": bson.M{"deleted_at": time.Now()}}
	_, err := r.collection.UpdateOne(ctx, filter, update)
	return err
}

func (r *MongoRepository[T, K]) List(ctx context.Context) ([]*T, error) {
	cursor, err := r.collection.Find(ctx, bson.M{"deleted_at": bson.M{"$exists": false}})
	if err != nil {
		return nil, err
	}
	var results []*T
	err = cursor.All(ctx, &results)
	return results, err
}

func (r *MongoRepository[T, K]) Find(ctx context.Context, filter map[string]any, sort bson.D) ([]*T, error) {
	filter["deleted_at"] = bson.M{"$exists": false}
	opts := options.Find().SetSort(sort)
	cursor, err := r.collection.Find(ctx, bson.M(filter), opts)
	if err != nil {
		return nil, err
	}
	var results []*T
	err = cursor.All(ctx, &results)
	return results, err
}

func (r *MongoRepository[T, K]) Paginate(ctx context.Context, offset int, limit int, filter map[string]any, sort bson.D) ([]*T, int64, error) {
	filter["deleted_at"] = bson.M{"$exists": false}
	total, err := r.collection.CountDocuments(ctx, bson.M(filter))
	if err != nil {
		return nil, 0, err
	}
	opts := options.Find().SetSkip(int64(offset)).SetLimit(int64(limit)).SetSort(sort)
	cursor, err := r.collection.Find(ctx, bson.M(filter), opts)
	if err != nil {
		return nil, 0, err
	}
	var results []*T
	err = cursor.All(ctx, &results)
	return results, total, err
}

func (r *MongoRepository[T, K]) Count(ctx context.Context, filter map[string]any) (int64, error) {
	filter["deleted_at"] = bson.M{"$exists": false}
	return r.collection.CountDocuments(ctx, bson.M(filter))
}

func (r *MongoRepository[T, K]) WithTransaction(ctx context.Context, fn func(txCtx context.Context) error) error {
	sess, err := r.collection.Database().Client().StartSession()
	if err != nil {
		return err
	}
	defer sess.EndSession(ctx)
	return mongo.WithSession(ctx, sess, func(sc mongo.SessionContext) error {
		if err := sess.StartTransaction(); err != nil {
			return err
		}
		err := fn(sc)
		if err != nil {
			_ = sess.AbortTransaction(sc)
			return err
		}
		return sess.CommitTransaction(sc)
	})
}

// setTimestamps 统一设置创建时间和更新时间
func setTimestamps[T any](entity *T, isCreate bool) {
	v := reflect.ValueOf(entity).Elem()
	now := time.Now()
	for i := range v.NumField() {
		f := v.Type().Field(i)
		if f.Tag.Get("bson") == "created_at" && isCreate {
			v.Field(i).Set(reflect.ValueOf(now))
		} else if f.Tag.Get("bson") == "updated_at" {
			v.Field(i).Set(reflect.ValueOf(now))
		}
	}
}

// toSnakeCase 将结构体名称转为下划线格式
func toSnakeCase(s string) string {
	var result []rune
	for i, r := range s {
		if i > 0 && r >= 'A' && r <= 'Z' {
			result = append(result, '_')
		}
		result = append(result, r)
	}
	return strings.ToLower(string(result))
}
