# MongoDB

基于项目已有的 MongoDB 官方 Go Driver v2，统一配置、连接检查和关闭行为。返回原生 `*mongo.Client`，由调用方共享和管理生命周期；本包不依赖应用配置、日志或 internal 包。

## 使用

```go
ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
defer cancel()

client, err := mongodb.New(ctx, mongodb.Config{
    URI: "mongodb://127.0.0.1:27017",
    MaxPoolSize: 100,
    MinPoolSize: 0,
    ConnectTimeout: 5*time.Second,
    ServerSelectionTimeout: 5*time.Second,
    Timeout: 5*time.Second,
})
if err != nil {
    return err
}
// client.Database("app").Collection("users") 获取原生集合句柄。
// 数据操作继续传递请求 Context，CRUD、索引和完整事务由 Repo 按业务封装。

// 关闭阶段先停止接收请求并等待在途操作，再使用新的限时 Context：
shutdownCtx, shutdownCancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
defer shutdownCancel()
return mongodb.Close(shutdownCtx, client)
```

示例假设已导入 `context`、`time` 和 `goempty/pkg/mongodb`，位于返回 `error` 的函数中。`context.Background()` 仅用于进程入口或独立工具；已有调用链时传递其 Context。URI 应由调用方配置提供，不提交真实凭据。

## 配置与边界

| 字段 / mapstructure 键 | 含义 |
| --- | --- |
| `URI` / `uri` | 必填，官方驱动连接串；支持认证、TLS、副本集、SRV 等驱动参数 |
| `MaxPoolSize` / `max_pool_size` | 每台服务器的最大连接数 |
| `MinPoolSize` / `min_pool_size` | 每台服务器的最小连接数 |
| `MaxConnIdleTime` / `max_conn_idle_time` | 空闲连接最大保留时长 |
| `ConnectTimeout` / `connect_timeout` | 单次连接建立超时 |
| `ServerSelectionTimeout` / `server_selection_timeout` | 服务器选择超时 |
| `Timeout` / `timeout` | 未携带 deadline 的操作使用的默认超时 |

- 正值字段覆盖 URI 中对应设置；零值保留 URI 或官方驱动默认值。连接池大小和时长不允许负值。最大连接数默认 100，最小默认 0；最小值不能超过有效最大值。需要无限连接池时通过 URI 的 `maxPoolSize=0` 显式指定。
- `Validate()` 校验最终配置，不连接 MongoDB；驱动解析 SRV URI 时可能查询 DNS，解析 TLS 配置时可能读取证书文件。这些解析步骤不受 Context 取消控制；初始化前后会检查 Context，整个初始化不承诺严格受 deadline 限制。
- `New()` 在创建客户端后按其读取偏好执行 Ping（默认 primary）；成功表示该检查通过，不保证所有节点、业务权限或后续操作可用。建议调用方始终传入限时 Context。
- Ping 失败时自动清理，清理 Context 保留调用链值但不继承取消状态，最多等待 5 秒；主错误和清理错误通过 `errors.Join` 返回。
- URI 解析及客户端创建错误使用固定消息，避免回显敏感连接设置；Ping/Close 保留驱动错误链，支持 `errors.Is/As`。这些运行时错误可能包含主机、拓扑等内部信息，不能直接作为对外业务响应，也不要记录完整配置或 URI。
- `Ping`、`Close` 拒绝 nil 客户端。关闭期间 Context 到期时，驱动仍会关闭连接，未完成操作可能失败；关闭前应先等待在途操作结束，避免复用已取消的请求 Context。
- 应用已通过 `config.Config.MongoDB` 和 YAML 的 `mongodb` 段接入本包。Wire 创建共享客户端，App 在 HTTP 和任务停止后调用清理函数；MongoDB 不可用时应用启动失败。后续业务通过构造函数注入 Repo，配置重载不自动重建客户端。完整接入说明见[应用配置](../../config/README.md)和[应用组装](../../internal/app/README.md)。

## 验证

```powershell
go test ./pkg/mongodb
```

单元测试包含配置优先级、非法设置、敏感信息保护、取消与关闭行为，以及仅连接测试自建 TCP 端点的初始化失败路径。

真实连接测试只对隔离 MongoDB 执行 Ping 和原生命令，不写持久数据：通过 `GOEMPTY_TEST_MONGODB_URI` 提供连接串后运行 `go test -count=1 -run TestMongoDBLifecycle -v ./integration-test`。缺少该变量时跳过，不代表已验证连接；请勿在命令行或提交内容中暴露真实凭据。
