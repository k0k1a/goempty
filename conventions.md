# 技术栈与公共规范

本文记录项目已确定的技术选型。已有方案文档中涉及以下组件的建议，以本文为准。

按任务选择章节：[组件选型](#组件选型)、[版本约定](#版本约定)、[错误处理约定](#错误处理约定)、[公共分页](#公共分页)、[外部 HTTP 调用](#外部-http-调用)、[配置](#配置)、[生命周期](#生命周期)、[MongoDB 原子性](#mongodb-原子性)、[数据库变更](#数据库变更)、[缓存使用规范](#缓存使用规范)、[定时任务](#定时任务)、[事务](#事务)。组件 README 链接用于需要具体细节时查阅。

## 组件选型

| 类别 | 选型 | 用途 |
| --- | --- | --- |
| Web 框架 | Gin | HTTP 路由、中间件及接口请求处理 |
| 数据访问 | GORM | 关系型数据库的 ORM 与数据访问 |
| 关系型数据库 | MySQL | 通过 GORM MySQL 驱动访问，基础封装位于 `pkg/db` |
| Redis 客户端 | go-redis | Redis 连接与数据操作 |
| MongoDB 客户端 | MongoDB 官方 Go Driver v2 | `pkg/mongodb` 提供连接封装，由 App 管理共享客户端，见[使用说明](pkg/mongodb/README.md) |
| 配置管理 | Viper | `config` 包加载 YAML 并监听文件变化；`APP_NAME`、`ENV` 选择文件，入口独立读取 `PORT`，连接参数不提供环境变量覆盖，见 [配置说明](config/README.md) |
| 日志 | Zap + Lumberjack | `pkg/log` 默认输出 JSON 到 stdout；独立实例支持文件输出与按大小轮转，完整配置由调用方传入，见 [使用说明](pkg/log/README.md) |
| 错误处理 | 自定义错误 | 承载业务错误码、错误消息等信息，供接口层统一解析 |
| 依赖注入 | Wire | 编译期依赖注入与依赖组装 |
| API 文档生成 | Swaggo v1.16.6 | 从 Go 注释离线生成可导入的 Swagger 2.0 JSON/YAML，见 [生成说明](docs/swagger/README.md) |

## 错误处理约定

- `entity.Error` 包含整数 `Code` 和可公开的 `Message`，错误类型、公共错误与后续具体业务错误统一放在 `internal/entity/error.go`；公共错误编号为 `100001`～`100008`，已有编号保持稳定。
- Controller 通过 `response.RespondError`（位于 `controller/response/error.go`） 使用 `errors.As` 提取业务错误，直接返回其 Code 和公开 Message，无需登记 code；支持 `%w` 包装。
- 请求绑定与格式校验沿用 Gin 的 `ShouldBind*` 方法和 `binding` 标签；失败时调用 `response.RespondError(c, entity.ErrInvalidArgument)`，统一返回 HTTP 400、code `100001`、`请求参数不合法`，不直接返回原始绑定错误或请求值。见 [HTTP 公共响应](internal/controller/response/README.md)。
- `entity.NewBusinessError(message)` 用于可公开的动态校验消息，返回 HTTP 400、code `-1`；与未知错误共享 code 和 HTTP 状态，需要稳定区分时使用独立业务 code。禁止传入内部错误原文。
- 普通 error（包括类型化 nil 业务错误）统一返回 HTTP 400、`code: -1`、`message: "未知错误"`，原始错误记录日志。
- 普通 JSON 响应使用 `code/message/data` 外壳，成功固定 HTTP 200、code 为 0，失败固定 HTTP 400；公共错误码和例外见 [HTTP 公共响应](internal/controller/response/README.md)。

## 版本约定

- 已引入依赖的版本以 `go.mod` 为准；其他组件按实际需求选型。

## 公共分页

- 请求与结果复用 `entity.Page`、`PageList[T]`，结果放在公共响应 data 中，包含 page、page_size、total、list。Controller 校验输入格式，Repo 使用 Service 已校验的 Offset/Limit。
- 页码默认 1，每页默认 20、最大 100；省略或为 0 时使用默认值。Service 先 Normalize 再 Validate，拒绝负数、超限及 offset 整数溢出，不静默截断，不设置固定最大页码。
- Repo 按相同筛选条件查询分页前总数和当前页，使用稳定排序。超出最后一页返回空列表，保留实际 total；List 应初始化为空 slice，输出 `[]`。

## 外部 HTTP 调用

客户端按系统放在 repo/webapi，通过构造函数复用并依赖接口；默认不发起外部请求。使用标准库的最小示例见 [Hello 客户端](internal/repo/webapi/hello/README.md)。传递已有 Context，设置超时，检查 HTTP 与业务状态，限制并解析响应，可靠关闭响应体，不泄露凭据或上游原始响应。仅按幂等性和实际协议设计重试。

## 配置

- 配置读取 `configs/{APP_NAME}_{ENV}.yaml`，以进程工作目录为基准；环境文件独立，不合并、不回退，缺失、格式错误或未知字段导致启动失败。
- 数据库连接参数只从 YAML 读取；端口由 `PORT` 控制，HTTP 和关闭超时由入口设置。具体字段和默认值以代码及环境模板为准，不在规范中复制完整配置。
- 配置热加载只更新文件配置快照，不重建连接池、不重新执行依赖组装；连接参数、入口 HTTP 和关闭参数变更需重启。业务读取快照时不得修改其中的引用数据。

## 生命周期

- 进程入口负责信号与退出码，App 负责依赖组装和运行组件启停；资源的初始化、运行和关闭应有明确所有者。
- 构造函数不提前监听或启动未托管的后台任务，路由和中间件在监听前完成注册；初始化失败时释放已创建资源。
- 退出时先停止接收请求和调度任务，取消并限时等待在途处理，再释放连接池，最后关闭日志；HTTP 排空超时后强制关闭连接，不能仅等待监听退出就结束进程。
- 沿调用链传递已有 Context，业务应响应取消；清理使用保留调用链值、独立于运行取消状态的限时 Context，关闭连接不能代替业务退出。

## MongoDB 原子性

- 根据访问模式和数据规模设计合理的嵌套结构，优先通过单文档条件更新保证原子性；多个嵌套字段可在同一条写操作中更新，单集合不等于单文档。
- 多个文档要求共同成功或失败时使用事务，由 Repo 封装；允许部分成功时，在业务方案中明确失败语义和重试边界。
- 不为规避事务而把持续增长的流水等数据无限嵌入一个文档，避免无界数组。

依据：[MongoDB 原子性与事务](https://www.mongodb.com/docs/manual/core/transactions/)、[避免无界数组](https://www.mongodb.com/docs/manual/data-modeling/design-antipatterns/unbounded-arrays/)。

## 数据库变更

- `migrations` 使用手工 SQL，不接入自动迁移；一个需求分支对应一个文件，可包含多个表的变更。
- 上线前可调整该需求文件；上线后保留已执行内容，后续变更新增脚本。
- 项目仅维护数据库变更 SQL，由用户确定执行范围、顺序及执行方式；Agent 未经明确授权不执行数据库变更。

## 缓存使用规范

- 缓存访问、key 和序列化放在 repo/cache，Service 依赖对应接口；不预先创建通用缓存框架。
- key 包含应用、环境、资源和版本，例如 `{APP_NAME}:{ENV}:products:v1:{id}`；多租户或多数据源还需加入隔离标识，不放入令牌或明文隐私数据。
- 普通查询使用 cache-aside：命中则返回，未命中则查源并回填；区分不存在、反序列化失败和连接故障。
- 对可降级的查询缓存，读取失败回源，写入失败保留本次源结果并记录安全日志；认证会话等权威状态不可按普通缓存降级放行。
- TTL 根据数据新鲜度设置；空结果使用更短 TTL，区分有效空列表与缓存缺失。具体值由业务决定。
- 源数据更新成功后按业务一致性要求失效缓存。热点并发回源、多实例协调和严格一致性需单独设计，不假定缓存回填本身能解决。

## 定时任务

任务入口放在 internal/job，默认 Hello Job 每分钟整点调用 HelloService，任务超时 5 秒。Wire 在启动前完成注册，App 统一管理启停。沿用调度器的取消、同任务跳过重叠、错误和 panic 日志规则；各进程独立调度，不提供跨实例去重。

## 事务

事务归属、原子操作和外部副作用边界统一见[事务职责](architecture.md#事务职责)。
