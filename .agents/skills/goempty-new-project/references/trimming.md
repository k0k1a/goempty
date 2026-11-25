# 组件目录与裁剪参考

用于创建和同步时确定组件范围与连带改动。每次执行核实源代码，模板演进后不得仅按此表删除文件。

## 不可裁剪的运行骨架

- `cmd/app` 入口、`config` 加载与热加载、`pkg/log`、`pkg/env`、`internal/app` 的 Wire 与生命周期、根文档和 `go.mod`。
- 骨架保留运行所需能力，不含业务示例。创建时至少选择一个运行入口：HTTP 入口、Kafka 消费者或定时任务。
- 除骨架外全部组件可裁剪，包括 HTTP 入口及其公共错误、响应、中间件和健康端点。

## 选择规则

- 组件集合由用户选择，没有默认值。未指定时列出下表全部选项与依赖，请用户选择。
- “全部”按全部组件处理；“最小”只保留骨架和一个运行入口，并说明保留内容。
- 依赖必须满足：事件总线依赖异步任务池；示例链路依赖对应运行入口和基础设施组件；无依赖组件可独立选择。
- 运行入口之外的组件可以全部去掉；运行入口本身也可以只保留一个。

## 组件目录

| 组件 | 依赖 | 专属实现 | 连带调整 |
| --- | --- | --- | --- |
| HTTP 入口 | — | `pkg/httpserver/`、`internal/controller/middleware/`、`internal/controller/response/`、`internal/controller/error.go`、`internal/app/http.go` 的路由与 healthz/readyz、`internal/entity/error.go` | `config.Config.HTTP`、`main.go` 的 `PORT` 解析与 `httpserver.Config`、`newHTTPServer`/`httpDependencies`、App 的 server 参数与校验、start 监听、Run 的 done、Shutdown 的 Stop；gin、cors、requestid 依赖；错误处理、HTTP、生命周期文档。无 HTTP 时 main 不读 `PORT`，App 只等待所选运行组件 |
| Hello HTTP 示例 | HTTP 入口 | `internal/controller/hello.go` 的 `Hello` 与构造、`internal/entity/hello.go`、`GET /hello`、Swagger 条目 | `httpDependencies.HelloService`（与其他示例共享）、README 接口说明；无任何 Hello 示例时删除 Hello 基础与绑定 |
| 定时任务 | — | `pkg/scheduler/` | `internal/app/jobs.go`、Wire `jobDependencies`/`newScheduler`、App 的 scheduler 参数、校验与启停、cron 依赖与文档 |
| 异步任务池 | — | `pkg/async/` | `config.Config.Async` 与 YAML `async` 段、`async.New` provider、App 的 taskPool 参数、校验与启停、Ants 依赖与文档 |
| 事件总线 | 异步任务池 | `pkg/event/` | `event.New` provider、App 的 eventBus 参数与启停、`internal/app/events.go`、Wire `HelloEventPublisher` 绑定、文档。无订阅时删除 `events.go` |
| Hello 任务与事件示例 | 定时任务、事件总线、异步任务池 | `internal/job/hello.go`、`internal/event/hello.go`、`internal/entity/hello_event.go`、`internal/service/hello_event.go`、`jobs.go`/`events.go` 中的注册 | `HelloService` 的事件发布接口、字段与方法、Wire 绑定、`jobDependencies`/`eventDependencies`。移除后保留零任务调度器和空总线 |
| MySQL | — | `pkg/db/` | `config.Config.MySQL`、全部 YAML `mysql` 段、`newDatabase`、App 的 database 字段、newApp 参数、Wire、GORM/MySQL 依赖、事务文档、README 测试变量 |
| MongoDB | — | `pkg/mongodb/` | `config.Config.MongoDB` 与 YAML 段、`newMongoDB`、App 的 mongoClient、newApp 参数、Wire、官方 Driver 依赖与文档 |
| Redis | — | `pkg/redis/` | `config.Config.Redis` 与 YAML 段、`newRedis`、App 的 redisClient、newApp 参数、Wire、go-redis 依赖与文档 |
| Kafka 生产者 | — | `pkg/kafka/` | `config.Config.Kafka` 与 YAML `kafka` 段、`newKafkaProducer`、Wire 字段与清理、文档。`internal/repo/publisher/` 随示例链路 |
| Kafka 消费者 | — | `pkg/kafka/`、`internal/app/messaging.go` | `config.Config.KafkaConsumer` 与 YAML `kafka_consumer` 段、Wire `messagingDependencies`/`newMessagingConsumers`、newApp 参数、App 的 consumers 字段与校验、start 的 Start 与 Done 等待、Shutdown 的 Stop 与等待、文档 |
| Hello MQ 示例 | Kafka 生产者（发布侧）、HTTP 入口（发布接口）、Kafka 消费者（消费侧） | `internal/repo/publisher/hello.go`、`internal/service/hello_message.go`、`internal/controller/hello.go` 的 `PublishMessage`、`POST /hello/message`、`internal/consumer/hello.go`、`internal/entity/hello_message.go`、messaging.go 中的注册 | 两侧可分别保留。发布侧删除 `HelloMessagePublisher`、`HelloService` 第三参数与 Wire 绑定；两侧都移除时删除 `entity/hello_message.go` 并重新生成 Swagger |
| 页码分页 | — | `internal/entity/page.go` 中 `Page`、`PageList` | conventions 公共分页的页码规则、architecture 公共分页引用 |
| 游标分页 | — | `pkg/cursor/`、`internal/entity/page.go` 中 `CursorPage`、`CursorPageList` 及 `FetchLimit` | conventions 公共分页的游标条目、architecture 目录树与 `pkg/cursor/README.md` |
| 外部 HTTP 示例 | — | `internal/repo/webapi/hello/` | 不参与装配。architecture 目录树与 README 导航；无其他外部客户端时删除空 `webapi/` 目录 |
| Swagger 生成 | HTTP 入口（注解） | `scripts/generate-api-docs/`、`docs/swagger/`、`main.go` 与 Controller 的 swag 注释 | Makefile `docs` 目标、README 生成命令 |
| 数据库迁移规范 | MySQL | `migrations/` | conventions 数据库变更章节；无 MySQL 时删除 SQL 专属要求，保留通用 Repo 原子性与事务职责 |

## 示例链路

- Hello 基础由 `internal/controller/hello.go`、`internal/service/hello.go`、`internal/repo/db/hello.go` 和 `internal/entity/hello.go` 构成，被各 Hello 示例共享。选择任一示例时保留基础；全部不选时删除基础、各示例文件、路由、Wire 绑定和相关文档。
- `internal/repo/db/hello.go` 提供固定问候语，无存储依赖。保留任一 Hello 示例时，即使移除全部存储组件也保留该 Repo 及其接口绑定。
- 只保留 HTTP 入口而无任何示例时，路由仅保留 healthz/readyz，Swagger 不含业务接口。
- 保留定时任务但移除任务示例时，装配零任务调度器，删除无用途的 `jobDependencies`。
- 保留事件总线但移除事件示例时，删除 `events.go` 与订阅，总线空启动。
- 保留 Kafka 生产者但移除发布示例时保留 Producer provider，删除 publisher 目录与发布接口；保留消费者但移除消费示例时保留 Router 与消费者装配并删除 `internal/consumer/`，无业务 handler 的消费者需在交付中说明，通常与示例一起移除。
- 保留 HTTP 入口但移除 Swagger 时，删除生成脚本、产物、注释与 Makefile 目标。

## 装配与生命周期

- `App.New`、`Run`、`Shutdown` 只接收所选运行组件：删除未选组件的参数、字段、校验和 import，不保留 nil、空切片或空接口占位。
- 无 HTTP 时，App 不启动监听，将所选消费者或调度器的结果交给 Run 等待；`main.go` 去掉 `PORT` 与 `httpserver.Config`，并保证退出路径仍释放已创建资源。
- 无异步任务池时连带移除事件总线；无事件总线时删除订阅装配与相关 Service 方法。
- 移除消费者时，`App.New` 去掉 consumers 参数、nil 校验和 `slices.Clone`，`start` 去掉消费者启动与 Done 等待，`Shutdown` 去掉消费者 Stop 与等待；两侧 Kafka 都移除时删除 `pkg/kafka`、`internal/consumer`、`internal/repo/publisher`，由 `go mod tidy` 移除 franz-go。
- 保持 HTTP 优雅关闭、取消传播、关闭超时、清理幂等和初始化失败释放已创建资源；增减运行组件后检查 channel 的发送、等待和错误合并是否仍成对。
- `config/load.go` 使用 `UnmarshalExact`：删除组件后同步删除配置字段和所有环境 YAML 段，无文件配置字段时使用合法空映射 `{}`。
- 依赖变化后运行 `go mod tidy`；只保留实际使用的依赖和 import，不用空白标识符掩盖裁剪不完整。
- 页码分页与游标分页都不保留时删除 `internal/entity/page.go` 和 conventions 公共分页整节；只保留一种时删除另一种类型与对应规则。
- 根文档只描述实际保留的组件：AGENTS.md 阅读入口与实施约束、architecture.md 目录树与章节、conventions.md 选型与规则、README.md 启动与导航。组件 README 按项目规则处理。

## 验收场景

在独立临时目录复制源到不同路径，从副本加载技能，分别验证：

1. 未给出组件集合 → 技能列出组件目录与依赖并询问，不生成任何文件。
2. “创建 plain-api，只保留 HTTP 入口” → 无存储、调度、Kafka、Hello 示例和 Swagger 生成。`GET /healthz`、`GET /readyz` 可运行，`GET /hello` 与 `POST /hello/message` 不存在，Wire/build/vet 通过。
3. “创建 order-service，只保留 MySQL、Redis、HTTP 入口和 Hello HTTP 示例” → 保留两个连接与 `GET /hello`，无调度、Kafka、MongoDB。三个环境配置与根文档一致。
4. “创建 audit-service，只保留 MongoDB、定时任务、异步任务池、事件总线和 Hello 任务与事件示例” → 无 HTTP，App 无 server 参数，调度与事件可运行并正确关闭，`PORT` 不再读取。
5. “创建 worker，只保留 Kafka 消费者和 Hello MQ 消费示例” → 无 HTTP、调度、存储、生产者；`kafka_consumer` 存在，App 只管理消费者；发布接口与 `POST /hello/message` 不存在。
6. “创建 clean-api，只保留定时任务” → 零任务调度器可启动，无任何 Hello、HTTP、存储和 Kafka 代码或配置。
7. “创建 full-service，全部组件” → 完整模板，module 与应用名替换正确，Wire 与 Swagger 重新生成，`go vet ./...`、`go build ./...` 通过。

每种组合应无 `_test.go`、不携带生成技能、不修改源。已有目标或目标位于源内时在写入前停止。真实连接只在隔离服务可用时验证。
