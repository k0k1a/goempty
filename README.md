# goempty

用于创建新 Go / Gin 服务的模板。已提供配置、日志、MySQL、Redis、MongoDB、统一响应、Wire 依赖组装、HTTP 优雅关闭和进程内定时调度。基础组件与生命周期测试保留，便于新项目修改后检查回归。

## 本地启动

1. 准备 MySQL、数据库、单机 Redis 和 MongoDB。
2. 修改 [开发配置](configs/app_dev.yaml) 中的连接信息；不要提交真实生产凭据。程序不会创建数据库或业务表。
3. 在项目根目录执行：

```powershell
go run ./cmd/app
```

默认 `APP_NAME=app`、`ENV=dev`、`PORT=8080`，读取 `configs/app_dev.yaml`；环境文件独立、不合并、不回退。编译后也从项目根目录运行，以便读取相对配置路径：

```powershell
go build -o bin/app.exe ./cmd/app
.\bin\app.exe
```

`GET /hello` 返回统一 JSON 响应；`GET /healthz` 和 `/readyz` 返回 200 空响应，目前不探测 MySQL、Redis 或 MongoDB。Hello Job 每分钟整点调用 HelloService 并记录问候日志；不会写业务数据，多实例各自执行。外部 HTTP 客户端仅为独立示例，默认不装配、不访问外部服务。

配置文件支持监听重载，但已有 MySQL、Redis、MongoDB 连接池不会重建；连接参数以及入口设置的 HTTP、关闭参数变更需重启。完整边界见 [配置说明](config/README.md)。

## 构建与测试

```powershell
go test ./...
go vet ./...
go build ./...
```

MySQL / Redis / MongoDB 真实服务测试在缺少对应变量时跳过，不等于已经验证连接。仅使用隔离服务：

| 测试环境变量 | 要求 |
| --- | --- |
| `GOEMPTY_TEST_MYSQL_DSN` | 包含数据库名、`parseTime=true`、正值 `timeout` / `readTimeout` / `writeTimeout`；账号需临时表权限 |
| `GOEMPTY_TEST_REDIS_ADDR` | 测试 Redis 地址，形如 `127.0.0.1:6379` |
| `GOEMPTY_TEST_REDIS_USERNAME` / `GOEMPTY_TEST_REDIS_PASSWORD` | 按需设置 |
| `GOEMPTY_TEST_MONGODB_URI` | 隔离 MongoDB 连接串，按需包含认证和 TLS 参数 |

这些变量只用于测试，不覆盖应用 YAML。设置后运行 `go test -count=1 ./internal/app ./integration-test`，应用组装生命周期测试同时需要三个服务。

## 开发导航

| 文档 | 内容 |
| --- | --- |
| [项目指导](AGENTS.md) | AI 阅读入口、修改与验证约束 |
| [目录职责](directory-structure.md) | 分层、对象复用、Repo 事务职责 |
| [技术栈与公共规范](technology-stack.md) | 组件选型、错误、分页、缓存规则 |
| [应用组装](internal/app/README.md) | Wire、路由、生命周期和任务接入 |
| [HTTP 公共响应](internal/controller/response/README.md) | 公共响应与业务错误码 |
| [HTTP Server](pkg/httpserver/README.md)、[日志](pkg/log/README.md)、[调度器](pkg/scheduler/README.md) | 组件使用和边界 |
| [MongoDB](pkg/mongodb/README.md) | 客户端封装、配置和连接测试 |
| [外部 HTTP 示例](internal/repo/webapi/hello/README.md) | 标准库客户端、超时、取消和响应解析 |
| [Swagger](docs/swagger/README.md) | 注释与离线文档生成 |

修改 Wire 声明后，在根目录运行固定版本并检查生成差异；普通构建无需生成：

```powershell
go run github.com/google/wire/cmd/wire@v0.7.0 ./internal/app
```

接口和注释变化后运行 `go run ./scripts/generate-api-docs`。创建新项目时替换模块名和应用名称，调整配置文件与文档；示例能力按需保留。组件 README 记录局部行为，根文档负责导航，避免复制多份实现说明。
