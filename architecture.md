# 目录结构与职责

本文规定项目目录、各层职责与对象复用规则。按任务阅读相关章节，目录树仅在需要定位或组织文件时查阅。

按任务选择章节：[分层与依赖](#分层与依赖)、[各层命名规范](#各层命名规范)、[对象与复用](#对象与复用)、[HTTP 与 MQ](#http-与-mq)、[定时任务](#定时任务)、[进程内异步任务与事件](#进程内异步任务与事件)、[事务职责](#事务职责)、[缓存接入](#缓存接入)、[公共分页](#公共分页)、[测试放置](#测试放置)、[目录结构](#目录结构)。

## 分层与依赖

- `controller`：HTTP 请求绑定与校验，调用 service，映射 HTTP 结果与错误。
- `consumer`：消息消费入口，解码和校验消息，调用 service，处理 ACK 与重试。
- `event`：进程内事件入口，校验事件类型和数据，调用 service。
- `job`：定时任务入口，准备执行参数并调用 service。
- `entity`：集中存放项目业务数据、消息和业务错误，具体遵循对象与复用规则。
- `service`：业务校验、权限规则、流程编排，不编写事务；不依赖 HTTP 框架；可以导入 repo 包，字段及构造函数依赖接口，不绑定具体实现类型。
- `repo`：数据库、缓存、外部 HTTP 调用、消息发布的具体实现；封装完整原子操作及事务，不依赖 controller 的请求或响应类型。
- `app`：创建具体实现，通过构造函数注入依赖；负责进程级生命周期。
- `pkg`：不包含业务知识的基础组件，不导入本项目的 internal 包。各功能按需落地。基础组件优先复用所选库的能力，封装项目需要统一的配置和生命周期行为。

`repo` 承担 DAO 职责。

调用流程：

```text
HTTP Controller ──┐
MQ consumer ──────┤
Event Controller ─┼──→ service → 依赖接口 → repo 实现 → 数据库 / 缓存 / 外部系统
定时任务 Job ─────┘
```

仓储、缓存及外部能力接口定义在对应 repo 文件中，与实现放在同一包，由 `app` 将具体实现注入 service。service 可以导入 repo 包，但字段和构造函数使用接口，不使用具体实现类型。repo 不导入 service，项目业务数据使用 entity 或 `repo/db/model` 中的纯数据类型，三方接口对象使用对应客户端包类型；model 不依赖 service 或 repo 实现。

service 放在同一个 Go 包内，通过业务文件和不同 Service 类型区分。避免对象之间双向调用；跨业务流程由职责明确的编排对象协调。业务边界稳定、规模增长后可拆分子包，并保持单向依赖。

## 各层命名规范

业务组件类型使用“业务或系统名称 + 职责后缀”，依赖字段、构造参数和局部变量使用对应的小驼峰名称，使读者能从名称判断用途。即使依赖通过接口声明，变量名仍表达其实际职责。

| 职责 | 类型命名示例 | 字段、参数和变量命名示例 |
| --- | --- | --- |
| HTTP Controller | `UserController` | `userController` |
| Event Controller | `UserEventController` | `userEventController` |
| 业务 Service | `UserService` | `userService` |
| 数据库 Repo | `UserRepo` | `userRepo` |
| 缓存 Cache | `UserCache` | `userCache` |
| 外部系统 Client | `InventoryClient` | `inventoryClient` |
| 定时任务 Job | `OrderExpirationJob` | `orderExpirationJob` |
| 消息消费者 Consumer | `OrderConsumer` | `orderConsumer` |
| 消息发布者 Publisher | `OrderPublisher` | `orderPublisher` |

- 构造函数与类型名对应，例如 `NewUserController`、`NewUserService`、`NewUserRepo`、`NewUserCache`、`NewInventoryClient`。
- 依赖变量不使用含义模糊的 `s`、`r`、`svc`、`repo`、`client` 或仅业务复数等名称；例如 Service 的数据库依赖命名为 `userRepo`，缓存依赖命名为 `userCache`，便于区分相同业务的不同能力。
- 方法接收者（receiver）不受职责后缀规则约束，使用简短单词或字母，同一类型保持一致。
- 接口按行为或能力命名，例如 `UserRepository`；注入该接口的数据库实现变量仍命名为 `userRepo`。不使用 `IUserRepo`、`UserInterface` 等无明确能力含义的命名。
- 此规则针对业务组件及其依赖，不给普通业务数据、标准库类型或基础库原生句柄强加层后缀；包名和文件名继续遵循 Go 与项目已有规范。

## 对象与复用

项目业务数据集中放在 entity，按业务拆文件；不另设业务 request、response 或 message 包。

| 对象 | 命名与约定 | 位置 |
| --- | --- | --- |
| Controller 专用请求、响应 | `XxxReq`、`XxxResp` | `entity/<业务>.go` |
| Service 业务对象 | 按业务语义命名，含查询条件、聚合结果，不强制 `Input/Result` 后缀 | `entity/<业务>.go` |
| Repo 存储模型 | 按表或集合命名，字段对应存储结构 | `repo/db/model` |
| MQ 消息 | `XxxMessage` | `entity/<业务>_message.go` |
| 进程内业务事件 | 实现 `event.Event`，例如 `UserRegisteredEvent`。字段和语义一致时复用已有业务对象 | `entity/<业务>_event.go` 或已有业务文件 |
| 三方接口对象 | 遵循对应接口语义，可直接供调用方使用 | 对应 `repo/webapi/<系统>` 包 |

- 字段和语义一致时直接复用，包括数据库模型；简单参数直接传递，任务参数复用业务对象，不为分层配齐多套对象。
- 仅在格式、语义、公开字段或契约兼容性要求存在差异时拆分，不增加仅逐字段复制的对象；消息契约的判断见[消息消费与发布](#消息消费与发布)。
- Repo 负责存储读写、必要的存储格式转换，以及数据库筛选、排序和分页；Service 负责业务组合、计算和状态推导。Repo 的筛选、排序条件应与业务规则保持一致。
- 入口负责绑定与格式校验，避免在 Service 中机械重复；Service 保证业务约束，兼顾 Job、消息等非 HTTP 调用。
- 业务实体可封装仅依赖自身字段和显式参数的状态判断、合法性判断及状态转换；实体不依赖 Repo、数据库、HTTP、缓存或日志，时间判断通过显式传入 `time.Time`。跨实体规则、外部调用和流程编排仍由 Service 负责。
- 请求仅绑定调用方可设置的字段，可信身份由入口取得并显式传入；对外输出排除敏感字段。entity 不依赖 Gin 或各层实现，可携带序列化、绑定及校验标签。

配置和基础组件类型放在 `config` 或对应的 `pkg` 包。组件实现与接口留在各自所属包。

### 错误与响应适配

错误类型、公共业务错误及具体业务错误统一放在 `internal/entity/error.go`。业务错误不包含 HTTP 状态。HTTP 响应输出和公共响应外壳放在 `internal/controller/response`，错误到 HTTP 响应的转换由该包的 `error.go` 负责。 [错误处理约定](conventions.md#错误处理约定)

## HTTP 与 MQ

### HTTP

本项目提供的 HTTP 接口统一放在 `controller`，不额外设置 http 子目录。内网可达性由监听地址、网关和网络配置等控制，调用权限由鉴权等机制控制。

### 外部 HTTP 客户端

`internal/repo/webapi` 仅用于主动调用其他系统。内部系统与第三方平台按系统或厂商名平级建子目录，不使用嵌套的 `internal` 作为系统分类，避免触发 Go 的包导入限制。客户端通过构造函数注入和复用，调用方依赖接口。 [外部 HTTP 调用约定](conventions.md#外部-http-调用)

### 消息消费与发布

- 消费入口放 `internal/consumer`：controller 实现 handler 接口、持有 service 依赖，按 topic + event type 注册分发并校验消息字段；data 解码、ACK、重试等协议行为由对应的基础封装处理。
- 消息发布实现放 `internal/repo/publisher`；service 通过接口使用发布能力。
- 消费者注册与启停装配放 `internal/app/messaging.go`。
- 通用连接与发布消费基础封装按实际中间件命名，如 `pkg/rabbitmq` 或 `pkg/kafka`，不包含订单等业务知识。目录树中的 rabbitmq 为命名示例，不指定中间件选型。
- 消息对象统一放 `internal/entity/<业务>_message.go`。消息由独立发布者、消费者维护，或有独立版本及兼容性要求时，单独定义消息类型；其余场景按字段和语义判断是否复用已有对象。

## 定时任务

任务放在 `internal/job`，实现 `scheduler.Task`。`Run(ctx)` 准备触发本次业务所需的参数，再调用 Service。同一业务可以由 HTTP、MQ、定时任务或事件触发，业务规则统一由 Service 实现。

`internal/app/jobs.go` 接收 Wire 装配的 Service，创建调度器和任务，并在应用启动前完成注册。 [调度行为](conventions.md#定时任务)

## 进程内异步任务与事件

- **事件数据**在 `internal/entity` 定义，事件用 `Name()` 提供稳定的名称。Event Controller 在 `internal/event` 校验具体类型和业务参数，再调用注入的 Service。
- **发布**由需要发事件的 Service 编排。Service 声明并接收发布接口，Wire 将共享 Bus 绑定到该接口。Bus 只依赖任务池，Service 和事件入口均可在 Bus 创建后装配。
- **订阅**在 `internal/app/events.go` 装配。App 创建具体 Event Controller，显式绑定事件名和处理器。
- **直接提交异步任务**时，通过构造函数接收共享任务池。需要按事件名向多个处理器分发时使用 Bus。 [提交与执行约定](conventions.md#进程内异步任务与事件)

## 缓存接入

缓存访问、key 和序列化放在 `internal/repo/cache`，Service 依赖对应接口。按业务需要接入缓存，不预先创建通用缓存框架。 [缓存使用规范](conventions.md#缓存使用规范)

## 测试放置

单元测试与被测代码就近放置。需要数据库、MQ 或完整 HTTP 服务的验证放在 `integration-test`。 [验证流程](AGENTS.md#验证)

## 事务职责

Service 不编写事务，不向 Repo 传入事务回调；完整原子操作及事务由 Repo 封装，Service 调用其方法进行业务编排。数据库事务不覆盖外部系统副作用，事务中不进行不必要的网络调用。

GORM / MySQL 与 PostgreSQL 的事务写在 Repo 层，不暴露 GORM 事务句柄。需要原子完成的多个数据库步骤由一个 Repo 方法封装，内部负责事务、锁、条件更新、提交和失败回滚；依赖数据库当前状态的校验也在事务内完成，不能以多次独立写操作代替事务。

MongoDB 的原子操作与事务同样由 Repo 封装。操作选择、失败语义和重试边界见 [MongoDB 原子性](conventions.md#mongodb-原子性)。

## 公共分页

分页类型、各层处理职责及边界规则统一见[公共分页](conventions.md#公共分页)。页码分页使用 `entity.Page`，游标分页使用 `entity.CursorPage`，游标编解码位于 `pkg/cursor`。

## 目录结构

目录树列出项目的主要目录和关键文件。业务文件以现有 Hello demo 为例，新增同类业务文件时无需更新目录树。标注“按需”的路径随实际需求创建。

```text
├── cmd/
│   └── app/
│       └── main.go                 # 程序入口，加载配置并启动应用
├── config/                         # 应用配置定义、加载与监听
├── configs/                        # 按服务名_环境.yaml 存放配置
│   ├── app_dev.yaml
│   ├── app_test.yaml
│   └── app_prod.yaml
├── internal/
│   ├── app/                        # 依赖装配与生命周期管理
│   │   ├── app.go                  # 统一管理应用启停与资源释放
│   │   ├── http.go                 # 注册路由和中间件
│   │   ├── events.go               # 注册进程内事件订阅
│   │   ├── jobs.go                 # 注册定时任务
│   │   ├── providers.go            # 初始化入口、组件构造与清理
│   │   ├── wire.go                 # Wire 依赖声明
│   │   ├── wire_gen.go             # Wire 生成的装配代码
│   │   └── messaging.go            # 注册消息消费者并管理启停
│   ├── controller/                 # HTTP 接口适配
│   │   ├── hello.go
│   │   ├── error.go                # 路由不存在、方法不支持的错误入口
│   │   ├── middleware/             # 请求标识、访问日志与 panic 恢复
│   │   │   └── auth.go             # 按需，接入身份认证
│   │   └── response/               # 统一响应和 HTTP 错误适配
│   ├── consumer/                   # MQ 消费入口
│   │   └── hello.go
│   ├── event/                      # 进程内事件入口
│   │   └── hello.go
│   ├── job/                        # 定时任务入口
│   │   └── hello.go
│   ├── entity/                     # 业务数据、事件、公共分页与错误定义
│   │   ├── hello.go
│   │   ├── hello_event.go
│   │   ├── hello_message.go
│   │   ├── error.go
│   │   └── page.go
│   ├── service/                    # 业务逻辑，按业务拆分文件
│   │   ├── hello.go
│   │   ├── hello_event.go
│   │   └── hello_message.go
│   └── repo/                       # 数据访问与外部能力实现
│       ├── db/                     # 数据库存储访问
│       │   ├── hello.go
│       │   └── model/              # 按需，数据库模型
│       ├── cache/                  # 按需，缓存访问
│       ├── webapi/                 # 外部 HTTP 客户端，按系统划分子包
│       │   └── hello/
│       │       └── client.go
│       └── publisher/              # 业务消息发布
│           └── hello.go
├── pkg/                            # 基础组件
│   ├── async/                      # 进程内异步任务池
│   ├── cursor/                     # 游标编解码与签名
│   ├── db/                         # MySQL / GORM 连接与连接池
│   ├── env/                        # 应用名称与运行环境
│   ├── event/                      # 进程内事件总线
│   ├── httpclient/                 # 按需，通用 HTTP 客户端封装
│   ├── httpserver/                 # HTTP 服务与优雅关闭
│   ├── log/                        # 日志与请求标识
│   ├── mongodb/                    # MongoDB 客户端
│   ├── postgres/                   # PostgreSQL / GORM 连接与连接池
│   ├── rabbitmq/                   # 按需，MQ 基础封装，按实际选型命名
│   ├── redis/                      # Redis 客户端
│   └── scheduler/                  # 定时任务调度
├── migrations/                     # 按需，数据库版本迁移脚本
├── integration-test/               # 依赖实际组件的集成测试
├── docs/
│   └── swagger/                    # Swagger 接口文档
├── scripts/
│   └── generate-api-docs/          # Swagger 文档生成入口
├── .gitignore
├── .env.example                    # 按需，环境变量示例
├── AGENTS.md                       # 项目规则与文档阅读入口
├── architecture.md                 # 目录、分层依赖与事务边界
├── CLAUDE.md                       # Claude Code 入口，导入 AGENTS.md
├── conventions.md                  # 组件选型与通用约定
├── Dockerfile                      # 按需，应用镜像构建
├── docker-compose.yml              # 按需，本地依赖服务编排
├── Makefile                        # 构建、生成与测试命令
├── README.md                       # 项目概览、启动与验证
├── go.mod
└── go.sum
```
