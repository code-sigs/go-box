package router

import (
	"context"
	"strings"

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
}

// New 创建一个新的 Box 实例
func New() *Router {
	return &Router{
		routes: []routeEntry{},
	}
}

func (r *Router) WithHeader(header ...string) *Router {
	r.proxyHeader = append(r.proxyHeader, header...)
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

// Run 启动 Box 服务
func (r *Router) Run(addr string) error {
	engine := gin.Default()
	for _, r := range r.routes {
		engine.POST(r.path, r.handler)
	}
	return engine.Run(addr)
}
