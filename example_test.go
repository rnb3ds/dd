package dd_test

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/cybergodev/dd"
)

// ExampleLogProvider demonstrates using the LogProvider interface
// for dependency injection and testing.
func ExampleLogProvider() {
	// Create a service that accepts any logger implementing LogProvider
	service := NewUserService(getLogger())
	service.CreateUser("john")
}

// Example_typeSafeContext demonstrates type-safe context key usage.
func Example_typeSafeContext() {
	cfg := dd.NewConfig()
	cfg.Level = dd.LevelDebug
	logger, _ := dd.New(cfg)
	defer logger.Close()

	// Create context with type-safe keys
	ctx := dd.WithTraceID(context.Background(), "trace-abc123")
	ctx = dd.WithSpanID(ctx, "span-def456")
	ctx = dd.WithRequestID(ctx, "req-ghi789")

	// Log with context - trace fields are automatically extracted
	logger.InfoCtx(ctx, "Processing request")

	// Retrieve values from context
	fmt.Printf("Trace ID: %s\n", dd.GetTraceID(ctx))
	fmt.Printf("Span ID: %s\n", dd.GetSpanID(ctx))
	fmt.Printf("Request ID: %s\n", dd.GetRequestID(ctx))
}

// Example_builderPattern demonstrates the recommended Config API.
func Example_builderPattern() {
	// Create logger using Config struct
	cfg := dd.NewConfig()
	cfg.Level = dd.LevelDebug
	cfg.Format = dd.FormatJSON
	logger, _ := dd.New(cfg)
	defer logger.Close()

	// Use structured logging
	logger.InfoWith("Application started",
		dd.String("version", "1.0.0"),
		dd.String("env", "production"),
	)

	// Chain fields for contextual logging
	reqLogger := logger.WithFields(
		dd.String("request_id", "req-123"),
		dd.String("method", "GET"),
	)
	reqLogger.InfoWith("Request received",
		dd.String("path", "/api/users"),
	)
}

// Example_logSampling demonstrates log sampling for high-throughput scenarios.
func Example_logSampling() {
	cfg := dd.NewConfig()
	cfg.Level = dd.LevelInfo
	cfg.Sampling = &dd.SamplingConfig{
		Enabled:    true,
		Initial:    10,
		Thereafter: 100,
		Tick:       time.Minute,
	}
	logger, _ := dd.New(cfg)
	defer logger.Close()

	// In high-throughput scenarios, only a subset of logs will be recorded
	for i := 0; i < 1000; i++ {
		logger.Infof("Processing item %d", i)
	}
}

// Example_hooks demonstrates using lifecycle hooks.
func Example_hooks() {
	cfg := dd.NewConfig()
	cfg.Level = dd.LevelDebug
	logger, _ := dd.New(cfg)
	defer logger.Close()

	// Add a hook that runs before each log entry
	logger.MustAddHook(dd.HookBeforeLog, func(ctx context.Context, hookCtx *dd.HookContext) error {
		fmt.Printf("About to log: %s\n", hookCtx.Message)
		return nil
	})

	// Add a hook that runs after each log entry
	logger.MustAddHook(dd.HookAfterLog, func(ctx context.Context, hookCtx *dd.HookContext) error {
		fmt.Printf("Logged at level: %d\n", hookCtx.Level)
		return nil
	})

	logger.Info("Hello, world!")
}

// Example_contextExtractors demonstrates custom context extractors.
func Example_contextExtractors() {
	cfg := dd.NewConfig()
	cfg.Level = dd.LevelDebug
	logger, _ := dd.New(cfg)
	defer logger.Close()

	// Add a custom context extractor for user ID
	logger.MustAddContextExtractor(func(ctx context.Context) []dd.Field {
		if userID := ctx.Value("user_id"); userID != nil {
			return []dd.Field{dd.String("user_id", fmt.Sprintf("%v", userID))}
		}
		return nil
	})

	// Create context with user ID
	ctx := context.WithValue(context.Background(), "user_id", "user-123")

	// Log with context - custom extractor will add user_id field
	logger.InfoCtx(ctx, "User action performed")
}

// Example_fieldChaining demonstrates field chaining for contextual logging.
func Example_fieldChaining() {
	cfg := dd.NewConfig()
	cfg.Level = dd.LevelInfo
	logger, _ := dd.New(cfg)
	defer logger.Close()

	// Create a logger entry with common fields
	serviceLogger := logger.WithFields(
		dd.String("service", "api"),
		dd.String("version", "2.0.0"),
	)

	// Create more specific loggers by chaining
	userLogger := serviceLogger.WithField("component", "user-service")
	orderLogger := serviceLogger.WithField("component", "order-service")

	// Each logger maintains its own context
	userLogger.Info("User service started")
	orderLogger.Info("Order service started")

	// Add more fields for specific log entries
	userLogger.InfoWith("User created",
		dd.String("user_id", "123"),
		dd.String("email", "user@example.com"),
	)
}

// ExampleWriter is a simple writer for demonstration purposes.
type ExampleWriter struct{}

func (w *ExampleWriter) Write(p []byte) (n int, err error) {
	return len(p), nil
}

// Example_multipleWriters demonstrates logging to multiple outputs.
func Example_multipleWriters() {
	logger, _ := dd.New(dd.DefaultConfig())
	defer logger.Close()

	// Add additional writers at runtime
	_ = logger.AddWriter(&ExampleWriter{})
	_ = logger.AddWriter(io.Discard)

	logger.Info("This goes to all writers")

	// Check writer count
	fmt.Printf("Active writers: %d\n", logger.WriterCount())
}

// UserService demonstrates using LogProvider interface
type UserService struct {
	logger dd.LogProvider
}

func NewUserService(logger dd.LogProvider) *UserService {
	logger.Info("user service initialized")
	return &UserService{logger: logger}
}

func (s *UserService) CreateUser(name string) {
	s.logger.InfoWith("Creating user",
		dd.String("name", name),
		dd.Time("created_at", time.Now()),
	)
}

// getLogger returns a logger for examples
func getLogger() dd.LogProvider {
	logger, _ := dd.New(dd.DefaultConfig())
	return logger
}
