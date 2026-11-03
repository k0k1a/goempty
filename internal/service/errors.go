package service

// Predefined errors pair stable API codes with public messages.
// These shared instances must not be modified. HTTP status mapping belongs to controller.
var (
	ErrInvalidArgument    = NewError(100001, "请求参数不合法")
	ErrUnauthenticated    = NewError(100002, "请先登录")
	ErrForbidden          = NewError(100003, "无权限")
	ErrNotFound           = NewError(100004, "资源不存在")
	ErrConflict           = NewError(100005, "状态冲突")
	ErrUnprocessable      = NewError(100006, "不满足业务规则")
	ErrRateLimited        = NewError(100007, "请求过于频繁")
	ErrServiceUnavailable = NewError(100008, "服务暂时不可用")
)

// Error carries a business code and a message safe to expose to callers.
// Do not use database errors or other internal details as Message.
type Error struct {
	Code    int
	Message string
}

// NewError creates an error with a stable code and a public message.
func NewError(code int, message string) *Error {
	return &Error{Code: code, Message: message}
}

func (e *Error) Error() string {
	return e.Message
}
