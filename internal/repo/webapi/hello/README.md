# 外部 HTTP 客户端示例

此包演示主动调用独立上游，与本应用的 HelloRepo 无关；未接入配置、Wire 或默认启动链，不需要真实外部服务。

```go
client, err := hello.New(hello.Config{BaseURL: "https://example.com/api"})
if err != nil {
    return err
}
result, err := client.Hello(ctx)
```

实际集成时将配置放入应用 Config，通过 Wire 创建一次客户端并以 `hello.Repository` 接口注入消费者。使用已有请求或任务 Context，不在 Service 内创建根 Context。

示例向 `/api/hello` 发起 GET，预期返回 `{"message":"hello"}`，不含公共响应外壳；不能直接用本应用的 `/hello` 代替这个上游。实际项目按上游协议调整路径和结构。

构造时校验 HTTP/HTTPS URL，拒绝 URL 凭据、查询串和 fragment。Timeout 默认 5 秒，负值非法。复用标准库客户端和默认连接池；每次请求传递 Context，关闭响应体，检查 2xx 状态，限制响应大小为 1 MiB，再解析 JSON 和必要字段。上游错误响应体不进入错误消息，不自动重试写操作。

此示例不增加通用 HTTP 框架。鉴权、业务状态码、重试和专用连接池按实际协议增加；凭据不要放入 URL、日志或公开错误。测试使用本地 httptest 服务覆盖成功、异常响应、超时与取消。
