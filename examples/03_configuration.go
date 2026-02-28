//go:build examples

package main

import (
	"fmt"
	"os"
	"time"

	"github.com/cybergodev/dd"
)

// Configuration - Unified Config API Examples
//
// This example demonstrates the unified Config API:
// 1. Basic configuration with direct field modification
// 2. Preset configurations
// 3. File output with rotation
// 4. JSON customization
// 5. Production-ready configurations
// 6. Clone for multiple loggers
func main() {
	fmt.Println("=== DD Configuration (Config API) ===\n")

	example1BasicConfig()
	example2PresetConfigs()
	example3FileRotation()
	example4JSONCustomization()
	example5ProductionSetup()
	example6CloneMultipleLoggers()

	fmt.Println("\n✅ Configuration examples completed!")
	fmt.Println("\nCheck logs/ directory for output files")
}

// Example 1: Basic configuration with direct field modification
func example1BasicConfig() {
	fmt.Println("1. Basic Configuration (Direct Field Modification)")
	fmt.Println("----------------------------------------------------")

	// Method A: Using DefaultConfig with dd.New() - Recommended
	cfg := dd.DefaultConfig()
	cfg.Level = dd.LevelDebug
	cfg.Format = dd.FormatJSON
	cfg.DynamicCaller = true

	logger, _ := dd.New(cfg)
	defer logger.Close()

	logger.Debug("Direct field modification with IDE autocomplete support")
	logger.InfoWith("Structured logging",
		dd.String("config_type", "struct-based"),
		dd.Bool("autocomplete", true),
	)

	// Method B: Simple usage with defaults
	simpleLogger, _ := dd.New()
	defer simpleLogger.Close()
	simpleLogger.Info("Simple usage with dd.New() - no config needed")

	fmt.Println()
}

// Example 2: Preset configurations
func example2PresetConfigs() {
	fmt.Println("2. Preset Configurations")
	fmt.Println("------------------------")

	// Development preset
	devLogger := dd.Must(dd.ConfigDevelopment())
	defer devLogger.Close()
	devLogger.Debug("Development logger - debug level enabled")

	// JSON preset
	jsonLogger := dd.Must(dd.ConfigJSON())
	defer jsonLogger.Close()
	jsonLogger.Info("JSON format with debug level")

	fmt.Println()
}

// Example 3: File output with rotation
func example3FileRotation() {
	fmt.Println("3. File Rotation")
	fmt.Println("----------------")

	// Configure file output with rotation settings
	cfg := dd.DefaultConfig()
	cfg.File = &dd.FileConfig{
		Path:       "logs/rotated.log",
		MaxSizeMB:  10,                 // Rotate at 10MB
		MaxBackups: 5,                  // Keep 5 old files
		MaxAge:     7 * 24 * time.Hour, // Delete after 7 days
		Compress:   true,               // Compress old logs (.gz)
	}

	logger, _ := dd.New(cfg)
	defer logger.Close()

	logger.Info("Logs rotate automatically at 10MB")
	logger.InfoWith("Rotation config",
		dd.Int("max_size_mb", 10),
		dd.Int("max_backups", 5),
		dd.Int("max_age_days", 7),
		dd.Bool("compress", true),
	)

	fmt.Println()
}

// Example 4: JSON customization
func example4JSONCustomization() {
	fmt.Println("4. JSON Customization")
	fmt.Println("---------------------")

	// Start with JSON preset and customize
	cfg := dd.ConfigJSON()

	// Customize JSON options directly
	cfg.JSON.PrettyPrint = true
	cfg.JSON.Indent = "  "
	cfg.JSON.FieldNames = &dd.JSONFieldNames{
		Timestamp: "time",     // Custom field name
		Level:     "severity", // Custom field name
		Message:   "msg",      // Custom field name
		Caller:    "source",
	}

	logger, _ := dd.New(cfg)
	defer logger.Close()

	logger.InfoWith("Custom JSON format",
		dd.Int("user_id", 123),
		dd.String("action", "login"),
	)

	fmt.Println()
}

// Example 5: Production setup
func example5ProductionSetup() {
	fmt.Println("5. Production Setup")
	fmt.Println("The log is saved in the `logs` directory.")
	fmt.Println("-------------------")

	// Application logger with comprehensive settings - using dd.New(cfg)
	appCfg := dd.DefaultConfig()
	appCfg.Format = dd.FormatJSON
	appCfg.Level = dd.LevelInfo
	appCfg.DynamicCaller = true
	appCfg.File = &dd.FileConfig{
		Path:       "logs/production.log",
		MaxSizeMB:  100,
		MaxBackups: 30,
		MaxAge:     30 * 24 * time.Hour,
		Compress:   true,
	}

	appLogger, _ := dd.New(appCfg)
	defer appLogger.Close()

	appLogger.InfoWith("Application started",
		dd.String("version", "1.0.0"),
		dd.Int("pid", os.Getpid()),
		dd.String("environment", "production"),
	)

	// Error logger with different settings - using dd.New(cfg)
	errCfg := dd.DefaultConfig()
	errCfg.Format = dd.FormatJSON
	errCfg.Level = dd.LevelError
	errCfg.File = &dd.FileConfig{
		Path:       "logs/errors.log",
		MaxSizeMB:  200,
		MaxBackups: 100,
		MaxAge:     90 * 24 * time.Hour,
		Compress:   true,
	}

	errorLogger, _ := dd.New(errCfg)
	defer errorLogger.Close()

	errorLogger.ErrorWith("Database error",
		dd.Err(fmt.Errorf("connection timeout")),
		dd.String("host", "db.example.com"),
		dd.Int("port", 5432),
	)

	fmt.Println()
}

// Example 6: Clone for multiple loggers with shared base config
func example6CloneMultipleLoggers() {
	fmt.Println("6. Clone for Multiple Loggers")
	fmt.Println("------------------------------")

	// Create base configuration and clone for different loggers
	baseCfg := dd.DefaultConfig()
	baseCfg.Format = dd.FormatJSON
	baseCfg.DynamicCaller = true

	// Clone for different purposes - using dd.New(cfg)
	appCfg := baseCfg.Clone()
	appCfg.Level = dd.LevelInfo
	appCfg.File = &dd.FileConfig{Path: "logs/app.log"}
	appLogger, _ := dd.New(appCfg)
	defer appLogger.Close()

	debugCfg := baseCfg.Clone()
	debugCfg.Level = dd.LevelDebug
	debugCfg.File = &dd.FileConfig{Path: "logs/debug.log"}
	debugLogger, _ := dd.New(debugCfg)
	defer debugLogger.Close()

	// Custom modification on cloned config
	auditCfg := baseCfg.Clone()
	auditCfg.Level = dd.LevelInfo
	auditCfg.File = &dd.FileConfig{
		Path:      "logs/audit.log",
		MaxSizeMB: 500, // Larger for audit logs
	}
	auditLogger, _ := dd.New(auditCfg)
	defer auditLogger.Close()

	appLogger.Info("Application logger - INFO level")
	debugLogger.Debug("Debug logger - DEBUG level")
	auditLogger.InfoWith("Audit log entry",
		dd.String("action", "user_login"),
		dd.String("user_id", "user123"),
		dd.Time("timestamp", time.Now()),
	)

	fmt.Println()
}
