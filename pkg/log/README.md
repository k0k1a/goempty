# 日志

包加载时自动创建包内的 `defaultLogger`，外部直接调用 `log.Debug/Info/Warn/Error`，无需初始化或设置实例。
默认使用 JSON 格式，同时输出到 stdout 和 `logs/{APP_NAME}_{ENV}.log`，Zap 内部错误写入 stderr。日志级别读取 `LEVEL`，未设置或为空时使用 info。
时间字段为 `time`，采用带时区的 ISO 8601 毫秒格式；记录调用位置，Error 及以上附带堆栈，不采样。

文件输出使用 `gopkg.in/natefinch/lumberjack.v2`，仅按大小轮转，不需要宿主机 logrotate 或定时任务。默认文件配置为 100 MB、90 天、无数量限制、压缩开启；独立实例的配置由调用方完整传入，`New` 不补默认值，也不读取环境变量。

## 使用

默认直接使用：

```go
import (
    "context"

    "goempty/pkg/log"
    "go.uber.org/zap"
)

func run() {
    ctx := context.Background()
    log.Info(ctx, "服务启动")
    ctx = log.WithRequestID(ctx, "request-1")
    log.Warn(ctx, "请求较慢", zap.Int("duration_ms", 1500))
}
```

全局 `Debug/Info/Warn/Error` 的首参均为标准 `context.Context`。`WithRequestID(ctx, id)` 返回携带请求 ID 的子 Context，不修改父 Context；全局方法自动附加非空的 `request_id`。未设置 ID、ID 为空或日志方法收到 nil Context 时省略该字段。业务代码应传递已有 Context；应用入口等没有请求上下文的地方可使用 `context.Background()`。request_id 由 Context 管理，调用方不要再手动添加同名 Zap 字段。

HTTP 中间件取得或生成请求 ID 后，应写回请求的标准 Context，供 Handler、Service 和 Repository 继续传递：

```go
ctx := log.WithRequestID(c.Request.Context(), requestID)
c.Request = c.Request.WithContext(ctx)
c.Header("X-Request-ID", requestID)
c.Next()
```

上述为 Gin 中间件内部用法示意；本包不依赖 Gin，不生成或校验请求 ID。`WithRequestID` 的父 Context 必须非 nil，内部采用私有类型的 key，与字符串 `"request_id"` 不互通。请求 ID 仅绑定到当前调用的日志，不修改全局 Logger。原先不带 ctx 的全局调用需要迁移；独立 `*zap.Logger` 的方法签名保持不变。

默认实例在包初始化时读取环境变量，须在启动进程前设置；运行中更改不会刷新默认实例。`APP_NAME`、`ENV` 在 `pkg/env` 包初始化时保存在私有变量中，通过只读访问函数与应用配置共用，未设置或为空时分别使用 `app`、`dev`，不校验或改写开发者提供的值。默认日志为 `logs/app_dev.log`，相对于进程启动工作目录；父目录自动创建，初始化时打开文件。`LEVEL` 非法或文件无法打开时，通过 `zap.Must` 终止启动。

需要其他输出路径或配置时，使用 `New(Config)` 创建独立的原生 `*zap.Logger`，直接调用或注入组件；它不改变包级函数使用的默认实例：

```go
import (
    "fmt"
    "os"

    "goempty/pkg/log"
    "go.uber.org/zap"
)

func run() error {
    logger, err := log.New(log.Config{
        Level:            "info",
        Encoding:         "json",
        OutputPaths:      []string{"stdout", "logs/app.log"},
        ErrorOutputPaths: []string{"stderr"},
        MaxSize:          100,
        MaxAge:           90,
        MaxBackups:       0,
        Compress:         true,
    })
    if err != nil {
        return err
    }
    defer func() {
        if err := log.Close(logger); err != nil {
            fmt.Fprintf(os.Stderr, "close log: %v\n", err)
        }
    }()

    logger.Info("started", zap.String("service", "goempty"))
    logger.With(zap.String("request_id", "example")).Info("request received")
    return nil
}
```

应用退出时先停止请求和后台任务，等待所有日志写入完成，再调用 `Shutdown()`。它只关闭默认实例的文件资源，不替换实例，也不切换为空实现；重复调用安全，stdout/stderr 不会关闭。关闭后不要再调用包级日志函数。

## 全局实例与接口

```go
type Logger interface {
    Debug(msg string, fields ...zap.Field)
    Info(msg string, fields ...zap.Field)
    Warn(msg string, fields ...zap.Field)
    Error(msg string, fields ...zap.Field)
}
```

- 包内只保存 `defaultLogger` 一个固定的 Zap 实例引用，同时用于打印和关闭；启动时由 `global.go` 的 `newDefaultLogger` 根据环境变量组装完整配置，传给 `New` 自动创建，不传空配置。
- 包级函数从 Context 读取请求 ID，有值时通过默认实例的 `With` 派生 Logger 再写入，不额外加全局锁；并发写入由 Zap 及输出实现保障。关闭顺序由应用负责，`Shutdown` 不等待业务任务退出。
- 默认实例为包级转发增加一个调用栈跳过层级，日志文件行号指向业务调用处；独立实例直接调用的行号保持不变。
- `Logger` 接口保留给业务组件定义依赖；独立实例或测试替身可通过组件构造参数注入，不修改全局实例。
- 字段仍采用 `zap.Field`，例如 `log.Error(ctx, "query failed", zap.Error(err))`。独立实例不自动读取 Context，请求专用字段可在独立实例上使用 `With`。

独立实例使用 `Close(logger)` 关闭。`With`、`Named` 派生 Logger 共享原实例的文件输出，关闭任意一个会关闭共同的文件，关闭后不要继续使用这些实例。

应保留初始化得到的根 Logger 用于关闭。通过 `WithOptions` 替换/包装 Core（包括 `WithLazy`）后，`Close` 可能无法识别它，需关闭原根 Logger。Lumberjack 不提供文件 `fsync`，原生 `Sync()` 不代替 `Close()`，也不保证文件强制落盘。

## 配置

独立实例的配置字段带 `mapstructure` 标签，可由应用配置加载器填充后传入 `New`；此配置不会自动覆盖包内默认实例：

```yaml
log:
  level: info
  encoding: json
  output_paths: [stdout, /app/logs/app.log]
  error_output_paths: [stderr]
  max_size: 100
  max_age: 90
  max_backups: 0
  compress: true
```

| 字段 | 含义与校验规则 |
| --- | --- |
| `level` | 必填；debug、info、warn、error、dpanic、panic、fatal |
| `encoding` | 必填；json 或 console |
| `output_paths` | 必填、非空列表；业务日志输出 |
| `error_output_paths` | 必填、非空列表；Zap 自身写入失败等内部错误，业务 Error 日志仍写入 `output_paths` |
| `max_size` | 必须大于 0；每个文件上限，单位 MB（1024 × 1024 字节） |
| `max_age` | 历史文件保留天数；0 关闭按时间清理 |
| `max_backups` | 最多保留的历史文件数量；0 不限制数量，仍按 `max_age` 清理 |
| `compress` | 是否 gzip 压缩；bool 值原样使用，false 不压缩 |

`New(Config{})` 返回配置校验错误。天数和数量不得为负数；当 `max_age` 和 `max_backups` 同时为 0 时不自动清理历史文件。输出路径只使用 stdout、stderr 或本地文件路径；不提供 URL sink 配置。Windows 可使用 `D:/logs/app.log`。Lumberjack 自动创建父目录，`New` 打开文件检查可写性，初始化失败时关闭已打开的文件。

轮转配置同时用于两组输出中的本地文件。同一 `New` 中相同绝对文件路径共用一个 Lumberjack 实例，避免彼此独立轮转；不要通过符号链接等不同路径指向同一个文件。

## 文件与保留规则

当前文件保持为 `app.log`。一次写入会使文件超过大小上限时，Lumberjack 将它重命名为带轮转时间戳的历史文件，再创建新的 `app.log`：

```text
logs/
  app.log
  app-2026-11-02T14-30-05.123.log.gz
  app-2026-11-02T10-15-02.456.log.gz
```

- 时间戳使用本地时区，代表轮转时间；不按天拆分，同一天可生成多个文件。
- 90 天按 90 × 24 小时计算。历史清理和压缩由 Lumberjack 异步执行，在打开文件或轮转等时机触发；没有新日志写入时，过期文件可能继续保留。
- 设置 `max_backups` 为正数后，可能在达到 90 天前就按数量删除历史文件。
- 单条编码后的日志超过 `max_size` 时写入失败，错误写到 `error_output_paths`。
- 此前按月目录、按天文件的旧日志不会自动迁移或清理，需由部署人员另行处理。
- 一个文件系列由一个进程、一个 Logger 初始化实例管理。多实例部署使用不同目录。

## Docker 挂载

使用上述 `/app/logs/app.log` 配置时，将日志目录挂载到宿主机。以下为 Compose 服务配置片段，镜像由项目部署流程提供：

```yaml
services:
  app:
    image: goempty:latest
    volumes:
      - /srv/goempty/logs:/app/logs
    logging:
      driver: local
      options:
        max-size: "20m"
        max-file: "5"
```

宿主机实际保存为 `/srv/goempty/logs/app.log` 及同目录的历史文件，容器运行用户需对挂载目录有写权限。容器和宿主机应按需要统一时区；使用时区名称的容器镜像需包含相应时区数据。

`docker logs` 查看 stdout/stderr 近期日志，Docker `local` 驱动管理这份输出的大小和数量；90 天文件保留由 Lumberjack 管理。宿主机不要再对这些应用文件配置 logrotate。90 天并非磁盘总容量上限，需按实际日志量预留空间。
