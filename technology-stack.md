# 技术选型

本文记录项目已确定的技术选型。已有方案文档中涉及以下组件的建议，以本文为准。

| 类别 | 选型 | 用途 |
| --- | --- | --- |
| Web 框架 | Gin | HTTP 路由、中间件及接口请求处理 |
| 数据访问 | GORM | 关系型数据库的 ORM 与数据访问 |
| 关系型数据库 | MySQL | 通过 GORM MySQL 驱动访问，基础封装位于 `pkg/db` |
| Redis 客户端 | go-redis | Redis 连接与数据操作 |
| 配置管理 | Viper | `config` 包加载 YAML 并监听文件变化；`APP_NAME`、`ENV` 选择文件，入口独立读取 `PORT`，连接参数不提供环境变量覆盖，见 [配置说明](config/README.md) |
| 日志 | Zap + Lumberjack | `pkg/log` 默认输出 JSON 到 stdout；独立实例支持文件输出与按大小轮转，完整配置由调用方传入，见 [使用说明](pkg/log/README.md) |
| 错误处理 | 自定义错误 | 承载业务错误码、错误消息等信息，供接口层统一解析 |
| 依赖注入 | Wire | 编译期依赖注入与依赖组装 |

## 错误处理约定

- `entity.Error` 包含整数 `Code` 和可公开的 `Message`，错误类型、公共错误与后续具体业务错误统一放在 `internal/entity/error.go`；公共错误编号为 `100001`～`100008`，已有编号保持稳定。
- Controller 通过 `response.RespondError`（位于 `controller/response/error.go`） 使用 `errors.As` 提取业务错误，再以 `errors.Is` 匹配已登记的预定义实例，映射 HTTP 状态码；支持 `%w` 包装。
- 未登记的业务错误和普通 error 统一返回 HTTP 500、`code: -1`、`message: "未知错误"`，原始错误记录日志。
- 普通 JSON 响应使用 `code/message/data` 外壳，成功 code 为 0；完整状态码映射和例外见 [HTTP 公共响应](internal/controller/response/README.md)。

## 版本约定

- 已引入依赖的版本以 `go.mod` 为准；其他组件按实际需求选型。
