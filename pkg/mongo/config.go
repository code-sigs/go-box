package mongo

import (
	"context"
	"github.com/code-sigs/go-box/pkg/logger"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

// Config 定义 MongoDB 客户端的初始化配置结构。
// 可通过 yaml/json/env 加载。
type MongoConfig struct {
	URI            string `mapstructure:"uri"`            // 支持多个节点（如: mongodb://host1,host2/?replicaSet=rs0）
	Database       string `mapstructure:"database"`       // 默认使用的数据库名
	MinPoolSize    uint64 `mapstructure:"minPoolSize"`    // 最小连接池大小
	MaxPoolSize    uint64 `mapstructure:"maxPoolSize"`    // 最大连接池大小
	ConnectTimeout int64  `mapstructure:"connectTimeout"` // 连接超时时间（单位：秒）
	ReadPreference string `mapstructure:"readPreference"` // 读取偏好（primary/nearest/secondaryPreferred）
}

// New 初始化 MongoDB 客户端并返回 client 和 database 实例。
// 推荐在工程启动时调用一次。支持副本集/分片集群连接。
func New(cfg *MongoConfig) (*mongo.Client, *mongo.Database, error) {
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

	// 创建连接
	client, err := mongo.Connect(ctx, opts)
	if err != nil {
		logger.Errorf("Mongo connect failed, %s", err.Error())
		return nil, nil, err
	}

	// Ping 主节点确保连接可用
	if err := client.Ping(ctx, readpref.Primary()); err != nil {
		logger.Errorf("Mongo ping failed, %s", err.Error())
		return nil, nil, err
	}

	logger.Infof("Mongo connect success.")
	return client, client.Database(cfg.Database), nil
}
