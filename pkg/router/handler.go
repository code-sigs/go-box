package router

import (
	"context"
	"net/http"
	"reflect"
	"strconv"

	"github.com/code-sigs/go-box/pkg/rpcerror"
	"github.com/gin-gonic/gin"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type StandardResponse[T any] struct {
	Code    int64  `json:"code"`
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
		if fnType.Kind() != reflect.Func || fnType.NumIn() < 2 || fnType.NumOut() != 2 {
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

		ctx := context.WithValue(c.Request.Context(), "clientip", c.ClientIP())
		userID := c.Value("user-id")
		platformID := c.Value("platform-id")
		if userID != nil {
			ctx = context.WithValue(ctx, "user-id", userID)
		}
		if platformID != nil {
			ctx = context.WithValue(ctx, "platform-id", platformID)
		}
		out := fnVal.Call([]reflect.Value{reflect.ValueOf(ctx), reqVal})

		if len(out) != 2 {
			c.JSON(http.StatusInternalServerError, StandardResponse[any]{Code: 500, Message: "grpcFunc must return two values"})
			return
		}

		if !out[1].IsNil() {
			if err, ok := out[1].Interface().(error); ok {
				if rpcErr := rpcerror.UnWrap(err); rpcErr != nil {
					c.JSON(http.StatusOK, StandardResponse[any]{
						Code:    rpcErr.Code,
						Message: rpcErr.Message,
						Details: rpcErr.Details,
						Data:    nil,
					})
					return
				}
				c.JSON(http.StatusInternalServerError, StandardResponse[any]{Code: 500, Message: err.Error(), Data: nil})
			} else {
				c.JSON(http.StatusInternalServerError, StandardResponse[any]{Code: 500, Message: "unknown error", Data: nil})
			}
			return
		}

		data, err := normalizeResponseData(out[0].Interface())
		if err != nil {
			c.JSON(http.StatusInternalServerError, StandardResponse[any]{Code: 500, Message: "marshal response failed: " + err.Error(), Data: nil})
			return
		}
		c.JSON(http.StatusOK, StandardResponse[any]{Code: 0, Message: "ok", Data: data})
	}
}

func normalizeResponseData(data any) (any, error) {
	message, ok := data.(proto.Message)
	if !ok {
		return data, nil
	}
	return protoMessageToAny(message.ProtoReflect()), nil
}

func protoMessageToAny(message protoreflect.Message) any {
	if !message.IsValid() {
		return nil
	}
	obj := make(map[string]any)
	fields := message.Descriptor().Fields()
	for index := 0; index < fields.Len(); index++ {
		field := fields.Get(index)
		if oneof := field.ContainingOneof(); oneof != nil && !oneof.IsSynthetic() && !message.Has(field) {
			continue
		}
		key := field.JSONName()
		if !message.Has(field) {
			obj[key] = protoZeroJSONValue(field)
			continue
		}
		obj[key] = protoFieldValueToAny(message.Get(field), field)
	}
	return obj
}

func protoFieldValueToAny(value protoreflect.Value, field protoreflect.FieldDescriptor) any {
	switch {
	case field.IsList():
		list := value.List()
		result := make([]any, 0, list.Len())
		for index := 0; index < list.Len(); index++ {
			result = append(result, protoSingularValueToAny(list.Get(index), field))
		}
		return result
	case field.IsMap():
		return protoMapToAny(value.Map(), field)
	default:
		return protoSingularValueToAny(value, field)
	}
}

func protoSingularValueToAny(value protoreflect.Value, field protoreflect.FieldDescriptor) any {
	switch field.Kind() {
	case protoreflect.BoolKind:
		return value.Bool()
	case protoreflect.EnumKind:
		return int32(value.Enum())
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind:
		return int32(value.Int())
	case protoreflect.Uint32Kind, protoreflect.Fixed32Kind:
		return uint32(value.Uint())
	case protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind:
		return value.Int()
	case protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		return value.Uint()
	case protoreflect.FloatKind:
		return float32(value.Float())
	case protoreflect.DoubleKind:
		return value.Float()
	case protoreflect.StringKind:
		return value.String()
	case protoreflect.BytesKind:
		return []byte(value.Bytes())
	case protoreflect.MessageKind:
		return protoMessageToAny(value.Message())
	default:
		return nil
	}
}

func protoMapToAny(mp protoreflect.Map, field protoreflect.FieldDescriptor) map[string]any {
	result := make(map[string]any, mp.Len())
	keyField := field.MapKey()
	valueField := field.MapValue()
	mp.Range(func(key protoreflect.MapKey, value protoreflect.Value) bool {
		result[protoMapKeyToString(key, keyField)] = protoSingularValueToAny(value, valueField)
		return true
	})
	return result
}

func protoMapKeyToString(key protoreflect.MapKey, field protoreflect.FieldDescriptor) string {
	switch field.Kind() {
	case protoreflect.BoolKind:
		return strconv.FormatBool(key.Bool())
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind,
		protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind:
		return strconv.FormatInt(key.Int(), 10)
	case protoreflect.Uint32Kind, protoreflect.Fixed32Kind,
		protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		return strconv.FormatUint(key.Uint(), 10)
	default:
		return key.String()
	}
}

func protoZeroJSONValue(field protoreflect.FieldDescriptor) any {
	switch {
	case field.IsList():
		return []any{}
	case field.IsMap():
		return map[string]any{}
	case field.Kind() == protoreflect.MessageKind:
		return nil
	}

	switch field.Kind() {
	case protoreflect.BoolKind:
		return false
	case protoreflect.EnumKind:
		return int32(0)
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind:
		return int32(0)
	case protoreflect.Uint32Kind, protoreflect.Fixed32Kind:
		return uint32(0)
	case protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind:
		return int64(0)
	case protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		return uint64(0)
	case protoreflect.FloatKind:
		return float32(0)
	case protoreflect.DoubleKind:
		return float64(0)
	case protoreflect.StringKind:
		return ""
	case protoreflect.BytesKind:
		return []byte{}
	default:
		return nil
	}
}
