# goempty

用于创建新 Go / Gin 服务的模板。已提供配置、日志、MySQL、Redis、MongoDB、统一响应、Wire 依赖组装、HTTP 优雅关闭和进程内定时调度。基础组件与生命周期测试保留，便于新项目修改后检查回归。

## 本地启动

1. 准备 MySQL、数据库、单机 Redis 和 MongoDB。
2. 修改 [开发配置](configs/app_dev.yaml) 中的连接信息；不要提交真实生产凭据。程序不会创建数据库或业务表。
3. 在项目根目录执行：

```sh
go run ./cmd/app
```

默认 `APP_NAME=app`、`ENV=dev`、`PORT=8080`，读取 `configs/app_dev.yaml`；环境文件独立、不合并、不回退。编译后也从项目根目录运行，以便读取相对配置路径：

```sh
make build
./dist/app
```

`GET /hello` 返回统一 JSON 响应；`GET /healthz` 和 `/readyz` 返回 200 空响应，目前不探测 MySQL、Redis 或 MongoDB。Hello Job 每分钟整点调用 HelloService 并记录问候日志；不会写业务数据，多实例各自执行。外部 HTTP 客户端仅为独立示例，默认不装配、不访问外部服务。

配置文件支持监听重载，但已有 MySQL、Redis、MongoDB 连接池不会重建；连接参数以及入口设置的 HTTP、关闭参数变更需重启。完整边界见 [配置说明](config/README.md)。

## 构建与测试

```sh
go test ./...
go vet ./...
go build ./...
```

Linux / macOS 安装 GNU Make 后，可在项目根目录使用以下命令；直接运行 `make` 显示帮助：

| 命令 | 用途 |
| --- | --- |
| `make run` | 启动应用，沿用 `APP_NAME`、`ENV`、`PORT` 环境变量 |
| `make build` | 构建当前平台可执行文件 `dist/app` |
| `make fmt` | 格式化 Go 源码，会修改文件 |
| `make test` / `make vet` | 执行测试 / 静态检查 |
| `make check` | 顺序执行测试、vet 和全包构建，失败即停止 |
| `make test-integration` | 不使用测试缓存，运行应用组装和真实服务测试 |
| `make wire` | 使用最新版本 Wire 生成依赖组装代码 |
| `make docs` | 使用现有脚本生成 Swagger 文档 |

例如 `ENV=test PORT=8081 make run`。普通构建不会自动生成 Wire 或 Swagger 文件；生成命令可能下载工具及其依赖，执行后需检查生成差异。

Makefile 开头可配置 `APP_NAME`（默认 `app`）和 `DIST_DIR`（默认 `dist`）。构建使用当前 Go 环境的 `GOOS`、`GOARCH`，默认 `CGO_ENABLED=0`，通过 `-trimpath` 移除源码路径，通过 `-ldflags="-s -w"` 移除符号表和调试信息。可通过 `LDFLAGS` 覆盖链接参数，例如 `make build LDFLAGS=` 保留符号信息。

```sh
# 构建 Linux amd64
make build GOOS=linux GOARCH=amd64
# 构建 macOS Apple Silicon
make build GOOS=darwin GOARCH=arm64
# 指定产物名称和目录
make build APP_NAME=myservice DIST_DIR=dist
```

不同平台构建默认写入同一路径，需保留多份产物时指定不同的 `DIST_DIR`。`APP_NAME` 决定可执行文件名，应用运行时仍通过环境变量 `APP_NAME` 选择配置文件。启用 CGO 时需准备目标平台的 C 工具链。

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

修改 Wire 声明后，在根目录运行以下命令并检查生成差异；普通构建无需生成：

```sh
make wire
```

接口和注释变化后运行 `go run ./scripts/generate-api-docs`。创建新项目时替换模块名和应用名称，调整配置文件与文档；示例能力按需保留。组件 README 记录局部行为，根文档负责导航，避免复制多份实现说明。
