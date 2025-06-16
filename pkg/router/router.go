package router

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/code-sigs/go-box/internal/handler"
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
	return ctx
}

// Register 注册一个 gRPC 方法与其绑定路径
func (r *Router) Register(path string, grpcFunc any) {
	h := handler.GenericGRPCHandler(grpcFunc, r.injector)
	r.routes = append(r.routes, routeEntry{
		path:    path,
		handler: h,
	})
}

// Run 启动 Box 服务，支持用户自定义中间件，并实现优雅关闭
func (r *Router) Run(addr string, shutdown func()) error {
	engine := gin.New()
	engine.Use(gin.Recovery(), gin.Logger())
	for _, mw := range r.middlewares {
		engine.Use(mw)
	}
	for _, route := range r.routes {
		engine.POST(route.path, route.handler)
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
