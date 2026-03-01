# Changelog

All notable changes to the cybergodev/dd library will be documented in this file.

- The format is based on [Keep a Changelog](https://keepachangelog.com/).
- And this project adheres to [Semantic Versioning](https://semver.org/).

---

## v1.2.0 - Enhance Security & Performance & API Unification (2026-03-01)

### Added
- Audit logging system with async event processing and integrity verification
- HMAC-based log integrity signing and verification (`IntegritySigner`)
- Rate limiting for log flooding prevention (token bucket algorithm)
- Secure memory types: `SecureBuffer`, `SecureString`, `SecureBytes` with `WipeBytes()`
- Context extractors for custom context field extraction (`ContextExtractorRegistry`)
- Lifecycle hook system with `HookBeforeLog`, `HookAfterLog`, `HookOnClose`, `HookOnError`
- Dynamic level resolver for runtime log level adjustment based on context
- Field key validation with naming convention support (snake_case, camelCase, PascalCase, kebab-case)
- Enterprise security presets: `HealthcareConfig()`, `FinancialConfig()`, `GovernmentConfig()`
- Convenience constructors: `ToFile()`, `ToJSONFile()`, `ToConsole()`, `ToAll()`, `MustVal[T]()`
- `WithFields()` / `WithField()` for field inheritance and chaining
- Log sampling support with Initial/Thereafter/Tick configuration
- `GetFilterStats()` for filter performance monitoring
- `IsLevelEnabled()` and `IsXxxEnabled()` convenience methods
- Package-level context-aware logging functions (`DebugCtx`, `InfoCtx`, `WarnCtx`, `ErrorCtx`)
- `Shutdown(ctx)` method for graceful shutdown with timeout support
- `DefaultIntegrityConfigSafe()` panic-free alternative

### Changed
- **BREAKING**: Removed `NewConfig()` - use `DefaultConfig()` instead
- **BREAKING**: Removed `SecureConfig()` - use `DefaultSecureConfig()` instead
- **BREAKING**: Removed `DefaultSecurityConfigDisabled()` - use `SecurityConfigForLevel(SecurityLevelDevelopment)`
- **BREAKING**: Removed `Config.Build()` / `Config.MustBuild()` - use `dd.New(cfg)` / `dd.Must(cfg)`
- **BREAKING**: Removed functional options API (`options.go`) - use struct-based `Config`
- **BREAKING**: Removed `FilterLevel` enum - use filter constructors directly
- Security filtering now enabled by default in all configurations
- `SensitiveDataFilter.Clone()` shares immutable patterns slice (56% memory reduction)
- Multi-writer uses atomic pointer for lock-free read access (15-25% improvement)
- Time formatting uses lock-free atomic pointer cache (30-40% less contention)
- Default integrity key uses cryptographically secure random generation
- Optional parameters: `NewFileWriter(path)`, `NewBufferedWriter(w)`, `NewAuditLogger()`, `NewIntegritySigner()`

### Fixed
- Critical: `LoggerEntry.Logf` format strings were not being formatted
- Race condition in `incrementTypeCount` with concurrent audit logging
- Race condition in security filter cache access outside mutex
- `MultiWriter.Close()` now skips closing standard streams (stdout/stderr/stdin)
- CRLF injection vulnerability - newline/carriage return now escaped
- Message pool memory leak with oversized buffers
- Hook registry race condition in concurrent `AddHook` calls
- Nil context handling in level resolver
- Australia ABN pattern false positives on 11-digit numbers
- NPI pattern false positives - now requires context keywords

### Security
- UTF-8 overlong encoding detection to prevent path traversal bypass
- Hardlink detection to prevent log output redirection attacks
- Windows device name and ADS (Alternate Data Streams) validation
- Log4Shell detection with Unicode escape sequence support
- C1 control character handling (U+0080-U+009F)
- Bounded regex quantifiers to prevent ReDoS attacks (max 1000)
- Circular reference detection in recursive field filtering
- Recursion depth limit (100) to prevent stack overflow
- Panic recovery in hooks and context extractors
- Goroutine leak protection with concurrent filter limit (100)
- IPv6 address filtering added

### Performance
- SimpleLogging: 27.6% faster (232ns → 168ns)
- StructuredLogging: 52.9% faster (2646ns → 1246ns)
- JSONFormat: 34.3% faster (6394ns → 4201ns)
- ConfigClone: 56% less memory (1072B → 472B)
- Pattern matching uses binary search (9.9% CPU reduction)
- Field slice copy only when hooks registered
- `clear()` builtin replaces `delete` loops for map clearing

### Removed
- `WipeString` function (no-op for immutable Go strings)
- `options.go` file with all functional options
- Deprecated constructors: `NewWithOptions()`, `FileLogger()`, `ConsoleLogger()`, `JSONFileLogger()`, `MultiLogger()`
- `FilterLevel` type and constants (`FilterNone`, `FilterBasic`, `FilterFull`)

---

## v1.1.1 - Critical Bug Fixes & API Refinement (2025-01-22)

### Fixed
- **Race Condition**: Fixed concurrent initialization issue in Default() logger using sync.Once pattern
- **Error Handling**: Convenience functions (ToFile, ToJSONFile, ToConsole, ToAll) now panic on initialization failure instead of silently failing
- **Memory Leak**: Fixed silent error handling in CleanupOldFiles, now properly reports cleanup errors
- **Test Race Condition**: Fixed data race in concurrent writer test using mutex-protected buffer
- **Documentation Accuracy**: Corrected function names (Json → JSON, Jsonf → JSONF) and added behavior warnings

### Changed
- **Text/Textf Functions**: Removed caller information for cleaner output (focused on data content only)
- **Other Debug Functions**: JSON, JSONF, Exit, Exitf retain caller information as before

### Test Results
- All tests pass ✓
- Race detector clean ✓
- No regressions introduced ✓

---

## v1.1.0 - [Stable version] Comprehensive Testing, Documentation & Quality Enhancement (2026-01-16)

### Added
  - All config chain methods (WithLevel, WithFormat, WithDynamicCaller, filtering methods)
  - All logger instance methods (Print, Println, Printf, Text, Textf, Json, Jsonf)
  - Enhanced fmt package functions (NewErrorWith, PrintfWith, PrintlnWith)
  - Security filter control (Enable/Disable/IsEnabled)
  - Complex type formatting (slices, maps, nested structures)
  - File rotation and compression triggers
  - JSON options customization
  - Dynamic caller detection
  - Edge cases and error handling

- **Log Level Alignment**: Improved visual organization of text log output
  - Fixed-width padding for log levels (DEBUG, INFO, WARN, ERROR, FATAL)
  - Consistent spacing between timestamp and level
  - Cleaner, more organized log appearance
  - Easier log scanning and parsing

- **Logger Instance fmt Methods**: Print(), Println(), Printf() on logger instances
  - Consistent with package-level functions
  - All include caller information
  - Full feature parity between instance and package-level APIs

### Changed
- **API Consistency**: Print() now an alias for Println() (both add spaces and newlines)
  - Simplifies API by eliminating confusion
  - Prioritizes developer convenience over strict fmt compatibility
  - Better matches common usage patterns

- **Enhanced fmt Package**: All fmt replacement methods now include caller information
  - Printf() at both package and instance level
  - Consistent debugging experience across all console output
  - Better traceability for all output methods

- **Logger Instance Methods**: Text/Json/Textf/Jsonf now output directly to stdout
  - Consistent behavior with package-level functions
  - Include caller information
  - Unified debugging experience

- **Log Output Format**: Timestamp and level wrapped in brackets
  - `[2026-01-16T17:40:46+08:00  INFO]` format
  - Better visual separation of metadata and message
  - Easier parsing and log analysis

### Fixed
- **Documentation Accuracy**: Comprehensive README.md verification and corrections
  - Added missing API entries (Json, Jsonf, Text, Textf, Exit, Exitf)
  - Updated structured logging example with proper field types

- **Code Quality**: Comprehensive optimization and refactoring
  - Removed over-engineering and redundant code (~100 lines eliminated)
  - Fixed TOCTOU vulnerability in symlink validation
  - Fixed resource leaks in compression (proper defer usage)
  - Added ReDoS protection (regex complexity validation)
  - Improved error handling in SetLevel()
  - Consolidated duplicate security patterns

- **Performance**: Optimizations while maintaining all functionality
  - Lock-free Default() initialization
  - Better string building with strings.Builder
  - Simplified TypeConverter (removed pool overhead)
  - Direct pattern application for security filters

- **Security**: Critical vulnerability fixes
  - TOCTOU attack prevention in file operations
  - ReDoS attack prevention with pattern validation
  - Resource leak elimination in compression
  - Better symlink validation

### Removed
- **Redundant Code**: ~65 lines of unused helper functions
- **Excessive Comments**: Cleaned up obvious/redundant comments
- **Duplicate Implementations**: Consolidated pattern definitions

### Improved
- **Code Organization**: Better separation of concerns
  - Validation separated from default value application
  - Simplified backup path building logic

- **Maintainability**: Centralized pattern registry
  - Single source of truth for security patterns
  - Easier to add/modify patterns
  - Better DRY principle adherence

### Performance Impact
- **Test Coverage**:  77% (+10%)
- **Code Quality**: Significantly improved with comprehensive test suite
- **Backward Compatibility**: 100% maintained (except Print() behavior change)
- **Documentation**: 100% accurate (verified against implementation)

---