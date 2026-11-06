# 目录结构与职责

本文规定项目目录、各层职责与对象复用规则。下方目录树结合现有实现与后续组织示例，不代表所有文件均已创建；目录和文件按实际功能创建。

## 目录结构

```text
├── cmd/
│   └── app/
│       └── main.go                 # 程序入口：加载配置，调用 app 启动
├── config/
│   ├── config.go                   # 应用配置结构，按实际需求增加字段
│   ├── load.go                     # 按 APP_NAME、ENV 选择 YAML，Viper 加载
│   ├── load_test.go                # 环境选择与非法配置测试
│   └── README.md                   # 配置加载与启动方式说明
├── configs/                       # 按服务名_环境.yaml 区分完整配置
│   ├── app_dev.yaml
│   ├── app_test.yaml
│   └── app_prod.yaml
├── internal/
│   ├── app/
│   │   ├── app.go                  # 组装依赖，统一管理启动、关闭与资源释放
│   │   ├── http.go                 # 创建 HTTP 服务和 Controller，集中装配路由和中间件
│   │   ├── providers.go            # Initialize 入口、MySQL/Redis provider 与清理函数
│   │   ├── wire.go                 # Wire 依赖声明与私有 injector
│   │   ├── wire_gen.go             # Wire 生成的组装及失败清理代码
│   │   ├── messaging.go            # 按需：注册消息消费者，管理消费启停
│   │   └── scheduler.go            # 按需：注册定时任务，管理调度启停
│   ├── controller/                 # HTTP 接口适配，不承载核心业务逻辑
│   │   ├── user.go                 # 用户 HTTP handler
│   │   ├── order.go                # 订单 HTTP handler
│   │   ├── error.go                # 404、405 路由错误入口
│   │   ├── middleware/
│   │   │   ├── auth.go             # 按需：身份认证
│   │   │   └── middleware.go       # 请求标识、访问日志与 panic 恢复
│   │   └── response/
│   │       ├── response.go         # 统一响应外壳：code、message、data 等
│   │       └── error.go            # 业务错误解析、HTTP 状态映射与错误响应
│   ├── consumer/
│   │   └── order.go                # 消息消费入口：解码、调用 service、ACK 与重试
│   ├── job/
│   │   └── order_timeout.go        # 定时任务入口，调用 service
│   ├── entity/                     # 业务数据与错误定义，不依赖各层实现
│   │   ├── error.go                # 错误类型、公共错误与具体业务错误
│   │   ├── order.go                # 订单输入、结果及共用业务对象
│   │   ├── order_message.go        # 订单消息对象
│   │   └── user_message.go         # 用户消息对象
│   ├── service/                    # 应用业务逻辑，同一包内按业务拆文件
│   │   ├── user.go                 # UserService 及业务方法
│   │   ├── user_test.go            # 用户业务单元测试
│   │   ├── order.go                # OrderService 及业务方法
│   │   └── order_test.go           # 订单业务单元测试
│   └── repo/                       # DAO 职责：数据访问及外部能力的具体实现
│       ├── db/
│       │   ├── user.go             # 用户数据库访问
│       │   ├── order.go            # 订单数据库访问
│       │   ├── transaction.go      # 按需：Repo 内部事务辅助
│       │   └── model/
│       │       ├── user.go         # 用户表 DO、ORM 映射
│       │       └── order.go        # 订单表 DO、ORM 映射
│       ├── cache/
│       │   ├── user.go             # 用户缓存访问、key 与序列化
│       │   └── order.go            # 订单缓存访问
│       ├── webapi/                 # 主动调用其他系统的 HTTP 客户端
│       │   ├── inventory/          # 示例：内部库存系统
│       │       └── client.go       # 具体客户端；业务数据类型放 entity
│       │   └── wechat/             # 示例：第三方平台，与内部系统平级
│       │       └── client.go
│       └── publisher/
│           └── order.go            # 业务消息发布、消息编码与投递
├── pkg/                           # 不含业务知识的基础组件，按需封装
│   ├── log/                       # 日志初始化、Context 请求标识与通用封装
│   ├── db/                        # MySQL / GORM 连接与连接池初始化
│   ├── redis/                     # Redis 客户端初始化
│   ├── httpclient/                # 通用 HTTP 客户端配置
│   ├── httpserver/                # HTTP 服务超时、启动与优雅关闭
│   ├── rabbitmq/                  # 消息中间件基础封装，按实际选型命名
│   └── scheduler/                 # 可选：调度器基础封装
├── migrations/                    # 数据库版本迁移脚本
├── integration-test/              # 数据库、HTTP、MQ 等集成测试
├── docs/                          # 接口文档及其他开发文档
├── scripts/                       # 构建、检查、代码生成等脚本
├── directory-structure.md         # 本文：目录结构与职责
├── .gitignore
├── .env.example                   # 可选：环境变量示例，不包含真实凭据
├── Dockerfile                     # 按需：应用镜像构建
├── docker-compose.yml             # 按需：本地依赖服务编排
├── Makefile                       # 按需：统一构建、测试等命令
├── README.md                      # 启动方式、配置说明与开发指南
├── go.mod
└── go.sum                         # 引入依赖后由 Go 工具生成
```

配置文件与环境变量按配置方案选用。目录树中的业务名称为组织示例。

## 分层与依赖

- `controller`：HTTP 请求绑定与校验，调用 service，映射 HTTP 结果与错误。
- `consumer`：消息消费入口，解码和校验消息，调用 service，处理 ACK 与重试。
- `job`：定时任务入口，准备执行参数并调用 service。
- `entity`：统一存放业务数据结构体，包括 HTTP 请求与响应数据、service 输入与结果、共用业务对象、消息和业务任务参数，按业务分文件，不依赖 controller、consumer、job、service、repo 的实现或 Gin。
- `service`：业务校验、权限规则、流程编排，不编写事务；不依赖 HTTP 框架；可以导入 repo 包，字段及构造函数依赖接口，不绑定具体实现类型。
- `repo`：数据库、缓存、外部 HTTP 调用、消息发布的具体实现；封装完整原子操作及事务，不依赖 controller 的请求或响应类型。
- `app`：创建具体实现，通过构造函数注入依赖；负责进程级生命周期。
- `pkg`：不包含业务知识的基础组件，不导入本项目的 internal 包。

`repo` 承担 DAO 职责。

调用流程：

```text
HTTP handler ─┐
MQ consumer ──┼──→ service → 依赖接口 → repo 实现 → 数据库 / 缓存 / 外部系统
定时任务 job ─┘
```

仓储、缓存及外部能力接口定义在对应 repo 文件中，与实现放在同一包，由 `app` 将具体实现注入 service。service 可以导入 repo 包，但字段和构造函数使用接口，不使用具体实现类型。repo 不导入 service，业务数据使用 entity 或 `repo/db/model` 中的纯数据类型；model 不依赖 service 或 repo 实现。接口不放入 entity，也不再集中维护 service/contracts.go。

service 放在同一个 Go 包内，通过业务文件和不同 Service 类型区分。避免对象之间双向调用；跨业务流程由职责明确的编排对象协调。业务边界稳定、规模增长后可拆分子包，并保持单向依赖。

## 各层命名规范

业务组件类型使用“业务或系统名称 + 职责后缀”，依赖字段、构造参数和局部变量使用对应的小驼峰名称，使读者能从名称判断用途。即使依赖通过接口声明，变量名仍表达其实际职责。

| 职责 | 类型命名示例 | 字段、参数和变量命名示例 |
| --- | --- | --- |
| HTTP Controller | `UserController` | `userController` |
| 业务 Service | `UserService` | `userService` |
| 数据库 Repo | `UserRepo` | `userRepo` |
| 缓存 Cache | `UserCache` | `userCache` |
| 外部系统 Client | `InventoryClient` | `inventoryClient` |
| 定时任务 Job | `OrderExpirationJob` | `orderExpirationJob` |
| 消息消费者 Consumer | `OrderConsumer` | `orderConsumer` |
| 消息发布者 Publisher | `OrderPublisher` | `orderPublisher` |

- 构造函数与类型名对应，例如 `NewUserController`、`NewUserService`、`NewUserRepo`、`NewUserCache`、`NewInventoryClient`。
- 依赖变量不使用含义模糊的 `s`、`r`、`svc`、`repo`、`client` 或仅业务复数等名称；例如 Service 的数据库依赖命名为 `userRepo`，缓存依赖命名为 `userCache`，便于区分相同业务的不同能力。
- 接口按行为或能力命名，例如 `UserRepository`；注入该接口的数据库实现变量仍命名为 `userRepo`。不使用 `IUserRepo`、`UserInterface` 等无明确能力含义的命名。
- 此规则针对业务组件及其依赖，不给普通业务数据、标准库类型或基础库原生句柄强加层后缀；包名和文件名继续遵循 Go 与项目已有规范。

## 输入、结果与数据库对象

| 对象 | 位置 | 约定 |
| --- | --- | --- |
| service 输入对象 | `entity/<业务>.go` | 如 `CreateOrderInput`；HTTP 入口默认直接复用，简单参数可直接传递 |
| service 结果对象 | `entity/<业务>.go` | 如 `OrderResult`，可直接作为 HTTP 响应的 data |
| HTTP 专用请求 | `entity/<业务>.go` | 仅在与业务输入存在差异时定义 `XxxRequest`，不另设 request 包 |
| HTTP 专用响应数据 | `entity/<业务>.go` | 仅在与业务结果存在差异时定义 `XxxResponse` |
| HTTP 统一响应外壳 | `controller/response` | 仅存放 `code/message/data` 等公共响应结构 |
| 数据库 DO | `repo/db/model` | 数据库表字段、ORM 标签和存储映射；service 与 repo 可共用 |
| 跨层业务对象 | `entity/<业务>.go` | 共用的业务数据与相关规则 |
| MQ 消息体 | `entity/<业务>_message.go` | 发布、消费共用消息定义，同一消息不重复定义 |
| 定时任务业务参数 | `entity/<业务>.go` | 调用 service 时复用其输入或传简单参数；调度器实现与配置仍留在所属包 |
| 外部 API 业务数据 | `entity/<业务>.go` | 按系统或业务语义命名，具体客户端实现留在 repo/webapi |
| Controller、Service、Repo 实现结构体 | 各自所属包 | 持有依赖、实现行为，不放入 entity |
| 应用及通用组件配置、组件类型 | `config` 或对应 `pkg/<组件>` | 如 `db.Config`、`httpserver.Config`，不放入 entity |

### controller 与 service 的对象复用

Controller 与 Service 默认共用 entity 中的 Input、Result，不因跨层调用重复定义结构体。例如创建订单接口解析 `entity.CreateOrderInput` 并交给 Service，返回 `entity.OrderResult` 作为响应 data。service 不导入 controller。

只有简单参数时，直接调用 `GetOrder(ctx, userID, orderID)`，不必创建输入对象。

字段、语义或格式存在差异并需要转换时，在 entity 中定义独立的 Request/Response，由 controller 转换，不新建 request/response 业务数据包。登录用户 ID 等可信数据从认证上下文提取，不直接相信客户端提交的同名字段。

entity 中的 Input 允许携带 JSON、绑定及校验标签，但不导入 Gin。标签本身不要求拆分类型；需要独立的入口校验规则时再拆分。HTTP 入口执行绑定与格式校验，service 保证必要的业务规则，不能依赖 HTTP 校验已经执行。

entity 中的 Result 可以包含 JSON 标签，但不包含 HTTP 状态或统一响应外壳。HTTP 状态、业务错误映射，以及 `code/message/data` 等外壳由 controller 处理。

### service 与 repo 的对象复用

service 与 repo 优先使用同一个对象类型。字段、语义和数据表达一致时，直接传递和返回，不创建仅用于逐字段复制的对象。DO 可作为 service 的输入或结果，不要求为每个 DO 再定义一套 Input、Result。

只有实际需要转换时才拆分，例如数据库以 JSON 字符串存储而业务使用结构体，或查询结果需要聚合、投影。存储格式转换由 repo 完成，业务组合由 service 完成。

controller 复用 service 返回对象时，必须确认其序列化字段均允许对外返回。密码哈希、内部标记等字段应排除序列化，或通过 entity 中的独立结果/响应数据对象明确选择字段；请求绑定同样只允许调用方可设置的字段。

### entity 对象

entity 统一放置业务数据结构体，包括 HTTP 请求参数、响应数据、service 输入与结果、跨层业务对象及消息，按业务拆文件。普通业务数据放在 `<业务>.go`，消息对象放在 `<业务>_message.go`，不另设独立 message 包。

业务对象表达业务状态和规则；消息对象表达事件或消息契约，如 `OrderPaidMessage`。两者放在同一包内，仍按各自语义定义类型，不因字段相同就合并。

数据库 DO 放在 `repo/db/model`。Controller、Service、Repo 的实现结构体仍留在各自包中；应用配置和通用组件类型留在 config 或所属 pkg 包中，不迁入 entity。共用业务对象可直接用于 service 和 repo 的契约，字段与语义一致时复用，不为跨层调用额外转换。错误类型、公共错误与具体业务错误统一定义在 `entity/error.go`，不包含 HTTP 状态码；HTTP 错误解析和状态映射放在 `controller/response/error.go`。

## HTTP、MQ 与定时任务

### HTTP

本项目提供的 HTTP 接口统一放在 `controller`，不额外设置 http 子目录。内网可达性由监听地址、网关和网络配置等控制，调用权限由鉴权等机制控制。

`repo/webapi` 仅用于主动调用其他系统。内部系统与第三方平台按系统或厂商名平级建子目录，不使用嵌套的 `internal` 作为系统分类，避免触发 Go 的包导入限制。

### 消息消费与发布

- 消费入口放 `internal/consumer`，处理消息解码、校验、service 调用以及 ACK、重试等协议行为。
- 消息发布实现放 `internal/repo/publisher`；service 通过接口使用发布能力。
- 消费者注册与启停装配放 `internal/app/messaging.go`。
- 通用连接与发布消费基础封装按实际中间件命名，如 `pkg/rabbitmq` 或 `pkg/kafka`，不包含订单等业务知识。目录树中的 rabbitmq 为命名示例，不指定中间件选型。
- 消息对象统一放 `internal/entity/<业务>_message.go`。MQ 消息契约不因字段相同就与 HTTP 请求或数据库 DO 共用；消息格式可能独立演进。

### 定时任务

- 任务入口放 `internal/job`，生成执行时间、批次等参数后调用 service。
- 注册放 `internal/job/register.go`，由 Wire 注入 Service；启停由 App 管理，调度器基础封装放 `pkg/scheduler`。
- HTTP、MQ 与定时任务触发同一业务时复用 service，避免分别实现业务规则。

## 实施与验证

各功能按需落地。基础组件优先复用所选库的能力，封装项目需要统一的配置和生命周期行为。

单元测试与被测代码就近放置；需要数据库、MQ 或完整 HTTP 服务的验证放 `integration-test`。代码落地后优先检查受影响包的编译、测试及依赖方向。

## 事务职责

Service 不编写事务，不向 Repo 传入事务回调，也不暴露 GORM 事务句柄。需要原子完成的多个数据库步骤由一个 Repo 方法封装，内部负责事务、锁、条件更新、提交和失败回滚。Service 调用该原子操作进行业务编排，不能以多次独立写操作代替事务；依赖数据库当前状态的校验也在事务内完成。事务中不进行不必要的外部 HTTP 调用。

## 公共分页

列表请求和结果复用 `internal/entity/page.go` 的 `Page`、`PageList[T]`。Controller 校验输入格式，Service 先 Normalize 再 Validate，Repo 使用已校验的 Offset/Limit；默认值、边界与空列表规则见技术栈公共规范。
