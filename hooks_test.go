package dd

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

func TestNewHookRegistry(t *testing.T) {
	registry := NewHookRegistry()
	if registry == nil {
		t.Fatal("expected non-nil registry")
	}
	if registry.Count() != 0 {
		t.Errorf("expected empty registry, got %d hooks", registry.Count())
	}
}

func TestHookRegistry_Add(t *testing.T) {
	registry := NewHookRegistry()

	hook := func(ctx context.Context, hc *HookContext) error {
		return nil
	}
	registry.Add(HookBeforeLog, hook)

	if registry.Count() != 1 {
		t.Errorf("expected 1 hook, got %d", registry.Count())
	}
	if registry.CountFor(HookBeforeLog) != 1 {
		t.Errorf("expected 1 BeforeLog hook, got %d", registry.CountFor(HookBeforeLog))
	}

	// Test adding nil hook (should be ignored)
	registry.Add(HookBeforeLog, nil)
	if registry.Count() != 1 {
		t.Errorf("expected 1 hook after adding nil, got %d", registry.Count())
	}

	// Add hook for different event
	registry.Add(HookAfterLog, hook)
	if registry.Count() != 2 {
		t.Errorf("expected 2 hooks, got %d", registry.Count())
	}
}

func TestHookRegistry_Remove(t *testing.T) {
	registry := NewHookRegistry()
	hook := func(ctx context.Context, hc *HookContext) error {
		return nil
	}
	registry.Add(HookBeforeLog, hook)
	registry.Add(HookAfterLog, hook)

	if registry.Count() != 2 {
		t.Fatalf("expected 2 hooks, got %d", registry.Count())
	}

	registry.Remove(HookBeforeLog)
	if registry.Count() != 1 {
		t.Errorf("expected 1 hook after remove, got %d", registry.Count())
	}
	if registry.CountFor(HookBeforeLog) != 0 {
		t.Errorf("expected 0 BeforeLog hooks, got %d", registry.CountFor(HookBeforeLog))
	}
}

func TestHookRegistry_Trigger(t *testing.T) {
	t.Run("no hooks", func(t *testing.T) {
		registry := NewHookRegistry()
		hookCtx := &HookContext{Event: HookBeforeLog}
		err := registry.Trigger(context.Background(), HookBeforeLog, hookCtx)
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
	})

	t.Run("single hook success", func(t *testing.T) {
		registry := NewHookRegistry()
		called := false
		registry.Add(HookBeforeLog, func(ctx context.Context, hc *HookContext) error {
			called = true
			return nil
		})

		hookCtx := &HookContext{Event: HookBeforeLog}
		err := registry.Trigger(context.Background(), HookBeforeLog, hookCtx)
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
		if !called {
			t.Error("expected hook to be called")
		}
	})

	t.Run("multiple hooks in order", func(t *testing.T) {
		registry := NewHookRegistry()
		var order []int

		registry.Add(HookBeforeLog, func(ctx context.Context, hc *HookContext) error {
			order = append(order, 1)
			return nil
		})
		registry.Add(HookBeforeLog, func(ctx context.Context, hc *HookContext) error {
			order = append(order, 2)
			return nil
		})

		hookCtx := &HookContext{Event: HookBeforeLog}
		err := registry.Trigger(context.Background(), HookBeforeLog, hookCtx)
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}

		if len(order) != 2 || order[0] != 1 || order[1] != 2 {
			t.Errorf("expected hooks to be called in order [1, 2], got %v", order)
		}
	})

	t.Run("hook returns error", func(t *testing.T) {
		registry := NewHookRegistry()
		expectedErr := errors.New("hook error")

		registry.Add(HookBeforeLog, func(ctx context.Context, hc *HookContext) error {
			return expectedErr
		})

		hookCtx := &HookContext{Event: HookBeforeLog}
		err := registry.Trigger(context.Background(), HookBeforeLog, hookCtx)
		if err != expectedErr {
			t.Errorf("expected error %v, got %v", expectedErr, err)
		}
	})

	t.Run("hook error stops execution", func(t *testing.T) {
		registry := NewHookRegistry()
		secondCalled := false

		registry.Add(HookBeforeLog, func(ctx context.Context, hc *HookContext) error {
			return errors.New("stop")
		})
		registry.Add(HookBeforeLog, func(ctx context.Context, hc *HookContext) error {
			secondCalled = true
			return nil
		})

		hookCtx := &HookContext{Event: HookBeforeLog}
		_ = registry.Trigger(context.Background(), HookBeforeLog, hookCtx)

		if secondCalled {
			t.Error("expected second hook not to be called after first returns error")
		}
	})

	t.Run("nil registry", func(t *testing.T) {
		var registry *HookRegistry
		hookCtx := &HookContext{Event: HookBeforeLog}
		err := registry.Trigger(context.Background(), HookBeforeLog, hookCtx)
		if err != nil {
			t.Errorf("expected no error for nil registry, got %v", err)
		}
	})
}

func TestHookRegistry_Clone(t *testing.T) {
	registry := NewHookRegistry()
	registry.Add(HookBeforeLog, func(ctx context.Context, hc *HookContext) error {
		return nil
	})

	clone := registry.Clone()
	if clone == nil {
		t.Fatal("expected non-nil clone")
	}

	if clone.Count() != registry.Count() {
		t.Errorf("clone count mismatch: got %d, want %d", clone.Count(), registry.Count())
	}

	// Modify original and verify clone is independent
	registry.Add(HookAfterLog, func(ctx context.Context, hc *HookContext) error {
		return nil
	})

	if registry.Count() == clone.Count() {
		t.Error("clone should be independent of original")
	}

	// Test nil registry
	var nilRegistry *HookRegistry
	if nilRegistry.Clone() != nil {
		t.Error("expected nil for nil registry clone")
	}
}

func TestHookRegistry_Clear(t *testing.T) {
	registry := NewHookRegistry()
	registry.Add(HookBeforeLog, func(ctx context.Context, hc *HookContext) error {
		return nil
	})
	registry.Add(HookAfterLog, func(ctx context.Context, hc *HookContext) error {
		return nil
	})

	if registry.Count() != 2 {
		t.Fatalf("expected 2 hooks, got %d", registry.Count())
	}

	registry.Clear()
	if registry.Count() != 0 {
		t.Errorf("expected 0 hooks after clear, got %d", registry.Count())
	}
}

func TestHookRegistry_ClearFor(t *testing.T) {
	registry := NewHookRegistry()
	registry.Add(HookBeforeLog, func(ctx context.Context, hc *HookContext) error {
		return nil
	})
	registry.Add(HookAfterLog, func(ctx context.Context, hc *HookContext) error {
		return nil
	})

	registry.ClearFor(HookBeforeLog)
	if registry.Count() != 1 {
		t.Errorf("expected 1 hook after ClearFor, got %d", registry.Count())
	}
	if registry.CountFor(HookBeforeLog) != 0 {
		t.Errorf("expected 0 BeforeLog hooks, got %d", registry.CountFor(HookBeforeLog))
	}
}

func TestHookEvent_String(t *testing.T) {
	tests := []struct {
		event    HookEvent
		expected string
	}{
		{HookBeforeLog, "BeforeLog"},
		{HookAfterLog, "AfterLog"},
		{HookOnFilter, "OnFilter"},
		{HookOnRotate, "OnRotate"},
		{HookOnClose, "OnClose"},
		{HookOnError, "OnError"},
		{HookEvent(999), "Unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.event.String(); got != tt.expected {
				t.Errorf("HookEvent(%d).String() = %q, want %q", tt.event, got, tt.expected)
			}
		})
	}
}

func TestHookContext(t *testing.T) {
	now := time.Now()
	hookCtx := &HookContext{
		Event:     HookBeforeLog,
		Level:     LevelInfo,
		Message:   "test message",
		Fields:    []Field{String("key", "value")},
		Error:     errors.New("test error"),
		Timestamp: now,
		Metadata:  map[string]any{"extra": "data"},
	}

	if hookCtx.Event != HookBeforeLog {
		t.Errorf("expected Event BeforeLog, got %v", hookCtx.Event)
	}
	if hookCtx.Level != LevelInfo {
		t.Errorf("expected Level Info, got %v", hookCtx.Level)
	}
	if hookCtx.Message != "test message" {
		t.Errorf("expected Message 'test message', got %q", hookCtx.Message)
	}
	if len(hookCtx.Fields) != 1 {
		t.Errorf("expected 1 field, got %d", len(hookCtx.Fields))
	}
	if hookCtx.Error == nil {
		t.Error("expected non-nil error")
	}
	if !hookCtx.Timestamp.Equal(now) {
		t.Errorf("expected Timestamp %v, got %v", now, hookCtx.Timestamp)
	}
	if hookCtx.Metadata["extra"] != "data" {
		t.Errorf("expected Metadata[extra] = 'data', got %v", hookCtx.Metadata["extra"])
	}
}

func TestHookBuilder(t *testing.T) {
	builder := NewHookBuilder()

	registry := builder.
		BeforeLog(func(ctx context.Context, hc *HookContext) error { return nil }).
		AfterLog(func(ctx context.Context, hc *HookContext) error { return nil }).
		OnFilter(func(ctx context.Context, hc *HookContext) error { return nil }).
		OnRotate(func(ctx context.Context, hc *HookContext) error { return nil }).
		OnClose(func(ctx context.Context, hc *HookContext) error { return nil }).
		OnError(func(ctx context.Context, hc *HookContext) error { return nil }).
		Build()

	if registry == nil {
		t.Fatal("expected non-nil registry")
	}
	if registry.Count() != 6 {
		t.Errorf("expected 6 hooks, got %d", registry.Count())
	}

	// Verify each event has one hook
	events := []HookEvent{HookBeforeLog, HookAfterLog, HookOnFilter, HookOnRotate, HookOnClose, HookOnError}
	for _, event := range events {
		if registry.CountFor(event) != 1 {
			t.Errorf("expected 1 hook for %v, got %d", event, registry.CountFor(event))
		}
	}
}

func TestHookRegistry_ConcurrentAccess(t *testing.T) {
	registry := NewHookRegistry()

	var wg sync.WaitGroup
	numGoroutines := 100

	// Concurrent adds
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			registry.Add(HookBeforeLog, func(ctx context.Context, hc *HookContext) error {
				return nil
			})
		}()
	}

	// Concurrent triggers
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			hookCtx := &HookContext{Event: HookBeforeLog}
			_ = registry.Trigger(context.Background(), HookBeforeLog, hookCtx)
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

	if registry.CountFor(HookBeforeLog) != numGoroutines {
		t.Errorf("expected %d BeforeLog hooks, got %d", numGoroutines, registry.CountFor(HookBeforeLog))
	}
}

func TestHookRegistry_NilContext(t *testing.T) {
	registry := NewHookRegistry()
	called := false

	registry.Add(HookBeforeLog, func(ctx context.Context, hc *HookContext) error {
		called = true
		return nil
	})

	hookCtx := &HookContext{Event: HookBeforeLog}
	err := registry.Trigger(nil, HookBeforeLog, hookCtx)

	// The hook should be called even with nil context (hooks handle nil context)
	if !called {
		t.Error("expected hook to be called even with nil context")
	}
	_ = err // We don't check the error as behavior with nil context may vary
}
