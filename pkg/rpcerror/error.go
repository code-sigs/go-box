package rpcerror

import (
	"runtime"
	"strconv"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/anypb"
)

// UnWrap 尝试从 error 中提取业务 RPCError
func UnWrap(err error) *RPCError {
	st, ok := status.FromError(err)
	if !ok {
		return nil
	}
	for _, d := range st.Details() {
		if anyVal, ok := d.(*anypb.Any); ok && strings.HasSuffix(string(anyVal.MessageName()), "RPCError") {
			var sr RPCError
			if err := anyVal.UnmarshalTo(&sr); err == nil {
				return &sr
			}
		}
	}
	return nil
}

// Wrap 将 RPCError 包装为 gRPC 结构化 error
func Wrap(e *RPCError) error {
	if e == nil {
		return nil
	}
	detailAny, err := anypb.New(e)
	if err != nil {
		return status.Errorf(codes.Internal, "wrap error failed: %v", err)
	}
	st := status.New(codes.Internal, e.Message)
	stWithDetail, err := st.WithDetails(detailAny)
	if err != nil {
		return st.Err()
	}
	return stWithDetail.Err()
}

// WrapCode 返回结构化错误，可指定 gRPC code
func WrapCode(code int32, msg string) error {
	e := &RPCError{
		Code:    code,
		Message: msg,
	}
	if code != 0 {
		if pc, _, line, ok := runtime.Caller(1); ok {
			funcName := runtime.FuncForPC(pc).Name()
			//e.Details = " [" + file + ":" + funcName + ":" + strconv.Itoa(line) + "]"
			// 只保留 file 的最后3级目录
			// fileParts := strings.Split(file, "/")
			// if len(fileParts) > 3 {
			// 	file = strings.Join(fileParts[len(fileParts)-3:], "/")
			// }

			// 只保留 funcName 的最后3级
			funcParts := strings.Split(funcName, "/")
			if len(funcParts) > 3 {
				funcName = strings.Join(funcParts[len(funcParts)-3:], "/")
			}
			e.Details = funcName + ":" + strconv.Itoa(line)
		}
	}
	detailAny, err := anypb.New(e)
	if err != nil {
		return status.Errorf(codes.Internal, "wrap error failed: %v", err)
	}
	st := status.New(codes.Internal, msg)
	stWithDetail, err := st.WithDetails(detailAny)
	if err != nil {
		return st.Err()
	}
	return stWithDetail.Err()
}

func WrapError(err error, msgs ...string) error {
	if err == nil {
		return nil
	}
	e := UnWrap(err)
	if e == nil {
		return WrapCode(404, "未定义错误: "+err.Error())
	} else {
		if len(msgs) > 0 {
			e.Message = e.Message + ", " + strings.Join(msgs, ", ")
		}
		if pc, _, line, ok := runtime.Caller(1); ok {
			funcName := runtime.FuncForPC(pc).Name()
			//e.Details = " [" + file + ":" + funcName + ":" + strconv.Itoa(line) + "]"
			// 只保留 file 的最后3级目录
			// fileParts := strings.Split(file, "/")
			// if len(fileParts) > 3 {
			// 	file = strings.Join(fileParts[len(fileParts)-3:], "/")
			// }

			// 只保留 funcName 的最后3级
			funcParts := strings.Split(funcName, "/")
			if len(funcParts) > 3 {
				funcName = strings.Join(funcParts[len(funcParts)-3:], "/")
			}
			if e.Details == "" {
				e.Details = funcName + ":" + strconv.Itoa(line)
			} else {
				e.Details = e.Details + "->" + funcName + ":" + strconv.Itoa(line)
			}
		}
		return Wrap(e)
	}
}

// IsRPCError 判断 error 是否为业务 RPCError
func IsRPCError(err error) bool {
	st, ok := status.FromError(err)
	if !ok {
		return false
	}
	for _, d := range st.Details() {
		if anyVal, ok := d.(*anypb.Any); ok && strings.HasSuffix(string(anyVal.MessageName()), "RPCError") {
			return true
		}
	}
	return false
}
