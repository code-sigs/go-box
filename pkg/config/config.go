// config.go
package config

import (
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/spf13/viper"
)

type GrpcConfig struct {
	Host string `mapstructure:"host"`
	Port int32  `mapstructure:"port"`
}

type HttpConfig struct {
	Host string `mapstructure:"host"`
	Port int32  `mapstructure:"port"`
}

// LoadConfig 是一个泛型函数，用于加载指定 key 下的配置到任意结构体中
func LoadConfig[T any](configPath string, fileName string, envPrefix string, configKey string) (*T, error) {
	v := viper.New()

	// 设置默认值（可选）
	defaultKey := fmt.Sprintf("%s.%s", envPrefix, configKey)
	log.Printf("Loading config from key: %s", defaultKey)

	// 自动读取环境变量（支持 MYAPP_HTTP_PORT=8000）
	v.AutomaticEnv()
	v.SetEnvPrefix(envPrefix) // 环境变量前缀
	replacer := strings.NewReplacer("_", ".")
	v.SetEnvKeyReplacer(replacer) //
	// 加载配置文件
	if configPath != "" {
		v.AddConfigPath(configPath)
	} else {
		v.AddConfigPath(".")
	}
	v.SetConfigName(fileName)
	v.SetConfigType("yaml")

	if err := v.ReadInConfig(); err != nil {
		var configFileNotFoundError viper.ConfigFileNotFoundError
		if !errors.As(err, &configFileNotFoundError) {
			return nil, fmt.Errorf("error reading config file: %w", err)
		}
		log.Println("Config file not found, using defaults and environment variables.")
	}

	// 解析指定路径下的配置到泛型结构体 T 中
	cfg := new(T)
	fullKey := fmt.Sprintf("%s.%s", envPrefix, configKey)
	if err := v.UnmarshalKey(fullKey, cfg); err != nil {
		return nil, fmt.Errorf("unable to decode '%s' into struct: %v", fullKey, err)
	}

	return cfg, nil
}
