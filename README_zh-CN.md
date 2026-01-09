# DD - High-Performance Go Logging Library

[![Go Version](https://img.shields.io/badge/Go-1.24+-00ADD8?style=flat&logo=go)](https://go.dev/)
[![pkg.go.dev](https://pkg.go.dev/badge/github.com/cybergodev/dd.svg)](https://pkg.go.dev/github.com/cybergodev/dd)
[![License](https://img.shields.io/badge/license-MIT-brightgreen.svg)](LICENSE)
[![Security](https://img.shields.io/badge/security-policy-blue.svg)](SECURITY.md)
[![Thread Safe](https://img.shields.io/badge/thread%20safe-yes-brightgreen.svg)](https://github.com/cybergodev/json)

一个生产级高性能 Go 日志库，零外部依赖，专为现代应用设计。

#### **[📖 English Docs](README.md)** - User guide


## ✨ 核心特性

- 🚀 **极致性能** - 简单日志 3M+ ops/sec，结构化日志 1M+ ops/sec，专为高吞吐量系统优化
- 🔒 **线程安全** - 原子操作 + 无锁设计，完全并发安全
- 🛡️ **内置安全** - 敏感数据过滤（信用卡、密码、API密钥、JWT等12种模式），防注入攻击
- 📊 **结构化日志** - 类型安全字段，支持 JSON/文本双格式，可自定义字段名
- 📁 **智能轮转** - 按大小/时间自动轮转，自动压缩 .gz，自动清理过期文件
- 📦 **零依赖** - 仅使用 Go 标准库，无第三方依赖
- 🎯 **简单易用** - 2分钟上手，直观的 API，4种便捷构造器
- 🔧 **灵活配置** - 3种预设配置 + Options 模式，支持多输出、缓冲写入
- 🌐 **云原生友好** - JSON 格式适配 ELK/Splunk/CloudWatch 等日志系统
- ⚡ **性能优化** - 对象池复用、预分配缓冲区、延迟格式化、动态调用者检测

## 📦 安装

```bash
go get github.com/cybergodev/dd
```

## 🚀 快速开始

### 30秒上手

```go
package main

import "github.com/cybergodev/dd"

func main() {
    // 方式1: 使用全局默认 logger（最简单）
    dd.Info("Application started")
    dd.Warn("Cache miss for key user:123")
    dd.Error("Failed to connect to database")
    
    // 方式2: 创建自定义 logger（推荐）
    logger := dd.ToFile()  // 输出到 logs/app.log
    defer logger.Close()

    logger.Info("Application started")
    logger.InfoWith("User login",
        dd.Int("id", 12345),
        dd.String("type", "vip"),
        dd.Any("usernames", []string{"alice", "bob"}),
    )
}
```

### 最简单的方式（输出控制台）

```go
import "github.com/cybergodev/dd"

func main() {
    dd.Debug("Debug message")
    dd.Info("Application started")
    dd.Warn("Cache miss for key user:123")
    dd.Error("Failed to connect to database")
    dd.Fatal("Application exiting")  // 结束运行（调用 os.Exit(1)）
    
    // 调用了 dd.Fatal()，以下代码不会被执行
    fmt.Println("Hello, World!")
}
```

### 文件日志（一行代码）

```go
logger := dd.ToFile()              // → 仅文件 logs/app.log
logger := dd.ToJSONFile()          // → 仅JSON格式文件 logs/app.log
logger := dd.ToAll()               // → 控制台 + logs/app.log
logger := dd.ToConsole()           // → 仅控制台
defer logger.Close()

logger.Info("Logging to file")

// 自定义文件名
logger := dd.ToFile("logs/myapp.log")
defer logger.Close()
```

### 结构化日志（生产环境）

```go
// 日志记录到文件
logger := dd.ToJSONFile()
defer logger.Close()

logger.InfoWith("HTTP Request",
    dd.Any("method", "POST"),
    dd.String("path", "/api/users"),
    dd.Int("status", 201),
    dd.Float64("duration_ms", 45.67),
)

err := errors.New("database connection failed")
logger.ErrorWith("Operation failed",
    dd.Err(err),
    dd.Any("operation", "insert"),
    dd.Int("retry_count", 3),
)
```

**JSON 输出**:
```json
{"timestamp":"2024-01-15T10:30:45Z","level":"INFO","message":"HTTP Request","fields":{"method":"POST","path":"/api/users","status":201,"duration_ms":45.67}}
```

### 自定义配置

```go
logger, err := dd.NewWithOptions(dd.Options{
    Level:         dd.LevelDebug,
    Format:        dd.FormatJSON,
    Console:       true,
    File:          "logs/myApp.log",
    IncludeCaller: true,
    FilterLevel:   "basic", // "none", "basic", "full"
})
if err != nil {
    panic(err)
}
defer logger.Close()
```

## 📖 核心功能

### 预设配置

三种预设配置，快速适配不同场景：

```go
// 生产环境 - 平衡性能与功能
logger, _ := dd.New(dd.DefaultConfig())

// 开发环境 - DEBUG级别 + 调用者信息
logger, _ := dd.New(dd.DevelopmentConfig())

// 云原生 - JSON格式，DEBUG级别，适配 ELK/Splunk/CloudWatch 等日志格式
logger, _ := dd.New(dd.JSONConfig())
```

### 日志文件大小分片与压缩备份

```go
logger, _ := dd.NewWithOptions(dd.Options{
    File: "app.log",
    FileConfig: dd.FileWriterConfig{
        MaxSizeMB:  100,                 // 分片大小 100MB
        MaxBackups: 10,                  // 保留 10 个备份
        MaxAge:     30 * 24 * time.Hour, // 30 天后删除
        Compress:   true,                // 压缩旧文件 (.gz)
    },
})
```

**特性**：按大小自动分片、按时间清理旧文件、自动压缩节省空间、线程安全、防路径遍历攻击


### 安全过滤

**默认禁用**：以保证性能，需要时启用：

```go
// 基础过滤（推荐，性能影响小）
config := dd.DefaultConfig().EnableBasicFiltering()
logger, _ := dd.New(config)

logger.Info("password=secret123")           // → password=[REDACTED]
logger.Info("api_key=sk-1234567890")        // → api_key=[REDACTED]
logger.Info("credit_card=4532015112830366") // → credit_card=[REDACTED]

// 或使用 Options
logger, _ := dd.NewWithOptions(dd.Options{
    FilterLevel: "basic", // "none", "basic", "full"
})
```

**基础过滤**（6种模式）:
- 信用卡号、SSN、密码、API密钥、OpenAI密钥、私钥

**完整过滤**（12种模式）:
- 信用卡号、SSN、密码、API密钥、JWT、私钥、AWS密钥、Google API密钥、OpenAI密钥、邮箱、IP、数据库连接串

**自定义过滤**:
```go
filter := dd.NewEmptySensitiveDataFilter()
filter.AddPattern(`(?i)internal[_-]?token[:\s=]+[^\s]+`)
filter.AddPattern(`...`)  // 可添加多个过滤

config := dd.DefaultConfig().WithFilter(filter)
```

**防注入攻击**（始终启用）:
- 自动转义换行符和控制字符
- 消息大小限制（默认5MB）
- 防路径遍历


防注入攻击可按需配置
```go
// 方式1: 创建配置时直接设置
config := dd.DefaultConfig()
config.SecurityConfig = &dd.SecurityConfig{
    MaxMessageSize:  10 * 1024 * 1024, // 自定义为10MB
    MaxWriters:      100,
    SensitiveFilter: nil,
}
logger, _ := dd.New(config)

// 方式2: 修改现有配置
config := dd.DefaultConfig()
config.SecurityConfig.MaxMessageSize = 10 * 1024 * 1024 // 自定义为 10MB
logger, _ := dd.New(config)
```

**安全特性总结**:

| 特性         | 默认状态 | 说明            |
|------------|------|---------------|
| 敏感数据过滤     | 禁用   | 需手动启用（性能考虑）   |
| 消息大小限制     | 5MB  | 防止内存溢出（默认5MB） |
| 换行符转义      | 启用   | 防止日志注入攻击      |
| 控制字符过滤     | 启用   | 自动移除危险字符      |
| 路径遍历防护     | 启用   | 文件写入时自动检查     |
| Writer数量限制 | 100  | 防止资源耗尽        |
| 字段键名验证     | 启用   | 自动清理非法字符      |

### 性能基准

在 Intel Core Ultra 9 185H 上的实测数据：

| 操作类型        | 吞吐量              | 内存/Op   | 分配/Op     | 场景说明             |
|-------------|------------------|---------|-----------|------------------|
| 简单日志        | **3.1M ops/sec** | 200 B   | 7 allocs  | 基础文本日志           |
| 格式化日志       | **2.4M ops/sec** | 272 B   | 8 allocs  | Infof/Errorf     |
| 结构化日志       | **1.9M ops/sec** | 417 B   | 12 allocs | InfoWith + 3字段   |
| 复杂结构化日志     | **720K ops/sec** | 1,227 B | 26 allocs | InfoWith + 8字段   |
| JSON格式      | **600K ops/sec** | 800 B   | 20 allocs | JSON 结构化输出       |
| 并发日志(22协程)  | **68M ops/sec**  | 200 B   | 7 allocs  | 22个goroutine并发   |
| 日志级别检查      | **2.5B ops/sec** | 0 B     | 0 allocs  | 级别过滤（不输出）        |
| 字段创建        | **50M ops/sec**  | 16 B    | 1 allocs  | String/Int字段构造   |

**性能优化技术**:
- 对象池（sync.Pool）复用缓冲区，减少 GC 压力
- 原子操作（atomic）替代互斥锁，实现无锁热路径
- 预分配缓冲区，避免动态扩容
- 延迟格式化，仅在需要时才格式化消息
- 动态调用者检测，自动适配调用深度
- 单写入器快速路径优化

## 📚 API 快速参考

### 日志方法

```go
// 简单日志
logger.Debug / Info / Warn / Error / Fatal (args ...any)

// 格式化日志
logger.Debugf / Infof / Warnf / Errorf / Fatalf (format string, args ...any)

// 结构化日志
logger.DebugWith / InfoWith / WarnWith / ErrorWith / FatalWith (msg string, fields ...Field)

// 调试数据可视化
logger.Json(data ...any)                    // 输出紧凑 JSON 到控制台
logger.Jsonf(format string, args ...any)    // 输出格式化 JSON 到控制台
logger.Text(data ...any)                    // 输出格式化文本到控制台
logger.Textf(format string, args ...any)    // 输出格式化文本到控制台
logger.Exit(data ...any)                    // 输出文本并退出程序 (os.Exit(0))
logger.Exitf(format string, args ...any)    // 输出格式化文本并退出程序

// 配置管理
logger.SetLevel(level LogLevel)
logger.GetLevel() LogLevel
logger.AddWriter(w io.Writer) error
logger.Close() error
```

### 包级函数

```go
// 使用全局默认 logger
dd.Debug / Info / Warn / Error / Fatal (args ...any)
dd.Debugf / Infof / Warnf / Errorf / Fatalf (format string, args ...any)
dd.DebugWith / InfoWith / WarnWith / ErrorWith / FatalWith (msg string, fields ...Field)

// 调试数据可视化
dd.Json(data ...any)                    // 输出紧凑 JSON 到控制台
dd.Jsonf(format string, args ...any)    // 输出格式化 JSON 到控制台
dd.Text(data ...any)                    // 输出格式化文本到控制台
dd.Textf(format string, args ...any)    // 输出格式化文本到控制台
dd.Exit(data ...any)                    // 输出文本并退出程序 (os.Exit(0))
dd.Exitf(format string, args ...any)    // 输出格式化文本并退出程序

// 全局 logger 管理
dd.Default() *Logger
dd.SetDefault(logger *Logger)
```

### 便捷构造器

```go
// 快速构造器（一行代码创建）
dd.ToFile(filename ...string) *Logger        // 仅文件（默认 logs/app.log）
dd.ToJSONFile(filename ...string) *Logger    // 仅JSON文件（默认 logs/app.log）
dd.ToConsole() *Logger                       // 仅控制台
dd.ToAll(filename ...string) *Logger         // 控制台+文件（默认 logs/app.log）

// 标准构造器
dd.New(config *LoggerConfig) (*Logger, error)        // 使用配置对象
dd.NewWithOptions(opts Options) (*Logger, error)     // 使用 Options 模式

// 预设配置
dd.DefaultConfig() *LoggerConfig      // 生产配置（Info级别，文本格式）
dd.DevelopmentConfig() *LoggerConfig  // 开发配置（Debug级别，带调用者信息）
dd.JSONConfig() *LoggerConfig         // JSON配置（Debug级别，适配云日志系统）
```

### fmt包 平替方法

DD 为 Go 语言的标准 `fmt` 包提供了一个类似的替代方案，该方案具有相似的 API，并且还增强了日志集成功能：
```go
// 直接输出（标准输出） - 带调用者信息
dd.Printf(format, args...)     // 将输出格式化后显示在标准输出端口上（带调用者信息）
dd.Print(args...)              // 默认格式输出至标准输出流（与 fmt 相同）
dd.Println(args...)            // 默认格式输出，并带有换行符和调用者信息

// 字符串 返回值 - 与 fmt 相同
dd.Sprintf(format, args...)    // 返回格式化后的字符串
dd.Sprint(args...)             // 返回默认格式字符串
dd.Sprintln(args...)           // 返回带有换行符的默认格式字符串

// Writer 输出 - 与 fmt 完全相同
dd.Fprintf(w, format, args...) // 向 Writer 输出格式化内容
dd.Fprint(w, args...)          // 默认格式输出给 Writer
dd.Fprintln(w, args...)        // 以换行的方式将默认格式输出至 Writer 中

// 输入扫描 - 与 fmt 相同
dd.Scan(a...)                  // 从标准输入获取以空格分隔的输入
dd.Scanf(format, a...)         // 从标准输入获取的格式化输入
dd.Scanln(a...)                // 从标准输入获取基于行的输入
dd.Fscan(r, a...) / Fscanf / Fscanln    // 来自 io 的输入
dd.Sscan(str, a...) / Sscanf / Sscanln  // 字符串输入

// 错误生成 - 改进的命名方式
dd.NewError(format, args...)   // 生成错误（例如 fmt.Errorf 函数）
dd.NewErrorWith(format, args...) // 产生错误并记录下来

// 缓冲区操作 - 与 fmt 相同
dd.AppendFormat(dst, format, args...) // 将格式化内容写入缓冲区
dd.Append(dst, args...)        // 将默认格式添加到缓冲区中
dd.Appendln(dst, args...)      // 向缓冲区追加换行符

// 增强功能与日志集成
dd.PrintfWith(format, args...) // 将输出发送至标准输出以及记录消息
dd.PrintlnWith(args...)        // 将输出发送至标准输出以及记录消息
```

### 字段构造器

```go
dd.Any(key string, value any) Field          // 通用类型（推荐，支持任意类型）
dd.String(key, value string) Field           // 字符串
dd.Int(key string, value int) Field          // 整数
dd.Int64(key string, value int64) Field      // 64位整数
dd.Float64(key string, value float64) Field  // 浮点数
dd.Bool(key string, value bool) Field        // 布尔值
dd.Err(err error) Field                      // 错误（自动提取 error.Error()）
```

## 🔧 配置指南

### Options 配置（推荐）

```go
logger, err := dd.NewWithOptions(dd.Options{
    Level:   dd.LevelInfo,    // 日志级别
    Format:  dd.FormatJSON,   // 输出格式（FormatText/FormatJSON）
    Console: true,            // 控制台输出
    File:    "logs/app.log",  // 文件路径
    
    FileConfig: dd.FileWriterConfig{
        MaxSizeMB:  100,                 // 100MB轮转
        MaxBackups: 10,                  // 保留10个备份
        MaxAge:     30 * 24 * time.Hour, // 30天后删除
        Compress:   true,                // 压缩旧文件（.gz）
    },
    
    IncludeCaller: true,            // 显示调用位置（文件名:行号）
    FullPath:      false,           // 显示完整路径（默认 false 仅显示文件名）
    DynamicCaller: false,           // 动态检测调用深度（自动适配封装）
    TimeFormat:    time.RFC3339,    // 时间格式
    FilterLevel:   "basic",         // 敏感数据过滤："none", "basic", "full"
    
    JSONOptions: &dd.JSONOptions{
        PrettyPrint: false,                 // 美化输出（开发环境可用）
        Indent:      "  ",                  // 缩进字符
        FieldNames: &dd.JSONFieldNames{     // 自定义字段名
            Timestamp: "timestamp",
            Level:     "level",
            Caller:    "caller",
            Message:   "message",
            Fields:    "fields",
        },
    },
    
    AdditionalWriters: []io.Writer{customWriter},  // 额外输出目标
})
```

### LoggerConfig 配置（高级）

```go
config := dd.DefaultConfig()
config.Level = dd.LevelDebug
config.Format = dd.FormatJSON
config.IncludeCaller = true
config.DynamicCaller = true
config.Writers = []io.Writer{os.Stdout, fileWriter}

// 链式配置
config.WithLevel(dd.LevelInfo).
       WithFormat(dd.FormatJSON).
       WithCaller(true).
       EnableBasicFiltering()

logger, err := dd.New(config)
```

### 日志级别

```go
dd.LevelDebug  // 调试信息（开发环境）
dd.LevelInfo   // 常规信息（默认，生产环境）
dd.LevelWarn   // 警告（需要关注但不影响运行）
dd.LevelError  // 错误（影响功能但不致命）
dd.LevelFatal  // 致命错误（调用 os.Exit(1) 终止程序）
```

**级别层次**: `Debug < Info < Warn < Error < Fatal`

**动态调整级别**:
```go
logger.SetLevel(dd.LevelDebug)  // 运行时调整
currentLevel := logger.GetLevel()
```

### 输出格式

**文本格式**（开发环境，易读）:
```
[2024-01-15T10:30:45+08:00] [INFO] Application started
[2024-01-15T10:30:46+08:00] [ERROR] main.go:42 Connection failed
```

**JSON格式**（生产环境，易解析）:
```json
{"timestamp":"2025-01-15T10:30:45Z","level":"INFO","message":"Application started"}
{"timestamp":"2025-01-15T10:30:46Z","level":"ERROR","caller":"main.go:42","message":"Connection failed"}
```

### 多输出目标

```go
// 方式1: 使用 Options
logger, _ := dd.NewWithOptions(dd.Options{
    Console: true,
    File:    "logs/app.log",
    AdditionalWriters: []io.Writer{
        customWriter,
        networkWriter,
    },
})

// 方式2: 动态添加
logger.AddWriter(newWriter)
logger.RemoveWriter(oldWriter)

// 方式3: 使用 MultiWriter
mw := dd.NewMultiWriter(writer1, writer2, writer3)
config := dd.DefaultConfig()
config.Writers = []io.Writer{mw}
logger, _ := dd.New(config)
```

### 缓冲写入（高性能场景）

```go
// 创建缓冲写入器（减少系统调用）
fileWriter, _ := dd.NewFileWriter("app.log", nil)
bufferedWriter, _ := dd.NewBufferedWriter(fileWriter, 4096)  // 4KB缓冲
defer bufferedWriter.Close()

config := dd.DefaultConfig()
config.Writers = []io.Writer{bufferedWriter}
logger, _ := dd.New(config)
```

### 全局默认 Logger

```go
// 设置全局默认 logger
customLogger, _ := dd.NewWithOptions(dd.Options{
    Level:  dd.LevelDebug,
    Format: dd.FormatJSON,
})
dd.SetDefault(customLogger)

// 使用全局 logger
dd.Info("Using global logger")
dd.InfoWith("Structured", dd.String("key", "value"))

// 获取当前默认 logger
logger := dd.Default()
```

## 高级特性

### 动态调用者检测

自动检测调用栈深度，适配各种封装场景：

```go
config := dd.DevelopmentConfig()
config.DynamicCaller = true  // 启用动态检测
logger, _ := dd.New(config)

// 即使通过多层封装调用，也能正确显示真实调用位置
func MyLogWrapper(msg string) {
    logger.Info(msg)  // 显示 MyLogWrapper 的调用者，而非此行
}
```

### JSON 字段名自定义

适配不同日志系统的字段命名规范：

```go
logger, _ := dd.NewWithOptions(dd.Options{
    Format: dd.FormatJSON,
    JSONOptions: &dd.JSONOptions{
        FieldNames: &dd.JSONFieldNames{
            Timestamp: "time",      // 默认 "timestamp"
            Level:     "severity",  // 默认 "level"
            Caller:    "source",    // 默认 "caller"
            Message:   "msg",       // 默认 "message"
            Fields:    "data",      // 默认 "fields"
        },
    },
})

// 输出: {"time":"...","severity":"INFO","msg":"test","data":{...}}
```

### 自定义 Fatal 处理器

控制 Fatal 级别日志的行为：

```go
config := dd.DefaultConfig()
config.FatalHandler = func() {
    // 自定义清理逻辑
    cleanup()
    os.Exit(2)  // 自定义退出码
}
logger, _ := dd.New(config)

logger.Fatal("Critical error")  // 调用自定义处理器
```

### 安全配置

精细控制安全限制：

```go
config := dd.DefaultConfig()
config.SecurityConfig = &dd.SecurityConfig{
    MaxMessageSize:  10 * 1024 * 1024,      // 10MB 消息限制
    MaxWriters:      50,                    // 最多 50 个输出目标
    SensitiveFilter: dd.NewBasicSensitiveDataFilter(),
}
logger, _ := dd.New(config)

// 运行时调整
logger.SetSecurityConfig(&dd.SecurityConfig{
    MaxMessageSize: 5 * 1024 * 1024,
})
```

### 自定义敏感数据过滤

```go
// 创建空过滤器，添加自定义规则
filter := dd.NewEmptySensitiveDataFilter()
filter.AddPattern(`(?i)internal[_-]?token[:\s=]+[^\s]+`)
filter.AddPattern(`\bSECRET_[A-Z0-9_]+\b`)

// 或批量添加
patterns := []string{
    `custom_pattern_1`,
    `custom_pattern_2`,
}
filter.AddPatterns(patterns...)

// 动态启用/禁用
filter.Enable()
filter.Disable()
if filter.IsEnabled() {
    // ...
}

// 使用自定义过滤器
config := dd.DefaultConfig()
config.SecurityConfig.SensitiveFilter = filter
logger, _ := dd.New(config)
```

### 克隆配置

安全复制配置对象：

```go
baseConfig := dd.DefaultConfig()
baseConfig.Level = dd.LevelInfo
baseConfig.EnableBasicFiltering()

// 克隆并修改
devConfig := baseConfig.Clone()
devConfig.Level = dd.LevelDebug
devConfig.IncludeCaller = true

logger1, _ := dd.New(baseConfig)  // 生产配置
logger2, _ := dd.New(devConfig)   // 开发配置
```

## 📚 最佳实践

### 1. 生产环境配置

```go
logger, _ := dd.NewWithOptions(dd.Options{
    Level:       dd.LevelInfo,
    Format:      dd.FormatJSON,
    File:        "logs/app.log",
    Console:     false,  // 生产环境不输出控制台
    FilterLevel: "basic",
    FileConfig: dd.FileWriterConfig{
        MaxSizeMB:  100,
        MaxBackups: 30,
        MaxAge:     7 * 24 * time.Hour,
        Compress:   true,
    },
})
defer logger.Close()
```

### 2. 开发环境配置

```go
logger, _ := dd.NewWithOptions(dd.Options{
    Level:         dd.LevelDebug,
    Format:        dd.FormatText,
    Console:       true,
    IncludeCaller: true,
    DynamicCaller: true,
    TimeFormat:    "15:04:05.000",
})
defer logger.Close()
```

### 3. 结构化日志最佳实践

```go
// ✅ 推荐：使用类型安全的字段
logger.InfoWith("User login",
    dd.String("user_id", userID),
    dd.String("ip", clientIP),
    dd.Int("attempt", attemptCount),
)

// ❌ 不推荐：字符串拼接
logger.Info(fmt.Sprintf("User %s login from %s", userID, clientIP))
```

### 示例代码

查看 [examples](examples) 目录获取完整的示例代码。



## 🤝 贡献指南

欢迎贡献代码、报告问题或提出建议！

## 📄 许可证

MIT License - 详见 [LICENSE](LICENSE) 文件。

---

**为 Go 社区用心打造** ❤️ | 如果这个项目对你有帮助，请给个 ⭐️ Star！
