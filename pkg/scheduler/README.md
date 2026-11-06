# 定时任务

基于 `github.com/robfig/cron/v3` 的进程内调度器。一个对象注册多个任务，统一启动和停止。每个进程独立执行，多副本部署不保证全局唯一执行。

```go
s, err := scheduler.New(scheduler.Config{}) // 跟随入口设置的 time.Local
if err != nil {
    return err
}
if err := s.Register(scheduler.Task{
    Name:    "cleanup-expired",
    Spec:    "@every 10m",
    Timeout: time.Minute,
    Handler: cleanupService.CleanupExpired, // func(context.Context) error
}); err != nil {
    return err
}
if err := s.Start(ctx); err != nil {
    return err
}
// 应用关闭时，使用独立于运行 context 的关闭超时。
<-ctx.Done()
stopCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
defer cancel()
return s.Stop(stopCtx)
```

- `New` 不启动 goroutine；零配置时区跟随构造时的 `time.Local`。应用入口在启动组件前统一设置为 `Asia/Shanghai`，二进制内置时区数据。需要覆盖时传入 `Config{Timezone: "UTC"}` 等配置；独立使用此包时，由调用方设置进程时区或显式指定 Timezone。
- `Register` 仅在启动前可用。名称非空、无首尾空白且唯一；Handler 必填，Timeout 不可为负数。非法表达式注册失败，不占用任务名。
- 使用标准五字段表达式：`分 时 日 月 周`，例如 `0 2 * * *`；也支持 `@daily`、`@every 10m`，不接受六字段秒级表达式。`@every` 最小调度精度为秒，首次执行等待下一触发点，启动时不立即执行。
- 不同任务并行；同一注册任务执行未结束时跳过后续触发，不排队、不补跑。同一个 Handler 以不同名称注册时视为不同任务。
- 任务执行错误、panic（含堆栈）、成功耗时和跳过均通过 `pkg/log` 记录。panic 恢复后仍能继续调度。任务自行启动的 goroutine 不在恢复和等待范围内。
- Start 的 context 传递给任务；每次运行按 Timeout 创建子 context，零值不增加超时。Handler 必须向下传递 context 并响应取消。超时不会强制结束 goroutine，不会自动重试。
- 重复 Start 不重复启动；必须调用 Stop 完成调度器清理，单独取消运行 context 不替代 Stop。Stop 可以在启动前调用；停止后不能重启或注册。
- Stop 先停止调度、取消运行 context，再等待已有任务退出。关闭 context 只限制等待时间；超时返回可通过 `errors.Is` 识别的 context 错误，再次 Stop 可继续等待。停止超时后任务可能仍在运行，应避免任务忽略取消后继续访问已释放的依赖。

应用在 `internal/job/register.go` 集中组装任务：把业务 Service 加入 `NewScheduler` 构造参数，将任务加入 `registrations`，再重新生成 Wire。默认 Hello Job 每分钟整点调用 HelloService 并输出日志，超时 5 秒，不写业务数据；多实例会各自执行。
