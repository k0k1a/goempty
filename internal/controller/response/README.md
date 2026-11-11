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
| 动态公开业务错误 | 422 | -1 |
| 未知错误 | 500 | -1 |

路由不存在复用 `100004`；请求方法不支持返回 HTTP 405，复用请求错误码 `100001`。后续具体业务可定义独立编号，例如 `200001`，同时登记对应 HTTP 状态码。前端依赖 code，不依赖 message 文案。

成功使用 `response.Success(c, http.StatusOK, result)`；创建资源时使用 201，并由调用方设置 Location。空列表应传入非 nil slice 以输出 `[]`。204 使用 `c.Status(http.StatusNoContent)`，不写外壳。文件和流式响应不使用此封装。

错误类型、公共错误与具体业务错误统一定义在 `entity/error.go`，只包含稳定 code 和可公开的 message，不包含 HTTP 状态码。将编号与默认文案一起创建，Service 直接返回预定义错误：

```go
// entity/error.go
var ErrNotFound = NewError(100004, "资源不存在")

// Service 方法中直接返回；有结果返回值时使用 return nil, entity.ErrNotFound。
return entity.ErrNotFound
```

预定义错误为共享实例，使用时不得修改其 Code/Message 字段或重新赋值。需要补充内部上下文时使用 `fmt.Errorf("lookup failed: %w", entity.ErrNotFound)`，公开文案仍取预定义错误的 Message。新增业务错误在 `entity/error.go` 中使用 `NewError(code, message)` 创建，并在 `controller/response/error.go` 中登记 HTTP 状态映射。

Controller 统一调用：

```go
result, err := h.service.Get(c.Request.Context(), id)
if err != nil {
    response.RespondError(c, err)
    return
}
response.Success(c, http.StatusOK, result)
```

`response.RespondError` 使用 `errors.As` 提取被 `%w` 包装的 `*entity.Error`，`businessStatus` 按 Code 映射 HTTP 状态码。相同 code 的不同错误实例使用相同 HTTP 状态，响应 message 取当前错误的公开文案；新增独立 code 时需登记映射。未登记 code 和普通 error 返回 `HTTP 500 + code -1 + 未知错误`，原始错误记录日志。

可使用 `entity.NewBusinessError("库存不足")` 返回动态公开消息，映射为 HTTP 422、code `-1`；也支持 `%w` 包装。它与未知内部错误共享 code `-1`，需要结合 HTTP 状态区分。只传入明确允许公开的文本，禁止以数据库错误、令牌或其他内部错误原文作为 message。不要在日志中输出敏感数据。

参数绑定与格式校验失败时使用 `response.RespondBindError(c, err)`，只传入绑定方法返回的非 nil 错误：

```go
var req entity.CreateUserReq // 按实际业务定义请求对象。
if err := c.ShouldBindJSON(&req); err != nil {
    response.RespondBindError(c, err)
    return
}
```

使用 `ShouldBindJSON`、`ShouldBindQuery`、`ShouldBind` 等返回错误且不自动写响应的方法；不要使用会提前写出 HTTP 400 的 `Bind*` 方法。`RespondError` 继续处理业务错误，不接收原始绑定错误。输出错误后必须 return；Abort 只能停止后续 Gin handler，不能终止当前函数。

应用在 HTTP Server 构造阶段调用 `response.InitBindingValidator()`，通过官方 `RegisterTagNameFunc` 配置 Gin 共享 validator；注册只执行一次，必须早于任何绑定或校验，之后不得替换共享 validator。独立使用响应包的测试或入口也需先初始化并检查返回错误。不支持的底层校验引擎导致初始化失败。

字段名由 validator 在解析类型并建立缓存时提取，响应阶段不再反射遍历请求对象，也不引入翻译器或维护各规则的翻译文案。公开名称取非空的 `json` 或 `form` 标签，去掉标签选项；同一字段同时存在两种名称时必须一致。名称确实不同时使用不同请求对象；冲突、隐藏标签（`-`）、缺失标签和匿名嵌入字段统一标记为不可公开，不回退到 Go 字段名。

绑定错误返回 HTTP 400、code `100001`、data `null`，不新增字段。`validator.ValidationErrors`（包括 `%w` 包装）只处理第一条错误，message 使用该字段的 `json` / `form` 标签值加固定文案 `格式校验失败`，例如 `"name 格式校验失败"`；不区分校验规则，不返回 Go 字段名、规则名、规则参数、请求值、内部结构体名称或原始错误。

嵌套对象只显示叶子字段名，不输出完整路径；含隐藏祖先、切片/数组下标或 Map 键的路径统一返回 `请求参数不合法`。JSON 语法错误、类型不匹配、空请求体和 Query/Form 类型转换失败也使用该通用提示。多字段错误顺序取 validator 返回顺序，不承诺固定某字段优先，客户端通过 code 判断参数错误，message 用于展示字段校验提示；message 为通用兜底时按普通参数错误展示。

`required` 沿用 validator 的零值语义：数值 `0`、布尔值 `false` 不能满足非指针字段的 `required`。需要区分“未传”和“传入零值”时使用指针字段。业务校验仍由 Service 保证，不使用 `NewBusinessError` 转换输入绑定错误，以免变成 HTTP 422。

正常绑定/校验失败不逐次记录错误日志，也不回显原始错误、请求值、Map 键或 Go 类型名。可识别的 `validator.InvalidValidationError` 和误传 nil 错误返回 HTTP 500、code `-1`、`未知错误`，只记录错误类型及代码位置，不记录原始错误。此入口只用于绑定失败，不能用它把 Service 或 Repo 的普通内部错误转换为 400。

401 的认证 challenge（WWW-Authenticate）由认证入口按实际认证方案设置；429/503 的 Retry-After 由掌握等待时间的调用方设置。当前项目未接入这些入口。

404、405 和响应尚未写出时的 panic 使用相同外壳。响应写出后的 panic 只记录日志并终止后续 handler，不能替换已经发送的内容。Service 不依赖 controller/response 或 Gin。
