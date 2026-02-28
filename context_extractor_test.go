package dd

import (
	"context"
	"sync"
	"testing"
)

func TestNewContextExtractorRegistry(t *testing.T) {
	registry := NewContextExtractorRegistry()
	if registry == nil {
		t.Fatal("expected non-nil registry")
	}
	if registry.Count() != 0 {
		t.Errorf("expected empty registry, got %d extractors", registry.Count())
	}
}

func TestContextExtractorRegistry_Add(t *testing.T) {
	registry := NewContextExtractorRegistry()

	// Test adding extractor
	extractor := func(ctx context.Context) []Field {
		return []Field{String("test", "value")}
	}
	registry.Add(extractor)

	if registry.Count() != 1 {
		t.Errorf("expected 1 extractor, got %d", registry.Count())
	}

	// Test adding nil extractor (should be ignored)
	registry.Add(nil)
	if registry.Count() != 1 {
		t.Errorf("expected 1 extractor after adding nil, got %d", registry.Count())
	}
}

func TestContextExtractorRegistry_Extract(t *testing.T) {
	t.Run("empty registry", func(t *testing.T) {
		registry := NewContextExtractorRegistry()
		ctx := context.Background()
		fields := registry.Extract(ctx)
		if fields != nil {
			t.Errorf("expected nil fields from empty registry, got %v", fields)
		}
	})

	t.Run("single extractor", func(t *testing.T) {
		registry := NewContextExtractorRegistry()
		registry.Add(func(ctx context.Context) []Field {
			return []Field{String("key1", "value1")}
		})

		ctx := context.Background()
		fields := registry.Extract(ctx)

		if len(fields) != 1 {
			t.Fatalf("expected 1 field, got %d", len(fields))
		}
		if fields[0].Key != "key1" || fields[0].Value != "value1" {
			t.Errorf("unexpected field: %v", fields[0])
		}
	})

	t.Run("multiple extractors", func(t *testing.T) {
		registry := NewContextExtractorRegistry()
		registry.Add(func(ctx context.Context) []Field {
			return []Field{String("key1", "value1")}
		})
		registry.Add(func(ctx context.Context) []Field {
			return []Field{String("key2", "value2")}
		})

		ctx := context.Background()
		fields := registry.Extract(ctx)

		if len(fields) != 2 {
			t.Fatalf("expected 2 fields, got %d", len(fields))
		}
	})

	t.Run("extractor returns nil", func(t *testing.T) {
		registry := NewContextExtractorRegistry()
		registry.Add(func(ctx context.Context) []Field {
			return nil
		})
		registry.Add(func(ctx context.Context) []Field {
			return []Field{String("key1", "value1")}
		})

		ctx := context.Background()
		fields := registry.Extract(ctx)

		if len(fields) != 1 {
			t.Fatalf("expected 1 field, got %d", len(fields))
		}
	})

	t.Run("nil context with nil-safe extractor", func(t *testing.T) {
		registry := NewContextExtractorRegistry()
		registry.Add(func(ctx context.Context) []Field {
			if ctx == nil {
				return nil
			}
			return []Field{String("key1", "value1")}
		})

		fields := registry.Extract(nil)
		if fields != nil {
			t.Errorf("expected nil fields for nil context with nil-safe extractor, got %v", fields)
		}
	})
}

func TestContextExtractorRegistry_Clone(t *testing.T) {
	registry := NewContextExtractorRegistry()
	registry.Add(func(ctx context.Context) []Field {
		return []Field{String("key1", "value1")}
	})

	clone := registry.Clone()
	if clone == nil {
		t.Fatal("expected non-nil clone")
	}

	if clone.Count() != registry.Count() {
		t.Errorf("clone count mismatch: got %d, want %d", clone.Count(), registry.Count())
	}

	// Modify original and verify clone is independent
	registry.Add(func(ctx context.Context) []Field {
		return []Field{String("key2", "value2")}
	})

	if registry.Count() == clone.Count() {
		t.Error("clone should be independent of original")
	}

	// Test nil registry
	var nilRegistry *ContextExtractorRegistry
	if nilRegistry.Clone() != nil {
		t.Error("expected nil for nil registry clone")
	}
}

func TestContextExtractorRegistry_Clear(t *testing.T) {
	registry := NewContextExtractorRegistry()
	registry.Add(func(ctx context.Context) []Field {
		return []Field{String("key1", "value1")}
	})

	if registry.Count() != 1 {
		t.Fatalf("expected 1 extractor, got %d", registry.Count())
	}

	registry.Clear()
	if registry.Count() != 0 {
		t.Errorf("expected 0 extractors after clear, got %d", registry.Count())
	}
}

func TestDefaultContextExtractorRegistry(t *testing.T) {
	registry := DefaultContextExtractorRegistry()
	if registry == nil {
		t.Fatal("expected non-nil registry")
	}
	if registry.Count() != 3 {
		t.Errorf("expected 3 default extractors, got %d", registry.Count())
	}

	t.Run("extracts trace_id", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), "trace_id", "abc123")
		fields := registry.Extract(ctx)

		found := false
		for _, f := range fields {
			if f.Key == "trace_id" && f.Value == "abc123" {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected trace_id field to be extracted")
		}
	})

	t.Run("extracts span_id", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), "span_id", "def456")
		fields := registry.Extract(ctx)

		found := false
		for _, f := range fields {
			if f.Key == "span_id" && f.Value == "def456" {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected span_id field to be extracted")
		}
	})

	t.Run("extracts request_id", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), "request_id", "req789")
		fields := registry.Extract(ctx)

		found := false
		for _, f := range fields {
			if f.Key == "request_id" && f.Value == "req789" {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected request_id field to be extracted")
		}
	})
}

func TestStringValue(t *testing.T) {
	tests := []struct {
		name     string
		input    any
		expected string
	}{
		{"nil", nil, ""},
		{"string", "hello", "hello"},
		{"int", 42, "42"},
		{"bool", true, "true"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := stringValue(tt.input)
			if result != tt.expected {
				t.Errorf("stringValue(%v) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestContextExtractorRegistry_ConcurrentAccess(t *testing.T) {
	registry := NewContextExtractorRegistry()

	var wg sync.WaitGroup
	numGoroutines := 100

	// Concurrent adds
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			registry.Add(func(ctx context.Context) []Field {
				return []Field{Int("index", idx)}
			})
		}(i)
	}

	// Concurrent extracts
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = registry.Extract(context.Background())
		}()
	}

	// Concurrent count
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = registry.Count()
		}()
	}

	wg.Wait()

	if registry.Count() != numGoroutines {
		t.Errorf("expected %d extractors, got %d", numGoroutines, registry.Count())
	}
}
