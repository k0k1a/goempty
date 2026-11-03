# HTTP Server

`New(cfg, handler)` 接收已装配好的 `http.Handler`，配置内部 `http.Server`。handler 不能为空。构造时不监听端口、不启动 goroutine。

本包负责 HTTP 超时、监听和优雅关闭，不依赖 Gin 或业务 Controller。当前应用在 `internal/app/http.go` 中创建 Gin Engine、装配 Recovery 和业务路由，再将 Engine 作为 handler 传入本包。

- `Start()` 阻塞运行；监听失败返回错误，正常关闭返回 `nil`。
- 通过 `net.Listen` 成功绑定端口后，使用 `pkg/log` 输出英文日志 `HTTP server started`，包含实际监听地址；监听失败时不输出启动成功日志，错误返回调用方统一记录。
- `Stop(ctx)` 停止接收请求并等待正在处理的请求结束；超时或取消时强制关闭连接，并返回可通过 `errors.Is` 判断的 context 错误。
- 由调用方保证启动、关闭各执行一次，本包不额外维护重复启停状态。停止后的实例不能重新启动。
- `Stop()` 返回后才能退出进程；`Start()` 会在停止监听时先返回，此时请求可能仍在处理中。
- WebSocket 等 hijacked 连接由业务持有者单独关闭。

配置提供 `Addr`、`ReadHeaderTimeout`、`ReadTimeout`、`WriteTimeout` 和 `IdleTimeout`。建议显式设置请求超时；零值遵循标准库行为。`Addr` 使用 `host:port` 格式，例如 `:8080` 或 `127.0.0.1:8080`。

当前应用的装配位置（应用包内片段；完整路由和中间件见 `internal/app/http.go`）：

```go
// internal/app/http.go：deps 中的 Service 由 Wire 注入。
// newHTTPHandler 装配 RequestID、AccessLog、Recovery、CORS 和路由。
handler := newHTTPHandler(deps)
server, err := httpserver.New(httpserver.Config{
    Addr:              ":8080",
    ReadHeaderTimeout: 5 * time.Second,
    ReadTimeout:       15 * time.Second,
    WriteTimeout:      15 * time.Second,
    IdleTimeout:       60 * time.Second,
}, handler)
```

Controller 创建和业务路由注册集中放在 `internal/app/http.go`；应用层负责 Gin、中间件与业务依赖的组装，以及调用启停；入口负责进程信号。Controller 只处理请求。路由和中间件在启动前完成注册。

验证：

```sh
go test ./pkg/httpserver
go test ./integration-test -run TestHTTPServer
```
