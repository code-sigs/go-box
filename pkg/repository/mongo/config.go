package mongo

import (
	"context"
	"time"

	"github.com/spf13/viper"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
	"go.mongodb.org/mongo-driver/mongo/writeconcern"
	"go.uber.org/zap"
)

// Config 定义 MongoDB 客户端的初始化配置结构。
// 可通过 yaml/json/env 加载。
type Config struct {
	URI            string `mapstructure:"uri"`             // 支持多个节点（如: mongodb://host1,host2/?replicaSet=rs0）
	Database       string `mapstructure:"database"`        // 默认使用的数据库名
	MinPoolSize    uint64 `mapstructure:"min_pool_size"`   // 最小连接池大小
	MaxPoolSize    uint64 `mapstructure:"max_pool_size"`   // 最大连接池大小
	ConnectTimeout int    `yaml:"connectTimeout"`          // 连接超时时间（单位：秒）
	ReadPreference string `mapstructure:"read_preference"` // 读取偏好（primary/nearest/secondaryPreferred）
	WriteConcern   string `mapstructure:"write_concern"`   // 写一致性级别（majority/w1/w2）
}

// LoadConfig 加载指定路径下的配置文件（支持 .yaml/.json）并返回 Config 实例。
// 同时支持读取环境变量（前缀为 MONGO_）。
func LoadConfig(path string) (*Config, error) {
	v := viper.New()
	v.SetConfigFile(path)
	v.SetConfigType("yaml")
	v.AutomaticEnv()
	v.SetEnvPrefix("MONGO") // 例如支持 MONGO_URI 等 env 变量覆盖

	if err := v.ReadInConfig(); err != nil {
		return nil, err
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// Init 初始化 MongoDB 客户端并返回 client 和 database 实例。
// 推荐在工程启动时调用一次。支持副本集/分片集群连接。
func Init(logger *zap.Logger, cfg *Config) (*mongo.Client, *mongo.Database, error) {
	timeout := time.Duration(cfg.ConnectTimeout) * time.Second

	// 创建上下文（连接超时）
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// 构建连接选项
	opts := options.Client().
		ApplyURI(cfg.URI).
		SetMinPoolSize(cfg.MinPoolSize).
		SetMaxPoolSize(cfg.MaxPoolSize).
		SetConnectTimeout(timeout)

	// 设置读偏好
	switch cfg.ReadPreference {
	case "nearest":
		opts.SetReadPreference(readpref.Nearest())
	case "secondary":
		opts.SetReadPreference(readpref.Secondary())
	case "primaryPreferred":
		opts.SetReadPreference(readpref.PrimaryPreferred())
	case "secondaryPreferred":
		opts.SetReadPreference(readpref.SecondaryPreferred())
	default:
		opts.SetReadPreference(readpref.Primary())
	}

	// 设置写一致性
	switch cfg.WriteConcern {
	case "majority":
		opts.SetWriteConcern(writeconcern.New(writeconcern.WMajority()))
	case "w1":
		opts.SetWriteConcern(writeconcern.New(writeconcern.W(1)))
	case "w2":
		opts.SetWriteConcern(writeconcern.New(writeconcern.W(2)))
	default:
		opts.SetWriteConcern(writeconcern.New(writeconcern.WMajority()))
	}

	// 创建连接
	client, err := mongo.Connect(ctx, opts)
	if err != nil {
		logger.Error("Mongo connect failed", zap.Error(err))
		return nil, nil, err
	}

	// Ping 主节点确保连接可用
	if err := client.Ping(ctx, readpref.Primary()); err != nil {
		logger.Error("Mongo ping failed", zap.Error(err))
		return nil, nil, err
	}

	logger.Info("Mongo connected successfully")

	return client, client.Database(cfg.Database), nil
}
