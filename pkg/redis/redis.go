package redis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
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

func (r *RedisClient) DB() redis.UniversalClient {
	return r.client
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

func (r *RedisClient) DeletePrefix(ctx context.Context, prefix string) (int64, error) {
	pattern := prefix + "*"
	var totalDeleted int64

	switch c := r.client.(type) {
	case *redis.ClusterClient:
		err := c.ForEachMaster(ctx, func(ctx context.Context, shard *redis.Client) error {
			iter := shard.Scan(ctx, 0, pattern, 1000).Iterator()
			pipe := shard.Pipeline()
			var cmds []*redis.IntCmd
			batch := 0

			for iter.Next(ctx) {
				cmd := pipe.Unlink(ctx, iter.Val())
				cmds = append(cmds, cmd)
				batch++
				if batch >= 1000 {
					n, err := execAndCount(ctx, pipe, cmds)
					if err != nil {
						return err
					}
					totalDeleted += n
					pipe = shard.Pipeline() // 重建 pipeline
					cmds = cmds[:0]
					batch = 0
				}
			}
			if err := iter.Err(); err != nil {
				return err
			}
			if len(cmds) > 0 {
				n, err := execAndCount(ctx, pipe, cmds)
				if err != nil {
					return err
				}
				totalDeleted += n
			}
			return nil
		})
		return totalDeleted, err

	case *redis.Client:
		iter := c.Scan(ctx, 0, pattern, 1000).Iterator()
		pipe := c.Pipeline()
		var cmds []*redis.IntCmd
		batch := 0

		for iter.Next(ctx) {
			cmd := pipe.Unlink(ctx, iter.Val())
			cmds = append(cmds, cmd)
			batch++
			if batch >= 1000 {
				n, err := execAndCount(ctx, pipe, cmds)
				if err != nil {
					return totalDeleted, err
				}
				totalDeleted += n
				pipe = c.Pipeline() // 重建 pipeline
				cmds = cmds[:0]
				batch = 0
			}
		}
		if err := iter.Err(); err != nil {
			return totalDeleted, err
		}
		if len(cmds) > 0 {
			n, err := execAndCount(ctx, pipe, cmds)
			if err != nil {
				return totalDeleted, err
			}
			totalDeleted += n
		}
		return totalDeleted, nil

	default:
		return 0, errors.New("unsupported redis client type (need *redis.Client or *redis.ClusterClient)")
	}
}

func execAndCount(ctx context.Context, pipe redis.Pipeliner, cmds []*redis.IntCmd) (int64, error) {
	_, err := pipe.Exec(ctx)
	if err != nil {
		return 0, err
	}
	var deleted int64
	for _, cmd := range cmds {
		deleted += cmd.Val()
	}
	return deleted, nil
}

//func (r *RedisClient) DeletePrefix(ctx context.Context, pattern string) error {
//	const (
//		batchSize = 500  // 每批删除数量
//		scanCount = 1000 // 每次扫描数量
//	)
//	pattern = pattern + "*"
//	var cursor uint64
//	for {
//		// 使用 SCAN 分批次获取键
//		keys, nextCursor, err := r.client.Scan(ctx, cursor, pattern, scanCount).Result()
//		if err != nil {
//			return err
//		}
//
//		// 分批删除
//		for i := 0; i < len(keys); i += batchSize {
//			end := i + batchSize
//			if end > len(keys) {
//				end = len(keys)
//			}
//			if err := r.client.Del(ctx, keys[i:end]...).Err(); err != nil {
//				return err
//			}
//		}
//
//		// 更新游标
//		cursor = nextCursor
//		if cursor == 0 {
//			break
//		}
//	}
//	return nil
//}

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

// RedisLock is a distributed lock implemented with Redis
type RedisLock struct {
	mux           sync.Mutex
	client        redis.UniversalClient
	key           string
	value         string
	expire        time.Duration
	renewInterval time.Duration
	cancelFunc    context.CancelFunc
	wg            sync.WaitGroup
}

// NewRedisLock creates a new RedisLock instance
func NewRedisLock(rdb *RedisClient, key string, expire time.Duration) *RedisLock {
	return &RedisLock{
		client:        rdb.client,
		key:           fmt.Sprintf("redis_lock:%s", key),
		value:         uuid.New().String(),
		expire:        expire,
		renewInterval: expire / 3, // safer than expire/2
	}
}

// Lock tries to acquire the lock
func (l *RedisLock) Lock() (bool, error) {
	l.mux.Lock()
	defer l.mux.Unlock()
	ctx := context.Background()
	status, err := l.client.SetArgs(ctx, l.key, l.value, redis.SetArgs{
		Mode: "NX",
		TTL:  l.expire,
	}).Result()
	if err != nil || status != "OK" {
		return false, err
	}

	lockCtx, cancel := context.WithCancel(ctx)
	l.cancelFunc = cancel
	l.wg.Add(1)
	go l.startAutoRenew(lockCtx)

	return true, nil
}

// Unlock safely releases the lock
func (l *RedisLock) Unlock() (bool, error) {
	l.mux.Lock()
	defer l.mux.Unlock()

	if l.cancelFunc == nil {
		return false, nil // already unlocked
	}

	l.cancelFunc()
	l.cancelFunc = nil
	l.wg.Wait()

	luaScript := `
		if redis.call("GET", KEYS[1]) == ARGV[1] then
			return redis.call("DEL", KEYS[1])
		else
			return 0
		end
	`
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		res, err := l.client.Eval(ctx, luaScript, []string{l.key}, l.value).Result()
		if err == nil {
			if v, ok := res.(int64); ok && v == 1 {
				return true, nil
			}
			return false, nil
		}
		time.Sleep(50 * time.Millisecond)
	}

	return false, errors.New("failed to release lock after retries")
}

// startAutoRenew periodically renews the lock TTL
func (l *RedisLock) startAutoRenew(ctx context.Context) {
	defer l.wg.Done()

	ticker := time.NewTicker(l.renewInterval)
	defer ticker.Stop()

	luaScript := `
		if redis.call("GET", KEYS[1]) == ARGV[1] then
			return redis.call("PEXPIRE", KEYS[1], ARGV[2])
		else
			return 0
		end
	`

	for {
		select {
		case <-ticker.C:
			_, _ = l.client.Eval(ctx, luaScript, []string{l.key}, l.value, int(l.expire.Milliseconds())).Result()
		case <-ctx.Done():
			return
		}
	}
}
