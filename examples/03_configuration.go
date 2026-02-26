//go:build examples

package main

import (
	"fmt"
	"os"
	"time"

	"github.com/cybergodev/dd"
)

// Configuration - File Output, Rotation, and Presets
//
// This example demonstrates:
// 1. Preset configurations (Default, Development, JSON)
// 2. File output with rotation
// 3. JSON customization
// 4. Production-ready configurations
func main() {
	fmt.Println("=== DD Configuration ===\n")

	example1PresetConfigs()
	example2FileRotation()
	example3JSONCustomization()
	example4ProductionSetup()

	fmt.Println("\n✅ Configuration examples completed!")
	fmt.Println("\nCheck logs/ directory for output files")
}

// Example 1: Preset configurations
func example1PresetConfigs() {
	fmt.Println("1. Preset Configurations")
	fmt.Println("------------------------")

	// DefaultConfig: INFO level, text format, console output
	logger1, _ := dd.New(dd.DefaultConfig())
	defer logger1.Close()
	logger1.Info("Default: production-ready text logging")

	// DevelopmentConfig: DEBUG level, caller info, dynamic caller detection
	logger2, _ := dd.New(dd.DevelopmentConfig())
	defer logger2.Close()
	logger2.Debug("Development: verbose debugging with caller info")

	// JSONConfig: structured JSON output for log aggregation
	logger3, _ := dd.New(dd.JSONConfig())
	defer logger3.Close()
	logger3.InfoWith("JSON: structured logging", dd.String("format", "json"))

	fmt.Println()
}

// Example 2: File rotation
func example2FileRotation() {
	fmt.Println("2. File Rotation")
	fmt.Println("----------------")

	// Basic rotation with size, age, and backup limits
	config, err := dd.DefaultConfig().WithFile("logs/app.log", dd.FileWriterConfig{
		MaxSizeMB:  10,                 // Rotate at 10MB
		MaxBackups: 5,                  // Keep 5 old files
		MaxAge:     7 * 24 * time.Hour, // Delete after 7 days
		Compress:   true,               // Compress old logs (.gz)
	})
	if err != nil {
		fmt.Printf("Failed to create config: %v\n", err)
		return
	}

	logger, _ := dd.New(config)
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

// Example 3: JSON customization
func example3JSONCustomization() {
	fmt.Println("3. JSON Customization")
	fmt.Println("---------------------")

	// Custom field names and pretty print
	config := dd.JSONConfig()
	config.JSON = &dd.JSONOptions{
		PrettyPrint: true,
		Indent:      "  ",
		FieldNames: &dd.JSONFieldNames{
			Timestamp: "time",     // Custom field name
			Level:     "severity", // Custom field name
			Message:   "msg",      // Custom field name
			Caller:    "source",
			// Fields omitted - will use default value "fields"
		},
	}

	logger, _ := dd.New(config)
	defer logger.Close()

	logger.InfoWith("Custom JSON format",
		dd.Int("user_id", 123),
		dd.String("action", "login"),
	)

	fmt.Println()
}

// Example 4: Production setup with multiple loggers
func example4ProductionSetup() {
	fmt.Println("4. Production Setup")
	fmt.Println("The log is saved in the `logs` directory.")
	fmt.Println("-------------------")

	// Application logger: JSON, INFO level, file output
	appConfig := dd.JSONConfig()
	appConfig.Level = dd.LevelInfo
	appConfig, _ = appConfig.WithFile("logs/app.log", dd.FileWriterConfig{
		MaxSizeMB:  100,
		MaxBackups: 30,
		MaxAge:     30 * 24 * time.Hour,
		Compress:   true,
	})

	appLogger, _ := dd.New(appConfig)
	defer appLogger.Close()

	appLogger.InfoWith("Application started",
		dd.String("version", "1.0.0"),
		dd.Int("pid", os.Getpid()),
		dd.String("environment", "production"),
	)

	// Error logger: separate file, longer retention
	errorConfig := dd.JSONConfig()
	errorConfig.Level = dd.LevelError
	errorConfig, _ = errorConfig.WithFileOnly("logs/errors.log", dd.FileWriterConfig{
		MaxSizeMB:  200,
		MaxBackups: 100,
		MaxAge:     90 * 24 * time.Hour,
		Compress:   true,
	})

	errorLogger, _ := dd.New(errorConfig)
	defer errorLogger.Close()

	errorLogger.ErrorWith("Database error",
		dd.Err(fmt.Errorf("connection timeout")),
		dd.String("host", "db.example.com"),
		dd.Int("port", 5432),
	)

	fmt.Println()
}
