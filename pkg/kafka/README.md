# Kafka

`pkg/kafka` 使用 franz-go。`NewProducer` 和 `NewConsumer` 在构造期校验配置，不建立连接。`Producer` 是共享发布接口，由 App 按 `kafka` 段创建，并在组件停止后通过 Wire cleanup 关闭；`Consumer` 是可启停的消费组件，由 App 创建并管理。`kafka.brokers` 为空时启动失败；`kafka_consumer` 段有内容时注册 hello 示例消费者，为空时不注册。

## 配置

`Config` 是生产者配置，包含 broker 和 topic 映射；`ConsumerConfig` 描述一个消费组，自带 broker、group、逻辑 topic、映射和重试参数。两者互不依赖。

生产者在各环境的 `configs/{APP_NAME}_{ENV}.yaml` 中读取 `kafka` 段：

```yaml
kafka:
  brokers:
    - "127.0.0.1:9092"
  topic_mapping:
    social-forum: social-forum-dev
```

App 在装配阶段用 `kafka` 段创建 `Producer`，业务层通过 `internal/repo/publisher` 的接口
发布消息。Wire cleanup 在 HTTP、任务和消费者停止后关闭生产者并 flush 待投递消息。

消费者从各环境的 `kafka_consumer` 段构造，内容是一个完整的 `ConsumerConfig`，由 App
装配为 `Consumer`：

```yaml
kafka_consumer:
  brokers:
    - "127.0.0.1:9092"
  group_name: goempty-hello
  topics:
    - hello
  topic_mapping:
    hello: hello-dev
  max_attempts: 3    # 可选，默认 3
  retry_backoff: 1s  # 可选，默认 1s
```

代码中发布和订阅都使用小写逻辑名，例如 `social-forum` 或 `forum.events`，实际 topic 保留配置中的大小写。`GroupName` 直接作为 Kafka group.id，环境区分写在名称本身。消费者的 `Topics` 与 `ConsumerConfig.TopicMapping` 要求一一对应，包内不做校验和回退；发布端用 `Config.TopicMapping`，缺失时回退为逻辑名。构造期不校验 topic 列表和映射内容，配置错误由 Kafka 在运行期报告。`MaxAttempts` 和 `RetryBackoff` 控制进程内重试，零值分别取默认的 3 次和 1 秒，负值构造失败。生产者和消费者在构造时读取配置，修改配置后重启应用。

## 事件格式

| JSON 字段 | 用途 |
| --- | --- |
| `_event_id` | 必填的事件唯一标识。`NewMessageRawData` 自动生成 UUID；重发时沿用原值，可供消费端去重。 |
| `_event_source` | 必填的事件来源服务名。`NewMessageRawData` 固定写入包内 `eventSource` 常量。 |
| `_event_type` | 必填的消息类型，例如 `forum.post.created`。同一 topic 可以承载不同类型。 |
| `_event_time` | 必填的事件发生时间。`NewMessageRawData` 使用当前 UTC 时间，发布时以 UTC 序列化。 |
| `data` | 必填的消息数据，Go 类型为 `any`，发布时由 `encoding/json` 序列化。 |

按 `_event_type` 分发。消费端用 `Subscribe` 注册的 handler 声明 `data` 的业务结构体类型，
框架按该类型解码，不再解析为通用 JSON。业务消息结构放在 `internal/entity`，发布和消费业务入口遵循 [MQ 分层约定](../../architecture.md#消息消费与发布)。

消费端还会在 `MessageRawData` 上填充 `Topic`、`Partition`、`Offset`。这三个字段带
`json:"-"`，不属于线格式，发布时为零值。`Topic` 是 `ConsumerConfig.Topics` 里的逻辑
topic（映射前的名字），实际 topic 只用于和 Kafka 通信；代码里的 handler 和分发逻辑因此
不随环境变化。分发按逻辑 topic + event type 进行，消息必须命中已注册的组合才会调用
handler；同名事件在不同 topic 语义不同时天然由 topic 维度区分。

## 发布

用 `NewMessageRawData(eventType, data)` 创建事件，它会生成 `_event_id`、写入当前 UTC
`_event_time` 和服务来源。生成新项目时把包内的 `eventSource` 常量改为实际服务名。

应用内发布实现放 `internal/repo/publisher`，Service 通过接口注入。`POST /hello/message`
是 hello 示例入口：请求体为消息 data，发布到 `hello` topic，由 hello 消费者处理。

同步调用 `Publish(ctx, topic, event, opts...)` 等待投递结果。异步调用
`PublishAsync(ctx, topic, event, done, opts...)` 的返回值报告本地校验错误。返回
`nil` 时，投递结果由 `done` 回调提供，也可能是缓冲区已满。异步调用的 context 应保持到
回调执行。回调应尽快返回。需要使用同一个生产者继续发布时，在回调外发起调用。

可以用 `WithKey` 设置 Kafka 分区键，用 `WithHeader` 添加消息头，用
`WithTimestamp` 设置 Kafka 记录时间。分区键与 JSON 的 `_event_type` 独立。
停止提交后用带截止时间的 context 调用 `Producer.Close`，等待已缓冲的投递。

## 消费

`NewConsumer(consumerCfg, handler)` 在构造期绑定 handler 并解析 topic，不建立连接。
handler 实现单个方法 `Handle(ctx, event)`；日常用法是用 `NewRouter` 和泛型函数
`Subscribe` 按逻辑 topic + event type 注册分发：

```go
router := kafka.NewRouter()
err := kafka.Subscribe(router, "social-forum", entity.PostCreated, consumer.NewPostCreatedConsumer(deps.PostService))
kafkaConsumer, err := kafka.NewConsumer(consumerCfg, router)
```

handler 实现 `TypedHandler[Data]` 接口：`Name()` 返回注册校验和日志使用的名称，
`Handle(ctx, event, data)` 处理单条消息，`Data` 是业务声明的 data 结构体类型，框架先
解码再调用。controller 构造函数注入 service 依赖，入口放 `internal/consumer`，写法与
进程内事件入口（`internal/event`）一致。需要完全自定义分发时，可直接实现 `Handler`
接口或用 `HandlerFunc` 适配函数。

消息来自未注册的 topic 和 event type 组合时，记日志跳过，不重试。

注册约束：topic + event type 组合在 Router 上唯一，重复注册返回 `ErrDuplicateRoute`；
topic、event type 和 handler 名称非空且不含首尾空白；handler 为 nil 报错。

`Start(ctx)` 创建 client 并在后台启动单个消费 goroutine，不等待 broker 连通；ctx 是运行
上下文的父节点，取消即停止消费。每个消费者只能 Start 一次。`Stop(ctx)` 取消消费并等待
goroutine 退出，可在 Start 之前、重复调用或在超时后再次调用；ctx 只限制等待时间，handler
必须响应取消。`Done()` 在 goroutine 退出后关闭，`Err()` 返回运行错误，正常停机为 `nil`。

消费循环按批顺序处理：

- handler 返回 `nil`：提交该记录，继续下一条；
- handler 返回错误：同一条记录退避后重试。`RetryBackoff` 从配置值开始，每次失败翻倍，
  封顶 30 秒；达到 `MaxAttempts` 后记录 `topic/partition/offset/event_id/event_type`
  并跳过该记录，继续后续消息；
- 消息信封解码失败、topic + event type 组合未注册、`data` 与 handler 声明类型不符：
  重试无法修复，记录日志并跳过；
- handler panic：recover 并记录堆栈，按 handler 错误处理，进程不退出；
- 轮询、提交或退组失败：消费者退出并通过 `Done`/`Err` 报告，App 视为致命错误。

重试是原地进行的，会阻塞这个消费者订阅的所有分区，并推迟 rebalance；应保持较小的
`MaxAttempts` 和退避。handler 失败前可能已经产生副作用，重试要求业务处理幂等。被跳过的
记录不会再次投递，可在 Kafka 保留期内按日志中的 offset 回溯。

停机时取消 `Start` 的 context 或调用 `Stop`。已处理的记录仍会尝试在限时内提交，随后退出
消费组；不响应取消的 handler 会让停止等待到超时。

## 验证

```sh
go test ./pkg/kafka ./config
GOEMPTY_TEST_KAFKA_BROKERS=127.0.0.1:9092 go test -count=1 ./integration-test -run TestKafkaLifecycle
```

集成测试需要隔离 broker，测试账号需要创建和删除测试 topic 的权限。
