package handler

import (
	"context"
	"net/http"
	"reflect"

	"github.com/code-sigs/go-box/pkg/rpcerror"
	"github.com/gin-gonic/gin"
)

type StandardResponse[T any] struct {
	Code    int32  `json:"code"`
	Message string `json:"message"`
	Details string `json:"details,omitempty"`
	Data    T      `json:"data,omitempty"`
}

// ContextInjector 定义上下文注入函数类型
type ContextInjector func(c *gin.Context, ctx context.Context) context.Context

// DefaultContextInjector 默认的上下文注入函数
func DefaultContextInjector(c *gin.Context, ctx context.Context) context.Context {
	traceID := c.GetHeader("X-Trace-ID")
	if traceID != "" {
		ctx = context.WithValue(ctx, "trace_id", traceID)
	}
	return ctx
}

// GenericGRPCHandler 适配任意签名的 gRPC 方法
func GenericGRPCHandler(grpcFunc any, ctxInjector ContextInjector) gin.HandlerFunc {
	fnVal := reflect.ValueOf(grpcFunc)
	fnType := fnVal.Type()

	return func(c *gin.Context) {
		if fnType.Kind() != reflect.Func || fnType.NumIn() != 2 || fnType.NumOut() != 2 {
			c.JSON(http.StatusInternalServerError, StandardResponse[any]{Code: 500, Message: "invalid grpcFunc signature"})
			return
		}

		reqType := fnType.In(1)
		var reqPtr reflect.Value
		if reqType.Kind() == reflect.Ptr {
			reqPtr = reflect.New(reqType.Elem())
		} else {
			reqPtr = reflect.New(reqType)
		}

		if err := c.ShouldBindJSON(reqPtr.Interface()); err != nil {
			c.JSON(http.StatusBadRequest, StandardResponse[any]{Code: 400, Message: "Invalid request: " + err.Error()})
			return
		}

		var reqVal reflect.Value
		if reqType.Kind() == reflect.Ptr {
			reqVal = reqPtr
		} else {
			reqVal = reqPtr.Elem()
		}

		ctx := ctxInjector(c, c.Request.Context())
		out := fnVal.Call([]reflect.Value{reflect.ValueOf(ctx), reqVal})

		if len(out) != 2 {
			c.JSON(http.StatusInternalServerError, StandardResponse[any]{Code: 500, Message: "grpcFunc must return two values"})
			return
		}

		if !out[1].IsNil() {
			if err, ok := out[1].Interface().(error); ok {
				// 优先提取业务错误
				if rpcErr := rpcerror.UnWrap(err); rpcErr != nil {
					c.JSON(http.StatusOK, StandardResponse[any]{
						Code:    rpcErr.Code,
						Message: rpcErr.Message,
						Details: rpcErr.Details,
						Data:    nil, // 保证错误时 Data 一定为 nil
					})
					return
				}
				// 普通错误
				c.JSON(http.StatusInternalServerError, StandardResponse[any]{Code: 500, Message: err.Error(), Data: nil})
			} else {
				c.JSON(http.StatusInternalServerError, StandardResponse[any]{Code: 500, Message: "unknown error", Data: nil})
			}
			return
		}
		c.JSON(http.StatusOK, StandardResponse[any]{Code: 0, Message: "ok", Data: out[0].Interface()})
	}
}
