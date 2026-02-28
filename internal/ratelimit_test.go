package internal

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestRateLimiter_BasicRateLimit(t *testing.T) {
	config := &RateLimitConfig{
		MaxMessagesPerSecond: 10,
		MaxBytesPerSecond:    0, // Disabled
		BurstSize:            5,
		Strategy:             RateLimitStrategyDrop,
	}

	rl := NewRateLimiter(config)

	// First messages should pass (up to MaxMessagesPerSecond + BurstSize)
	allowed := 0
	for i := 0; i < 20; i++ {
		if !rl.ShouldRateLimit(100) {
			allowed++
		}
	}

	// Should allow MaxMessagesPerSecond (10) + BurstSize (5) = 15
	if allowed != 15 {
		t.Errorf("Expected 15 messages allowed, got %d", allowed)
	}
}

func TestRateLimiter_NoRateLimit(t *testing.T) {
	config := &RateLimitConfig{
		MaxMessagesPerSecond: 0, // Disabled
		MaxBytesPerSecond:    0, // Disabled
		BurstSize:            5,
		Strategy:             RateLimitStrategyDrop,
	}

	rl := NewRateLimiter(config)

	// All messages should pass
	for i := 0; i < 100; i++ {
		if rl.ShouldRateLimit(100) {
			t.Error("Message should not be rate limited when limits are disabled")
		}
	}
}

func TestRateLimiter_ByteRateLimit(t *testing.T) {
	config := &RateLimitConfig{
		MaxMessagesPerSecond: 0,   // Disabled
		MaxBytesPerSecond:    100, // 100 bytes/sec
		BurstSize:            10,
		Strategy:             RateLimitStrategyDrop,
	}

	rl := NewRateLimiter(config)

	// Small messages should pass
	allowed := 0
	for i := 0; i < 20; i++ {
		if !rl.ShouldRateLimit(5) { // 5 bytes each
			allowed++
		}
	}

	// Should allow about 100/5 = 20 messages (byte limit)
	// Plus burst buffer
	if allowed < 15 {
		t.Errorf("Expected at least 15 messages allowed, got %d", allowed)
	}
}

func TestRateLimiter_SampleStrategy(t *testing.T) {
	config := &RateLimitConfig{
		MaxMessagesPerSecond: 5,
		MaxBytesPerSecond:    0,
		BurstSize:            2,
		Strategy:             RateLimitStrategySample,
		SamplingRate:         2, // Keep 1 in 2
	}

	rl := NewRateLimiter(config)

	// After burst is exhausted, sampling should keep ~50%
	allowed := 0
	for i := 0; i < 100; i++ {
		if !rl.ShouldRateLimit(100) {
			allowed++
		}
	}

	// Should allow MaxMessagesPerSecond (5) + BurstSize (2) + ~50% of remaining
	// 5 + 2 = 7 guaranteed, then sampling from 93 messages = ~46 more
	if allowed < 30 || allowed > 70 {
		t.Errorf("Expected ~50 messages with sampling, got %d", allowed)
	}
}

func TestRateLimiter_Concurrency(t *testing.T) {
	config := &RateLimitConfig{
		MaxMessagesPerSecond: 100, // 100 messages/sec
		MaxBytesPerSecond:    0,
		BurstSize:            10, // +10 burst
		Strategy:             RateLimitStrategyDrop,
	}

	rl := NewRateLimiter(config)

	var wg sync.WaitGroup
	var allowed atomic.Int64

	// Run 10 goroutines, each trying to send 100 messages (1000 total)
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				if !rl.ShouldRateLimit(100) {
					allowed.Add(1)
				}
			}
		}()
	}

	wg.Wait()

	// Should allow MaxMessagesPerSecond (100) + BurstSize (10) = 110
	// From 1000 total messages, expect at least 800 rate-limited
	stats := rl.Stats()
	if stats.RateLimitedCount < 800 {
		t.Errorf("Expected many rate-limited messages, got %d (allowed=%d)", stats.RateLimitedCount, allowed.Load())
	}
}

func TestRateLimiter_Reset(t *testing.T) {
	config := &RateLimitConfig{
		MaxMessagesPerSecond: 10,
		MaxBytesPerSecond:    0,
		BurstSize:            5,
		Strategy:             RateLimitStrategyDrop,
	}

	rl := NewRateLimiter(config)

	// Exhaust the rate limit
	for i := 0; i < 20; i++ {
		rl.ShouldRateLimit(100)
	}

	stats := rl.Stats()
	if stats.RateLimitedCount == 0 {
		t.Error("Expected some rate-limited messages")
	}

	// Reset
	rl.Reset()

	stats = rl.Stats()
	if stats.RateLimitedCount != 0 {
		t.Errorf("Expected rate-limited count to be 0 after reset, got %d", stats.RateLimitedCount)
	}
	if stats.Tokens != int64(config.BurstSize) {
		t.Errorf("Expected tokens to be %d after reset, got %d", config.BurstSize, stats.Tokens)
	}
}

func TestRateLimiter_Stats(t *testing.T) {
	config := &RateLimitConfig{
		MaxMessagesPerSecond: 100,
		MaxBytesPerSecond:    1000,
		BurstSize:            10,
		Strategy:             RateLimitStrategyDrop,
	}

	rl := NewRateLimiter(config)

	stats := rl.Stats()
	if stats.MaxMessagesPerSec != 100 {
		t.Errorf("Expected MaxMessagesPerSec=100, got %d", stats.MaxMessagesPerSec)
	}
	if stats.MaxBytesPerSec != 1000 {
		t.Errorf("Expected MaxBytesPerSec=1000, got %d", stats.MaxBytesPerSec)
	}
	if stats.Tokens != 10 {
		t.Errorf("Expected initial tokens=10, got %d", stats.Tokens)
	}
}

func TestRateLimiter_NilSafety(t *testing.T) {
	var rl *RateLimiter

	// Should not panic
	if rl.ShouldRateLimit(100) {
		t.Error("Nil rate limiter should not rate limit")
	}

	stats := rl.Stats()
	if stats.MaxMessagesPerSec != 0 || stats.MaxBytesPerSec != 0 {
		t.Error("Nil rate limiter should return zero stats")
	}

	rl.Reset() // Should not panic
}

func TestDefaultRateLimitConfig(t *testing.T) {
	config := DefaultRateLimitConfig()

	if config.MaxMessagesPerSecond != 10000 {
		t.Errorf("Expected default MaxMessagesPerSecond=10000, got %d", config.MaxMessagesPerSecond)
	}
	if config.MaxBytesPerSecond != 10*1024*1024 {
		t.Errorf("Expected default MaxBytesPerSecond=10MB, got %d", config.MaxBytesPerSecond)
	}
	if config.BurstSize != 1000 {
		t.Errorf("Expected default BurstSize=1000, got %d", config.BurstSize)
	}
	if config.Strategy != RateLimitStrategyDrop {
		t.Errorf("Expected default Strategy=Drop, got %d", config.Strategy)
	}
}

func TestRateLimitConfig_Clone(t *testing.T) {
	original := &RateLimitConfig{
		MaxMessagesPerSecond: 500,
		MaxBytesPerSecond:    1024,
		BurstSize:            50,
		Strategy:             RateLimitStrategySample,
		SamplingRate:         10,
	}

	cloned := original.Clone()

	if cloned == original {
		t.Error("Clone should return a new instance")
	}

	if cloned.MaxMessagesPerSecond != original.MaxMessagesPerSecond {
		t.Error("MaxMessagesPerSecond should be copied")
	}
	if cloned.MaxBytesPerSecond != original.MaxBytesPerSecond {
		t.Error("MaxBytesPerSecond should be copied")
	}
	if cloned.Strategy != original.Strategy {
		t.Error("Strategy should be copied")
	}

	// Modify original, ensure clone is unaffected
	original.MaxMessagesPerSecond = 999
	if cloned.MaxMessagesPerSecond == 999 {
		t.Error("Clone should not be affected by original modifications")
	}
}

func TestRateLimitConfig_CloneNil(t *testing.T) {
	var config *RateLimitConfig
	cloned := config.Clone()
	if cloned != nil {
		t.Error("Cloning nil should return nil")
	}
}

func TestNewRateLimiter_NilConfig(t *testing.T) {
	rl := NewRateLimiter(nil)

	if rl == nil {
		t.Fatal("NewRateLimiter should not return nil")
	}

	if rl.config.MaxMessagesPerSecond != 10000 {
		t.Error("Nil config should use defaults")
	}
}

func TestRateLimiter_NewSecondReset(t *testing.T) {
	config := &RateLimitConfig{
		MaxMessagesPerSecond: 5,
		MaxBytesPerSecond:    0,
		BurstSize:            2,
		Strategy:             RateLimitStrategyDrop,
	}

	rl := NewRateLimiter(config)

	// Exhaust rate limit
	for i := 0; i < 20; i++ {
		rl.ShouldRateLimit(100)
	}

	// Simulate moving to next second
	rl.currentSecond.Store(time.Now().Unix() - 1)

	// Should reset and allow messages again
	if rl.ShouldRateLimit(100) {
		t.Error("Message should be allowed after second reset")
	}
}
