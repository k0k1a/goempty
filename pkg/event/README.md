# 进程内事件分发

`Bus` 复用 `pkg/async.Pool`，支持一个事件注册多个具名处理器。通用外壳 `Event` 位于本包的 `data.go`；业务 Payload 类型放在 `internal/entity`，例如 `entity/user_event.go` 中的 `UserRegisteredEvent`。已有业务对象字段和语义一致时可直接复用，不为事件机械复制一套类型。基础包不导入 internal，也不包含业务数据定义或调用业务 Service。

事件通过稳定名称区分业务，例如 `user.registered`；`Payload` 携带对应数据。处理器检查载荷类型并返回明确错误。具体业务逻辑由注入的 Service 完成，不在基础包中实现。

```go
events, err := event.New(taskPool)
if err != nil {
    return err
}
if err := events.Register("user.registered", "welcome", welcomeHandler); err != nil {
    return err
}
if err := events.Register("user.registered", "initialize-profile", profileHandler); err != nil {
    return err
}
// App 启动共享 taskPool 后调用 events.Start()，冻结注册。
err = events.Publish(ctx, event.Event{Name: "user.registered", Payload: userID})
```

处理器签名是 `func(context.Context, event.Event) error`。发布者和处理器约定载荷类型；`Payload` 可以为空，不对任意对象进行反射匹配、序列化或自动转换。发布后不要修改载荷及其引用的 map、slice、指针数据；所有处理器也应把载荷视为只读数据。

- `Register` 只允许在 Start 前调用，同一事件内处理器名称必须唯一；注册失败让应用初始化失败。Start 重复调用无副作用。
- 一个事件作为一个任务提交，占用一个 Worker，处理器按注册顺序执行，共享整个事件的执行超时。不同事件可能并行，没有全局顺序。
- 单个处理器返回错误或 panic 时，记录 `event_name`、`handler_name`、耗时、request/trace ID，panic 另带堆栈，然后继续后续处理器；处理器仍须响应 Context 取消。
- 发布成功只表示已接收，不代表所有处理器执行成功。Worker 全忙时，Publish 阻塞等待；等待人数达到上限、没有处理器、未启动或池已关闭时返回错误，调用方必须处理。等待行为与任务池一致，不提供提交等待超时。
- 不要在事件处理器中同步向同一个池发布新事件或提交任务，池饱和时可能因嵌套阻塞而死锁。
- 一个处理器较慢会延迟同一事件的后续处理器；超时对整个事件生效，不为每个处理器重置。后续处理器可能收到已取消的 Context。
- 不持久化、不自动重试、不跨进程分发；业务数据库写入与事件发布不构成原子操作。

Wire 默认创建共享 Bus，由 App 启动。接入真实业务时，在 `internal/app` 的 `newApp` 中注册 Handler，将处理器需要的 Service 加入该构造函数参数，然后运行 `make wire`。注册失败应直接返回错误，注册必须在 App.Run 前完成。Service 或其他发布者通过构造函数接收 Bus，不通过 App 或全局变量查找；Bus 的创建不依赖 Service，以免与发布者形成依赖循环。

Bus 不拥有 Pool，也不单独停止 Pool。App 退出时停止任务池，所有事件继承其取消及关闭规则，详见[任务池](../async/README.md)。
