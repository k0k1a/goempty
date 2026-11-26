# 向已有项目同步模板组件

用于把模板中的组件及其示例、装配和根文档补充到已由本模板创建的项目。创建新项目的规则见 [组件目录](trimming.md)。

## 适用范围

- 目标应为使用本模板创建、结构与模板一致的 Go 项目。module、目录布局或 `internal/app` 结构明显不同时，先说明差异并确认是否继续。
- 目标可以是当前工作目录，也可以是用户指定路径；同步前读取目标 AGENTS.md，遵循其确认门禁。

## 同步语义

- 只新增与增量修改装配，不删除目标文件，不覆盖业务实现。
- 已存在组件默认不改实现，只在需要时补缺失的装配和根文档段落；新增组件连同其示例一起补充。与模板的差异列出供用户决定。
- 冲突先报告：目标已有同名路由、配置键、Service 方法、Wire provider 或函数时，不擅自并存或改名，先给出处理建议。
- 不新增测试：不复制任何 `_test.go`、`integration-test/` 和测试辅助代码；保留目标现有测试。装配签名变化导致现有测试无法编译时，做最小适配并说明，或列冲突交由用户决定。
- 不复制组件文档：`pkg/*/README.md`、`config/README.md`、`internal/*/README.md` 等由目标项目自行维护。
- 不复制生成技能、构建产物、日志、真实凭据和本机私有配置。

## 步骤

1. **定位与核对**：由技能路径向上定位源模板，读取源与目标的 AGENTS.md、go.mod、README.md；核对 module、目录布局和 `internal/app` 结构，判断是否为模板派生项目。
2. **盘点目标现状**：检查 `config.Config` 字段、各环境 YAML 段、`internal/app` 的 providers/wire/http/jobs/events/messaging、go.mod 依赖、根文档章节，得出目标已有组件集合。
3. **计算差异**：请求集合减去已有集合得到待新增组件；检查依赖是否满足（事件总线需要异步任务池；定时任务示例需要事件总线，因此新增定时任务连带事件总线和异步任务池；Kafka 两侧独立）。
4. **列出方案**：待新增文件、装配改动、配置键、根文档段落、冲突项和无法自动处理的部分，按目标项目的确认门禁取得授权后实施。
5. **复制代码**：复制组件专属实现与示例，替换 module 前缀和内部导入；目标已有同名文件时不覆盖，改为报告差异。骨架文件（`main.go`、`app.go`、`providers.go`、Wire）按目标现状增量修改，不整文件替换。
6. **同步装配**：补 Wire 声明与 `FieldsOf`、provider 与清理函数、App 参数字段与启停、配置字段与三个环境 YAML、路由/订阅/任务/消费者注册、组件示例。沿用目标已有的命名和错误处理方式，不引入模板中目标尚未使用的模式。
7. **同步根文档**：只更新 AGENTS.md（阅读入口、实施约束、验证）、architecture.md（目录树与相关章节）、conventions.md（选型、规则章节）、README.md（启动、导航、测试变量）。CLAUDE.md 仅保持导入 AGENTS.md。不复制组件 README。
8. **生成与验证**：运行目标 README 规定的 Wire 生成命令（模板为 `make wire`），有 Swagger 时重新生成；依赖变化时运行 `go mod tidy`；对修改的 Go 文件运行 gofmt，运行 `go vet ./...`、`go build ./...`。不新增测试。纯新增组合可用临时端口验证启动与关闭；外部组件只在隔离环境可用时验证。
9. **交付**：列出新增组件及其示例、修改文件、新增配置键、文档章节、冲突与未处理项、验证结果与未验证内容；未经授权不执行 Git 操作。

## 常见组合要点

- **新增存储（MySQL/PostgreSQL/MongoDB/Redis）**：补 `pkg` 包、config 字段、YAML 段、provider、App 字段、`newApp` 参数、Wire 和根文档；确认目标现有 Service/Repo 是否已自行接入同类连接。同时接入 MySQL 与 PostgreSQL 时，provider 需用包装类型区分两个 `*gorm.DB`。
- **新增 Kafka**：生产与消费分开处理；生产者需补 `kafka` 段、provider、发布示例与 `HelloService` 发布方法，消费者还需 `kafka_consumer`、`messaging.go`、消费示例、App 的 consumers 与启停。示例随组件同步。
- **向无 HTTP 项目新增 HTTP 入口**：补 server、middleware、response、error、路由、`GET /hello`、Hello Service/Repo 和外部 HTTP 示例，恢复 `main.go` 的 `PORT` 解析和 `httpserver.Config`，调整 App 的 server 参数、Run 等待和 Shutdown，确认退出路径仍释放资源。
- **新增调度或事件**：事件总线依赖异步任务池；新增定时任务必须同时补事件总线、异步任务池和 `HelloJob` 示例，不保留零任务调度器或空总线；至少保留一个运行入口的约束仍然适用。
- **新增游标分页**：补 `pkg/cursor`、`entity` 分页类型和文档规则；签名 key 由目标业务注入，不新增模板配置字段。

## 验收场景

在临时目录复制一个只保留 HTTP 入口的模板派生项目，验证：

1. 同步 MySQL 和 Redis → 两个连接创建、Ping、关闭正确，三个环境 YAML 与 `config.Config` 一致，现有 handler 不被修改。
2. 同步 Kafka 消费者 → `kafka_consumer`、`internal/consumer`、Hello 消费示例、App 消费者启停存在；不新增 `POST /hello/message`，不新增测试。
3. 向该项目同步 HTTP 入口 → 监听、healthz/readyz、`GET /hello`、外部 HTTP 示例、取消退出和清理可用，Wire 重新生成，根文档只增加实际保留内容。
4. 向该项目同步 PostgreSQL → `postgres` 配置段、`newPostgres`、App postgres 字段与关闭路径存在，不新增 MySQL，Wire 重新生成。
5. 向该项目同步定时任务 → 同时新增事件总线、异步任务池、`HelloJob` 和对应配置段，调度可启动和关闭，不出现零任务调度器。
