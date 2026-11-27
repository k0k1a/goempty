# 组件目录与边界

创建与同步共用本文件，组件归属和依赖只维护一份。创建流程见 [创建技能](../SKILL.md)，同步流程见 [同步技能](../../goempty-sync-project/SKILL.md)。每次执行核实源代码及调用方，路径不是机械复制或删除列表。

## 公共开发基线

- 保留 `cmd/app`、`config` 加载与热加载、`pkg/log`、`pkg/env`、`internal/app` 的 Wire 与生命周期、go.mod 和根文档；App 具体参数与启停逻辑随所选组件调整。
- 保留页码与游标分页：`internal/entity/page.go`、`pkg/cursor` 的生产代码、公共分页规则。即使没有 HTTP 或暂时无引用也不删除；签名 key 由业务注入，不新增模板配置字段。
- 保留公共业务错误 `internal/entity/error.go`、通用工具和通用的分层、事务、错误处理、外部 HTTP 调用等开发规范；HTTP 响应适配与中间件属于 HTTP 组件。
- 保留 lo、cast 等统一选型约定。go mod tidy 可以移除未引用模块，不用空白导入强留依赖，不把选型约定一起删除。
- 只有下表显式列出的组件可裁剪，不能依据 `pkg` 目录位置或暂时没有引用认定其他公共能力可删。发现新增能力尚未归类时先说明并确认。

## 可选组件与依赖

组件集合由用户选择，无默认值。至少保留 HTTP、Kafka 消费者或定时任务中的一个运行入口；“最小”仍保留全部公共基线。选中组件必须保留相应现有 demo、装配和根文档，示例不单独选择；没有独立 demo 的组件沿用其现有使用方式，不凭空新增业务示例。

| 组件 | 生产代码与 demo | 配置、装配和连带范围 |
| --- | --- | --- |
| HTTP + Swagger | `pkg/httpserver/`、`internal/controller/` 的 HTTP 入口、中间件、响应适配、`internal/app/http.go`；Hello Handler、Service/Repo 与 `internal/repo/webapi/hello/` 外部 HTTP 示例；`scripts/generate-api-docs/`、Swagger JSON/YAML 和接口注释 | `config.Config.HTTP`、`main.go` 的 PORT 与 httpserver.Config、provider、Wire、App server 启停、HTTP 专属依赖、Makefile docs 目标和根文档生成命令；健康端点与实际接口同步。HTTP 与 Swagger 必须同时保留或移除 |
| MySQL | `pkg/db/` | MySQL 配置字段与全部环境 mysql 段、newMySQL、App mysql 字段与注入、Wire、驱动依赖、清理和数据库专属说明；包装类型按单库/双库规则处理 |
| PostgreSQL | `pkg/postgres/` | Postgres 配置字段与全部环境 postgres 段、newPostgres、App postgres 字段与注入、Wire、驱动依赖、清理和数据库专属说明；与 MySQL 独立选择 |
| MongoDB | `pkg/mongodb/` | MongoDB 配置与 YAML、provider、App 客户端与清理、Driver 依赖、专属建模与原子更新规则 |
| Redis | `pkg/redis/` | Redis 配置与 YAML、provider、App 客户端与清理、go-redis 依赖和 Redis 专属说明 |
| Kafka 生产者 | 完整 `pkg/kafka/` 生产代码；`internal/repo/publisher/hello.go`、`internal/service/hello_message.go`、消息 Entity 和 HelloService 发布依赖；有 HTTP 时包括发布 Handler 与 POST 示例路由 | config.Config.Kafka 与 kafka 段、newKafkaProducer、Wire、发布接口绑定与清理；不因 pkg 内包含消费者代码就增加消费配置、demo 或 App 消费者装配 |
| Kafka 消费者 | 完整 `pkg/kafka/` 生产代码；`internal/consumer/hello.go`、消息 Entity、HelloService.LogGreeting、`internal/app/messaging.go` | config.Config.KafkaConsumer 与 kafka_consumer 段、Wire、消费者注册、App consumers 的启动、Done 等待和关闭；不增加生产配置、发布 demo 或发布路由 |
| 定时任务 | `pkg/scheduler/`、`internal/job/hello.go`、jobs 注册、HelloService.PublishGreeting 及发布接口 | scheduler provider、Wire、App 参数与启停、cron 依赖；现有 demo 依赖事件总线和异步任务池，必须连带保留，不留零任务调度器 |
| 异步任务池 | `pkg/async/` | config.Config.Async 与 async 段、provider、Wire、App 参数与启停、Ants 依赖；事件总线存在时不能移除 |
| 进程内事件总线 | `pkg/event/`、`internal/event/hello.go`、事件 Entity、HelloService.LogGreeting 与 events 注册 | provider、Wire、订阅装配、App 参数与启停；依赖异步任务池。未选定时任务时仍保留订阅 demo，由后续业务发布事件 |

数据库迁移不是独立选项：有 MySQL 或 PostgreSQL 时保留 migrations 和适用 SQL 迁移规范；两者都不选时移除 SQL 专属内容，保留通用 Repo 原子性与事务职责。

## 代码完整性与示例归属

- Kafka 任意一侧启用，都完整保留或同步 `pkg/kafka` 的生产代码，包括 producer、consumer、config、event、handler；包内未被当前 App 使用的另一侧代码不是裁剪对象。两侧都不选时才移除整个包；测试和组件文档始终不复制。
- `HelloService` 按所选 demo 保留字段、构造参数、接口和方法：helloRepo 随需要 Hello() 的示例保留，helloPublisher 与 PublishGreeting 随定时任务保留，helloMessagePublisher 与 PublishGreetingMessage 随生产者保留。删除无引用示例部分，不留空实现。
- `internal/repo/db/hello.go` 是无存储依赖的问候示例，不属于 MySQL 连接封装；需要 Hello() 时即使没有任何存储也保留。
- 消息与事件 Entity、LogGreeting 等共享示例按任一所选组件的调用关系保留，不因名称包含 event 就强制引入进程内事件总线。
- HTTP 入口保留外部 HTTP 示例生产代码，不保留其 README；无 Kafka 生产者时不保留发布 Handler 和 POST 路由。
- Swagger 保留实际接口注释、生成脚本、JSON/YAML 和生成命令，不保留 docs/swagger/README.md。basePath 与完整路径表示沿用源规范，替换项目身份后与运行路由一致；当前模板为 basePath `/`、paths 使用 `/goempty/v1/...`。

## 装配与生命周期

- App.New、Run、Shutdown 只装配所选运行能力，移除未选组件的参数、字段、校验、import 和等待逻辑，不用 nil、空切片或空接口占位。
- MySQL 和 PostgreSQL 只有一种时，newMySQL 或 newPostgres 返回 `(*gorm.DB, func(), error)`，newApp 和受影响注入点直接传递 `*gorm.DB`；移除 mysqlHandle、postgresHandle 和 `.DB` 解包。Redis、MongoDB 不影响此判断。
- 两种关系型数据库都保留时，使用 mysqlHandle、postgresHandle 区分 Wire provider；单库和双库均保留清理函数、初始化失败清理和 App 退出清理。同步新增第二种数据库时，先将原注入点的适配纳入确认范围。
- 无 HTTP 时移除 PORT、httpserver.Config 与监听，App 等待所选消费者或调度器退出；有 HTTP 时确保 Swagger 同时接入。
- 移除 Kafka 消费者时去掉 App consumers 参数与校验、克隆、Start、Done 等待、Stop 和关闭等待，不删除仍随生产者保留的 pkg/kafka 消费代码。
- 保持取消传播、关闭超时、清理幂等及初始化失败释放；检查 channel 发送、等待与错误合并成对。
- config 使用严格解码，裁剪时同步移除配置字段与所有环境 YAML 段；仅在所有文件配置字段都为空时使用合法空映射 `{}`。
- 只移除不再被公共基线或保留组件使用的依赖，不主动升级依赖。

## 测试与文档边界

- 创建时不保留任何模板 `_test.go`、integration-test、测试辅助代码和仅供测试使用的资源、依赖与命令；同步不复制这些内容，不删除目标已有测试，必要的测试适配纳入方案。
- 不保留或同步组件局部说明，包括 `pkg/*/README.md`、config/README.md、internal 下的组件 README、docs/swagger/README.md 及其他组件使用文档；不按“所有 Markdown”批量删除。
- 保留并按公共基线和所选组件调整 AGENTS.md、CLAUDE.md、architecture.md、conventions.md、根 README.md。根文档不残留已裁剪组件的专属规则、配置说明和目录项，也不能删除公共分页、统一选型等公共规则。
- 删除指向已排除组件文档的链接；必要的使用规则、Swagger/Wire 生成命令放到职责对应的根文档。CLAUDE.md 只导入 AGENTS.md。
- 项目文档只记录各自职责内的内容：AGENTS.md 负责阅读入口、开发约束与验证，architecture.md 负责目录、分层与依赖，conventions.md 负责选型与使用约定，根 README.md 负责概览、启动、构建与生成命令；CLAUDE.md 仅导入 AGENTS.md。模板创建、同步、组件裁剪、demo 和测试及组件文档的保留策略只写在技能中，不写入这些项目文档。
- 创建时按应用名、用户已提供的项目用途及实际保留能力，改写根文档中涉及项目定位、能力和使用方式的描述，不只是替换模板名称或删除“模板”二字；不再将目标描述成用于创建其他项目的模板。未提供用途时只作项目名称、技术栈和实际能力的中性介绍，不虚构业务职责。保留适用的通用规范，不改写成创建或裁剪过程说明。
- 创建结果不保留模板维护专属表述，不宣称保留模板测试；保留后续业务开发的测试要求。同步不清理目标现有组件文档，也不把模板维护规则覆盖到业务项目。

## 验收场景

在独立且已授权的临时目录验证，不修改源，不连接未确认的真实服务：

1. 未给组件集合 → 列出可选项与依赖并询问，不写文件；列表不含分页、独立 Swagger 或迁移选项。
2. 只选 HTTP → 公共基线、分页与 cursor、Hello 与外部 HTTP demo、健康端点、Swagger 注释/脚本/JSON/YAML/生成命令存在；无存储、Kafka、调度和发布接口，Wire/build/vet 通过。
3. HTTP + MySQL + Redis → 单关系型数据库直接注入 *gorm.DB，无两种 Handle 包装；HTTP 配套 Swagger，配置、根文档和 demo 一致。
4. HTTP + PostgreSQL + Redis → 无 MySQL，newPostgres 与 newApp 直接使用 *gorm.DB，无包装类型，Postgres 清理完整。
5. MongoDB + 定时任务 → 连带事件总线、异步任务池与 HelloJob/订阅 demo；无 HTTP、PORT、Swagger，分页和公共工具仍在，取消退出和清理完整。
6. 只选 Kafka 消费者 → pkg/kafka 含完整生产者和消费者生产代码；只保留消费配置、消费 demo、App 消费启停，无生产配置、发布 demo、HTTP 与 Swagger。
7. HTTP + Kafka 生产者 → pkg/kafka 仍包含消费者代码；有发布 demo、发布路由和 Swagger，无消费配置、消费 demo、App consumers 装配。
8. 全部组件 → 双库分别以 mysqlHandle/postgresHandle 注入，Kafka 两侧装配与 demo 完整，HTTP 配套 Swagger，项目身份替换正确。
9. 无关系型数据库 → 不含 SQL 迁移专属内容，通用事务规范、公共分页及 lo/cast 选型保留；不恢复未选组件。
10. 任意组合 → 无模板测试、组件文档和模板技能，根文档只描述项目实际能力与开发规范，不含模板操作及内容保留策略，链接有效；已有目标或目标在源内时写入前停止。

11. 项目描述改写 → 已提供用途时，README 概览及其他根文档中的相关描述与用户说明、实际组件一致；未提供用途时只介绍应用名、技术栈和已有能力，不虚构业务。两种情况均不将新项目描述为项目模板，不以简单名称替换作为验收；正常的“配置模板”等术语和通用开发规范不误删。

上述为执行技能时的验收要求，不代表仅修改技能文本就已经生成并验证了这些项目。
