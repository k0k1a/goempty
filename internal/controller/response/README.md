# HTTP 公共响应

普通 JSON 接口成功固定返回 HTTP 200，失败固定返回 HTTP 400，响应体使用 `code/message/data`，通过 code 区分具体业务结果。响应体中的 `request_id` 为预留可选字段，`Success` 和 `Failure` 当前不填充，因此不会输出该字段。应用已通过 RequestID 中间件取得或生成请求 ID，写入 `X-Request-ID` 响应头并关联请求日志。没有 `details` 字段。

`code` 的 Go 类型为 `int`，JSON 中为数字，供前端判断具体业务结果，与 HTTP 状态码独立。预定义业务错误显式指定编号与默认文案，避免新增或调整顺序影响已有编号：

| 场景 | HTTP 状态码 | code |
| --- | --- | --- |
| 成功 | 200 | 0 |
| 参数错误 | 400 | 100001 |
| 未认证 | 400 | 100002 |
| 无权限 | 400 | 100003 |
| 资源不存在 | 400 | 100004 |
| 状态冲突 | 400 | 100005 |
| 违反业务规则 | 400 | 100006 |
| 请求限流 | 400 | 100007 |
| 服务不可用 | 400 | 100008 |
| 动态公开业务错误 | 400 | -1 |
| 未知错误 | 400 | -1 |

路由不存在和请求方法不支持均返回 HTTP 400，分别复用 `100004` 和 `100001`。后续具体业务可定义独立编号，例如 `200001`，无需登记 HTTP 状态映射。前端依赖 code，不依赖 message 文案。

成功使用 `response.Success(c, result)`，创建资源也返回 HTTP 200。空列表应传入非 nil slice 以输出 `[]`。204 使用 `c.Status(http.StatusNoContent)`，不写外壳。文件和流式响应不使用此封装。

错误类型、公共错误与具体业务错误统一定义在 `entity/error.go`，只包含稳定 code 和可公开的 message，不包含 HTTP 状态码。将编号与默认文案一起创建，Service 直接返回预定义错误：

```go
// entity/error.go
var ErrNotFound = NewError(100004, "资源不存在")

// Service 方法中直接返回；有结果返回值时使用 return nil, entity.ErrNotFound。
return entity.ErrNotFound
```

预定义错误为共享实例，使用时不得修改其 Code/Message 字段或重新赋值。需要补充内部上下文时使用 `fmt.Errorf("lookup failed: %w", entity.ErrNotFound)`，公开文案仍取预定义错误的 Message。新增业务错误在 `entity/error.go` 中使用 `NewError(code, message)` 创建，无需在响应层登记 code。

Controller 统一调用：

```go
result, err := h.service.Get(c.Request.Context(), id)
if err != nil {
    response.RespondError(c, err)
    return
}
response.Success(c, result)
```

`response.RespondError` 使用 `errors.As` 提取被 `%w` 包装的非 nil `*entity.Error`，直接返回该错误的 Code 和 Message，HTTP 状态固定为 400。所有业务错误 code 均可使用，无需维护白名单或状态映射，因此每个 `entity.Error.Message` 都必须允许公开。普通 error 和类型化 nil 业务错误返回 `HTTP 400 + code -1 + 未知错误`，原始错误记录日志。

可使用 `entity.NewBusinessError("库存不足")` 返回动态公开消息，HTTP 400、code `-1`；也支持 `%w` 包装。它与未知内部错误共享 code 和 HTTP 状态，客户端无法据此区分二者；需要稳定区分的业务结果应使用独立 code。只传入明确允许公开的文本，禁止以数据库错误、令牌或其他内部错误原文作为 message。不要在日志中输出敏感数据。

直接输出错误时使用 `response.Failure(c, code, message)`，它固定写入 HTTP 400、data `null` 并终止后续 Gin handler；当前函数仍需 return。

参数绑定与格式校验沿用 Gin 默认行为及 `binding` 标签。失败时直接返回统一参数错误：

```go
var req entity.CreateUserReq // 按实际业务定义请求对象。
if err := c.ShouldBindJSON(&req); err != nil {
    response.RespondError(c, entity.ErrInvalidArgument)
    return
}
```

使用 `ShouldBindJSON`、`ShouldBindQuery`、`ShouldBind` 等返回错误且不自动写响应的方法，失败时统一返回 HTTP 400、code `100001`、message `请求参数不合法`、data `null`。不要直接把原始绑定错误传给 `RespondError` 或回显给客户端。输出错误后必须 return；Abort 只能停止后续 Gin handler，不能终止当前函数。

业务规则仍由 Service 保证。

公共响应不按业务 code 设置 WWW-Authenticate、Location 或 Retry-After 等响应头；具体业务如有协议需求，应另行设计。

路由不存在、方法不支持和响应尚未写出时的 panic 均使用 HTTP 400 和相同外壳。响应写出后的 panic 只记录日志并终止后续 handler，不能替换已经发送的内容。Service 不依赖 controller/response 或 Gin。
