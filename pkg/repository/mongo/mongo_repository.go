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
	t := reflect.TypeOf(entity)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	collectionName := toSnakeCase(t.Name())
	collection := db.Collection(collectionName)
	return &MongoRepository[T, K]{
		collection: collection,
		idField:    "_id",
	}
}

// CreateIndexGeneric 创建 MongoDB 索引
// 字段与排序方式 {"email": 1, "createdAt": -1}
// 索引选项 {"unique": true, "background": true}
func (r *MongoRepository[T, K]) CreateIndex(ctx context.Context, keys map[string]int, optionsMap map[string]any) (string, error) {
	// 构建索引字段 bson.D
	var indexKeys bson.D
	for key, order := range keys {
		indexKeys = append(indexKeys, bson.E{Key: key, Value: order})
	}

	// 构建索引选项 bson.M
	var indexOpts *options.IndexOptions
	if len(optionsMap) > 0 {
		indexOpts = new(options.IndexOptions)
		for k, v := range optionsMap {
			switch k {
			case "unique":
				if b, ok := v.(bool); ok {
					indexOpts.SetUnique(b)
				}
			case "background":
				if b, ok := v.(bool); ok {
					indexOpts.SetBackground(b)
				}
			case "name":
				if name, ok := v.(string); ok {
					indexOpts.SetName(name)
				}
			case "expireAfterSeconds":
				if sec, ok := v.(int); ok {
					indexOpts.SetExpireAfterSeconds(int32(sec))
				}
			case "sparse":
				if b, ok := v.(bool); ok {
					indexOpts.SetSparse(b)
				}
			case "storageEngine":
				if engine, ok := v.(bson.M); ok {
					indexOpts.SetStorageEngine(engine)
				}
			default:
			}
		}
	}

	model := mongo.IndexModel{
		Keys:    indexKeys,
		Options: indexOpts,
	}

	return r.collection.Indexes().CreateOne(ctx, model)
}

func (r *MongoRepository[T, K]) Create(ctx context.Context, entity *T) error {
	setTimestamps(entity, true)
	_, err := r.collection.InsertOne(ctx, entity)
	return err
}

// CreateMany 批量插入多个文档
func (r *MongoRepository[T, K]) CreateMany(ctx context.Context, entities []*T) error {
	if len(entities) == 0 {
		return nil // 空列表直接返回
	}

	// 为每个实体设置时间戳，并构造 interface{} 切片
	var docs []interface{}
	for _, entity := range entities {
		setTimestamps(entity, true)
		docs = append(docs, entity)
	}

	// 插入数据库
	_, err := r.collection.InsertMany(ctx, docs)
	return err
}

func (r *MongoRepository[T, K]) GetByID(ctx context.Context, id K) (*T, error) {
	filter := bson.M{r.idField: id, "deletedAt": bson.M{"$exists": false}}
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

// UpdateFields 只更新指定字段
func (r *MongoRepository[T, K]) UpdateFields(ctx context.Context, id K, updates map[string]any) error {
	// 自动添加 updatedAt 字段（如果结构体中包含）
	if _, ok := updates["updatedAt"]; !ok {
		updates["updatedAt"] = time.Now()
	}

	// 构造 filter 和 update 操作
	filter := bson.M{r.idField: id}
	update := bson.M{"$set": updates}

	// 执行更新
	result, err := r.collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return err
	}

	if result.MatchedCount == 0 {
		return errors.New("未找到匹配的文档")
	}

	return nil
}

func (r *MongoRepository[T, K]) Delete(ctx context.Context, id K) error {
	filter := bson.M{r.idField: id}
	update := bson.M{"$set": bson.M{"deletedAt": time.Now()}}
	_, err := r.collection.UpdateOne(ctx, filter, update)
	return err
}

// DeleteMany 软删除多个文档，根据传入的 ID 列表进行更新
func (r *MongoRepository[T, K]) DeleteMany(ctx context.Context, ids []K) error {
	if len(ids) == 0 {
		return nil // 空列表直接返回
	}

	// 构造 filter：匹配多个 ID
	filter := bson.M{
		r.idField: bson.M{"$in": ids},
	}

	// 构造 update：设置 deletedAt
	update := bson.M{
		"$set": bson.M{
			"deletedAt": time.Now(),
		},
	}

	// 执行更新
	_, err := r.collection.UpdateMany(ctx, filter, update)
	return err
}

// HardDelete 直接从数据库中物理删除文档（非软删除）
func (r *MongoRepository[T, K]) HardDelete(ctx context.Context, id K) error {
	filter := bson.M{r.idField: id}
	_, err := r.collection.DeleteOne(ctx, filter)
	return err
}

// HardDeleteMany 直接物理删除多个文档，根据传入的 ID 列表进行删除
func (r *MongoRepository[T, K]) HardDeleteMany(ctx context.Context, ids []K) error {
	if len(ids) == 0 {
		return nil // 空列表直接返回
	}

	// 构造 filter：匹配多个 ID
	filter := bson.M{
		r.idField: bson.M{"$in": ids},
	}

	// 执行删除
	_, err := r.collection.DeleteMany(ctx, filter)
	return err
}

func (r *MongoRepository[T, K]) List(ctx context.Context) ([]*T, error) {
	cursor, err := r.collection.Find(ctx, bson.M{"deletedAt": bson.M{"$exists": false}})
	if err != nil {
		return nil, err
	}
	var results []*T
	err = cursor.All(ctx, &results)
	return results, err
}

// FindOne 根据复杂条件查询一条记录（排除已软删除的文档）
func (r *MongoRepository[T, K]) FindOne(ctx context.Context, filter map[string]any) (*T, error) {
	// 自动排除软删除数据
	filter["deletedAt"] = bson.M{"$exists": false}
	var result T
	err := r.collection.FindOne(ctx, bson.M(filter)).Decode(&result)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, nil
	}
	return &result, err
}

func (r *MongoRepository[T, K]) Find(ctx context.Context, filter map[string]any, sort map[string]int) ([]*T, error) {
	// 自动添加未删除条件
	filter["deletedAt"] = bson.M{"$exists": false}

	// 将 map[string]int 转换为 bson.D
	var bsonSort bson.D
	for key, order := range sort {
		bsonSort = append(bsonSort, bson.E{Key: key, Value: order})
	}

	// 设置查询选项
	opts := options.Find().SetSort(bsonSort)

	// 执行查询
	cursor, err := r.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}

	// 解析结果
	var results []*T
	err = cursor.All(ctx, &results)
	return results, err
}

func (r *MongoRepository[T, K]) Paginate(
	ctx context.Context,
	offset int,
	limit int,
	filter map[string]any,
	sort map[string]int,
) ([]*T, int64, error) {
	// 自动添加未删除条件
	filter["deletedAt"] = bson.M{"$exists": false}

	// 将 map[string]int 转换为 bson.D
	var bsonSort bson.D
	for key, order := range sort {
		bsonSort = append(bsonSort, bson.E{Key: key, Value: order})
	}

	// 统计总数
	total, err := r.collection.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}

	// 设置分页与排序选项
	opts := options.Find().
		SetSkip(int64(offset)).
		SetLimit(int64(limit)).
		SetSort(bsonSort)

	// 执行查询
	cursor, err := r.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, 0, err
	}

	// 解析结果
	var results []*T
	if err := cursor.All(ctx, &results); err != nil {
		return nil, 0, err
	}

	return results, total, nil
}

func (r *MongoRepository[T, K]) Count(ctx context.Context, filter map[string]any) (int64, error) {
	filter["deletedAt"] = bson.M{"$exists": false}
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
		if f.Tag.Get("bson") == "createdAt" && isCreate {
			v.Field(i).Set(reflect.ValueOf(now))
		} else if f.Tag.Get("bson") == "updatedAt" {
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
