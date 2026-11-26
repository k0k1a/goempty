# 组件目录与裁剪参考

用于创建和同步时确定组件范围与连带改动。每次执行核实源代码，模板演进后不得仅按此表删除文件。

## 不可裁剪的运行骨架

- `cmd/app` 入口、`config` 加载与热加载、`pkg/log`、`pkg/env`、`internal/app` 的 Wire 与生命周期、根文档和 `go.mod`。
- 骨架保留运行所需能力，不含业务示例；示例属于组件，随所选组件保留。
- 创建时至少选择一个运行入口：HTTP 入口、Kafka 消费者或定时任务。
- 除骨架外全部组件可裁剪，包括 HTTP 入口及其公共错误、响应、中间件和健康端点。

## 选择规则

- 组件集合由用户选择，没有默认值。未指定时列出下表全部选项与依赖，请用户选择。
- 组件是选择单位，示例属于组件：选中组件就保留它的示例实现、装配和根文档描述，示例没有独立开关。
- “全部”按全部组件处理；“最小”只保留骨架和一个运行入口及其示例，并说明保留内容。
- 依赖必须满足：事件总线依赖异步任务池；定时任务的示例通过事件总线发布问候事件，所以选择定时任务时连带事件总线和异步任务池；HTTP 入口随带外部 HTTP 示例；Kafka 生产者与消费者相互独立。
- 运行入口之外的组件可以全部去掉；运行入口本身也可以只保留一个。

## 组件目录

| 组件 | 依赖 | 专属实现（含示例） | 连带调整 |
| --- | --- | --- | --- |
| HTTP 入口 | — | `pkg/httpserver/`、`internal/controller/middleware/`、`internal/controller/response/`、`internal/controller/error.go`、`internal/app/http.go` 的路由与 `GET /goempty/v1/healthz`、`GET /goempty/v1/readyz`、`internal/entity/error.go`；示例：`internal/controller/hello.go` 的 `Hello`、`internal/entity/hello.go`、`internal/service/hello.go` 的 `Hello`、`internal/repo/db/hello.go`、`GET /goempty/v1/hello`、`internal/repo/webapi/hello/` | `config.Config.HTTP`、`main.go` 的 `PORT` 解析与 `httpserver.Config`、`newHTTPServer`/`httpDependencies`、App 的 server 参数与校验、start 监听、Run 的 done、Shutdown 的 Stop；gin、cors、requestid 依赖；Hello Service/Repo 装配；错误处理、HTTP、生命周期文档；README 接口说明与导航、conventions 外部 HTTP 示例链接；Swagger 条目随 Swagger 组件处理。无 HTTP 时 main 不读 `PORT`，App 只等待所选运行组件 |
| 定时任务 | 事件总线、异步任务池（示例依赖） | `pkg/scheduler/`；示例：`internal/job/hello.go`、`internal/app/jobs.go` 的注册 | `internal/app/jobs.go`、Wire `jobDependencies`/`newScheduler`、App 的 scheduler 参数、校验与启停、cron 依赖与文档；`HelloService.PublishGreeting` 与 `helloPublisher`。不保留零任务调度器 |
| 异步任务池 | — | `pkg/async/` | `config.Config.Async` 与 YAML `async` 段、`async.New` provider、App 的 taskPool 参数、校验与启停、Ants 依赖与文档 |
| 事件总线 | 异步任务池 | `pkg/event/`；示例：`internal/event/hello.go`、`internal/entity/hello_event.go`、`internal/service/hello_event.go` 的 `LogGreeting`、`internal/app/events.go` 的注册 | `event.New` provider、App 的 eventBus 参数与启停、Wire `HelloEventPublisher` 绑定（随定时任务示例保留时）、文档。总线不保留空订阅 |
| MySQL | — | `pkg/db/` | `config.Config.MySQL`、全部 YAML `mysql` 段、`newMySQL`、`mysqlHandle`、App 的 mysql 字段、newApp 参数、Wire、GORM/MySQL 依赖、事务文档、README 测试变量 |
| PostgreSQL | — | `pkg/postgres/` | `config.Config.Postgres`、全部 YAML `postgres` 段、`newPostgres`、`postgresHandle`、App 的 postgres 字段、newApp 参数、Wire、GORM/PostgreSQL 与 pgx 依赖、事务文档、README 测试变量。与 MySQL 相互独立，可同时保留 |
| MongoDB | — | `pkg/mongodb/` | `config.Config.MongoDB` 与 YAML 段、`newMongoDB`、App 的 mongoClient、newApp 参数、Wire、官方 Driver 依赖与文档 |
| Redis | — | `pkg/redis/` | `config.Config.Redis` 与 YAML 段、`newRedis`、App 的 redisClient、newApp 参数、Wire、go-redis 依赖与文档 |
| Kafka 生产者 | — | `pkg/kafka/`；示例：`internal/repo/publisher/hello.go`、`internal/service/hello_message.go`、`internal/entity/hello_message.go`、`HelloService.helloMessagePublisher`；同时选择 HTTP 时含 `PublishMessage` 与 `POST /goempty/v1/hello/message` | `config.Config.Kafka` 与 YAML `kafka` 段、`newKafkaProducer`、Wire 字段与清理、`HelloMessagePublisher` 绑定、文档 |
| Kafka 消费者 | — | `pkg/kafka/`、`internal/app/messaging.go`；示例：`internal/consumer/hello.go`、`messaging.go` 的注册、`internal/entity/hello_message.go`、`HelloService.LogGreeting` | `config.Config.KafkaConsumer` 与 YAML `kafka_consumer` 段、Wire `messagingDependencies`/`newMessagingConsumers`、newApp 参数、App 的 consumers 字段与校验、start 的 Start 与 Done 等待、Shutdown 的 Stop 与等待、文档 |
| 页码分页 | — | `internal/entity/page.go` 中 `Page`、`PageList` | conventions 公共分页的页码规则、architecture 公共分页引用 |
| 游标分页 | — | `pkg/cursor/`、`internal/entity/page.go` 中 `CursorPage`、`CursorPageList` 及 `FetchLimit` | conventions 公共分页的游标条目、architecture 目录树与 `pkg/cursor/README.md` |
| Swagger 生成 | HTTP 入口（注解） | `scripts/generate-api-docs/`、`docs/swagger/`、保留接口的 swag 注释 | Makefile `docs` 目标、README 生成命令 |
| 数据库迁移规范 | MySQL 或 PostgreSQL | `migrations/` | conventions 数据库变更章节；无关系型数据库时删除 SQL 专属要求，保留通用 Repo 原子性与事务职责 |

## 示例归属与共享基础

- `HelloService` 是各示例的共享枢纽：`internal/service/hello.go` 的 `Hello`、`internal/service/hello_event.go` 的 `PublishGreeting`/`LogGreeting` 与 `HelloEventPublisher`、`internal/service/hello_message.go` 的 `PublishGreetingMessage` 与 `HelloMessagePublisher`。按所选组件的实际引用保留字段、构造参数和方法：`helloRepo` 随需要 `Hello()` 的示例保留，`helloPublisher` 与 `PublishGreeting` 随定时任务示例保留，`helloMessagePublisher` 随 Kafka 生产者示例保留；无引用的方法、接口和参数删除。
- `internal/repo/db/hello.go` 提供固定问候语，无存储依赖；保留需要 `Hello()` 的示例时，即使移除全部存储组件也保留该 Repo 及其接口绑定。
- 示例之间共享的文件按任一所选组件保留并裁剪：`internal/entity/hello_message.go` 同时被 Kafka 生产者和消费者示例使用；`internal/entity/hello_event.go` 同时被事件总线、定时任务和 Kafka 消费者示例使用。
- 外部 HTTP 示例 `internal/repo/webapi/hello/` 不参与装配，随 HTTP 入口保留，包括其 README、architecture 目录树、README 导航和 conventions 示例链接。
- 只保留 HTTP 入口时，路由除 `GET /goempty/v1/healthz`、`GET /goempty/v1/readyz` 外包含 `GET /goempty/v1/hello`；不选 Kafka 生产者时不存在 `POST /goempty/v1/hello/message`。
- 保留 Kafka 消费者时包含 `internal/consumer/hello.go` 消费示例，不出现无业务 handler 的消费者。
- 保留事件总线时包含 Hello 事件订阅示例；未选定时任务时事件由后续业务代码发布，订阅本身随组件保留。
- Swagger 只包含实际保留接口的注解；不选 Swagger 时删除生成脚本、产物、注释与 Makefile 目标。

## 装配与生命周期

- `App.New`、`Run`、`Shutdown` 只接收所选运行组件：删除未选组件的参数、字段、校验和 import，不保留 nil、空切片或空接口占位。
- 无 HTTP 时，App 不启动监听，将所选消费者或调度器的结果交给 Run 等待；`main.go` 去掉 `PORT` 与 `httpserver.Config`，并保证退出路径仍释放已创建资源。
- 选择定时任务时同时保留事件总线与异步任务池，App 的 scheduler、taskPool、eventBus 参数与启停保持一致；不保留零任务调度器。
- 无异步任务池时连带移除事件总线；无事件总线时删除订阅装配与相关 Service 方法。
- 移除消费者时，`App.New` 去掉 consumers 参数、nil 校验和 `slices.Clone`，`start` 去掉消费者启动与 Done 等待，`Shutdown` 去掉消费者 Stop 与等待；两侧 Kafka 都移除时删除 `pkg/kafka`、`internal/consumer`、`internal/repo/publisher`，由 `go mod tidy` 移除 franz-go。
- 保持 HTTP 优雅关闭、取消传播、关闭超时、清理幂等和初始化失败释放已创建资源；增减运行组件后检查 channel 的发送、等待和错误合并是否仍成对。
- `config/load.go` 使用 `UnmarshalExact`：删除组件后同步删除配置字段和所有环境 YAML 段，无文件配置字段时使用合法空映射 `{}`。
- 依赖变化后运行 `go mod tidy`；只保留实际使用的依赖和 import，不用空白标识符掩盖裁剪不完整。
- 页码分页与游标分页都不保留时删除 `internal/entity/page.go` 和 conventions 公共分页整节；只保留一种时删除另一种类型与对应规则。
- 根文档只描述实际保留的组件及其示例：AGENTS.md 阅读入口与实施约束、architecture.md 目录树与章节、conventions.md 选型与规则、README.md 启动与导航。组件 README 按项目规则处理。

## 验收场景

在独立临时目录复制源到不同路径，从副本加载技能，分别验证：

1. 未给出组件集合 → 技能列出组件目录与依赖并询问，不生成任何文件。
2. “创建 plain-api，只保留 HTTP 入口” → 有 `GET /goempty/v1/healthz`、`GET /goempty/v1/readyz`、`GET /goempty/v1/hello`、Hello Service/Repo 和外部 HTTP 示例；无 `POST /goempty/v1/hello/message`、存储、调度、Kafka 和 Swagger 生成，Wire/build/vet 通过。
3. “创建 order-service，只保留 MySQL、Redis 和 HTTP 入口” → 保留两个连接、`GET /goempty/v1/hello` 和外部 HTTP 示例，无调度、Kafka、MongoDB。三个环境配置与根文档一致。
4. “创建 audit-service，只保留 MongoDB 和定时任务” → 自动连带事件总线和异步任务池；无 HTTP，App 无 server 参数，`HelloJob` 与 Hello 事件订阅可运行并正确关闭，`PORT` 不再读取。
5. “创建 worker，只保留 Kafka 消费者” → 无 HTTP、调度、存储、生产者；`kafka_consumer` 存在，Hello 消费示例与 `LogGreeting` 保留，App 只管理消费者；发布接口与 `POST /goempty/v1/hello/message` 不存在。
6. “创建 clean-api，只保留定时任务” → 自动连带事件总线和异步任务池，`HelloJob` 可运行，无 HTTP、存储和 Kafka 代码或配置。
7. “创建 full-service，全部组件” → 完整模板，module 与应用名替换正确，Wire 与 Swagger 重新生成，`go vet ./...`、`go build ./...` 通过。
8. “创建 pg-service，只保留 PostgreSQL、Redis 和 HTTP 入口” → 无 MySQL、调度、Kafka、MongoDB；`postgres` 配置段与 `newPostgres` 保留，App 无 mysql 字段与参数，`GET /goempty/v1/hello` 与外部 HTTP 示例保留，三个环境配置与根文档一致。

每种组合应无 `_test.go`、不携带生成技能、不修改源。已有目标或目标位于源内时在写入前停止。真实连接只在隔离服务可用时验证。
