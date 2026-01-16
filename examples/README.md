# DD Logger Examples

This directory contains comprehensive, production-ready examples demonstrating all features of the `cybergodev/dd`
logging library.

## 📚 Quick Navigation

1. **[01_quick_start.go](01_quick_start.go)** - Master dd logger in 5 minutes
    - Package-level functions
    - Logger instances
    - All log levels
    - Formatted logging
    - Dynamic level control


2. **[02_structured_logging.go](02_structured_logging.go)** - Production-ready patterns
    - Core field types
    - HTTP request logging
    - Database operations
    - Business events
    - Error logging templates


3. **[03_configuration.go](03_configuration.go)** - File output and rotation
    - Preset configurations
    - File rotation (size/age/backup limits)
    - JSON customization
    - Production setup examples


4. **[04_security.go](04_security.go)** - Sensitive data protection
    - Basic filtering (passwords, API keys)
    - Full filtering (emails, IPs, SSNs)
    - Custom filtering patterns
    - Message size limits


5. **[05_production_patterns.go](05_production_patterns.go)** - Real-world patterns
    - Error handling & panic recovery
    - Context propagation
    - Graceful shutdown
    - Background job logging


6. **[06_advanced_features.go](06_advanced_features.go)** - Advanced features
    - Caller detection (static/dynamic)
    - Cloud logging (ELK, CloudWatch)
    - Performance benchmarking
    - Debug utilities (Text, Json, Exit)

## 🚀 Running Examples

### Run a single example:

```bash
go run -tags examples ./examples/01_quick_start.go
```

## 📝 Notes

- All examples use the `//go:build examples` build tag
- Examples create log files in the `logs/` directory
- Each example is self-contained and can run independently
- Examples demonstrate best practices and production-ready patterns

---

**Happy coding! 🎉**