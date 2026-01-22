# DD - 高性能 Go 日志库

[![Go Version](https://img.shields.io/badge/Go-1.24+-00ADD8?style=flat&logo=go)](https://go.dev/)
[![pkg.go.dev](https://pkg.go.dev/badge/github.com/cybergodev/dd.svg)](https://pkg.go.dev/github.com/cybergodev/dd)
[![License](https://img.shields.io/badge/license-MIT-brightgreen.svg)](LICENSE)
[![Security](https://img.shields.io/badge/security-policy-blue.svg)](SECURITY.md)
[![Thread Safe](https://img.shields.io/badge/thread%20safe-yes-brightgreen.svg)](https://github.com/cybergodev/json)

一款生产级高性能 Go 日志库，零外部依赖，专为现代应用设计。

#### **[📖 English Documentation](README.md)** - 用户指南

## ✨ 核心特性

- 🚀 **极致性能** - 简单日志 300万+ 次/秒，结构化日志 100万+ 次/秒，为高吞吐系统优化
- 🔒 **线程安全** - 原子操作 + 无锁设计，完全并发安全
- 🛡️ **内置安全** - 敏感数据过滤（包括数据库、密码、API 密钥等）、注入攻击防护
- 📊 **结构化日志** - 类型安全字段，支持 JSON/文本双格式，可自定义字段名
- 📁 **智能轮转** - 按大小/时间自动轮转，自动压缩为 .gz，自动清理过期文件
- 📦 **零依赖** - 仅使用 Go 标准库，无第三方依赖
- 🎯 **易于使用** - 2 分钟上手，直观的 API，4 种便捷构造函数
- 🔧 **灵活配置** - 3 种预设配置 + Options 模式，支持多输出、缓冲写入
- 🌐 **云原生友好** - JSON 格式兼容 ELK/Splunk/CloudWatch 等日志系统
- ⚡ **性能优化** - 对象池复用、预分配缓冲区、延迟格式化、动态调用者检测

## 📦 安装

```bash
go get github.com/cybergodev/dd
```

## 🚀 快速开始

### 30 秒快速上手

```go
package main

import "github.com/cybergodev/dd"

func main() {
    // 方式 1：使用全局默认 logger（最简单）
    dd.Info("应用已启动")
    dd.Warn("缓存未命中: user:123")
    dd.Error("连接数据库失败")

    // 方式 2：创建自定义 logger（推荐）
    logger := dd.ToFile()  // 输出到 logs/app.log
    defer logger.Close()

    logger.Info("应用已启动")
    logger.InfoWith("用户登录",
        dd.Int("id", 12345),
        dd.String("type", "vip"),
        dd.Any("usernames", []string{"alice", "bob"}),
    )
}
```

### 最简单的方式（控制台输出）

```go
import "github.com/cybergodev/dd"

func main() {
    dd.Debug("调试信息")
    dd.Info("应用已启动")
    dd.Warn("缓存未命中: user:123")
    dd.Error("连接数据库失败")
    dd.Fatal("应用即将退出")  // 退出程序 (调用 os.Exit(1))

    // dd.Fatal() 之后的代码不会执行
    fmt.Println("Hello, World!")
}
```

### 文件日志（一行代码）

```go
logger := dd.ToFile()              // → 仅文件：logs/app.log
logger := dd.ToJSONFile()          // → JSON 格式文件：logs/app.log
logger := dd.ToAll()               // → 控制台 + logs/app.log
logger := dd.ToConsole()           // → 仅控制台
defer logger.Close()

logger.Info("记录到文件")

// 自定义文件名
logger := dd.ToFile("logs/myapp.log")
defer logger.Close()
```

### 结构化日志（生产环境）

```go
// 记录到文件
logger := dd.ToJSONFile()
defer logger.Close()

logger.InfoWith("HTTP 请求",
    dd.String("method", "POST"),
    dd.String("path", "/api/users"),
    dd.Int("status", 201),
    dd.Float64("duration_ms", 45.67),
)

err := errors.New("数据库连接失败")
logger.ErrorWith("操作失败",
    dd.Err(err),
    dd.Any("operation", "insert"),
    dd.Int("retry_count", 3),
)
```

**JSON 输出**：
```json
{"timestamp":"2024-01-15T10:30:45Z","level":"INFO","message":"HTTP 请求","fields":{"method":"POST","path":"/api/users","status":201,"duration_ms":45.67}}
```

### 自定义配置

```go
logger, err := dd.NewWithOptions(dd.Options{
    Level:         dd.LevelDebug,
    Format:        dd.FormatJSON,
    Console:       true,
    File:          "logs/myApp.log",
    DynamicCaller: true,
    FilterLevel:   "basic", // "none", "basic", "full"
})
if err != nil {
    panic(err)
}
defer logger.Close()
```

## 📖 核心功能

### 预设配置

三种预设配置，快速适应不同场景：

```go
// 生产环境 - 平衡性能和功能
logger, _ := dd.New(dd.DefaultConfig())

// 开发环境 - DEBUG 级别 + 调用者信息
logger, _ := dd.New(dd.DevelopmentConfig())

// 云原生 - JSON 格式、DEBUG 级别，兼容 ELK/Splunk/CloudWatch
logger, _ := dd.New(dd.JSONConfig())
```

### 日志文件轮转与压缩

```go
logger, _ := dd.NewWithOptions(dd.Options{
    File: "app.log",
    FileConfig: dd.FileWriterConfig{
        MaxSizeMB:  100,                 // 100MB 时轮转
        MaxBackups: 10,                  // 保留 10 个备份
        MaxAge:     30 * 24 * time.Hour, // 30 天后删除
        Compress:   true,                // 压缩旧文件 (.gz)
    },
})
```

**特性**：按大小自动轮转、按时间清理、自动压缩节省空间、线程安全、路径遍历防护


### 安全过滤

**默认禁用**以优化性能，需要时启用：

```go
// 基础过滤（推荐，性能影响最小）
config := dd.DefaultConfig().EnableBasicFiltering()
logger, _ := dd.New(config)

logger.Info("password=secret123")             // → password=[REDACTED]
logger.Info("api_key=sk-1234567890")          // → api_key=[REDACTED]
logger.Info("credit_card=4532015112830366")   // → credit_card=[REDACTED]
logger.Info("phone=+1-415-555-2671")          // → phone=[REDACTED]
logger.Info("mysql://user:pass@host:3306/db") // → mysql://[REDACTED]

// 或使用 Options
logger, _ := dd.NewWithOptions(dd.Options{
    FilterLevel: "basic", // "none", "basic", "full"
})
```

**基础过滤**（16+ 种模式）：
- 信用卡、SSN、密码、API 密钥、OpenAI 密钥、私钥
- 电话号码（10+ 国家/地区：美国、中国、英国、德国、日本等）
- 邮箱地址、数据库连接字符串

**完整过滤**（20+ 种模式）：
- 所有基础过滤模式，加上：
- JWT 令牌、AWS 密钥、Google API 密钥
- IPv4 地址
- 扩展数据库连接检测（MySQL、PostgreSQL、MongoDB、Redis、Oracle、SQL Server、JDBC 等）

**数据库连接过滤示例**：
```go
// 自动过滤 10+ 种数据库连接格式
logger.Info("mysql://user:pass@localhost:3306/db")
// → mysql://[REDACTED]

logger.Info("postgresql://admin:secret@db.example.com:5432/production")
// → postgresql://[REDACTED]

logger.Info("mongodb://admin:pass@host:27017/db")
// → mongodb://[REDACTED]

logger.Info("jdbc:mysql://localhost:3306/db?user=root&password=secret")
// → jdbc:mysql://[REDACTED]

logger.Info("Server=localhost;user id=sa;password=secret")
// → Server=[REDACTED]
```

**自定义过滤**：
```go
filter := dd.NewEmptySensitiveDataFilter()
filter.AddPattern(`(?i)internal[_-]?token[:\s=]+[^\s]+`)
filter.AddPattern(`...`)  // 添加多个模式

config := dd.DefaultConfig().WithFilter(filter)
```

**注入攻击防护**（始终启用）：
- 自动转义换行符和控制字符
- 消息大小限制（默认 5MB）
- 路径遍历防护


注入攻击防护可以根据需要进行配置：
```go
// 方法 1：创建配置时直接设置
config := dd.DefaultConfig()
config.SecurityConfig = &dd.SecurityConfig{
    MaxMessageSize:  10 * 1024 * 1024, // 自定义 10MB
    MaxWriters:      100,
    SensitiveFilter: nil,
}
logger, _ := dd.New(config)

// 方法 2：修改现有配置
config := dd.DefaultConfig()
config.SecurityConfig.MaxMessageSize = 10 * 1024 * 1024 // 自定义 10MB
logger, _ := dd.New(config)
```

**安全特性总结**：

| 特性                  | 默认值   | 说明                                |
|-----------------------|----------|-------------------------------------|
| 敏感数据过滤          | 禁用     | 需手动启用（性能考虑）              |
| 消息大小限制          | 5MB      | 防止内存溢出（默认 5MB）            |
| 换行符转义            | 启用     | 防止日志注入攻击                    |
| 控制字符过滤          | 启用     | 自动移除危险字符                    |
| 路径遍历防护          | 启用     | 写入文件时自动检查                  |
| Writer 数量限制       | 100      | 防止资源耗尽                        |
| 字段键验证            | 启用     | 自动清理非法字符                    |

### 性能基准

在 Intel Core Ultra 9 185H 上的实际测试数据：

| 操作类型                | 吞吐量            | 内存/操作 | 分配/操作 | 场景描述                |
|-------------------------|-------------------|-----------|-----------|-------------------------|
| 简单日志                | **3.1M 次/秒**    | 200 B     | 7 次分配  | 基础文本日志             |
| 格式化日志              | **2.4M 次/秒**    | 272 B     | 8 次分配  | Infof/Errorf            |
| 结构化日志              | **1.9M 次/秒**    | 417 B     | 12 次分配 | InfoWith + 3 个字段      |
| 复杂结构化              | **720K 次/秒**    | 1,227 B   | 26 次分配 | InfoWith + 8 个字段      |
| JSON 格式               | **600K 次/秒**    | 800 B     | 20 次分配 | JSON 结构化输出          |
| 并发 (22 个 goroutine)  | **68M 次/秒**     | 200 B     | 7 次分配  | 22 个 goroutine 并发     |
| 级别检查                | **2.5B 次/秒**    | 0 B       | 0 次分配  | 级别过滤（无输出）       |
| 字段创建                | **50M 次/秒**     | 16 B      | 1 次分配  | String/Int 字段构造      |

## 📚 API 快速参考

### 包级别函数

```go
// 使用全局默认 logger
dd.Debug / Info / Warn / Error / Fatal (args ...any)
dd.Debugf / Infof / Warnf / Errorf / Fatalf (format string, args ...any)
dd.DebugWith / InfoWith / WarnWith / ErrorWith / FatalWith (msg string, fields ...Field)

// 调试可视化（输出到 stdout 并包含调用者信息）
dd.JSON(data ...any)                    // 输出紧凑 JSON 到控制台并包含调用者信息
dd.JSONF(format string, args ...any)    // 输出格式化 JSON 到控制台并包含调用者信息
dd.Text(data ...any)                    // 输出美化文本到控制台（无调用者信息）
dd.Textf(format string, args ...any)    // 输出格式化文本到控制台（无调用者信息）
dd.Exit(data ...any)                    // 输出文本到控制台并包含调用者信息并退出程序 (os.Exit(0))
dd.Exitf(format string, args ...any)    // 输出格式化文本到控制台并包含调用者信息并退出程序

// 全局 logger 管理
dd.Default() *Logger
dd.SetDefault(logger *Logger)
```

### Logger 实例方法

```go
// Logger 实例
logger := dd.New()

// 简单日志
logger.Debug / Info / Warn / Error / Fatal (args ...any)

// 格式化日志
logger.Debugf / Infof / Warnf / Errorf / Fatalf (format string, args ...any)

// 结构化日志
logger.DebugWith / InfoWith / WarnWith / ErrorWith / FatalWith (msg string, fields ...Field)

// 调试可视化（输出到 stdout 并包含调用者信息）
logger.JSON(data ...any)                    // 输出紧凑 JSON 到控制台并包含调用者信息
logger.JSONF(format string, args ...any)    // 输出格式化 JSON 到控制台并包含调用者信息
logger.Text(data ...any)                    // 输出美化文本到控制台（无调用者信息）
logger.Textf(format string, args ...any)    // 输出格式化文本到控制台（无调用者信息）
logger.Exit(data ...any)                    // 输出文本到控制台并包含调用者信息并退出程序 (os.Exit(0))
logger.Exitf(format string, args ...any)    // 输出格式化文本到控制台并包含调用者信息并退出程序

// fmt 包替换方法（输出到 stdout 并包含调用者信息）
logger.Println(args ...any)                 // 默认格式输出并带换行和调用者信息
logger.Print(args ...any)                   // Println() 的便捷简写 - 行为相同
logger.Printf(format string, args ...any)   // 格式化输出到 stdout 并包含调用者信息

// 配置管理
logger.SetLevel(level LogLevel)
logger.GetLevel() LogLevel
logger.AddWriter(w io.Writer) error
logger.Close() error
```

### 便捷构造函数

> ⚠️ **注意**：`ToFile()`、`ToJSONFile()`、`ToConsole()` 和 `ToAll()` 构造函数在初始化失败时会 **panic**。对于需要错误处理的生产代码，请使用 `dd.NewWithOptions()` 代替。

```go
// 快速构造函数（出错时会 panic）
dd.ToFile(filename ...string) *Logger        // 仅文件（默认 logs/app.log）
dd.ToJSONFile(filename ...string) *Logger    // 仅 JSON 文件（默认 logs/app.log）
dd.ToConsole() *Logger                       // 仅控制台
dd.ToAll(filename ...string) *Logger         // 控制台 + 文件（默认 logs/app.log）

// 标准构造函数（返回错误）
dd.New(config *LoggerConfig) (*Logger, error)        // 使用配置对象
dd.NewWithOptions(opts Options) (*Logger, error)     // 使用 Options 模式

// 预设配置
dd.DefaultConfig() *LoggerConfig      // 生产配置（Info 级别，文本格式）
dd.DevelopmentConfig() *LoggerConfig  // 开发配置（Debug 级别，包含调用者信息）
dd.JSONConfig() *LoggerConfig         // JSON 配置（Debug 级别，云日志系统兼容）
```

### fmt 包替代方法

DD 提供了 Go 标准 `fmt` 包的完整替代，具有相似的 API 以及增强的日志集成：

```go
// 直接输出（stdout）- 所有方法都包含调用者信息
dd.Printf(format, args...)     // 格式化输出到 stdout 并包含调用者信息
dd.Println(args...)            // 默认格式输出并带换行和调用者信息
dd.Print(args...)              // Println() 的便捷简写 - 行为相同

// 字符串返回 - 与 fmt 相同
dd.Sprintf(format, args...)    // 返回格式化字符串
dd.Sprint(args...)             // 返回默认格式字符串
dd.Sprintln(args...)           // 返回默认格式字符串并带换行

// Writer 输出 - 与 fmt 相同
dd.Fprintf(w, format, args...) // 格式化输出到 writer
dd.Fprint(w, args...)          // 默认格式输出到 writer
dd.Fprintln(w, args...)        // 默认格式输出到 writer 并带换行

// 输入扫描 - 与 fmt 相同
dd.Scan(a...)                  // 从 stdin 读取空格分隔输入
dd.Scanf(format, a...)         // 从 stdin 读取格式化输入
dd.Scanln(a...)                // 从 stdin 读取基于行的输入
dd.Fscan(r, a...) / Fscanf / Fscanln    // 从 io.Reader 读取
dd.Sscan(str, a...) / Sscanf / Sscanln  // 从字符串读取

// 错误创建 - 增强命名
dd.NewError(format, args...)     // 创建错误（类似 fmt.Errorf）
dd.NewErrorWith(format, args...) // 创建错误并记录日志

// 缓冲区操作 - 与 fmt 相同
dd.AppendFormat(dst, format, args...) // 追加格式化到缓冲区
dd.Append(dst, args...)               // 追加默认格式到缓冲区
dd.Appendln(dst, args...)             // 追加到缓冲区并带换行

// 带日志集成的增强函数
dd.PrintfWith(format, args...) // 输出到 stdout 并记录日志
dd.PrintlnWith(args...)        // 输出到 stdout 并记录日志
```

> **💡 注意**：与 Go 的 fmt 包不同，在 dd 中，`Print()` 和 `Println()` 的行为完全相同——都在参数之间添加空格并追加换行符——使 `Print()` 成为简化使用的便捷别名，避免混淆。

### 字段构造函数

```go
dd.Any(key string, value any) Field          // 通用类型（推荐，支持任何类型）
dd.String(key, value string) Field           // 字符串
dd.Int(key string, value int) Field          // 整数
dd.Int64(key string, value int64) Field      // 64 位整数
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
        MaxSizeMB:  100,                 // 100MB 时轮转
        MaxBackups: 10,                  // 保留 10 个备份
        MaxAge:     30 * 24 * time.Hour, // 30 天后删除
        Compress:   true,                // 压缩旧文件 (.gz)
    },

    FullPath:      false,           // 显示完整路径（默认 false，仅文件名）
    DynamicCaller: true,            // 启用调用者检测和动态深度（自动适应包装器）
    TimeFormat:    time.RFC3339,    // 时间格式
    FilterLevel:   "basic",         // 敏感数据过滤："none", "basic", "full"

    JSONOptions: &dd.JSONOptions{
        PrettyPrint: false,              // 美化输出（对开发有用）
        Indent:      "  ",               // 缩进字符
        FieldNames: &dd.JSONFieldNames{  // 自定义字段名
            Timestamp: "timestamp",
            Level:     "level",
            Caller:    "caller",
            Message:   "message",
            Fields:    "fields",
        },
    },

    AdditionalWriters: []io.Writer{customWriter},  // 附加输出目标
})
```

### LoggerConfig 配置（高级）

```go
config := dd.DefaultConfig()
config.Level = dd.LevelDebug
config.Format = dd.FormatJSON
config.DynamicCaller = true
config.Writers = []io.Writer{os.Stdout, fileWriter}

// 链式配置
config.WithLevel(dd.LevelInfo).
       WithFormat(dd.FormatJSON).
       WithDynamicCaller(true).
       EnableBasicFiltering()

logger, err := dd.New(config)
```

### 日志级别

```go
dd.LevelDebug  // 调试信息（开发）
dd.LevelInfo   // 常规信息（默认，生产）
dd.LevelWarn   // 警告（需要注意但不影响运行）
dd.LevelError  // 错误（影响功能但不致命）
dd.LevelFatal  // 致命错误（调用 os.Exit(1) 终止程序）
```

**级别层次**：`Debug < Info < Warn < Error < Fatal`

**动态级别调整**：
```go
logger.SetLevel(dd.LevelDebug)  // 运行时调整
currentLevel := logger.GetLevel()
```

### 输出格式

**文本格式**（开发，可读）：
```
[2024-01-15T10:30:45+08:00  INFO] 应用已启动
[2024-01-15T10:30:46+08:00 ERROR] main.go:42 连接失败
```

**JSON 格式**（生产，可解析）：
```json
{"timestamp":"2025-01-15T10:30:45Z","level":"INFO","message":"应用已启动"}
{"timestamp":"2025-01-15T10:30:46Z","level":"ERROR","caller":"main.go:42","message":"连接失败"}
```

### 多输出目标

```go
// 方法 1：使用 Options
logger, _ := dd.NewWithOptions(dd.Options{
    Console: true,
    File:    "logs/app.log",
    AdditionalWriters: []io.Writer{
        customWriter,
        networkWriter,
    },
})

// 方法 2：动态添加
logger.AddWriter(newWriter)
logger.RemoveWriter(oldWriter)

// 方法 3：使用 MultiWriter
mw := dd.NewMultiWriter(writer1, writer2, writer3)
config := dd.DefaultConfig()
config.Writers = []io.Writer{mw}
logger, _ := dd.New(config)
```

### 缓冲写入（高性能场景）

```go
// 创建缓冲 writer（减少系统调用）
fileWriter, _ := dd.NewFileWriter("app.log", nil)
bufferedWriter, _ := dd.NewBufferedWriter(fileWriter, 4096)  // 4KB 缓冲
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
dd.Info("使用全局 logger")
dd.InfoWith("结构化", dd.String("key", "value"))

// 获取当前默认 logger
logger := dd.Default()
```

## 高级功能

### 动态调用者检测

自动检测调用栈深度，适应各种包装器场景：

```go
config := dd.DevelopmentConfig()
config.DynamicCaller = true  // 启用动态检测
logger, _ := dd.New(config)

// 即使通过多层包装，也显示真实调用者位置
func MyLogWrapper(msg string) {
    logger.Info(msg)  // 显示 MyLogWrapper 的调用者，而非此行
}
```

### JSON 字段名自定义

适应不同日志系统的字段命名约定：

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

// 输出：{"time":"...","severity":"INFO","msg":"test","data":{...}}
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

logger.Fatal("严重错误")  // 调用自定义处理器
```

### 安全配置

细粒度控制安全限制：

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
devConfig.DynamicCaller = true

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
    Console:     false,  // 生产环境不输出到控制台
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
    DynamicCaller: true,
    TimeFormat:    "15:04:05.000",
})
defer logger.Close()
```

### 3. 结构化日志最佳实践

```go
// ✅ 推荐：使用类型安全字段
logger.InfoWith("用户登录",
    dd.String("user_id", userID),
    dd.String("ip", clientIP),
    dd.Int("attempt", attemptCount),
)

// ❌ 不推荐：字符串拼接
logger.Info(fmt.Sprintf("用户 %s 从 %s 登录", userID, clientIP))
```

### 示例代码

完整示例代码请参见 [examples](examples) 目录。

## 🤝 贡献

欢迎贡献、问题报告和建议！

## 📄 许可证

MIT 许可证 - 详见 [LICENSE](LICENSE) 文件

---

**用心为 Go 社区打造** ❤️ | 如果这个项目对您有帮助，请给它一个 ⭐️ Star！
