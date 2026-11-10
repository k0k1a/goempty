# 当前模板裁剪参考

用于定位关联实现。每次执行核实源代码，模板演进后不得仅按此表删除文件。

## 基础与示例

保留入口、配置加载及热加载、日志、env、HTTP Server、请求 ID/访问日志/恢复中间件、CORS、统一响应及错误、公共分页、healthz/readyz、Wire 和必要文档。保持健康端点现有语义，不擅自增加依赖探测或认证。

Hello 默认移除：

- internal/controller/hello.go、internal/service/hello.go、internal/repo/db/hello.go、internal/entity/hello.go。
- internal/job/hello.go 及 register.go 中的 Hello 注册。
- internal/repo/webapi/hello/ 示例及局部说明。
- internal/app/http.go 中 Hello 路由、Controller 构造和 httpDependencies.Hello；没有剩余服务时去掉空依赖结构及传参。
- internal/app/wire.go 中 Hello Repo/Service provider、接口绑定和 imports。
- Swagger、组件说明和根 README 中的示例接口/任务描述。

internal/repo/db/hello.go 不使用数据库，不能通过目录推断 MySQL 是否需要。无实际文件的业务目录不保留占位文件。

## 可选组件

| 组件 | 专属实现 | 同步调整 |
| --- | --- | --- |
| MySQL | pkg/db/ | config.Config.MySQL、全部 YAML 的 mysql、newDatabase、App.database、newApp 参数、Wire FieldsOf/providers、GORM/MySQL 依赖与文档 |
| MongoDB | pkg/mongodb/ | config.Config.MongoDB、全部 YAML 的 mongodb、newMongoDB、App.mongoClient、newApp 参数、Wire、官方 Driver 依赖与文档 |
| Redis | pkg/redis/ | config.Config.Redis、全部 YAML 的 redis、newRedis、App.redisClient、newApp 参数、Wire、go-redis 依赖与文档 |
| 定时任务 | pkg/scheduler/、internal/job/ | Wire job.NewScheduler、App.scheduler、New/newApp 签名和校验、Run 中 Start/Stop 及等待逻辑、cron 依赖与文档 |

- 初始化目前强制创建 MySQL、Redis、MongoDB。移除组件应解除硬依赖，不能只删包或让 provider 返回 nil。
- App.New 拒绝 nil scheduler；无调度组合去掉必需参数和运行逻辑，不留空调度占位。保留调度但去掉 Hello 时保留合法的零任务注册入口，不增加示例任务。
- 保持 HTTP 优雅关闭、取消传播、关闭超时、清理幂等和初始化失败释放已创建资源；删任务逻辑后检查 channel 的发送、等待和错误合并是否仍成对。
- 所有存储移除后仍保留配置加载；无文件配置字段时 YAML 使用合法空映射 `{}`，结合 Viper 实际行为验证。config/load.go 使用 UnmarshalExact，删字段后不能残留对应 YAML 段。
- App 持有连接和 Wire cleanup 仍应符合资源所有权；只保留使用的参数及 imports，不用空白标识符掩盖裁剪不完整。
- 保留 MySQL 时保留 SQL 迁移规范；只有 MongoDB 或无数据库时清除 SQL 专属要求。保留通用 Repo 原子性和事务职责，不引入新迁移框架。

## 全部去测试

删除任意目录的 `*_test.go`，包括 cmd、config、internal、pkg、integration-test。测试资源确认无生产用途后删除，空测试目录不保留。

不迁移测试辅助函数或改写测试。保留组件文档的配置和生命周期边界，移除失效测试导航、GOEMPTY_TEST_* 变量表及隔离测试说明。app_test.yaml 是 ENV=test 配置，不是 Go 测试文件，应随应用名改为 `<应用名>_test.yaml`。

删测试后再整理依赖，避免测试 imports 把已移除组件带回 go.mod。没有自动化测试覆盖应如实说明。

## 命名与生成

- 默认应用名在 pkg/env 等实际定义处核实；检查入口、配置加载、三个环境配置、README 启动说明、文档标题。cmd/app、internal/app 是职责名，无需随应用名重命名。
- module 可带路径，不能用作配置文件名前缀；应用名和 module 分开替换。
- 连接样例保留安全占位值，不复制本机真实密码或生产端点；不能判断时澄清。
- Wire 当前命令：`go run github.com/google/wire/cmd/wire@v0.7.0 ./internal/app`。以执行时源 README 的固定版本为准。
- Swagger 当前命令：`go run ./scripts/generate-api-docs`。核实 scripts/generate-api-docs/main.go 的扫描目录，删除整个扫描目录时同步调整参数；产物不应留下 /hello。
- 检查 AGENTS.md 导航、目录规范和技术栈表，区分未来开发规范与宣称已存在的能力；删失效引用，不扩大到重写全部文档。

## 验收场景

在独立临时目录复制源到不同路径，从副本加载技能，分别验证：

1. “创建 plain-api，去掉 MySQL、MongoDB、Redis 和定时任务”：目标为模板同级 plain-api，module 为 plain-api；无存储/调度 imports，空 YAML 有效，健康接口和关闭可运行。
2. “创建 order-service，只保留 MySQL 和 Redis”：保留两个连接和 cleanup，无 MongoDB/调度/Hello，三个环境配置与文档一致。
3. “创建 audit-service，module 为 example.com/team/audit，目标为指定新目录，只保留 MongoDB 和定时任务”：路径和 module 覆盖默认值，配置前缀仍为 audit-service，无 MySQL/Redis，调度器可零任务启动。

每种组合应无 `_test.go`、不携带生成技能、不修改源、通过 vet/build。已有目标或目标位于源内时在写入前停止。真实连接只在隔离服务可用时验证。
