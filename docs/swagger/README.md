# API 文档生成

使用固定版本 Swaggo v1.16.6 从 Go 注释生成 Swagger 2.0 文件，无运行时文档依赖。

在项目根目录执行：

```sh
go run ./scripts/generate-api-docs
```

也可以在项目根目录直接执行以下跨平台命令：

```text
go run github.com/swaggo/swag/cmd/swag@v1.16.6 init --generalInfo main.go --dir cmd/app,internal/app,internal/controller,internal/entity --parseInternal --output docs/swagger --outputTypes json,yaml
```

需要 Go 环境；首次执行需要下载生成工具及其依赖。不会安装全局命令或向应用 go.mod 添加 Swaggo 依赖。

生成的 `swagger.json` 和 `swagger.yaml` 内容等价，任选一个导入支持 Swagger 2.0 的 API 工具。文件未固定 host 和 schemes，导入后在目标工具中设置实际服务地址。项目不提供在线文档路由。

## 维护约定

- `cmd/app/main.go` 维护文档标题、版本、基础路径与公共说明。
- Controller 方法通过 `@Summary`、`@Tags`、`@Param`、`@Success`、`@Failure`、`@Router` 等注释描述接口。路径和方法必须与 `internal/app/http.go` 中的路由一致。
- HTTP 数据类型放在 `internal/entity`，公共响应复用 `response.Response[T]`。优先使用明确的数据结构，避免 `gin.H` 导致字段描述缺失。
- `/healthz` 和 `/readyz` 的注释随对应 handler 维护；两者均为 200 空响应，当前 readiness 不检查数据库或 Redis。
- `request_id` 为响应体预留可选字段，当前不输出；实际请求 ID 使用 `X-Request-ID` 响应头。
- 修改接口、数据结构或文档注释后重新生成，并将生成文件与代码一起提交。不要直接编辑生成文件。
- 生成器依据注释描述接口，不会自动推导所有路由和业务分支。新建接口目录时同步更新 `scripts/generate-api-docs/main.go` 及上述命令的扫描目录。

公共错误码及响应约定见 [HTTP 公共响应](../../internal/controller/response/README.md)。仅在对应接口实际可能返回时声明错误响应。
