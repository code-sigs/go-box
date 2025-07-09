package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/redis/go-redis/v9"
	"time"
)

// RedisConfig Redis配置
type RedisConfig struct {
	Address      []string `mapstructure:"address"`      // 地址 host:port
	Password     string   `mapstructure:"password"`     // 密码
	DB           int      `mapstructure:"db"`           // 数据库编号
	PoolSize     int      `mapstructure:"poolSize"`     // 连接池大小
	MinIdleConns int      `mapstructure:"minIdleConns"` // 最小空闲连接数
	ReadTimeout  int64    `mapstructure:"readTimeout"`  // 读取超时(秒)
	WriteTimeout int64    `mapstructure:"writeTimeout"` // 写入超时(秒)
	IdleTimeout  int64    `mapstructure:"idleTimeout"`  // 空闲连接超时时间(秒)
}

// RedisClient 封装后的Redis客户端
type RedisClient struct {
	client redis.UniversalClient
}

func NewRedisClient(cfg *RedisConfig) (*RedisClient, error) {
	var rdb redis.UniversalClient
	if len(cfg.Address) > 1 {
		rdb = redis.NewClusterClient(&redis.ClusterOptions{
			Addrs:        cfg.Address,
			Password:     cfg.Password,
			PoolSize:     cfg.PoolSize,
			MinIdleConns: cfg.MinIdleConns,
			ReadTimeout:  time.Duration(cfg.ReadTimeout) * time.Second,
			WriteTimeout: time.Duration(cfg.WriteTimeout) * time.Second,
		})
	} else {
		rdb = redis.NewClient(&redis.Options{
			Addr:         cfg.Address[0],
			Password:     cfg.Password,
			DB:           cfg.DB,
			PoolSize:     cfg.PoolSize,
			MinIdleConns: cfg.MinIdleConns,
			ReadTimeout:  time.Duration(cfg.ReadTimeout) * time.Second,
			WriteTimeout: time.Duration(cfg.WriteTimeout) * time.Second,
		}) // 测试连接
	}

	ctx := context.Background()
	if _, err := rdb.Ping(ctx).Result(); err != nil {
		return nil, fmt.Errorf("connect to redis failed: %v", err)
	}

	return &RedisClient{client: rdb}, nil
}

// Get 获取字符串值
func (r *RedisClient) Get(ctx context.Context, key string) (string, error) {
	return r.client.Get(ctx, key).Result()
}

// Set 设置字符串值
func (r *RedisClient) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	return r.client.Set(ctx, key, value, expiration).Err()
}

// Del 删除键
func (r *RedisClient) Del(ctx context.Context, keys ...string) error {
	return r.client.Del(ctx, keys...).Err()
}

// Exists 判断键是否存在
func (r *RedisClient) Exists(ctx context.Context, keys ...string) (int64, error) {
	return r.client.Exists(ctx, keys...).Result()
}

// Expire 设置过期时间
func (r *RedisClient) Expire(ctx context.Context, key string, expiration time.Duration) error {
	return r.client.Expire(ctx, key, expiration).Err()
}

// HGet 获取哈希字段值
func (r *RedisClient) HGet(ctx context.Context, key, field string) (string, error) {
	return r.client.HGet(ctx, key, field).Result()
}

// HSet 设置哈希字段值
func (r *RedisClient) HSet(ctx context.Context, key string, field string, value interface{}) error {
	return r.client.HSet(ctx, key, field, value).Err()
}

// HGetAll 获取整个哈希表
func (r *RedisClient) HGetAll(ctx context.Context, key string) (map[string]string, error) {
	return r.client.HGetAll(ctx, key).Result()
}

func (r *RedisClient) ZAdd(ctx context.Context, key string, members ...redis.Z) (int64, error) {
	return r.client.ZAdd(ctx, key, members...).Result()
}

func (r *RedisClient) ZRem(ctx context.Context, key string, members ...interface{}) (int64, error) {
	return r.client.ZRem(ctx, key, members...).Result()
}

func (r *RedisClient) ZRangeWithScores(ctx context.Context, key string, start, stop int64) ([]redis.Z, error) {
	return r.client.ZRangeWithScores(ctx, key, start, stop).Result()
}

func (r *RedisClient) ZCard(ctx context.Context, key string) (int64, error) {
	return r.client.ZCard(ctx, key).Result()
}

func (r *RedisClient) ZCount(ctx context.Context, key, min, max string) (int64, error) {
	return r.client.ZCount(ctx, key, min, max).Result()
}

func (r *RedisClient) ZScore(ctx context.Context, key string, member string) (float64, error) {
	return r.client.ZScore(ctx, key, member).Result()
}

func (r *RedisClient) ZMScore(ctx context.Context, key string, members ...string) ([]float64, error) {
	return r.client.ZMScore(ctx, key, members...).Result()
}

func (r *RedisClient) Pipelined(ctx context.Context, fn func(redis.Pipeliner) error) ([]redis.Cmder, error) {
	return r.client.Pipelined(ctx, fn)
}

func (r *RedisClient) TxPipelined(ctx context.Context, fn func(redis.Pipeliner) error) ([]redis.Cmder, error) {
	return r.client.TxPipelined(ctx, fn)
}

func (r *RedisClient) Pipeline() redis.Pipeliner {
	return r.client.Pipeline()
}

func (r *RedisClient) Eval(ctx context.Context, script string, keys []string, args ...interface{}) (interface{}, error) {
	return r.client.Eval(ctx, script, keys, args...).Result()
}
func (r *RedisClient) EvalSha(ctx context.Context, sha1 string, keys []string, args ...interface{}) (interface{}, error) {
	return r.client.EvalSha(ctx, sha1, keys, args...).Result()
}
func (r *RedisClient) EvalRO(ctx context.Context, script string, keys []string, args ...interface{}) (interface{}, error) {
	return r.client.EvalRO(ctx, script, keys, args).Result()
}
func (r *RedisClient) EvalShaRO(ctx context.Context, sha1 string, keys []string, args ...interface{}) (interface{}, error) {
	return r.client.EvalShaRO(ctx, sha1, keys, args).Result()
}
func (r *RedisClient) ScriptExists(ctx context.Context, hashes ...string) (interface{}, error) {
	return r.client.ScriptExists(ctx, hashes...).Result()
}
func (r *RedisClient) ScriptFlush(ctx context.Context) (interface{}, error) {
	return r.client.ScriptFlush(ctx).Result()
}
func (r *RedisClient) ScriptKill(ctx context.Context) (interface{}, error) {
	return r.client.ScriptKill(ctx).Result()
}
func (r *RedisClient) ScriptLoad(ctx context.Context, script string) (interface{}, error) {
	return r.client.ScriptLoad(ctx, script).Result()
}

// TTL 获取键的剩余生存时间
func (r *RedisClient) TTL(ctx context.Context, key string) (time.Duration, error) {
	return r.client.TTL(ctx, key).Result()
}

func (r *RedisClient) DeletePrefix(ctx context.Context, pattern string) error {
	const (
		batchSize = 500  // 每批删除数量
		scanCount = 1000 // 每次扫描数量
	)

	var cursor uint64
	for {
		// 使用 SCAN 分批次获取键
		keys, nextCursor, err := r.client.Scan(ctx, cursor, pattern, scanCount).Result()
		if err != nil {
			return err
		}

		// 分批删除
		for i := 0; i < len(keys); i += batchSize {
			end := i + batchSize
			if end > len(keys) {
				end = len(keys)
			}
			if err := r.client.Del(ctx, keys[i:end]...).Err(); err != nil {
				return err
			}
		}

		// 更新游标
		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}
	return nil
}

func (r *RedisClient) GetUnmarshal(ctx context.Context, key string, out interface{}) error {
	data, err := r.client.Get(ctx, key).Bytes()
	if err != nil {
		return err
	}
	return json.Unmarshal(data, out)
}

func (r *RedisClient) SetMarshal(ctx context.Context, key string, in interface{}, ttl time.Duration) error {
	jsonData, err := json.Marshal(in)
	if err != nil {
		return err
	}
	return r.client.Set(ctx, key, jsonData, ttl).Err()
}
