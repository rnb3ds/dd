//go:build examples

package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/cybergodev/dd"
)

// Production Patterns - Real-World Usage
//
// This example demonstrates:
// 1. Error handling and panic recovery
// 2. Context propagation for request tracking
// 3. Graceful shutdown
// 4. Background job logging
func main() {
	fmt.Println("=== DD Production Patterns ===\n")

	example1ErrorHandling()
	example2ContextPropagation()
	example3GracefulShutdown()
	example4BackgroundJobs()

	fmt.Println("\n✅ Production patterns completed!")
}

// Example 1: Error handling and panic recovery
func example1ErrorHandling() {
	fmt.Println("1. Error Handling & Panic Recovery")
	fmt.Println("----------------------------------")

	cfg := dd.DefaultConfig()
	cfg.Format = dd.FormatJSON
	cfg.File = &dd.FileConfig{Path: "logs/errors.json"}

	logger, err := dd.New(cfg)
	if err != nil {
		fmt.Printf("Failed to create logger: %v\n", err)
		return
	}
	defer logger.Close()

	// Basic error logging
	err = errors.New("database connection failed")
	logger.ErrorWith("Operation failed",
		dd.Err(err),
		dd.String("operation", "user_query"),
		dd.String("host", "db.example.com"),
		dd.Int("retry_count", 3),
	)

	// Panic recovery pattern
	func() {
		defer func() {
			if r := recover(); r != nil {
				logger.ErrorWith("Panic recovered",
					dd.String("panic_value", fmt.Sprintf("%v", r)),
					dd.String("function", "processRequest"),
				)
			}
		}()
		panic("nil pointer dereference")
	}()

	fmt.Println("✓ Errors logged and panic recovered\n")
}

// Example 2: Context propagation for request tracking
func example2ContextPropagation() {
	fmt.Println("2. Context Propagation")
	fmt.Println("----------------------")

	cfg := dd.DefaultConfig()
	cfg.Format = dd.FormatJSON
	cfg.File = &dd.FileConfig{Path: "logs/requests.json"}

	logger, err := dd.New(cfg)
	if err != nil {
		fmt.Printf("Failed to create logger: %v\n", err)
		return
	}
	defer logger.Close()

	// Create context with request metadata
	ctx := context.Background()
	ctx = context.WithValue(ctx, "request_id", "req-abc-123")
	ctx = context.WithValue(ctx, "user_id", "user-456")

	// Helper to log with context
	logWithContext := func(ctx context.Context, msg string, fields ...dd.Field) {
		allFields := []dd.Field{
			dd.String("request_id", ctx.Value("request_id").(string)),
			dd.String("user_id", ctx.Value("user_id").(string)),
		}
		allFields = append(allFields, fields...)
		logger.InfoWith(msg, allFields...)
	}

	// Simulate request flow
	logWithContext(ctx, "Request received", dd.String("endpoint", "/api/checkout"))
	logWithContext(ctx, "Validating user", dd.String("step", "validation"))
	logWithContext(ctx, "Processing payment", dd.Float64("amount", 99.99))
	logWithContext(ctx, "Request completed", dd.String("status", "success"))

	fmt.Println("✓ Request tracked across multiple steps\n")
}

// Example 3: Graceful shutdown
func example3GracefulShutdown() {
	fmt.Println("3. Graceful Shutdown")
	fmt.Println("--------------------")

	cfg := dd.DefaultConfig()
	cfg.Format = dd.FormatJSON
	cfg.File = &dd.FileConfig{Path: "logs/shutdown.json"}

	logger, err := dd.New(cfg)
	if err != nil {
		fmt.Printf("Failed to create logger: %v\n", err)
		return
	}

	logger.InfoWith("Application started", dd.Int("pid", os.Getpid()))

	// Setup signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	done := make(chan bool)

	// Background worker
	go func() {
		for i := 0; ; i++ {
			select {
			case <-done:
				logger.Info("Worker stopped")
				return
			default:
				logger.InfoWith("Working", dd.Int("iteration", i))
				time.Sleep(200 * time.Millisecond)
			}
		}
	}()

	// Auto-trigger shutdown for demo
	go func() {
		time.Sleep(800 * time.Millisecond)
		sigChan <- syscall.SIGTERM
	}()

	// Wait for signal
	sig := <-sigChan
	logger.InfoWith("Received signal", dd.String("signal", sig.String()))

	done <- true
	time.Sleep(100 * time.Millisecond)

	logger.Info("Shutting down gracefully")
	logger.Close()

	fmt.Println("✓ Graceful shutdown completed\n")
}

// Example 4: Background jobs
func example4BackgroundJobs() {
	fmt.Println("4. Background Jobs")
	fmt.Println("------------------")

	cfg := dd.DefaultConfig()
	cfg.Format = dd.FormatJSON
	cfg.File = &dd.FileConfig{Path: "logs/jobs.json"}

	logger, err := dd.New(cfg)
	if err != nil {
		fmt.Printf("Failed to create logger: %v\n", err)
		return
	}
	defer logger.Close()

	var wg sync.WaitGroup

	// Simulate 3 background jobs
	jobs := []struct {
		id      string
		jobType string
	}{
		{"job-001", "send_email"},
		{"job-002", "generate_report"},
		{"job-003", "process_image"},
	}

	for _, job := range jobs {
		wg.Add(1)
		go func(id, jobType string) {
			defer wg.Done()

			start := time.Now()
			logger.InfoWith("Job started",
				dd.String("job_id", id),
				dd.String("job_type", jobType),
			)

			// Simulate work
			time.Sleep(100 * time.Millisecond)

			duration := time.Since(start)
			logger.InfoWith("Job completed",
				dd.String("job_id", id),
				dd.String("status", "success"),
				dd.Float64("duration_ms", float64(duration.Microseconds())/1000),
			)
		}(job.id, job.jobType)
	}

	wg.Wait()
	fmt.Println("✓ Background jobs completed\n")
}
