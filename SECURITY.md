# Security Policy

## ⚠️ IMPORTANT: Security Configuration Warning

**By default, sensitive data filtering is DISABLED for maximum performance.**

This means passwords, API keys, credit card numbers, and other sensitive data may appear in your logs unless you explicitly enable security filtering.

### How to Enable Security Filtering

```go
// Option 1: Basic filtering (recommended for most production systems)
cfg := dd.DefaultConfig()
cfg.Security = dd.DefaultSecurityConfig()
logger, _ := dd.New(cfg)

// Option 2: Full filtering (maximum security for sensitive environments)
cfg := dd.DefaultConfig()
cfg.Security = dd.DefaultSecureConfig()
logger, _ := dd.New(cfg)

// Option 3: Industry-specific presets
cfg := dd.DefaultConfig()
cfg.Security = dd.HealthcareConfig()   // HIPAA compliance
// OR
cfg.Security = dd.FinancialConfig()    // PCI-DSS compliance
// OR
cfg.Security = dd.GovernmentConfig()   // Government/defense systems
logger, _ := dd.New(cfg)
```

### Risk Assessment

| Configuration | Risk Level | Use Case |
|---------------|------------|----------|
| No filtering (default) | **HIGH** | Development only |
| `DefaultSecurityConfig()` | **MEDIUM** | General production |
| `DefaultSecureConfig()` | **LOW** | High-security environments |
| Industry presets | **LOW** | Compliance requirements |

**Never use the default (no filtering) configuration in production systems that handle sensitive data.**

---

## Overview

The DD logging library is designed with security as a core principle. This document outlines the security features, best practices, and vulnerability reporting procedures for the DD library.

## Security Features

### 1. Sensitive Data Filtering

DD provides built-in protection against accidental logging of sensitive information through configurable pattern-based filtering.

#### Default Behavior

**Sensitive data filtering is DISABLED by default** to maximize performance. Users must explicitly enable filtering when handling sensitive data.

#### Filtering Levels

**Basic Filtering** (recommended for most use cases):
- Credit card numbers (13-19 digits)
- Social Security Numbers (SSN format: XXX-XX-XXXX)
- Password fields (password/passwd/pwd with values)
- API keys and tokens
- OpenAI API keys (sk-* format)
- Private key headers (PEM format)
- Phone numbers (multiple formats)
- Database connection strings

**Full Filtering** (comprehensive protection):
- All basic patterns plus:
- JWT tokens (eyJ* format)
- AWS access keys (AKIA* format)
- Google API keys (AIza* format)
- Email addresses
- IPv4 and IPv6 addresses
- JDBC connection strings
- Server/data source patterns

**Enterprise Patterns** (available in full filtering and industry presets):
- **Financial Services**: SWIFT/BIC codes, IBAN, CVV/CVC codes
- **Healthcare**: ICD-10 codes, NPI, MRN, HICN
- **Government**: Passport numbers, Driver's License, Tax ID/EIN, UK NI, Canadian SIN
- **Cloud Providers**: GitHub tokens, Slack tokens, Stripe keys, GCP service accounts, Azure connection strings

#### Usage Examples

```go
// Enable basic filtering (recommended)
config := dd.DefaultConfig().EnableBasicFiltering()
logger, _ := dd.New(config)

// Enable full filtering
config := dd.DefaultConfig().EnableFullFiltering()
logger, _ := dd.New(config)

// Using Options pattern
logger, _ := dd.NewWithOptions(dd.Options{
    FilterLevel: "basic", // "none", "basic", "full"
})

// Custom filtering patterns
filter := dd.NewEmptySensitiveDataFilter()
filter.AddPattern(`(?i)internal[_-]?token[:\s=]+[^\s]+`)
filter.AddPattern(`\bSECRET_[A-Z0-9_]+\b`)
config := dd.DefaultConfig().WithFilter(filter)
logger, _ := dd.New(config)
```

#### Field-Level Filtering

The library automatically redacts values for fields with sensitive key names:

```go
logger.InfoWith("User data",
    dd.String("password", "secret123"),      // → password=[REDACTED]
    dd.String("api_key", "sk-1234567890"),   // → api_key=[REDACTED]
    dd.String("token", "abc123xyz"),         // → token=[REDACTED]
    dd.String("username", "john_doe"),       // → username=john_doe (not filtered)
)
```

Sensitive keywords detected:
- password, passwd, pwd
- secret, token
- api_key, apikey, api-key
- access_key, accesskey, access-key
- secret_key, secretkey, secret-key
- private_key, privatekey, private-key
- auth, authorization
- credit_card, creditcard
- ssn, social_security

### 2. Injection Attack Prevention

DD automatically protects against log injection attacks through message sanitization.

#### Always-Enabled Protections

**Newline Escaping**: Prevents log injection by escaping newline characters
```go
logger.Info("User input: \nmalicious\nlog\nentry")
// Output: User input: \nmalicious\nlog\nentry
```

**Control Character Filtering**: Removes dangerous control characters (except tab)
- Filters characters < 32 (except \t)
- Filters character 127 (DEL)
- Preserves UTF-8 characters (≥ 128)

**ANSI Escape Sequence Removal**: Strips all ANSI escape sequences
- CSI (Control Sequence Introducer): Colors, cursor movement
- OSC (Operating System Command): Window titles, hyperlinks
- DCS (Device Control String): Device control data
- APC (Application Program Command): Application-specific data
- PM (Privacy Message): Privacy messages
- SOS (Start of String): String delimiters

**Unicode Control Character Removal**: Removes invisible Unicode characters that can be used for log injection or obfuscation
- Zero Width Space (ZWSP, U+200B)
- Zero Width Non-Joiner (ZWNJ, U+200C)
- Zero Width Joiner (ZWJ, U+200D)
- Left-to-Right/Right-to-Left Marks (U+200E, U+200F)
- Line/Paragraph Separators (U+2028, U+2029)
- Bidirectional Formatting (U+202A-U+202E)
- Byte Order Mark (BOM, U+FEFF)

**Message Size Limiting**: Prevents memory exhaustion attacks
```go
config := dd.DefaultConfig()
config.SecurityConfig = &dd.SecurityConfig{
    MaxMessageSize: 5 * 1024 * 1024, // Default: 5MB
}
```

Messages exceeding the limit are truncated with `... [TRUNCATED]` suffix.

### 3. ReDoS (Regular Expression Denial of Service) Protection

The sensitive data filter includes multiple layers of protection against ReDoS attacks:

#### Timeout Protection

Each regex operation has a configurable timeout (default: 50ms):
```go
filter := dd.NewSensitiveDataFilter()
// Timeout is automatically applied to prevent hanging
result := filter.Filter(potentiallyMaliciousInput)
```

If a regex operation exceeds the timeout, it returns `[REDACTED - REGEX TIMEOUT]`.

#### Input Length Limiting

Filters enforce maximum input lengths to prevent catastrophic backtracking:
- Default filter: 256KB max input
- Basic filter: 64KB max input
- Empty filter: 1MB max input

Inputs exceeding limits are truncated with `... [TRUNCATED FOR SECURITY]` suffix.

#### Panic Recovery

The filter includes panic recovery to handle regex engine crashes:
```go
// If regex panics, returns: [REDACTED - REGEX ERROR: <error>]
```

### 4. Resource Exhaustion Protection

#### Writer Count Limiting

Prevents resource exhaustion by limiting the number of output writers:
```go
config := dd.DefaultConfig()
config.SecurityConfig = &dd.SecurityConfig{
    MaxWriters: 100, // Default: 100
}
logger, _ := dd.New(config)

// Attempting to exceed limit returns error
err := logger.AddWriter(newWriter) // Returns error if limit exceeded
```

#### Field Key Validation

Automatically sanitizes field keys to prevent injection:
- Maximum key length: 256 characters
- Allowed characters: a-z, A-Z, 0-9, _, -, .
- Invalid keys replaced with `invalid_key`

```go
logger.InfoWith("Test",
    dd.String("valid_key-123", "value"),     // → valid_key-123=value
    dd.String("invalid key!", "value"),      // → invalidkey=value
    dd.String("", "value"),                  // → invalid_key=value
)
```

### 5. Path Traversal Protection

File writers automatically validate paths to prevent directory traversal attacks:
```go
// Safe: Creates file in logs directory
fileWriter, _ := dd.NewFileWriter("logs/app.log", nil)

// Protected: Path traversal attempts are blocked
fileWriter, _ := dd.NewFileWriter("../../../etc/passwd", nil) // Returns error
```

#### Symlink and Hardlink Protection

After opening a log file, the library validates the file handle to detect and prevent symlink and hardlink attacks:

- **Symlink Detection**: Files that are symbolic links are rejected to prevent attackers from redirecting log output to sensitive files.
- **Hardlink Detection**: Files with multiple hard links are rejected to prevent attackers from accessing log content through alternative paths.
- **TOCTOU Prevention**: Validation is performed on the opened file handle (not the path) to prevent time-of-check-time-of-use vulnerabilities.

#### UTF-8 Overlong Encoding Detection

The library detects UTF-8 overlong encoding attacks, which can be used to bypass path traversal checks:
```go
// These attacks are blocked:
// - 0xC0 0xAE represents '.' (overlong encoding)
// - 0xC0 0xAF represents '/' (overlong encoding)
// - 0xE0 0x80 0xAF represents '/' (3-byte overlong)
```

### 6. Thread Safety

All public methods are fully concurrent-safe:
- Atomic operations for hot paths (level checks, state management)
- RWMutex for writer management (infrequent operations)
- Lock-free design for logging operations
- Immutable configuration after logger creation

```go
// Safe for concurrent use
var wg sync.WaitGroup
for i := 0; i < 100; i++ {
    wg.Add(1)
    go func() {
        defer wg.Done()
        logger.Info("Concurrent logging")
    }()
}
wg.Wait()
```

## Security Best Practices

### 1. Enable Filtering for Sensitive Data

Always enable filtering when logging user input or potentially sensitive data:

```go
// Production configuration with security
logger, _ := dd.NewWithOptions(dd.Options{
    Level:       dd.LevelInfo,
    Format:      dd.FormatJSON,
    FilterLevel: "basic",
    File:        "logs/app.log",
    FileConfig: dd.FileWriterConfig{
        MaxSizeMB:  100,
        MaxBackups: 30,
        Compress:   true,
    },
})
defer logger.Close()
```

### 2. Validate User Input Before Logging

Never log raw user input without validation:

```go
// ❌ Bad: Direct logging of user input
logger.Info(userInput)

// ✅ Good: Validate and use structured logging
if len(userInput) > 1000 {
    userInput = userInput[:1000]
}
logger.InfoWith("User action",
    dd.String("action", sanitize(userInput)),
    dd.String("user_id", userID),
)
```

### 3. Use Structured Logging for Sensitive Fields

Use structured logging with field-level filtering:

```go
// ✅ Recommended: Field-level filtering
logger.InfoWith("Authentication",
    dd.String("username", username),
    dd.String("password", password), // Automatically redacted
    dd.String("ip", clientIP),
)

// ❌ Not recommended: String concatenation
logger.Info(fmt.Sprintf("Auth: %s:%s from %s", username, password, clientIP))
```

### 4. Configure Appropriate Message Size Limits

Set message size limits based on your application's needs:

```go
config := dd.DefaultConfig()
config.SecurityConfig = &dd.SecurityConfig{
    MaxMessageSize: 1 * 1024 * 1024, // 1MB for high-security environments
    MaxWriters:     50,
}
```

### 5. Secure File Permissions

When logging to files, ensure appropriate file permissions:

```go
// Set restrictive permissions on log files
fileWriter, _ := dd.NewFileWriter("logs/app.log", dd.FileWriterConfig{
    // File is created with 0644 permissions by default
    // Adjust OS-level permissions as needed
})
```

### 6. Rotate and Compress Logs

Enable log rotation and compression to prevent disk exhaustion:

```go
logger, _ := dd.NewWithOptions(dd.Options{
    File: "logs/app.log",
    FileConfig: dd.FileWriterConfig{
        MaxSizeMB:  100,                 // Rotate at 100MB
        MaxBackups: 10,                  // Keep only 10 backups
        MaxAge:     7 * 24 * time.Hour,  // Delete after 7 days
        Compress:   true,                // Compress old logs
    },
})
```

### 7. Handle Fatal Logs Carefully

Fatal logs terminate the application - use with caution:

```go
// Custom fatal handler for graceful shutdown
config := dd.DefaultConfig()
config.FatalHandler = func() {
    // Cleanup resources
    cleanup()
    // Custom exit code
    os.Exit(2)
}
logger, _ := dd.New(config)

// Only use Fatal for truly unrecoverable errors
logger.Fatal("Critical system failure")
```

### 8. Close Loggers Properly

Always close loggers to flush buffers and release resources:

```go
logger, _ := dd.NewWithOptions(dd.Options{
    File: "logs/app.log",
})
defer logger.Close() // Ensures proper cleanup

// Or use explicit close with error handling
if err := logger.Close(); err != nil {
    fmt.Fprintf(os.Stderr, "Failed to close logger: %v\n", err)
}
```

For production environments, use the `Shutdown` method with a timeout for graceful shutdown:

```go
logger, _ := dd.New(dd.DefaultConfig())
defer func() {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    if err := logger.Shutdown(ctx); err != nil {
        fmt.Fprintf(os.Stderr, "Logger shutdown error: %v\n", err)
    }
}()
```

### 9. Monitor Hook Errors

Use the HookErrorRecorder to monitor hook health in production:

```go
recorder := dd.NewHookErrorRecorder()

// Create registry with error handler
registry := dd.NewHookRegistryWithErrorHandler(recorder.Handler())

// Add hooks that may fail
registry.Add(dd.HookAfterLog, myUnreliableHook)

// Attach to logger
logger.SetHooks(registry)

// Periodically check for errors
if recorder.HasErrors() {
    for _, err := range recorder.Errors() {
        fmt.Fprintf(os.Stderr, "Hook error: %v\n", err)
    }
    recorder.Clear()
}
```

## Security Configuration Reference

### Minimal Security Configuration

```go
config := dd.DefaultConfig()
// Injection protection is always enabled
// No sensitive data filtering (best performance)
```

### Recommended Security Configuration

```go
config := dd.DefaultConfig()
config.EnableBasicFiltering()
config.SecurityConfig.MaxMessageSize = 5 * 1024 * 1024 // 5MB
config.SecurityConfig.MaxWriters = 100
```

### Maximum Security Configuration

```go
config := dd.DefaultConfig()
config.EnableFullFiltering()
config.SecurityConfig = &dd.SecurityConfig{
    MaxMessageSize:  1 * 1024 * 1024, // 1MB
    MaxWriters:      50,
    SensitiveFilter: dd.NewSensitiveDataFilter(),
}
```

### Custom Security Configuration

```go
// Create custom filter with specific patterns
filter := dd.NewEmptySensitiveDataFilter()
filter.AddPattern(`(?i)internal[_-]?token[:\s=]+[^\s]+`)
filter.AddPattern(`\bCUSTOM_SECRET_[A-Z0-9]+\b`)

config := dd.DefaultConfig()
config.SecurityConfig = &dd.SecurityConfig{
    MaxMessageSize:  2 * 1024 * 1024,
    MaxWriters:      75,
    SensitiveFilter: filter,
}
```

## Performance vs Security Trade-offs

| Feature               | Performance Impact | Security Benefit | Recommendation             |
|-----------------------|--------------------|------------------|----------------------------|
| No filtering          | None               | Low              | Development only           |
| Basic filtering       | ~5-10%             | Medium           | Recommended for production |
| Full filtering        | ~10-20%            | High             | High-security environments |
| Custom filtering      | Varies             | Varies           | Specific compliance needs  |
| Message size limiting | Minimal            | High             | Always enable              |
| Newline escaping      | Minimal            | High             | Always enabled             |
| Field key validation  | Minimal            | Medium           | Always enabled             |

## Reporting a Vulnerability

If you discover a security vulnerability in the DD library, please report it responsibly:

1. **Do NOT** open a public GitHub issue
2. Email security reports to: cybergodev@gmail.com
3. Include:
   - Description of the vulnerability
   - Steps to reproduce
   - Potential impact
   - Suggested fix (if available)


## Security Checklist

- [ ] Enable appropriate filtering level for your use case
- [ ] Configure message size limits
- [ ] Set up log rotation and retention policies
- [ ] Validate and sanitize user input before logging
- [ ] Use structured logging for sensitive data
- [ ] Implement proper file permissions on log files
- [ ] Close loggers properly to flush buffers
- [ ] Review logged data regularly for sensitive information
- [ ] Implement access controls on log files
- [ ] Consider encryption for logs at rest (external to DD)
- [ ] Monitor log file sizes and disk usage
- [ ] Test security configurations in staging environment

## Additional Resources

- [README.md](README.md) - General documentation
- [examples/04_security_features.go](examples/04_security_features.go) - Security examples
- [security_advanced_test.go](security_advanced_test.go) - Security test cases

---

## License

This security policy is part of the DD logging library and is covered under the same MIT License.
