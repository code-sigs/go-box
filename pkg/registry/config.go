package registry

type RegistryConfig struct {
	Enable string     `mapstructure:"enable"` // 是否启用 etcd
	Etcd   EtcdConfig `mapstructure:"etcd"`
}

type EtcdConfig struct {
	RootDirectory string   `mapstructure:"rootDirectory"`
	Address       []string `mapstructure:"address"`
	Username      string   `mapstructure:"username"`
	Password      string   `mapstructure:"password"`
}
