# go-box

**go-box** 是一个基于 Go 语言的高性能微服务开发工具箱，集成了服务注册与发现（支持 etcd、zookeeper、内存）、gRPC 服务治理、通用中间件、优雅关闭、结构化错误处理、通用 handler 适配、实用工具函数等能力，适合企业级微服务项目快速开发和最佳实践。

---

## 主要特性

- **服务注册与发现**：支持 etcd、zookeeper、内存多种注册中心，统一接口，易于扩展。
- **gRPC 服务治理**：内置服务注册、发现、优雅关闭、连接池、负载均衡等能力。
- **结构化错误处理**：支持 gRPC 结构化业务错误，便于前后端统一处理。
- **通用 handler 适配**：支持任意签名 gRPC 方法的 HTTP 适配，自动标准化返回结构。
- **丰富工具函数**：如本地 IP 获取、时间格式化、UUID、MD5、随机字符串等。
- **高可测试性**：内置丰富单元测试用例，便于二次开发和持续集成。

---

## 目录结构

```
go-box/
├── internal/
│   ├── handler/         # 通用 handler 适配与上下文注入
│   ├── registry/        # 注册中心接口与实现（etcd、memory、zk）
│   ├── resolver/        # gRPC 服务发现 resolver
│   └── rpc/             # gRPC server/client 封装
├── pkg/
│   ├── grpc/            # gRPC 服务治理入口
│   ├── registry_factory/# 注册中心工厂
│   ├── router/          # 路由与 handler 测试
│   ├── rpcerror/        # 结构化错误处理
│   └── utils/           # 工具函数
└── go.mod
```

---

## 快速开始

### 1. 安装依赖

```sh
go mod tidy
```

### 2. 启动 gRPC 服务

```go
import (
    "github.com/code-sigs/go-box/pkg/grpc"
)

func main() {
    g := grpc.New()
    // 注册服务并监听
    g.ListenAndRegister("hello-service", "127.0.0.1:9000", ":9000")
}
```

### 3. 获取 gRPC 连接

```go
conn, err := g.GetRPConnection("hello-service")
if err != nil {
    // handle error
}
```

### 4. HTTP 适配 gRPC handler

```go
import (
    "github.com/code-sigs/go-box/internal/handler"
    "github.com/gin-gonic/gin"
)

router := gin.Default()
router.POST("/api/hello", handler.GenericGRPCHandler(YourGRPCFunc, handler.DefaultContextInjector))
```

---

## 结构化错误处理

- 支持业务错误码、消息、堆栈信息自动注入
- 统一响应结构 `StandardResponse`

```go
import "github.com/code-sigs/go-box/pkg/rpcerror"

return rpcerror.WrapError(5100, "参数错误")
```

---

## 工具函数示例

```go
ip, _ := utils.GetLocalIP()
uuid := utils.GenerateUUID()
now := utils.FormatTimeNow()
```

---

## 贡献

欢迎提交 issue 和 PR，完善 go-box 生态！

---

## License

MIT