//go:build examples

package main

import (
	"fmt"
	"os"
	"time"

	"github.com/cybergodev/dd"
)

// Configuration - Complete Config API Guide
//
// Topics covered:
// 1. DefaultConfig and direct modification
// 2. Preset configurations (Development, JSON)
// 3. File output with rotation
// 4. JSON customization
// 5. Clone for multiple loggers
func main() {
	fmt.Println("=== DD Configuration ===\n")

	section1BasicConfig()
	section2Presets()
	section3FileRotation()
	section4JSONCustomization()
	section5Clone()

	fmt.Println("\n✅ Configuration examples completed!")
	fmt.Println("\nCheck logs/ directory for output files")
}

// Section 1: Basic configuration
func section1BasicConfig() {
	fmt.Println("1. Basic Configuration")
	fmt.Println("----------------------")

	// DefaultConfig with IDE autocomplete support
	cfg := dd.DefaultConfig()
	cfg.Level = dd.LevelDebug
	cfg.Format = dd.FormatJSON
	cfg.DynamicCaller = true // Show caller file:line

	logger, _ := dd.New(cfg)
	defer logger.Close()

	logger.Debug("Direct field modification with autocomplete")

	// Simple: dd.New() uses defaults
	simpleLogger, _ := dd.New()
	defer simpleLogger.Close()
	simpleLogger.Info("No config needed - uses defaults")

	fmt.Println()
}

// Section 2: Preset configurations
func section2Presets() {
	fmt.Println("2. Preset Configurations")
	fmt.Println("-------------------------")

	// Development: Debug level, text format, caller info
	devLogger := dd.Must(dd.DevelopmentConfig())
	defer devLogger.Close()
	devLogger.Debug("Development mode - verbose output")

	// JSON: Debug level, JSON format, structured for production
	jsonLogger := dd.Must(dd.JSONConfig())
	defer jsonLogger.Close()
	jsonLogger.Info("JSON format ready for log aggregation")

	fmt.Println()
}

// Section 3: File output with rotation
func section3FileRotation() {
	fmt.Println("3. File Rotation")
	fmt.Println("-----------------")

	cfg := dd.DefaultConfig()
	cfg.Format = dd.FormatJSON
	cfg.File = &dd.FileConfig{
		Path:       "logs/app.log",
		MaxSizeMB:  100,                 // Rotate at 100MB
		MaxBackups: 10,                  // Keep 10 old files
		MaxAge:     30 * 24 * time.Hour, // Delete after 30 days
		Compress:   true,                // Gzip old files
	}

	logger, _ := dd.New(cfg)
	defer logger.Close()

	logger.InfoWith("File rotation configured",
		dd.Int("max_size_mb", 100),
		dd.Int("max_backups", 10),
		dd.Bool("compress", true),
	)

	fmt.Println("✓ Logs written to logs/app.log\n")
}

// Section 4: JSON customization
func section4JSONCustomization() {
	fmt.Println("4. JSON Customization")
	fmt.Println("----------------------")

	cfg := dd.JSONConfig()

	// Customize JSON field names (for ELK, CloudWatch, etc.)
	cfg.JSON.FieldNames = &dd.JSONFieldNames{
		Timestamp: "@timestamp", // ELK standard
		Level:     "severity",
		Message:   "msg",
		Caller:    "source",
	}

	// Pretty print for development
	cfg.JSON.PrettyPrint = true
	cfg.JSON.Indent = "  "

	logger, _ := dd.New(cfg)
	defer logger.Close()

	logger.InfoWith("Custom JSON format",
		dd.String("service", "user-api"),
		dd.Int("version", 1),
	)

	fmt.Println()
}

// Section 5: Clone for multiple loggers
func section5Clone() {
	fmt.Println("5. Clone for Multiple Loggers")
	fmt.Println("-------------------------------")

	// Base configuration
	baseCfg := dd.DefaultConfig()
	baseCfg.Format = dd.FormatJSON
	baseCfg.DynamicCaller = true

	// Clone for application logs
	appCfg := baseCfg.Clone()
	appCfg.Level = dd.LevelInfo
	appCfg.File = &dd.FileConfig{Path: "logs/app.log"}
	appLogger, _ := dd.New(appCfg)
	defer appLogger.Close()

	// Clone for audit logs (larger size)
	auditCfg := baseCfg.Clone()
	auditCfg.Level = dd.LevelInfo
	auditCfg.File = &dd.FileConfig{
		Path:       "logs/audit.log",
		MaxSizeMB:  500,
		MaxBackups: 50,
	}
	auditLogger, _ := dd.New(auditCfg)
	defer auditLogger.Close()

	// Clone for error logs (errors only)
	errCfg := baseCfg.Clone()
	errCfg.Level = dd.LevelError
	errCfg.File = &dd.FileConfig{Path: "logs/errors.log"}
	errLogger, _ := dd.New(errCfg)
	defer errLogger.Close()

	appLogger.InfoWith("Application started",
		dd.Int("pid", os.Getpid()),
	)
	auditLogger.InfoWith("Audit entry",
		dd.String("action", "user_login"),
		dd.String("user_id", "123"),
	)
	errLogger.ErrorWith("Error logged",
		dd.Err(fmt.Errorf("example error")),
	)

	fmt.Println("✓ Multiple loggers from cloned config\n")
}
