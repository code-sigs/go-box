package errs

import (
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
)

// WrapError 定义错误类型
type WrapError struct {
	msg   string
	code  int
	file  string
	line  int
	cause error
}

// New 创建新错误，不包含 cause 和 code
func New(msg string) error {
	_, file, line, ok := runtime.Caller(1)
	if !ok {
		file = "unknown"
		line = 0
	}
	return &WrapError{
		msg:  msg,
		file: shortPath(file, 3),
		line: line,
	}
}

// Wrap 包装错误，msg 可为空，不为空则表示本层错误描述
func Wrap(err error, msgs ...string) error {
	if err == nil {
		return nil
	}
	_, file, line, ok := runtime.Caller(1)
	if !ok {
		file = "unknown"
		line = 0
	}
	//if msg == "" {
	//	msg = err.Error()
	//}
	msg := ""
	if len(msgs) > 0 {
		msg = msgs[0]
	}
	return &WrapError{
		msg:   msg,
		file:  shortPath(file, 3),
		line:  line,
		cause: err,
	}
}

// WithCode 为错误设置 code
func WithCode(err error, code int) error {
	if err == nil {
		return nil
	}
	var w *WrapError
	ok := errors.As(err, &w)
	if !ok {
		// 不是 WrapError，重新包装一个
		return &WrapError{
			msg:   err.Error(),
			code:  code,
			file:  "unknown",
			line:  0,
			cause: err,
		}
	}
	w.code = code
	return w
}

func (e *WrapError) Error() string {
	if e.code != 0 {
		return fmt.Sprintf("%s:%d [%d] %s", e.file, e.line, e.code, e.msg)
	}
	return fmt.Sprintf("%s:%d %s", e.file, e.line, e.msg)
}

func (e *WrapError) Unwrap() error {
	return e.cause
}

func (e *WrapError) Code() int {
	return e.code
}

// Format 实现 %+v 打印完整错误链（避免末尾多余箭头）
func (e *WrapError) Format(s fmt.State, verb rune) {
	switch verb {
	case 'v':
		if s.Flag('+') {
			// 直接遍历并写入，不构建中间切片
			err := error(e)
			for {
				if we, ok := err.(*WrapError); ok {
					if we.code == 0 {
						fmt.Fprintf(s, "%s:%d: %s", we.file, we.line, we.msg)
					} else {
						fmt.Fprintf(s, "%s:%d: [%d] %s", we.file, we.line, we.code, we.msg)
					}
					err = we.Unwrap()
					if err != nil {
						fmt.Fprint(s, " -> ")
					} else {
						break
					}
				} else if err != nil {
					fmt.Fprint(s, err.Error())
					break
				} else {
					break
				}
			}
			return
		}
		fallthrough
	case 's':
		fmt.Fprint(s, e.Error())
	case 'q':
		fmt.Fprintf(s, "%q", e.Error())
	}
}

// shortPath 取文件路径最后 n 级目录
func shortPath(path string, n int) string {
	parts := strings.Split(filepath.ToSlash(path), "/")
	if len(parts) <= n {
		return strings.Join(parts, "/")
	}
	return strings.Join(parts[len(parts)-n:], "/")
}

func Stack(err error) string {
	return fmt.Sprintf("%+v", err)
}

// IsWrapError 判断给定的 error 是否是 *WrapError 类型
func IsWrapError(err error) bool {
	if err == nil {
		return false
	}
	var wrapError *WrapError
	ok := errors.As(err, &wrapError)
	return ok
}

// AsWrapError 尝试将 error 转换为 *WrapError，返回是否成功和对应的值
func AsWrapError(err error) (*WrapError, bool) {
	if err == nil {
		return nil, false
	}
	var we *WrapError
	ok := errors.As(err, &we)
	return we, ok
}
