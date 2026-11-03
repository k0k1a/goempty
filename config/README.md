# 应用配置

`config` 包使用 Viper 加载启动配置，返回普通 Go 结构体。`Config.MySQL` 复用 `pkg/db.Config`，`Config.Redis` 复用 `pkg/redis.Config`，提供连接和连接池参数。`Config.HTTP` 和 `Config.ShutdownTimeout` 使用 `mapstructure:"-"` 排除 YAML 解码，由 `main` 在加载后的启动副本中设置，随后将完整的 `config.Config` 直接传给 `app.Initialize(ctx, cfg)`。HTTP 超时和关闭超时固定设置，端口独立读取 `PORT`。

## 选择环境文件

配置文件放在项目根目录 `configs/`，通过环境变量选择 `configs/{APP_NAME}_{ENV}.yaml`：

| 环境变量 | 用途 | 未设置或为空时 |
| --- | --- | --- |
| `APP_NAME` | 服务名称 | `app` |
| `ENV` | 运行环境 | `dev` |

默认在项目根目录执行 `go run ./cmd/app`，读取 `configs/app_dev.yaml`。切换到生产环境示例：

```powershell
$env:APP_NAME = "app"
$env:ENV = "prod"
$env:PORT = "9090"
go run ./cmd/app
```

以上选择 `configs/app_prod.yaml`；先填写该文件中的 MySQL 和 Redis 配置并确保服务可用，初始化成功后监听 `:9090`。编译后可使用同样的环境变量启动 `.\app.exe`。

配置路径以进程启动时的工作目录为基准，部署时将 `configs/` 放在该目录。`APP_NAME`、`ENV` 在 `pkg/env` 包初始化时读取并保存在私有变量中，通过 `AppName()`、`Env()` 供应用配置与默认日志共用；运行时修改环境变量不会改变已读取的值，由开发与部署人员设置，直接用于拼接文件名，不额外校验名称格式。不再提供 `-config` 参数。

目标文件必须存在且可读；文件不存在、YAML 格式错误、未知字段或配置值非法时直接启动失败，不回退到其他环境。各环境文件独立维护，不合并：

```text
configs/app_dev.yaml
configs/app_test.yaml
configs/app_prod.yaml
```

## 端口与固定 HTTP 配置

入口只通过 `PORT` 决定监听端口，未设置时默认 `8080`。显式设置为空、非整数或超出 `1～65535` 时启动失败。监听地址为 `:端口`，监听所有网络接口。

HTTP 超时在 `cmd/app/main.go` 中固定设置：ReadHeaderTimeout 为 5 秒，ReadTimeout 和 WriteTimeout 为 15 秒，IdleTimeout 为 60 秒。YAML 不提供 `http` 配置，`GOEMPTY_HTTP_*` 环境变量不再生效。

## MySQL 与固定关闭超时

本地默认读取 `configs/app_dev.yaml`：

```yaml
mysql:
  dsn: "goempty:CHANGE_ME@tcp(127.0.0.1:3306)/goempty?charset=utf8mb4&parseTime=true&loc=Local&timeout=5s&readTimeout=5s&writeTimeout=5s"
  max_open_conns: 10
  max_idle_conns: 5
  conn_max_lifetime: 30m
  conn_max_idle_time: 5m
```

先准备本地 MySQL、`goempty` 数据库和账号，将 `CHANGE_ME` 替换为本地密码。程序不安装 MySQL，也不创建数据库或表。本地 YAML 可保存本地凭据，不保存线上配置；不读取 `GOEMPTY_MYSQL_DSN`。`loc=Local` 使用应用进程的本地时区。

`app_test.yaml` 和 `app_prod.yaml` 保持空模板，运行这些环境前需自行填写相应 `mysql` 和 `redis` 配置；缺少配置会在初始化阶段启动失败。不提供线上连接信息。

DSN 必须包含数据库名、`parseTime=true`，并设置正值 `timeout`、`readTimeout`、`writeTimeout`。初始化复用 `pkg/db.New`，context 仅在初始化前检查，连接过程由 DSN 网络超时约束，并非初始化总超时。连接池要求最大连接数大于零、空闲连接数在零至最大连接数之间、连接有效期非负。

关闭超时在 `cmd/app/main.go` 中固定为 10 秒，YAML 不提供 `shutdown_timeout` 字段，也不绑定 `GOEMPTY_SHUTDOWN_TIMEOUT` 环境变量。当前仍要求选中的配置文件存在。

## Redis

开发环境使用以下本地示例，按本机服务调整地址与认证信息：

```yaml
redis:
  addr: "127.0.0.1:6379"
  username: ""
  password: ""
  db: 0
  pool_size: 10
  min_idle_conns: 2
  dial_timeout: 5s
  read_timeout: 3s
  write_timeout: 3s
  pool_timeout: 4s
```

只支持单机 Redis；地址为 `host:port`，DB 非负，连接池大小和超时非负，显式设置 pool_size 时 min_idle_conns 不得超过它。连接池与超时零值使用 go-redis 默认值。空 username/password 用于本地无认证服务；需要认证时填写实际信息，不提交生产凭据。配置只从 YAML 读取，不绑定环境变量覆盖。

应用通过 `pkg/redis.New` 创建一个共享客户端并执行 PING；Redis 不可用时启动失败。业务消费者后续通过构造函数接收客户端。退出时先等待 HTTP 关闭，再释放 Redis 和 MySQL 连接池。文件重载不会重建 Redis 客户端，修改连接配置后需重启。

## 动态加载

入口启动时调用一次 `Load()`，首次加载失败时启动失败。随后通过 Viper 的 `WatchConfig()` 和 `OnConfigChange()` 监听选中的文件。Viper 尝试重新读取文件后执行回调，回调直接调用同一实例的 `v.UnmarshalExact()`；反序列化成功后整体替换内存配置，失败则记录错误并保留旧值，不重启应用。

`Load()` 和重载回调只负责读取与反序列化，不调用 MySQL/Redis 配置的 `Validate()`；连接参数的语义校验在启动组装依赖时执行。重载成功不代表新的连接参数有效或已应用到现有连接池。

业务通过 `config.Get()` 获取最新成功反序列化的文件配置快照，其中 HTTP 和关闭超时保持零值，不包含 `main` 对启动副本的赋值。`Load()` 的返回值只是启动时的快照，已有变量不会自动变化。每次请求或任务开始时读取一次即可。读写通过读写锁保护；以后增加 map、slice 等引用字段时，调用方不得修改快照中的内容。

Viper 重新读取失败时仍可能触发回调，此时反序列化的可能是上一次成功读取的内容，回调成功不代表本次磁盘文件读取成功。文件读取错误由 Viper 自身记录。

监听路径在启动时固定，运行期间不通过 `APP_NAME`、`ENV` 切换文件。使用 Viper 自带的进程级监听，不提供单独停止接口；文件删除后的监听行为遵循 Viper，本方案面向已有文件的修改和替换。HTTP、PORT 和关闭超时保持上述行为。

数据库连接只使用启动快照。文件重载不会重建连接池，修改 DSN 或连接池参数后必须重启应用。`app_test.yaml` 是测试环境运行配置，加载单元测试主要使用临时目录与文件。
