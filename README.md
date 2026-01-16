# DD - High-Performance Go Logging Library

[![Go Version](https://img.shields.io/badge/Go-1.24+-00ADD8?style=flat&logo=go)](https://go.dev/)
[![pkg.go.dev](https://pkg.go.dev/badge/github.com/cybergodev/dd.svg)](https://pkg.go.dev/github.com/cybergodev/dd)
[![License](https://img.shields.io/badge/license-MIT-brightgreen.svg)](LICENSE)
[![Security](https://img.shields.io/badge/security-policy-blue.svg)](SECURITY.md)
[![Thread Safe](https://img.shields.io/badge/thread%20safe-yes-brightgreen.svg)](https://github.com/cybergodev/json)

A production-grade high-performance Go logging library with zero external dependencies, designed for modern applications.

#### **[📖 中文文档](README_zh-CN.md)** - User guide

## ✨ Core Features

- 🚀 **Extreme Performance** - 3M+ ops/sec simple logging, 1M+ ops/sec structured logging, optimized for high-throughput systems
- 🔒 **Thread-Safe** - Atomic operations + lock-free design, fully concurrent-safe
- 🛡️ **Built-in Security** - Sensitive data filtering (including database, passwords, API keys...), injection attack prevention
- 📊 **Structured Logging** - Type-safe fields, supports JSON/text dual formats, customizable field names
- 📁 **Smart Rotation** - Auto-rotate by size/time, auto-compress to .gz, auto-cleanup expired files
- 📦 **Zero Dependencies** - Only Go standard library, no third-party dependencies
- 🎯 **Easy to Use** - Get started in 2 minutes, intuitive API, 4 convenient constructors
- 🔧 **Flexible Configuration** - 3 preset configs + Options pattern, supports multiple outputs, buffered writes
- 🌐 **Cloud-Native Friendly** - JSON format compatible with ELK/Splunk/CloudWatch and other log systems
- ⚡ **Performance Optimized** - Object pool reuse, pre-allocated buffers, lazy formatting, dynamic caller detection

## 📦 Installation

```bash
go get github.com/cybergodev/dd
```

## 🚀 Quick Start

### Get Started in 30 Seconds

```go
package main

import "github.com/cybergodev/dd"

func main() {
    // Method 1: Use global default logger (simplest)
    dd.Info("Application started")
    dd.Warn("Cache miss for key user:123")
    dd.Error("Failed to connect to database")
    
    // Method 2: Create custom logger (recommended)
    logger := dd.ToFile()  // Output to logs/app.log
    defer logger.Close()

    logger.Info("Application started")
    logger.InfoWith("User login",
        dd.Int("id", 12345),
        dd.String("type", "vip"),
        dd.Any("usernames", []string{"alice", "bob"}),
    )
}
```

### Simplest Way (Console Output)

```go
import "github.com/cybergodev/dd"

func main() {
    dd.Debug("Debug message")
    dd.Info("Application started")
    dd.Warn("Cache miss for key user:123")
    dd.Error("Failed to connect to database")
    dd.Fatal("Application exiting")  // Exits program (calls os.Exit(1))
    
    // After dd.Fatal(), the following code will not execute
    fmt.Println("Hello, World!")
}
```

### File Logging (One Line of Code)

```go
logger := dd.ToFile()              // → File only: logs/app.log
logger := dd.ToJSONFile()          // → JSON format file only: logs/app.log
logger := dd.ToAll()               // → Console + logs/app.log
logger := dd.ToConsole()           // → Console only
defer logger.Close()

logger.Info("Logging to file")

// Custom filename
logger := dd.ToFile("logs/myapp.log")
defer logger.Close()
```

### Structured Logging (Production)

```go
// Log to file
logger := dd.ToJSONFile()
defer logger.Close()

logger.InfoWith("HTTP Request",
    dd.String("method", "POST"),
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

**JSON Output**:
```json
{"timestamp":"2024-01-15T10:30:45Z","level":"INFO","message":"HTTP Request","fields":{"method":"POST","path":"/api/users","status":201,"duration_ms":45.67}}
```

### Custom Configuration

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

## 📖 Core Features

### Preset Configurations

Three preset configurations for quick adaptation to different scenarios:

```go
// Production - Balance performance and features
logger, _ := dd.New(dd.DefaultConfig())

// Development - DEBUG level + caller information
logger, _ := dd.New(dd.DevelopmentConfig())

// Cloud-Native - JSON format, DEBUG level, compatible with ELK/Splunk/CloudWatch
logger, _ := dd.New(dd.JSONConfig())
```

### Log File Rotation & Compression

```go
logger, _ := dd.NewWithOptions(dd.Options{
    File: "app.log",
    FileConfig: dd.FileWriterConfig{
        MaxSizeMB:  100,                 // Rotate at 100MB
        MaxBackups: 10,                  // Keep 10 backups
        MaxAge:     30 * 24 * time.Hour, // Delete after 30 days
        Compress:   true,                // Compress old files (.gz)
    },
})
```

**Features**: Auto-rotate by size, cleanup by time, auto-compress to save space, thread-safe, path traversal protection


### Security Filtering

**Disabled by default** for performance, enable when needed:

```go
// Basic filtering (recommended, minimal performance impact)
config := dd.DefaultConfig().EnableBasicFiltering()
logger, _ := dd.New(config)

logger.Info("password=secret123")             // → password=[REDACTED]
logger.Info("api_key=sk-1234567890")          // → api_key=[REDACTED]
logger.Info("credit_card=4532015112830366")   // → credit_card=[REDACTED]
logger.Info("phone=+1-415-555-2671")          // → phone=[REDACTED]
logger.Info("mysql://user:pass@host:3306/db") // → mysql://[REDACTED]

// Or use Options
logger, _ := dd.NewWithOptions(dd.Options{
    FilterLevel: "basic", // "none", "basic", "full"
})
```

**Basic Filtering** (16+ patterns):
- Credit cards, SSN, passwords, API keys, OpenAI keys, private keys
- Phone numbers (10+ countries: US, China, UK, Germany, Japan, etc.)
- Email addresses, database connection strings

**Full Filtering** (20+ patterns):
- All Basic patterns plus:
- JWT tokens, AWS keys, Google API keys
- IPv4 addresses
- Extended database connection detection (MySQL, PostgreSQL, MongoDB, Redis, Oracle, SQL Server, JDBC, etc.)

**Database Connection Filtering Examples**:
```go
// Automatically filters 10+ database connection formats
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

**Custom Filtering**:
```go
filter := dd.NewEmptySensitiveDataFilter()
filter.AddPattern(`(?i)internal[_-]?token[:\s=]+[^\s]+`)
filter.AddPattern(`...`)  // Add multiple patterns

config := dd.DefaultConfig().WithFilter(filter)
```

**Injection Attack Prevention** (always enabled):
- Auto-escape newlines and control characters
- Message size limit (default 5MB)
- Path traversal protection


Injection attack prevention can be configured as needed:
```go
// Method 1: Set directly when creating config
config := dd.DefaultConfig()
config.SecurityConfig = &dd.SecurityConfig{
    MaxMessageSize:  10 * 1024 * 1024, // Custom 10MB
    MaxWriters:      100,
    SensitiveFilter: nil,
}
logger, _ := dd.New(config)

// Method 2: Modify existing config
config := dd.DefaultConfig()
config.SecurityConfig.MaxMessageSize = 10 * 1024 * 1024 // Custom 10MB
logger, _ := dd.New(config)
```

**Security Features Summary**:

| Feature                   | Default  | Description                           |
|---------------------------|----------|---------------------------------------|
| Sensitive Data Filtering  | Disabled | Must enable manually (performance)    |
| Message Size Limit        | 5MB      | Prevent memory overflow (default 5MB) |
| Newline Escaping          | Enabled  | Prevent log injection attacks         |
| Control Character Filter  | Enabled  | Auto-remove dangerous characters      |
| Path Traversal Protection | Enabled  | Auto-check on file writes             |
| Writer Count Limit        | 100      | Prevent resource exhaustion           |
| Field Key Validation      | Enabled  | Auto-clean illegal characters         |

### Performance Benchmarks

Real-world data on Intel Core Ultra 9 185H:

| Operation Type             | Throughput       | Memory/Op | Allocs/Op | Scenario Description          |
|----------------------------|------------------|-----------|-----------|-------------------------------|
| Simple Logging             | **3.1M ops/sec** | 200 B     | 7 allocs  | Basic text logging            |
| Formatted Logging          | **2.4M ops/sec** | 272 B     | 8 allocs  | Infof/Errorf                  |
| Structured Logging         | **1.9M ops/sec** | 417 B     | 12 allocs | InfoWith + 3 fields           |
| Complex Structured         | **720K ops/sec** | 1,227 B   | 26 allocs | InfoWith + 8 fields           |
| JSON Format                | **600K ops/sec** | 800 B     | 20 allocs | JSON structured output        |
| Concurrent (22 goroutines) | **68M ops/sec**  | 200 B     | 7 allocs  | 22 goroutines concurrent      |
| Level Check                | **2.5B ops/sec** | 0 B       | 0 allocs  | Level filtering (no output)   |
| Field Creation             | **50M ops/sec**  | 16 B      | 1 allocs  | String/Int field construction |

## 📚 API Quick Reference

### Package-Level Functions

```go
// Use global default logger
dd.Debug / Info / Warn / Error / Fatal (args ...any)
dd.Debugf / Infof / Warnf / Errorf / Fatalf (format string, args ...any)
dd.DebugWith / InfoWith / WarnWith / ErrorWith / FatalWith (msg string, fields ...Field)

// Debug visualization (output to stdout with caller info, not to log writers)
dd.Json(data ...any)                    // Output compact JSON to console
dd.Jsonf(format string, args ...any)    // Output formatted JSON to console
dd.Text(data ...any)                    // Output pretty-printed text to console
dd.Textf(format string, args ...any)    // Output formatted text to console
dd.Exit(data ...any)                    // Output text to console and exit program (os.Exit(0))
dd.Exitf(format string, args ...any)    // Output formatted text to console and exit program

// Global logger management
dd.Default() *Logger
dd.SetDefault(logger *Logger)
```

### Logging Instance Methods

```go
// Logging Instance
logger := dd.New()

// Simple logging
logger.Debug / Info / Warn / Error / Fatal (args ...any)

// Formatted logging
logger.Debugf / Infof / Warnf / Errorf / Fatalf (format string, args ...any)

// Structured logging
logger.DebugWith / InfoWith / WarnWith / ErrorWith / FatalWith (msg string, fields ...Field)

// Debug visualization (output to stdout with caller info, not to log writers)
logger.Json(data ...any)                    // Output compact JSON to console
logger.Jsonf(format string, args ...any)    // Output formatted JSON to console
logger.Text(data ...any)                    // Output pretty-printed text to console
logger.Textf(format string, args ...any)    // Output formatted text to console
logger.Exit(data ...any)                    // Output text to console and exit program (os.Exit(0))
logger.Exitf(format string, args ...any)    // Output formatted text to console and exit program

// fmt package replacement methods (output to stdout with caller info)
logger.Println(args ...any)                 // Default format output with newline and caller info
logger.Print(args ...any)                   // Convenient shorthand for Println() - identical behavior
logger.Printf(format string, args ...any)   // Formatted output to stdout with caller info

// Configuration management
logger.SetLevel(level LogLevel)
logger.GetLevel() LogLevel
logger.AddWriter(w io.Writer) error
logger.Close() error
```

### Convenience Constructors

```go
// Quick constructors (create with one line)
dd.ToFile(filename ...string) *Logger        // File only (default logs/app.log)
dd.ToJSONFile(filename ...string) *Logger    // JSON file only (default logs/app.log)
dd.ToConsole() *Logger                       // Console only
dd.ToAll(filename ...string) *Logger         // Console + file (default logs/app.log)

// Standard constructors
dd.New(config *LoggerConfig) (*Logger, error)        // Use config object
dd.NewWithOptions(opts Options) (*Logger, error)     // Use Options pattern

// Preset configurations
dd.DefaultConfig() *LoggerConfig      // Production config (Info level, text format)
dd.DevelopmentConfig() *LoggerConfig  // Development config (Debug level, with caller info)
dd.JSONConfig() *LoggerConfig         // JSON config (Debug level, cloud log system compatible)
```

### fmt Package alternative method

DD provides a complete replacement for Go's standard `fmt` package with similar APIs plus enhanced logging integration:

```go
// Direct Output (stdout) - all methods include caller information
dd.Printf(format, args...)     // Formatted output to stdout with caller info
dd.Println(args...)            // Default format output with newline and caller info
dd.Print(args...)              // Convenient shorthand for Println() - identical behavior

// String Return - identical to fmt
dd.Sprintf(format, args...)    // Return formatted string
dd.Sprint(args...)             // Return default format string
dd.Sprintln(args...)           // Return default format string with newline

// Writer Output - identical to fmt
dd.Fprintf(w, format, args...) // Formatted output to writer
dd.Fprint(w, args...)          // Default format output to writer
dd.Fprintln(w, args...)        // Default format output with newline to writer

// Input Scanning - identical to fmt
dd.Scan(a...)                  // Space-separated input from stdin
dd.Scanf(format, a...)         // Formatted input from stdin
dd.Scanln(a...)                // Line-based input from stdin
dd.Fscan(r, a...) / Fscanf / Fscanln    // Input from io.Reader
dd.Sscan(str, a...) / Sscanf / Sscanln  // Input from string

// Error Creation - enhanced naming
dd.NewError(format, args...)     // Create error (like fmt.Errorf)
dd.NewErrorWith(format, args...) // Create error AND log it

// Buffer Operations - identical to fmt
dd.AppendFormat(dst, format, args...) // Append formatted to buffer
dd.Append(dst, args...)               // Append default format to buffer
dd.Appendln(dst, args...)             // Append with newline to buffer

// Enhanced Functions with Logging Integration
dd.PrintfWith(format, args...) // Output to stdout AND log message
dd.PrintlnWith(args...)        // Output to stdout AND log message
```

> **💡 Note:** Unlike Go’s fmt package, in dd both `Print()` and `Println()` behave identically—adding spaces between arguments and appending a newline—making `Print()` just a convenient alias that simplifies usage and avoids confusion.

### Field Constructors

```go
dd.Any(key string, value any) Field          // Generic type (recommended, supports any type)
dd.String(key, value string) Field           // String
dd.Int(key string, value int) Field          // Integer
dd.Int64(key string, value int64) Field      // 64-bit integer
dd.Float64(key string, value float64) Field  // Float
dd.Bool(key string, value bool) Field        // Boolean
dd.Err(err error) Field                      // Error (auto-extracts error.Error())
```

## 🔧 Configuration Guide

### Options Configuration (Recommended)

```go
logger, err := dd.NewWithOptions(dd.Options{
    Level:   dd.LevelInfo,    // Log level
    Format:  dd.FormatJSON,   // Output format (FormatText/FormatJSON)
    Console: true,            // Console output
    File:    "logs/app.log",  // File path

    FileConfig: dd.FileWriterConfig{
        MaxSizeMB:  100,                 // Rotate at 100MB
        MaxBackups: 10,                  // Keep 10 backups
        MaxAge:     30 * 24 * time.Hour, // Delete after 30 days
        Compress:   true,                // Compress old files (.gz)
    },

    FullPath:      false,           // Show full path (default false, filename only)
    DynamicCaller: true,            // Enable caller detection with dynamic depth (auto-adapt wrappers)
    TimeFormat:    time.RFC3339,    // Time format
    FilterLevel:   "basic",         // Sensitive data filtering: "none", "basic", "full"
    
    JSONOptions: &dd.JSONOptions{
        PrettyPrint: false,              // Pretty print (useful for development)
        Indent:      "  ",               // Indent characters
        FieldNames: &dd.JSONFieldNames{  // Custom field names
            Timestamp: "timestamp",
            Level:     "level",
            Caller:    "caller",
            Message:   "message",
            Fields:    "fields",
        },
    },
    
    AdditionalWriters: []io.Writer{customWriter},  // Additional output targets
})
```

### LoggerConfig Configuration (Advanced)

```go
config := dd.DefaultConfig()
config.Level = dd.LevelDebug
config.Format = dd.FormatJSON
config.DynamicCaller = true
config.Writers = []io.Writer{os.Stdout, fileWriter}

// Chained configuration
config.WithLevel(dd.LevelInfo).
       WithFormat(dd.FormatJSON).
       WithDynamicCaller(true).
       EnableBasicFiltering()

logger, err := dd.New(config)
```

### Log Levels

```go
dd.LevelDebug  // Debug information (development)
dd.LevelInfo   // Regular information (default, production)
dd.LevelWarn   // Warning (needs attention but doesn't affect operation)
dd.LevelError  // Error (affects functionality but not fatal)
dd.LevelFatal  // Fatal error (calls os.Exit(1) to terminate program)
```

**Level Hierarchy**: `Debug < Info < Warn < Error < Fatal`

**Dynamic Level Adjustment**:
```go
logger.SetLevel(dd.LevelDebug)  // Adjust at runtime
currentLevel := logger.GetLevel()
```

### Output Formats

**Text Format** (development, readable):
```
[2024-01-15T10:30:45+08:00  INFO] Application started
[2024-01-15T10:30:46+08:00 ERROR] main.go:42 Connection failed
```

**JSON Format** (production, parseable):
```json
{"timestamp":"2025-01-15T10:30:45Z","level":"INFO","message":"Application started"}
{"timestamp":"2025-01-15T10:30:46Z","level":"ERROR","caller":"main.go:42","message":"Connection failed"}
```

### Multiple Output Targets

```go
// Method 1: Use Options
logger, _ := dd.NewWithOptions(dd.Options{
    Console: true,
    File:    "logs/app.log",
    AdditionalWriters: []io.Writer{
        customWriter,
        networkWriter,
    },
})

// Method 2: Add dynamically
logger.AddWriter(newWriter)
logger.RemoveWriter(oldWriter)

// Method 3: Use MultiWriter
mw := dd.NewMultiWriter(writer1, writer2, writer3)
config := dd.DefaultConfig()
config.Writers = []io.Writer{mw}
logger, _ := dd.New(config)
```

### Buffered Writes (High-Performance Scenarios)

```go
// Create buffered writer (reduce system calls)
fileWriter, _ := dd.NewFileWriter("app.log", nil)
bufferedWriter, _ := dd.NewBufferedWriter(fileWriter, 4096)  // 4KB buffer
defer bufferedWriter.Close()

config := dd.DefaultConfig()
config.Writers = []io.Writer{bufferedWriter}
logger, _ := dd.New(config)
```

### Global Default Logger

```go
// Set global default logger
customLogger, _ := dd.NewWithOptions(dd.Options{
    Level:  dd.LevelDebug,
    Format: dd.FormatJSON,
})
dd.SetDefault(customLogger)

// Use global logger
dd.Info("Using global logger")
dd.InfoWith("Structured", dd.String("key", "value"))

// Get current default logger
logger := dd.Default()
```

## Advanced Features

### Dynamic Caller Detection

Auto-detect call stack depth, adapts to various wrapper scenarios:

```go
config := dd.DevelopmentConfig()
config.DynamicCaller = true  // Enable dynamic detection
logger, _ := dd.New(config)

// Even through multiple wrapper layers, shows real caller location
func MyLogWrapper(msg string) {
    logger.Info(msg)  // Shows caller of MyLogWrapper, not this line
}
```

### JSON Field Name Customization

Adapt to different log system field naming conventions:

```go
logger, _ := dd.NewWithOptions(dd.Options{
    Format: dd.FormatJSON,
    JSONOptions: &dd.JSONOptions{
        FieldNames: &dd.JSONFieldNames{
            Timestamp: "time",      // Default "timestamp"
            Level:     "severity",  // Default "level"
            Caller:    "source",    // Default "caller"
            Message:   "msg",       // Default "message"
            Fields:    "data",      // Default "fields"
        },
    },
})

// Output: {"time":"...","severity":"INFO","msg":"test","data":{...}}
```

### Custom Fatal Handler

Control Fatal level log behavior:

```go
config := dd.DefaultConfig()
config.FatalHandler = func() {
    // Custom cleanup logic
    cleanup()
    os.Exit(2)  // Custom exit code
}
logger, _ := dd.New(config)

logger.Fatal("Critical error")  // Calls custom handler
```

### Security Configuration

Fine-grained control of security limits:

```go
config := dd.DefaultConfig()
config.SecurityConfig = &dd.SecurityConfig{
    MaxMessageSize:  10 * 1024 * 1024,      // 10MB message limit
    MaxWriters:      50,                    // Max 50 output targets
    SensitiveFilter: dd.NewBasicSensitiveDataFilter(),
}
logger, _ := dd.New(config)

// Adjust at runtime
logger.SetSecurityConfig(&dd.SecurityConfig{
    MaxMessageSize: 5 * 1024 * 1024,
})
```

### Custom Sensitive Data Filtering

```go
// Create empty filter, add custom rules
filter := dd.NewEmptySensitiveDataFilter()
filter.AddPattern(`(?i)internal[_-]?token[:\s=]+[^\s]+`)
filter.AddPattern(`\bSECRET_[A-Z0-9_]+\b`)

// Or batch add
patterns := []string{
    `custom_pattern_1`,
    `custom_pattern_2`,
}
filter.AddPatterns(patterns...)

// Dynamically enable/disable
filter.Enable()
filter.Disable()
if filter.IsEnabled() {
    // ...
}

// Use custom filter
config := dd.DefaultConfig()
config.SecurityConfig.SensitiveFilter = filter
logger, _ := dd.New(config)
```

### Clone Configuration

Safely copy configuration objects:

```go
baseConfig := dd.DefaultConfig()
baseConfig.Level = dd.LevelInfo
baseConfig.EnableBasicFiltering()

// Clone and modify
devConfig := baseConfig.Clone()
devConfig.Level = dd.LevelDebug
devConfig.DynamicCaller = true

logger1, _ := dd.New(baseConfig)  // Production config
logger2, _ := dd.New(devConfig)   // Development config
```

## 📚 Best Practices

### 1. Production Configuration

```go
logger, _ := dd.NewWithOptions(dd.Options{
    Level:       dd.LevelInfo,
    Format:      dd.FormatJSON,
    File:        "logs/app.log",
    Console:     false,  // No console output in production
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

### 2. Development Configuration

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

### 3. Structured Logging Best Practices

```go
// ✅ Recommended: Use type-safe fields
logger.InfoWith("User login",
    dd.String("user_id", userID),
    dd.String("ip", clientIP),
    dd.Int("attempt", attemptCount),
)

// ❌ Not recommended: String concatenation
logger.Info(fmt.Sprintf("User %s login from %s", userID, clientIP))
```

### Example Code

See the [examples](examples) directory for complete example code.

## 🤝 Contributing

Contributions, issue reports, and suggestions are welcome!

## 📄 License

MIT License - See [LICENSE](LICENSE) file for details.

---

**Crafted with care for the Go community** ❤️ | If this project helps you, please give it a ⭐️ Star!
