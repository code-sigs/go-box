package box

import (
	"github.com/code-sigs/go-box/pkg/router"
)

type Box struct {
	Router *router.Router
}

// New 创建一个新的 Box 实例
func New() *Box {
	return &Box{
		Router: router.New(),
	}
}
