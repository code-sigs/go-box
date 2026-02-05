package router

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"reflect"
	"strings"
	"syscall"
	"time"

	"github.com/code-sigs/go-box/pkg/logger"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"google.golang.org/grpc/metadata"
)

type routeEntry struct {
	path    string
	handler gin.HandlerFunc
}

type Router struct {
	routes      []routeEntry
	proxyHeader []string
	middlewares []gin.HandlerFunc // 新增：用户自定义中间件
	group       []*RouterGroup
}

type RouterGroup struct {
	name     string
	handlers []gin.HandlerFunc
	routes   []routeEntry
	injector func(c *gin.Context, ctx context.Context) context.Context
}

// New 创建一个新的 Box 实例
func New() *Router {
	return &Router{
		routes: []routeEntry{},
	}
}

// WithHeader 设置需要代理到 gRPC 的 header
func (r *Router) WithHeader(header ...string) *Router {
	r.proxyHeader = append(r.proxyHeader, header...)
	return r
}

// Use 添加用户自定义 gin 中间件
func (r *Router) Use(mw ...gin.HandlerFunc) *Router {
	r.middlewares = append(r.middlewares, mw...)
	return r
}

func (r *Router) injector(c *gin.Context, ctx context.Context) context.Context {
	md := metadata.New(nil)
	md.Append("clientip", c.ClientIP())
	logger.Infof(c, "injector clientip: %s", c.ClientIP())
	if len(r.proxyHeader) == 0 {
		for key, values := range c.Request.Header {
			for _, value := range values {
				if value != "" {
					md.Append(strings.ToLower(key), value)
				}
			}
		}
		return ctx
	}
	for _, key := range r.proxyHeader {
		val := c.GetHeader(key)
		if val != "" {
			md.Append(strings.ToLower(key), val)
		}
	}
	if len(md) > 0 {
		ctx = metadata.NewOutgoingContext(ctx, md)
	}
	clientIP := c.ClientIP()
	ctx = context.WithValue(ctx, "clientip", clientIP)
	return ctx
}

func (r *Router) Group(name string, handlers ...gin.HandlerFunc) *RouterGroup {
	group := &RouterGroup{
		name:     name,
		handlers: handlers,
		routes:   []routeEntry{},
		injector: r.injector,
	}
	r.group = append(r.group, group)
	return group
}

// Register 注册一个 gRPC 方法与其绑定路径
func (r *Router) POST(path string, grpcFunc any) {
	h := GenericGRPCHandler(grpcFunc, r.injector)
	r.routes = append(r.routes, routeEntry{
		path:    path,
		handler: h,
	})
}

func (r *RouterGroup) RegisterRPCClient(path string, grpcFunc any) {
	h := GenericGRPCHandler(func(ctx context.Context, req any) (any, error) {
		fnVal := reflect.ValueOf(grpcFunc)
		fnType := fnVal.Type()

		// 函数必须至少有两个入参和两个出参
		if fnType.Kind() != reflect.Func ||
			fnType.NumIn() < 2 || // 允许有变参或额外参数
			fnType.NumOut() != 2 ||
			!fnType.In(0).Implements(reflect.TypeOf((*context.Context)(nil)).Elem()) ||
			!fnType.Out(1).Implements(reflect.TypeOf((*error)(nil)).Elem()) {
			return nil, fmt.Errorf("grpcFunc must be func(context.Context, *T, ...any) (any, error)")
		}

		// 处理参数
		in := []reflect.Value{reflect.ValueOf(ctx)}

		// 检查 req 类型
		reqVal := reflect.ValueOf(req)
		expectedReqType := fnType.In(1)
		if !reqVal.Type().AssignableTo(expectedReqType) {
			return nil, fmt.Errorf("request type mismatch: expect %v, got %v", expectedReqType, reqVal.Type())
		}
		in = append(in, reqVal)

		// 处理可选参数，填充默认零值（如 grpc.CallOption）
		for i := 2; i < fnType.NumIn(); i++ {
			argType := fnType.In(i)
			in = append(in, reflect.Zero(argType)) // 默认 nil、0、空切片等
		}

		// 调用 grpcFunc
		results := fnVal.Call(in)

		resp := results[0].Interface()

		var err error
		if !results[1].IsNil() {
			err = results[1].Interface().(error)
		}
		return resp, err
	}, r.injector)

	r.routes = append(r.routes, routeEntry{
		path:    path,
		handler: h,
	})
}

func (r *RouterGroup) POST(path string, grpcFunc any) {
	h := GenericGRPCHandler(grpcFunc, r.injector)
	r.routes = append(r.routes, routeEntry{
		path:    path,
		handler: h,
	})
}

// Run 启动 Box 服务，支持用户自定义中间件，并实现优雅关闭
func (r *Router) Run(addr string, beforeRun func(g *gin.Engine), shutdown func(), isDebug bool) error {
	if !isDebug {
		gin.SetMode(gin.ReleaseMode)
	}
	engine := gin.New()
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"}, // 如果 AllowCredentials: true，请指定域名
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "PATCH", "OPTIONS", "HEAD"},
		AllowHeaders:     []string{"*"}, // 允许所有请求头（仅当 AllowCredentials 为 false 时有效）
		ExposeHeaders:    []string{"*"}, // 并非所有浏览器支持，推荐手动列出
		AllowCredentials: false,         // 为 true 时，不允许 * 出现在 AllowOrigins、AllowHeaders 中
		MaxAge:           12 * time.Hour,
	}))
	engine.Use(gin.Recovery(), gin.Logger())
	for _, mw := range r.middlewares {
		engine.Use(mw)
	}
	for _, route := range r.routes {
		engine.POST(route.path, route.handler)
	}
	for _, group := range r.group {
		groupEngine := engine.Group(group.name, group.handlers...)
		for _, route := range group.routes {
			groupEngine.POST(route.path, route.handler)
		}
	}
	if beforeRun != nil {
		beforeRun(engine)
	}
	srv := &http.Server{
		Addr:    addr,
		Handler: engine,
	}

	// 启动服务
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			panic("listen: " + err.Error())
		}
	}()

	// 优雅关闭
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	if shutdown != nil {
		shutdown()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return srv.Shutdown(ctx)
}
