# 应用组装

HTTP 中间件按 RequestID → AccessLog → Recovery → CORS 的顺序注册。访问日志在请求处理完成后输出 `HTTP request completed`，包含 `method`、`route`、`status`、`duration_us` 和上下文中的 `request_id`；`route` 使用匹配的路由模板，未匹配时为空，不记录实际 URL、查询参数、请求体或请求头。AccessLog 位于 Recovery 外层，因此 panic 恢复后也会记录最终响应状态。

Panic 错误日志额外包含 `stack`，保留原始 panic 调用位置；响应已经写出时不覆盖响应，只记录错误并中止后续处理。

Wire 生成依赖组装代码，`App` 管理运行组件的生命周期。当前接入 MySQL、PostgreSQL、Redis、MongoDB、Gin、HTTP Server、定时调度器、异步任务池、事件分发、Kafka 生产者与消费者，以及 Hello Controller → Service → Repo → Publisher 示例。

| 文件 | 装配职责 |
| --- | --- |
| `wire.go` | 声明构造函数、接口绑定及各入口所需的依赖，通过私有 `initialize(ctx, cfg)` 生成依赖装配和失败清理。 |
| `providers.go` | `Initialize` 调用生成代码并将清理函数交给 App。连接 provider 创建 MySQL、PostgreSQL、Redis、MongoDB 客户端和 Kafka 生产者并返回清理函数，MySQL、PostgreSQL、Redis、MongoDB 初始化包含 Ping；生产者清理时 flush 并关闭。两个数据库 provider 用包装类型区分，Wire 才能同时注入两个 `*gorm.DB`。 |
| `http.go` | 接收 HTTP 所需的 Service，构造 Gin Engine 和 Controller，注册中间件及路由。 |
| `jobs.go` | 接收 Job 所需的 Service，创建调度器并注册任务。 |
| `events.go` | 接收事件处理所需的 Service，构造 Event Controller 和订阅。`newApp` 在初始化期间调用注册函数。 |
| `messaging.go` | 接收消息消费所需的 Service 和 `kafka_consumer` 配置，构造 Kafka Router、注册消费 handler 并创建消费者。 |
| `app.go` | `Run` 管理启动后到退出前的运行过程。`Shutdown` 关闭组件并执行 Wire cleanup。 |

上述入口各自通过构造函数接收实际依赖。App 保存运行组件和共享连接，业务对象从构造参数获得所需能力。`wire_gen.go` 由 `make wire` 生成，普通构建直接使用该文件。

Hello 示例调用链为 `GET /hello → HelloController.Hello → HelloService.Hello → HelloRepo.Hello`。仓储接口 `db.HelloRepository` 与具体实现 `HelloRepo` 均定义在 `internal/repo/db/hello.go`，直接返回 `hello`，不访问外部存储。Service 导入 Repo 包并依赖接口，Repo 不反向导入 Service。

`cmd/app/main.go` 是可运行入口，负责监听 `os.Interrupt`（如 Ctrl+C）和 `SIGTERM`，将取消通知传给 `App.Run`，等待关闭完成后退出；初始化或运行失败时统一记录错误并以状态码 1 退出。

生命周期日志复用 `pkg/log`，默认以 JSON 同时输出到 stdout 和 `logs/{APP_NAME}_{ENV}.log`，级别由 `LEVEL` 决定（未设置或为空时为 info），消息统一使用英文：`Application starting`、端口监听成功后的 `HTTP server started`、`Shutting down application`、正常退出时的 `Application stopped`。初始化、运行或关闭失败时由入口记录一次 `Application exited with an error`。最终日志写入后再调用 `log.Shutdown()`；日志关闭本身失败时向 stderr 输出错误并以状态码 1 退出。

```sh
go run ./cmd/app
```

入口使用 `config.Load()` 加载 `configs/{APP_NAME}_{ENV}.yaml`。`APP_NAME` 未设置或为空时默认 `app`，`ENV` 未设置或为空时默认 `dev`；路径以启动工作目录为基准。目标文件不存在或非法时启动失败，不回退到其他环境，不再提供 `-config` 参数。

入口只通过环境变量 `PORT` 决定端口，未设置时默认 `8080`；显式设置时必须是 `1～65535` 的整数，空值或非法值启动失败，监听地址为 `:端口`。HTTP 超时固定在入口代码中：ReadHeaderTimeout 为 5 秒，ReadTimeout/WriteTimeout 为 15 秒，IdleTimeout 为 60 秒；不读取 YAML 或 `GOEMPTY_HTTP_*` 环境变量。

关闭超时在入口固定为 10 秒，不从 YAML 或环境变量读取。MySQL DSN 和连接池参数从 YAML 的 `mysql` 段读取，PostgreSQL 从 `postgres` 段读取，Redis 连接参数从 `redis` 段读取，MongoDB 连接参数从 `mongodb` 段读取，不提供环境变量覆盖。本地配置见 `configs/app_dev.yaml`，需先替换本地密码并准备 MySQL、PostgreSQL、Redis、MongoDB 和 Kafka；`kafka.brokers` 为空时启动失败，清空 `kafka_consumer` 只跳过消费者。test/prod 文件保留空模板，不保存线上配置，使用前需填写。完整说明见 [应用配置](../../config/README.md)。

`config.Load()` 同时启动 Viper 文件监听；变化回调直接使用同一 Viper 实例重新反序列化，成功才替换内存配置，失败保留旧值。需要动态配置的业务通过 `config.Get()` 读取快照，不重启应用或重新执行 Wire 组装。

`GET /hello` 返回 HTTP 200 和 JSON `{"code":0,"message":"success","data":{"message":"hello"}}`。`POST /hello/message` 接收 `{"message":"..."}`，发布 hello 消息后返回 HTTP 200 和 `{"code":0,"message":"success","data":{"message":"..."}}`；空消息或非法 JSON 返回 code `100001`，发布失败返回 code `-1`。未注册路径、不支持的方法及响应写出前的 panic 均返回 HTTP 400；响应尚未写出时统一使用 JSON 外壳，code 为独立于 HTTP 状态码的整数业务码。错误解析与输出统一通过 `response.RespondError`，约定见 [HTTP 公共响应](../controller/response/README.md)。部署时建议先 `go build -o app ./cmd/app`，直接运行编译产物，并将退出信号发送给应用进程。Wire 接收 context 和配置结构体，不负责加载文件。

更新依赖组装后，在项目根目录运行：

```sh
go run github.com/google/wire/cmd/wire@latest ./internal/app
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
// 实际入口使用 defer 兜底；Run 会返回关闭错误，见 cmd/app/main.go。
defer application.Shutdown(context.WithoutCancel(ctx))
return application.Run(ctx)
```

`Run` 仅调用一次；取消属于正常退出，启动或关闭失败返回错误。必须等待 `Run` 返回后才能退出进程。强制关闭连接不会终止 handler 的 goroutine，handler 应响应请求 context 的取消。

MySQL、PostgreSQL、Redis 或 MongoDB 连接失败时应用启动失败。Wire 在后续装配失败时释放已创建的连接池（PostgreSQL 初始化失败时释放 MySQL，Redis 初始化失败时释放 PostgreSQL 和 MySQL，MongoDB 初始化失败时释放 Redis、PostgreSQL 和 MySQL）；成功初始化后由 App 持有 Wire 的清理函数。`Shutdown` 完成 HTTP 和任务关闭（或关闭超时）后，依次释放 MongoDB、Redis、PostgreSQL、MySQL 连接池，并关闭 Kafka 生产者（flush 待投递消息）；提前取消和运行失败同样会清理。MongoDB 清理使用保留调用链值但独立于启动取消状态的 Context，超时取 `ShutdownTimeout`；这段时限在 HTTP 和任务排空之后开始，整个退出过程不保证在入口的 10 秒内完成。Shutdown 返回组件停止错误；Wire cleanup 的签名仍为 `func()`，连接清理失败由 provider 记录日志，不作为返回错误。

公开关闭入口统一为 `Shutdown(ctx) error`，不再提供 `Close()`。初始化后未调用 Run、正在运行、部分启动失败都可以使用它。第一次调用的 Context 和配置中的 `ShutdownTimeout` 共同约束组件停止等待时间；并发调用者等待同一次关闭完成，重复调用返回保存的结果，不重新执行清理或重新等待超时任务。Run 收到取消信号时传入 `context.WithoutCancel(ctx)`，保留上下文值并提供独立关闭时间；直接调用 Shutdown 则遵循调用方的取消和 deadline。入口使用 defer 兜底，正常运行时的关闭错误由 Run 返回，不重复报告。进程退出码和日志最终关闭仍由 main 负责，App 不调用 os.Exit。

MySQL、PostgreSQL、Redis 和 MongoDB 配置变更需重启，Viper 重载不会重新组装连接池。HTTP 和关闭超时由入口填充在启动配置副本上，不从 YAML 解码；`config.Get()` 仍只提供文件快照，这两个字段保持零值。`/hello` 仍返回固定问候，不建表、不执行自动迁移、不读写缓存或 MongoDB 集合，`/healthz` 和 `/readyz` 仍返回 HTTP 200 空响应体，不检查 MySQL、PostgreSQL、Redis 或 MongoDB。

测试 Redis provider 生命周期时设置 `GOEMPTY_TEST_REDIS_ADDR`，按需设置 `GOEMPTY_TEST_REDIS_USERNAME` 和 `GOEMPTY_TEST_REDIS_PASSWORD`。MongoDB provider 生命周期测试设置 `GOEMPTY_TEST_MONGODB_URI`；应用组装生命周期测试需同时设置 Redis、MongoDB、`GOEMPTY_TEST_MYSQL_DSN` 和 `GOEMPTY_TEST_POSTGRES_DSN`。运行 `go test ./internal/app ./integration-test`；只连接隔离服务，未配置对应服务时相关真实服务测试跳过，不安装服务、不写业务数据。

Wire 官方仓库已归档，生成工具固定使用 v0.7.0：https://github.com/google/wire 。

## 定时任务接入

任务在 `internal/job` 实现 `scheduler.Task`，在 [jobs.go](jobs.go) 构造并注册。新增 Service 依赖时更新 Wire 声明并运行 `make wire`。注册失败会使应用初始化失败。

模板调度器使用零配置，采用构造时的 `time.Local`。`cmd/app` 在加载配置、启动文件监听和组装应用前将本地时区设为 `Asia/Shanghai`。进程时间、Zap 日志时间和 MySQL DSN 的 `loc=Local` 也使用该设置。直接调用 `app.Initialize` 的其他入口应按需要设置进程时区。数据库服务端和会话时区由各自环境决定。 [表达式和运行行为](../../pkg/scheduler/README.md)

## 异步任务与事件

Wire 创建共享任务池和 Bus，并将 Bus 绑定到 `HelloEventPublisher`。在 [events.go](events.go) 添加订阅时，构造只接收所需 Service 的处理器，并将事件名与处理器显式绑定。订阅必须在 Bus 启动前完成。

### Hello 示例

1. [HelloJob](../job/hello.go) 在每分钟整点调用 `HelloService.PublishGreeting`。Service 读取固定问候语并发布 [HelloGreetingEvent](../entity/hello_event.go)。
2. [HelloEventController](../event/hello.go) 校验事件，调用 Service 输出 `Hello task greeting`。调度、分发和处理日志可以用同一个 `trace_id` 关联。
3. Job 为本次调用设置 5 秒的上下文超时。Worker 开始处理事件后使用任务池自己的执行超时。`Publish` 的返回值只反映任务是否被接收，若 Worker 已满，等待期间 Job 的超时也无法中断这次提交。 [任务池](../../pkg/async/README.md) · [事件总线](../../pkg/event/README.md)
4. `kafka_consumer` 段有内容时，[messaging.go](messaging.go) 注册 hello 消费者：`hello` topic 的 `hello.greeting` 消息经 [HelloGreetingConsumer](../consumer/hello.go) 校验后调用 `HelloService.LogGreeting`。dev 配置把 `hello` 映射为 `hello-dev`；配置为空时不创建消费者。 [Kafka](../../pkg/kafka/README.md)
5. `POST /hello/message` 绑定 `entity.HelloGreetingMessage` 并调用 `HelloService.PublishGreetingMessage`，经 [HelloPublisher](../repo/publisher/hello.go) 发布 `hello` topic 的 `hello.greeting` 消息，由上述消费者处理。该链路与 Job 的进程内事件示例相互独立。

### 启动与关闭

`App.Run` 依次启动任务池、启用 Bus、启动调度器、Kafka 消费者和 HTTP 服务。直接调用 `New(server, jobs, taskPool, eventBus, shutdownTimeout)` 时，调用方负责预先注册任务和订阅，并确保 Bus 使用传给 App 的同一个任务池。

取消运行 Context、HTTP 退出或部分启动失败都会进入 `Shutdown`。App 并行停止 HTTP、调度器和任务池，等待关闭调用返回后再清理共享连接。池停止时会唤醒阻塞的提交者，取消在途任务并限时等待退出。关闭结果由首次 `Shutdown` 保存，并发或后续调用共用该结果。调用方应等待 `Run` 返回再结束进程。
