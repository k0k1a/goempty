# 进程内异步任务池

`Pool` 使用 Ants v2 限制并发，由 App 统一启停。构造函数只校验配置，`Start` 才创建实际池。不同任务可并行执行，不保证提交顺序。

```go
tasks, err := async.New(async.Config{Concurrency: 4, MaxBlockingTasks: 1000, Timeout: 30 * time.Second})
if err != nil {
    return err
}
if err := tasks.Start(appCtx); err != nil {
    return err
}
err = tasks.Submit(requestCtx, "report.generate", func(ctx context.Context) error {
    return reportService.Generate(ctx, reportID)
})
```

业务通过构造函数接收共享 Pool；使用它的业务对象不单独 Start/Stop。任务名称使用稳定业务标识，不拼接用户输入或资源 ID。提交成功仅表示已接收，执行结果通过日志观察。

- `Concurrency` 为零默认 4，`MaxBlockingTasks` 为零默认 1000，`Timeout` 为零默认 30 秒；负数配置和超出 int32 的并发数会被拒绝。
- Worker 全忙时，提交方阻塞等待空闲 Worker；已有 `MaxBlockingTasks` 个等待者时，新提交返回 `ErrOverloaded`。限制的是阻塞在 Submit 上的 goroutine 数量，不另建任务队列，调用方不能忽略提交错误。
- 启动前返回 `ErrNotStarted`，停止后返回 `ErrClosed`。调用 Submit 前已取消的 Context 会被拒绝。Ants 原生阻塞期间不响应提交 Context 的取消，也没有提交等待超时；`Timeout` 仅约束任务执行。
- 接收后保留提交 Context 的值，移除其取消和 deadline，允许任务在 HTTP 请求结束后继续；执行受任务超时及应用 Context 取消控制。超时从 Worker 开始执行算起。
- Handler 必须向下传递 Context 并响应取消。闭包中不能持有 Gin Context、ResponseWriter 等请求期对象；捕获的切片、map、指针对象不能被调用方并发修改。
- 开始、完成、错误和 panic 接入现有日志，包含 `task_name`、执行耗时及已有 request/trace ID；panic 日志包含堆栈。错误只记录一次，不自动重试。

不要在池内任务中同步向同一个池再次提交任务：所有 Worker 都在等待嵌套任务时会死锁。

`Stop(ctx)` 先同步关闭底层池，唤醒阻塞提交者并返回 `ErrClosed`，再取消任务，限时等待执行中的 Handler 和提交调用退出；Ants 维护 goroutine 由底层池关闭时取消。超时返回错误，之后可以再次 Stop 等待；并发 Stop 安全，停止后不能重启。独立使用时，即使运行 Context 已取消，也必须使用关闭专用 Context 调用 Stop。

只保证进程存活期间的执行管理，不提供持久化、故障恢复或强制终止。超时不会强杀 goroutine；不响应取消的任务可能越过应用关闭时限并访问已关闭的依赖。
