# 应用组装

HTTP 中间件按 RequestID → AccessLog → Recovery → CORS 的顺序注册。访问日志在请求处理完成后输出 `HTTP request completed`，包含 `method`、`route`、`status`、`duration_us` 和上下文中的 `request_id`；`route` 使用匹配的路由模板，未匹配时为空，不记录实际 URL、查询参数、请求体或请求头。AccessLog 位于 Recovery 外层，因此 panic 恢复后也会记录最终响应状态。

Panic 错误日志额外包含 `stack`，保留原始 panic 调用位置；响应已经写出时不覆盖响应，只记录错误并中止后续处理。

Wire 生成依赖组装代码，`App` 管理运行组件的生命周期。当前接入 MySQL、Redis、MongoDB、Gin、HTTP Server、定时调度器和 Hello Controller → Service → Repo 示例。

- `wire.go`：维护 Wire provider 和私有 injector `initialize(ctx, cfg)`，返回 `(*App, cleanup, error)`，创建 `HelloRepo`，将它绑定到 `db.HelloRepository` 接口，再创建 `HelloService`、`httpDependencies`、HTTP Server、定时调度器、MySQL 连接、Redis 和 MongoDB 客户端及 App，此时尚未监听 HTTP 或启动调度。不再维护单独的 `app.Config`。
- `providers.go`：公开的 `Initialize(ctx, cfg)` 返回 `(*App, error)`，调用私有 `initialize` 并将清理函数交给 App。集中定义 `newDatabase`、`newRedis`、`newMongoDB` 和 `newApp`，分别复用对应 pkg 包创建共享连接并提供清理函数；Redis 和 MongoDB 初始化会执行 Ping。`newApp` 调用 `New` 校验运行参数，再把连接保存到 App，使 Wire 能识别这些依赖并生成初始化和清理代码。后续业务消费者通过构造函数接收连接，不通过全局变量访问。
- `wire_gen.go`：生成文件，不手工修改；普通构建不需要执行 Wire。
- `app.go`：`App.Run(ctx)` 启动定时调度器并运行 HTTP 服务；取消或 HTTP 退出时停止调度、取消运行任务，在同一个独立关闭超时内并行等待 HTTP 请求和定时任务完成。HTTP 超时强制关闭连接；任务超时返回错误，不能强制终止不响应 context 的 Handler。退出前调用 `App.Close()` 释放依赖，提前取消和启动失败也会清理；`Close()` 通过 `sync.Once` 保证清理只执行一次。
- `http.go`：`newHTTPHandler(deps)` 手写创建 Gin Engine 和 Controller，按上述顺序装配中间件并集中注册路由；`newHTTPServer(cfg, deps)` 将 Gin Engine 交给 `pkg/httpserver`。Controller 只处理请求，不负责注册路由，也不由 Wire 创建。RequestID、AccessLog 和 Recovery 实现在 `internal/controller/middleware/middleware.go`，直接依赖公共 response 和日志，不反向依赖 controller 包。
- `App` 不作为 service/controller 注册表。后续通过构造函数把 service 注入 controller、consumer、job，再将需要启停的组件注入 App。

应用层私有的 `httpDependencies` 对象由 Wire 填充 Service；手写创建 Controller 时只传入它需要的 Service，不传入整个依赖对象。当前只包含 `HelloService`。

Hello 示例调用链为 `GET /hello → HelloController.Hello → HelloService.Hello → HelloRepo.Hello`。仓储接口 `db.HelloRepository` 与具体实现 `HelloRepo` 均定义在 `internal/repo/db/hello.go`，直接返回 `hello`，不访问外部存储。Service 导入 Repo 包并依赖接口，Repo 不反向导入 Service。

`cmd/app/main.go` 是可运行入口，负责监听 `os.Interrupt`（如 Ctrl+C）和 `SIGTERM`，将取消通知传给 `App.Run`，等待关闭完成后退出；初始化或运行失败时统一记录错误并以状态码 1 退出。

生命周期日志复用 `pkg/log`，默认以 JSON 同时输出到 stdout 和 `logs/{APP_NAME}_{ENV}.log`，级别由 `LEVEL` 决定（未设置或为空时为 info），消息统一使用英文：`Application starting`、端口监听成功后的 `HTTP server started`、`Shutting down application`、正常退出时的 `Application stopped`。初始化、运行或关闭失败时由入口记录一次 `Application exited with an error`。最终日志写入后再调用 `log.Shutdown()`；日志关闭本身失败时向 stderr 输出错误并以状态码 1 退出。

```sh
go run ./cmd/app
```

入口使用 `config.Load()` 加载 `configs/{APP_NAME}_{ENV}.yaml`。`APP_NAME` 未设置或为空时默认 `app`，`ENV` 未设置或为空时默认 `dev`；路径以启动工作目录为基准。目标文件不存在或非法时启动失败，不回退到其他环境，不再提供 `-config` 参数。

入口只通过环境变量 `PORT` 决定端口，未设置时默认 `8080`；显式设置时必须是 `1～65535` 的整数，空值或非法值启动失败，监听地址为 `:端口`。HTTP 超时固定在入口代码中：ReadHeaderTimeout 为 5 秒，ReadTimeout/WriteTimeout 为 15 秒，IdleTimeout 为 60 秒；不读取 YAML 或 `GOEMPTY_HTTP_*` 环境变量。

关闭超时在入口固定为 10 秒，不从 YAML 或环境变量读取。MySQL DSN 和连接池参数从 YAML 的 `mysql` 段读取，Redis 连接参数从 `redis` 段读取，MongoDB 连接参数从 `mongodb` 段读取，不提供环境变量覆盖。本地配置见 `configs/app_dev.yaml`，需先替换本地密码并准备 MySQL、Redis 和 MongoDB；test/prod 文件保留空模板，不保存线上配置，使用前需填写。完整说明见 [应用配置](../../config/README.md)。

`config.Load()` 同时启动 Viper 文件监听；变化回调直接使用同一 Viper 实例重新反序列化，成功才替换内存配置，失败保留旧值。需要动态配置的业务通过 `config.Get()` 读取快照，不重启应用或重新执行 Wire 组装。

`GET /hello` 返回 HTTP 200 和 JSON `{"code":0,"message":"success","data":{"message":"hello"}}`。未注册路径返回 404，不支持的方法返回 405，panic 返回 500；响应尚未写出时统一使用 JSON 外壳，code 为独立于 HTTP 状态码的整数业务码。错误解析与输出统一通过 `response.RespondError`，约定见 [HTTP 公共响应](../controller/response/README.md)。部署时建议先 `go build -o app ./cmd/app`，直接运行编译产物，并将退出信号发送给应用进程。Wire 接收 context 和配置结构体，不负责加载文件。

更新依赖组装后，在项目根目录运行：

```sh
go run github.com/google/wire/cmd/wire@v0.7.0 ./internal/app
```

入口调用示例（配置由入口准备，Gin、中间件、路由和依赖在应用层内部组装）：

```go
ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
defer stop()

cfg, err := config.Load()
if err != nil {
    return err
}
cfg.HTTP = httpserver.Config{
    Addr:              ":8080",
    ReadHeaderTimeout: 5 * time.Second,
    ReadTimeout:       15 * time.Second,
    WriteTimeout:      15 * time.Second,
    IdleTimeout:       60 * time.Second,
}
cfg.ShutdownTimeout = 10 * time.Second
application, err := app.Initialize(ctx, cfg)
if err != nil {
    return err
}
defer application.Close()
return application.Run(ctx)
```

`Run` 仅调用一次；取消属于正常退出，启动或关闭失败返回错误。必须等待 `Run` 返回后才能退出进程。强制关闭连接不会终止 handler 的 goroutine，handler 应响应请求 context 的取消。

MySQL、Redis 或 MongoDB 连接失败时应用启动失败。Wire 在后续装配失败时释放已创建的连接池（Redis 初始化失败时释放 MySQL，MongoDB 初始化失败时释放 Redis 和 MySQL）；成功初始化后由 App 持有 Wire 的清理函数。`Run` 完成 HTTP 和任务关闭（或关闭超时）后通过 `Close()` 依次释放 MongoDB、Redis、MySQL 连接池；提前取消和运行失败同样会清理。MongoDB 清理使用保留调用链值但独立于启动取消状态的 Context，超时取 `ShutdownTimeout`；这段时限在 HTTP 和任务排空之后开始，整个退出过程不保证在入口的 10 秒内完成。清理失败由 provider 记录英文错误日志，`Close()` 不返回错误。

入口使用 `defer application.Close()` 兜底，保证初始化成功但未调用 `Run` 时也能释放资源；与 `Run` 的自动清理不会重复执行。`Close()` 只释放依赖，应在 `Run` 返回后或从未运行时调用；停止正在运行的应用应取消传给 `Run` 的 context。进程退出码和日志最终关闭仍由 `main` 负责，App 不调用 `os.Exit`。

MySQL、Redis 和 MongoDB 配置变更需重启，Viper 重载不会重新组装连接池。HTTP 和关闭超时由入口填充在启动配置副本上，不从 YAML 解码；`config.Get()` 仍只提供文件快照，这两个字段保持零值。`/hello` 仍返回固定问候，不建表、不执行自动迁移、不读写缓存或 MongoDB 集合，`/healthz` 和 `/readyz` 仍返回 HTTP 200 空响应体，不检查 MySQL、Redis 或 MongoDB。

测试 Redis provider 生命周期时设置 `GOEMPTY_TEST_REDIS_ADDR`，按需设置 `GOEMPTY_TEST_REDIS_USERNAME` 和 `GOEMPTY_TEST_REDIS_PASSWORD`。MongoDB provider 生命周期测试设置 `GOEMPTY_TEST_MONGODB_URI`；应用组装生命周期测试需同时设置 Redis、MongoDB 和 `GOEMPTY_TEST_MYSQL_DSN`。运行 `go test ./internal/app ./integration-test`；只连接隔离服务，未配置对应服务时相关真实服务测试跳过，不安装服务、不写业务数据。

Wire 官方仓库已归档，生成工具固定使用 v0.7.0：https://github.com/google/wire 。

## 定时任务接入

`internal/job/register.go` 的 `NewScheduler` 由 Wire 调用，注入 HelloService，使用零配置跟随 `time.Local`；默认注册 Hello Job，每分钟整点调用 HelloService 并记录日志，任务超时 5 秒，不写业务数据。Handler 在 `internal/job/hello.go`，业务仍由 Service 提供。新增任务时注入所需 Service，在 `registrations` 中添加任务，然后重新生成 Wire。注册失败会让应用初始化失败。

`cmd/app` 入口在加载配置、启动文件监听和组装应用前，将 `time.Local` 统一设为 `Asia/Shanghai`。此后 `time.Now()`、当前 Zap 时间日志、默认调度器和 MySQL DSN 的 `loc=Local` 使用这一进程本地时区。只在启动阶段设置，不在运行中修改；直接调用 `app.Initialize` 的其他入口需自行初始化时区。该设置不会改变 MySQL 服务端或会话时区，也不会覆盖显式使用 UTC 或其他时区的时间操作。

直接组装 App 时使用 `New(server, jobs, shutdownTimeout)`，调度器不可为 nil。App 负责 Start/Stop，调用方通过取消 `Run` 的 context 退出，不需要单独停止调度器。任务应响应取消；关闭超时后 App 仍会清理依赖，因此不响应取消的任务可能访问已关闭的连接池，必须在业务代码中避免。

调度器采用标准五字段表达式，支持 `@every`；同一任务重叠执行时跳过，不同任务可并行。错误和 panic 统一记录日志，不自动重试。注册仅限启动前，停止后不能重启，不提供分布式唯一执行保证。用法及边界见 [定时任务](../../pkg/scheduler/README.md)。时区和任务定义由代码管理，配置热加载不会改变已注册任务。
