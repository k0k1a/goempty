# HTTP 公共响应

普通 JSON 接口使用 HTTP 状态码表达结果大类，响应体使用 `code/message/data`。响应体中的 `request_id` 为预留可选字段，`Success` 和 `Failure` 当前不填充，因此不会输出该字段。应用已通过 RequestID 中间件取得或生成请求 ID，写入 `X-Request-ID` 响应头并关联请求日志。没有 `details` 字段。

`code` 的 Go 类型为 `int`，JSON 中为数字，供前端判断具体业务结果，与 HTTP 状态码独立。预定义业务错误显式指定编号与默认文案，避免新增或调整顺序影响已有编号：

| 场景 | HTTP 状态码 | code |
| --- | --- | --- |
| 成功 | 200 / 201 | 0 |
| 参数错误 | 400 | 100001 |
| 未认证 | 401 | 100002 |
| 无权限 | 403 | 100003 |
| 资源不存在 | 404 | 100004 |
| 状态冲突 | 409 | 100005 |
| 违反业务规则 | 422 | 100006 |
| 请求限流 | 429 | 100007 |
| 服务不可用 | 503 | 100008 |
| 未知错误 | 500 | -1 |

路由不存在复用 `100004`；请求方法不支持返回 HTTP 405，复用请求错误码 `100001`。后续具体业务可定义独立编号，例如 `200001`，同时登记对应 HTTP 状态码。前端依赖 code，不依赖 message 文案。

成功使用 `response.Success(c, http.StatusOK, result)`；创建资源时使用 201，并由调用方设置 Location。空列表应传入非 nil slice 以输出 `[]`。204 使用 `c.Status(http.StatusNoContent)`，不写外壳。文件和流式响应不使用此封装。

业务错误定义在 `service/errors.go`，只包含稳定 code 和可公开的 message，不包含 HTTP 状态码。将编号与默认文案一起创建，Service 直接返回预定义错误：

```go
// service/errors.go
var ErrNotFound = NewError(100004, "资源不存在")

// Service 方法中直接返回；有结果返回值时使用 return nil, ErrNotFound。
return ErrNotFound
```

预定义错误为共享实例，使用时不得修改其 Code/Message 字段或重新赋值。需要补充内部上下文时使用 `fmt.Errorf("lookup failed: %w", ErrNotFound)`，公开文案仍取预定义错误的 Message。新增业务错误使用 `NewError(code, message)` 创建，并在 controller 中登记 HTTP 状态映射。

Controller 统一调用：

```go
result, err := h.service.Get(c.Request.Context(), id)
if err != nil {
    controller.RespondError(c, err) // controller 包内直接使用 RespondError
    return
}
response.Success(c, http.StatusOK, result)
```

`RespondError` 使用 `errors.As` 提取被 `%w` 包装的 `*service.Error`，`businessStatus` 使用 `errors.Is` 判断是否为预定义错误并选择 HTTP 状态码。新增预定义错误时必须同步添加匹配分支，多个错误可以映射为同一 HTTP 状态码。临时创建的新错误即使 code/message 与预定义错误相同，也不会匹配；未登记的错误和普通 error 一律返回 `HTTP 500 + code -1 + 未知错误`，原始错误记录日志。不要将数据库、令牌等敏感内容填入公开 message 或错误日志。

参数绑定失败时使用 `RespondError(c, service.ErrInvalidArgument)`，不直接暴露绑定器的原始错误。输出错误后必须 return；Abort 只能停止后续 Gin handler，不能终止当前函数。

401 的认证 challenge（WWW-Authenticate）由认证入口按实际认证方案设置；429/503 的 Retry-After 由掌握等待时间的调用方设置。当前项目未接入这些入口。

404、405 和响应尚未写出时的 panic 使用相同外壳。响应写出后的 panic 只记录日志并终止后续 handler，不能替换已经发送的内容。Service 不依赖 controller/response 或 Gin。
