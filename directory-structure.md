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
│   │   ├── error.go                # 业务错误到 HTTP 状态、错误响应的映射
│   │   ├── middleware/
│   │   │   ├── auth.go             # 按需：身份认证
│   │   │   └── middleware.go       # 请求标识、访问日志与 panic 恢复
│   │   ├── request/
│   │   │   └── order.go            # 可选：与 service 输入有差异的请求对象
│   │   └── response/
│   │       ├── response.go         # 统一响应外壳：code、message、data 等
│   │       └── order.go            # 可选：与 service 结果有差异的响应对象
│   ├── consumer/
│   │   └── order.go                # 消息消费入口：解码、调用 service、ACK 与重试
│   ├── job/
│   │   └── order_timeout.go        # 定时任务入口及专用参数，调用 service
│   ├── entity/                     # 跨层共用的业务数据类型，不依赖各层实现
│   │   ├── order.go                # 业务对象及相关状态、规则
│   │   ├── order_message.go        # 订单消息对象
│   │   └── user_message.go         # 用户消息对象
│   ├── service/                    # 应用业务逻辑，同一包内按业务拆文件
│   │   ├── user.go                 # UserService 及业务方法
│   │   ├── user_types.go           # 用户业务 Input、Result
│   │   ├── user_test.go            # 用户业务单元测试
│   │   ├── order.go                # OrderService 及业务方法
│   │   ├── order_types.go          # 订单业务 Input、Result
│   │   ├── order_test.go           # 订单业务单元测试
│   │   └── contracts.go            # service 所需的仓储、缓存、消息发布等接口
│   └── repo/                       # DAO 职责：数据访问及外部能力的具体实现
│       ├── db/
│       │   ├── user.go             # 用户数据库访问
│       │   ├── order.go            # 订单数据库访问
│       │   ├── transaction.go      # 按需：事务执行适配
│       │   └── model/
│       │       ├── user.go         # 用户表 DO、ORM 映射
│       │       └── order.go        # 订单表 DO、ORM 映射
│       ├── cache/
│       │   ├── user.go             # 用户缓存访问、key 与序列化
│       │   └── order.go            # 订单缓存访问
│       ├── webapi/                 # 主动调用其他系统的 HTTP 客户端
│       │   ├── inventory/          # 示例：内部库存系统
│       │       ├── client.go
│       │       └── types.go        # 对方系统专用的请求、响应结构
│       │   └── wechat/             # 示例：第三方平台，与内部系统平级
│       │       ├── client.go
│       │       └── types.go
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
- `entity`：跨层共用的业务数据类型，包括业务对象和消息对象，不依赖 controller、consumer、job、service、repo 的实现。
- `service`：业务校验、权限规则、流程编排与事务边界；不依赖 HTTP 框架或 repo 具体实现。
- `repo`：数据库、缓存、外部 HTTP 调用、消息发布的具体实现；不依赖 controller 的请求或响应类型。
- `app`：创建具体实现，通过构造函数注入依赖；负责进程级生命周期。
- `pkg`：不包含业务知识的基础组件，不导入本项目的 internal 包。

`repo` 承担 DAO 职责。

调用流程：

```text
HTTP handler ─┐
MQ consumer ──┼──→ service → 依赖接口 → repo 实现 → 数据库 / 缓存 / 外部系统
定时任务 job ─┘
```

service 所需的依赖接口定义在 `service/contracts.go`，由 `app` 将 repo 实现注入。service 不导入 repo 的具体实现。repo 可以使用 service 定义的契约类型；service 可以使用 `repo/db/model` 中的纯数据类型，该包不得依赖 service 或 repo 实现，以避免循环依赖。

service 放在同一个 Go 包内，通过业务文件和不同 Service 类型区分。避免对象之间双向调用；跨业务流程由职责明确的编排对象协调。业务边界稳定、规模增长后可拆分子包，并保持单向依赖。

## 输入、结果与数据库对象

| 对象 | 位置 | 约定 |
| --- | --- | --- |
| service 输入对象 | `service/*_types.go` | 需要操作专用输入时定义，如 `CreateOrderInput`；与 DO 一致时可复用 DO |
| service 结果对象 | `service/*_types.go` | 需要业务投影或组合结果时定义，如 `OrderResult`；与 DO 一致时可复用 DO |
| HTTP 专用请求 | `controller/request` | HTTP 请求到 service 输入需要转换时定义 |
| HTTP 专用响应 | `controller/response` | 与 service 结果存在展示差异时定义；统一响应外壳也放这里 |
| 数据库 DO | `repo/db/model` | 数据库表字段、ORM 标签和存储映射；service 与 repo 可共用 |
| 跨层业务对象 | `entity/<业务>.go` | 共用的业务数据与相关规则 |
| MQ 消息体 | `entity/<业务>_message.go` | 发布、消费共用消息定义，同一消息不重复定义 |
| 定时任务专用参数 | 对应 `internal/job` 文件内 | 仅入口使用的参数留在 job；调用 service 时复用其输入或传简单参数 |
| 外部 API 对象 | `repo/webapi/<系统>/types.go` | 被调用系统的请求、响应格式，留在对应客户端包 |

### controller 与 service 的对象复用

默认复用 service 输入、结果类型，不因跨层调用重复定义结构体。HTTP controller 可以直接绑定 service Input，也可以直接返回 service Result。service 不导入 controller。

只有简单参数时，直接调用 `GetOrder(ctx, userID, orderID)`，不必创建输入对象。

字段、语义或格式存在差异并需要转换时，定义独立 request/response，由 controller 转换。登录用户 ID 等可信数据从认证上下文提取，不直接相信客户端提交的同名字段。

复用的 service Input 允许携带 JSON、绑定及校验标签。标签本身不要求拆分类型；需要独立的入口校验规则时再拆分。HTTP 入口执行绑定与格式校验，service 保证必要的业务规则，不能依赖 HTTP 校验已经执行。

service Result 可以包含 JSON 标签，但不包含 HTTP 状态或统一响应外壳。HTTP 状态、业务错误映射，以及 `code/message/data` 等外壳由 controller 处理。

### service 与 repo 的对象复用

service 与 repo 优先使用同一个对象类型。字段、语义和数据表达一致时，直接传递和返回，不创建仅用于逐字段复制的对象。DO 可作为 service 的输入或结果，不要求为每个 DO 再定义一套 Input、Result。

只有实际需要转换时才拆分，例如数据库以 JSON 字符串存储而业务使用结构体，或查询结果需要聚合、投影。存储格式转换由 repo 完成，业务组合由 service 完成。

controller 复用 service 返回对象时，必须确认其序列化字段均允许对外返回。密码哈希、内部标记等字段应排除序列化，或通过独立 response 明确选择字段；请求绑定同样只允许调用方可设置的字段。

### entity 对象

entity 放置跨层共用的业务数据类型，按业务拆文件。业务对象放在 `<业务>.go`，消息对象放在 `<业务>_message.go`，不另设独立 message 包。

业务对象表达业务状态和规则；消息对象表达事件或消息契约，如 `OrderPaidMessage`。两者放在同一包内，仍按各自语义定义类型，不因字段相同就合并。

service 专用 Input、Result 放在 service；数据库 DO 放在 `repo/db/model`。共用业务对象可直接用于 service 和 repo 的契约，字段与语义一致时复用，不为跨层调用额外转换。业务错误定义在 `service/errors.go`。

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
- 注册、启停放 `app/scheduler.go`，调度器基础封装确有需要时放 `pkg/scheduler`。
- HTTP、MQ 与定时任务触发同一业务时复用 service，避免分别实现业务规则。

## 实施与验证

各功能按需落地。基础组件优先复用所选库的能力，封装项目需要统一的配置和生命周期行为。

单元测试与被测代码就近放置；需要数据库、MQ 或完整 HTTP 服务的验证放 `integration-test`。代码落地后优先检查受影响包的编译、测试及依赖方向。
