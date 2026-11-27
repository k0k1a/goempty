# AGENTS.md

本文件规定项目的阅读入口、开发约束与验证流程。

## 阅读入口

先读本文件，再按任务选读下表对应章节。跨主题任务同时阅读相关章节，默认不通读全部文档或遍历整个仓库。

| 任务涉及 | 阅读章节 |
| --- | --- |
| 新增业务、调整分层或依赖 | [分层与依赖](architecture.md#分层与依赖) |
| 类型、依赖及 receiver 命名 | [各层命名规范](architecture.md#各层命名规范) |
| 输入、结果、数据库对象及跨层复用 | [对象与复用](architecture.md#对象与复用) |
| 参数校验、调用契约与防御检查 | [校验与调用契约](conventions.md#校验与调用契约) |
| HTTP 或 MQ 入口 | [HTTP 与 MQ](architecture.md#http-与-mq) |
| 进程内事件或直接提交异步任务 | [入口与装配](architecture.md#进程内异步任务与事件)、[提交与执行约定](conventions.md#进程内异步任务与事件) |
| 原子操作、事务或并发更新 | [事务职责](architecture.md#事务职责) |
| 技术选型、依赖或版本 | [组件选型](conventions.md#组件选型)、[版本约定](conventions.md#版本约定) |
| 类型转换或集合操作 | [组件选型](conventions.md#组件选型) |
| 业务错误及 HTTP 响应 | [文件放置](architecture.md#错误与响应适配)、[错误处理约定](conventions.md#错误处理约定) |
| 列表与分页 | [公共分页](conventions.md#公共分页) |
| 外部 HTTP 调用 | [客户端放置与依赖](architecture.md#外部-http-客户端)、[调用约定](conventions.md#外部-http-调用) |
| 配置来源或热加载 | [配置](conventions.md#配置) |
| 组件启动、关闭或资源释放 | [生命周期](conventions.md#生命周期) |
| MongoDB 建模或原子更新 | [事务职责](architecture.md#事务职责)、[MongoDB 原子性](conventions.md#mongodb-原子性) |
| 数据库结构变更或迁移脚本 | [数据库变更](conventions.md#数据库变更) |
| 缓存 | [缓存接入](architecture.md#缓存接入)、[缓存使用规范](conventions.md#缓存使用规范) |
| 定时任务 | [任务入口与装配](architecture.md#定时任务)、[调度约定](conventions.md#定时任务) |
| 文件定位或测试放置 | [目录结构](architecture.md#目录结构)、[测试放置](architecture.md#测试放置) |

## 文档职责

- `AGENTS.md` — 阅读入口、硬性约束、变更与验证流程。
- `CLAUDE.md` — Claude Code 入口，仅导入 `AGENTS.md`，不承载独立规则。
- `architecture.md` — 目录、分层依赖与事务边界。
- `conventions.md` — 组件选型与通用约定。
- 根 `README.md` — 面向开发者的项目概览、启动与验证命令。
- 组件 README — 组件使用方式、行为边界与必要约束，避免重复代码和根文档。

长期文档只记录职责范围内的规则和必要说明，不记录单次需求的方案、计划或实施过程。

## 文档维护

- 行为变更时同步相关文档。业务开发授权不包含文档修改。尚未获准时，列明文件、章节和拟改内容，请求用户确认。
- 实现细节以代码为准，结合相关代码和测试核实。文档与代码有出入时，说明差异和影响，按已确认范围处理。

## 实施约束

- 业务入口调用 Service，Service 依赖 Repo 接口。完整原子操作及事务由 Repo 封装。 [分层与依赖](architecture.md#分层与依赖) · [事务职责](architecture.md#事务职责)
- 校验按职责与调用契约分配。保留业务约束和真实可达的异常处理，不重复检查在当前使用点仍有明确保证的条件，不为假想的调用误用添加防御分支。 [校验与调用契约](conventions.md#校验与调用契约)
- 业务实体的判断和状态转换，若仅依赖自身字段和显式参数，优先封装为实体方法。适用边界见 [对象与复用](architecture.md#对象与复用)。
- `internal/app` 负责装配本项目启用的入口和运行组件。应用启动前完成路由、任务和订阅的注册，由 App 统一管理启停。 [生命周期](conventions.md#生命周期)
- 按目录职责放置对象和错误。跨层传递时不增加仅逐字段复制的对象。 [对象与复用](architecture.md#对象与复用) · [错误与响应适配](architecture.md#错误与响应适配)
- 业务组件及依赖使用职责后缀。receiver 使用简短名称，同一类型保持一致。 [各层命名规范](architecture.md#各层命名规范)
- 按需创建目录和接入组件，不添加占位能力，不预设或擅自新增认证方案。
- 不手工修改 `internal/app/wire_gen.go`。Wire 声明变化后运行 `make wire`，并检查生成差异。 [构建与测试](README.md#构建与测试)

## 验证

代码变更后优先检查受影响包的编译、测试及依赖方向，再按改动范围执行测试、vet 和构建。命令、真实服务测试的环境变量与隔离要求见 [构建与测试](README.md#构建与测试)。仅修改文档时检查链接和规范一致性，无需运行业务测试。
