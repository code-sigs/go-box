package errs

const (
	ErrorInternal     = 500000 //系统异常
	ErrorArgs         = 500001 //参数错误
	ErrorNotFound     = 500002 //记录不存在
	ErrorNoPermission = 500004 //无操作权限
	ErrorNoUser       = 500005 //用户不存在
	ErrorPassword     = 500006 //密码错误
	ErrorInvalidToken = 500007 //无效token
)
