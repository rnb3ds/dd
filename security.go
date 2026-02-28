package dd

import (
	"context"
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cybergodev/dd/internal"
)

type SensitiveDataFilter struct {
	// patternsPtr stores an immutable slice of patterns using atomic pointer.
	// This eliminates slice copying during filter operations (hot path).
	// The slice is replaced atomically when patterns are added/removed.
	patternsPtr    atomic.Pointer[[]*regexp.Regexp]
	mu             sync.RWMutex // protects pattern modifications
	maxInputLength int
	timeout        time.Duration
	enabled        atomic.Bool
	closed         atomic.Bool // prevents new goroutines when true
	// semaphore limits concurrent regex filtering goroutines to prevent resource exhaustion
	semaphore chan struct{}
	// activeGoroutines tracks the number of currently running filter goroutines
	activeGoroutines atomic.Int32
	// patternCount caches the number of patterns for O(1) access
	patternCount atomic.Int32

	// Performance monitoring counters
	totalFiltered   atomic.Int64 // Total number of filter operations
	totalRedactions atomic.Int64 // Total number of redactions performed
	totalTimeouts   atomic.Int64 // Total number of timeout events
	totalLatencyNs  atomic.Int64 // Total latency in nanoseconds (for average calculation)
}

// newSensitiveDataFilterWithPatterns is the internal constructor for SensitiveDataFilter.
// It creates a filter with the specified patterns and timeout.
func newSensitiveDataFilterWithPatterns(patterns []*regexp.Regexp, timeout time.Duration) *SensitiveDataFilter {
	filter := &SensitiveDataFilter{
		maxInputLength: MaxInputLength,
		timeout:        timeout,
		semaphore:      make(chan struct{}, MaxConcurrentFilters),
	}
	filter.enabled.Store(true)

	if patterns != nil {
		copiedPatterns := make([]*regexp.Regexp, len(patterns))
		copy(copiedPatterns, patterns)
		filter.patternsPtr.Store(&copiedPatterns)
		filter.patternCount.Store(int32(len(copiedPatterns)))
	} else {
		emptyPatterns := make([]*regexp.Regexp, 0)
		filter.patternsPtr.Store(&emptyPatterns)
		filter.patternCount.Store(0)
	}

	return filter
}

func NewSensitiveDataFilter() *SensitiveDataFilter {
	internal.InitPatterns()
	return newSensitiveDataFilterWithPatterns(internal.CompiledFullPatterns, DefaultFilterTimeout)
}

func NewEmptySensitiveDataFilter() *SensitiveDataFilter {
	return newSensitiveDataFilterWithPatterns(nil, EmptyFilterTimeout)
}

func NewCustomSensitiveDataFilter(patterns ...string) (*SensitiveDataFilter, error) {
	filter := NewEmptySensitiveDataFilter()

	for _, pattern := range patterns {
		if err := filter.AddPattern(pattern); err != nil {
			return nil, err
		}
	}

	return filter, nil
}

func (f *SensitiveDataFilter) addPattern(pattern string) error {
	if len(pattern) > MaxPatternLength {
		return fmt.Errorf("%w: %d exceeds maximum %d", ErrPatternTooLong, len(pattern), MaxPatternLength)
	}

	if internal.HasNestedQuantifiers(pattern, MaxQuantifierRange) {
		return ErrReDoSPattern
	}

	re, err := regexp.Compile(pattern)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidPattern, err)
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	currentPatterns := f.patternsPtr.Load()
	newPatterns := make([]*regexp.Regexp, len(*currentPatterns)+1)
	copy(newPatterns, *currentPatterns)
	newPatterns[len(*currentPatterns)] = re
	f.patternsPtr.Store(&newPatterns)
	f.patternCount.Store(int32(len(newPatterns)))

	return nil
}

func (f *SensitiveDataFilter) AddPattern(pattern string) error {
	if f == nil {
		return ErrNilFilter
	}
	if pattern == "" {
		return ErrEmptyPattern
	}
	return f.addPattern(pattern)
}

func (f *SensitiveDataFilter) AddPatterns(patterns ...string) error {
	if f == nil {
		return ErrNilFilter
	}
	for _, pattern := range patterns {
		if pattern == "" {
			continue
		}
		if err := f.addPattern(pattern); err != nil {
			return fmt.Errorf("%w: %q", ErrPatternFailed, pattern)
		}
	}
	return nil
}

func (f *SensitiveDataFilter) ClearPatterns() {
	f.mu.Lock()
	defer f.mu.Unlock()
	emptyPatterns := make([]*regexp.Regexp, 0)
	f.patternsPtr.Store(&emptyPatterns)
	f.patternCount.Store(0)
}

func (f *SensitiveDataFilter) PatternCount() int {
	if f == nil {
		return 0
	}
	return int(f.patternCount.Load())
}

func (f *SensitiveDataFilter) Enable() {
	if f != nil {
		f.enabled.Store(true)
	}
}

func (f *SensitiveDataFilter) Disable() {
	if f != nil {
		f.enabled.Store(false)
	}
}

func (f *SensitiveDataFilter) IsEnabled() bool {
	if f == nil {
		return false
	}
	return f.enabled.Load()
}

// ActiveGoroutineCount returns the number of currently active filter goroutines.
// This can be used for monitoring and detecting potential goroutine leaks in
// high-concurrency scenarios. A consistently high count may indicate that
// filter operations are timing out frequently.
func (f *SensitiveDataFilter) ActiveGoroutineCount() int32 {
	if f == nil {
		return 0
	}
	return f.activeGoroutines.Load()
}

// FilterStats holds filter statistics for monitoring and observability.
// This provides a snapshot of the filter's current state for health checks
// and performance monitoring.
type FilterStats struct {
	ActiveGoroutines  int32         // Number of currently running filter goroutines
	PatternCount      int32         // Number of registered sensitive data patterns
	SemaphoreCapacity int           // Maximum concurrent filter operations
	MaxInputLength    int           // Maximum input length before truncation
	Enabled           bool          // Whether filtering is enabled
	TotalFiltered     int64         // Total number of filter operations
	TotalRedactions   int64         // Total number of redactions performed
	TotalTimeouts     int64         // Total number of timeout events
	AverageLatency    time.Duration // Average latency per filter operation
}

// GetFilterStats returns current filter statistics for monitoring.
// This is useful for health checks, metrics collection, and debugging.
//
// Example:
//
//	stats := filter.GetFilterStats()
//	fmt.Printf("Active goroutines: %d\n", stats.ActiveGoroutines)
//	fmt.Printf("Patterns: %d\n", stats.PatternCount)
//	fmt.Printf("Enabled: %v\n", stats.Enabled)
//	fmt.Printf("Total filtered: %d\n", stats.TotalFiltered)
//	fmt.Printf("Average latency: %v\n", stats.AverageLatency)
func (f *SensitiveDataFilter) GetFilterStats() FilterStats {
	if f == nil {
		return FilterStats{
			SemaphoreCapacity: 0,
			MaxInputLength:    0,
			Enabled:           false,
		}
	}

	var avgLatency time.Duration
	totalFiltered := f.totalFiltered.Load()
	if totalFiltered > 0 {
		avgLatency = time.Duration(f.totalLatencyNs.Load() / totalFiltered)
	}

	return FilterStats{
		ActiveGoroutines:  f.activeGoroutines.Load(),
		PatternCount:      f.patternCount.Load(),
		SemaphoreCapacity: cap(f.semaphore),
		MaxInputLength:    f.maxInputLength,
		Enabled:           f.enabled.Load(),
		TotalFiltered:     totalFiltered,
		TotalRedactions:   f.totalRedactions.Load(),
		TotalTimeouts:     f.totalTimeouts.Load(),
		AverageLatency:    avgLatency,
	}
}

// WaitForGoroutines waits for all active filter goroutines to complete or until
// the timeout is reached.
//
// IMPORTANT: Call this method before program exit to prevent goroutine leaks.
// In high-concurrency scenarios with large inputs, filter operations may spawn
// background goroutines for regex processing. Failing to wait for these goroutines
// can result in resource leaks and incomplete log filtering.
//
// Recommended usage in shutdown sequence:
//
//	// 1. Stop accepting new log messages
//	// 2. Wait for filter goroutines to complete
//	logger.WaitForFilterGoroutines(5 * time.Second)
//	// 3. Close the logger
//	logger.Close()
//
// Returns true if all goroutines completed, false if timeout was reached.
func (f *SensitiveDataFilter) WaitForGoroutines(timeout time.Duration) bool {
	if f == nil {
		return true
	}

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if f.activeGoroutines.Load() == 0 {
			return true
		}
		time.Sleep(10 * time.Millisecond)
	}
	return f.activeGoroutines.Load() == 0
}

// Close marks the filter as closed and waits for active goroutines to complete.
// After calling Close, the Filter method will return input unchanged without
// spawning new goroutines. This prevents goroutine leaks during shutdown.
//
// IMPORTANT: Always call Close (or WaitForGoroutines) before program exit to
// ensure all background goroutines complete gracefully.
//
// Returns true if all goroutines completed within the timeout, false otherwise.
func (f *SensitiveDataFilter) Close() bool {
	if f == nil {
		return true
	}

	f.closed.Store(true)
	return f.WaitForGoroutines(DefaultFilterTimeout * 2)
}

// Clone creates a copy of the SensitiveDataFilter.
//
// Deep copy:
//   - patterns slice (but the compiled *regexp.Regexp instances are shared)
//
// New instances:
//   - semaphore channel (new channel with same capacity)
//
// Returns nil if the receiver is nil.
func (f *SensitiveDataFilter) Clone() *SensitiveDataFilter {
	if f == nil {
		return nil
	}

	f.mu.RLock()
	defer f.mu.RUnlock()

	currentPatterns := f.patternsPtr.Load()
	clone := &SensitiveDataFilter{
		maxInputLength: f.maxInputLength,
		timeout:        f.timeout,
		semaphore:      make(chan struct{}, MaxConcurrentFilters),
	}
	clone.enabled.Store(f.enabled.Load())

	// Copy patterns slice and store atomically
	patterns := make([]*regexp.Regexp, len(*currentPatterns))
	copy(patterns, *currentPatterns)
	clone.patternsPtr.Store(&patterns)
	clone.patternCount.Store(f.patternCount.Load())

	return clone
}

func (f *SensitiveDataFilter) Filter(input string) string {
	if f == nil || !f.enabled.Load() || f.closed.Load() {
		return input
	}

	inputLen := len(input)
	if inputLen == 0 {
		return input
	}

	// Fast path: atomic load of patterns pointer (lock-free read)
	patternsPtr := f.patternsPtr.Load()
	if patternsPtr == nil || len(*patternsPtr) == 0 {
		return input
	}

	startTime := time.Now()

	patterns := *patternsPtr
	timeout := f.timeout

	// Handle truncation with boundary-aware sensitive data detection.
	// This prevents sensitive data patterns that span the truncation boundary
	// from being leaked.
	if f.maxInputLength > 0 && inputLen > f.maxInputLength {
		// Check the boundary region for sensitive data before truncating
		boundaryStart := f.maxInputLength - BoundaryCheckSize
		if boundaryStart < 0 {
			boundaryStart = 0
		}
		boundaryRegion := input[boundaryStart:]

		// Check if boundary region contains any sensitive patterns
		boundaryHasSensitive := false
		for _, pattern := range patterns {
			if pattern.MatchString(boundaryRegion) {
				boundaryHasSensitive = true
				break
			}
		}

		if boundaryHasSensitive {
			// Filter the boundary region separately
			filteredBoundary := boundaryRegion
			for i := range patterns {
				filteredBoundary = f.replaceWithPattern(filteredBoundary, patterns[i])
				if filteredBoundary == "" || filteredBoundary == "[REDACTED]" {
					break
				}
			}
			// Reconstruct: keep the non-boundary part + filtered boundary + truncation marker
			input = input[:boundaryStart] + filteredBoundary + "... [TRUNCATED FOR SECURITY]"
		} else {
			// No sensitive data in boundary, safe to truncate directly
			input = input[:f.maxInputLength] + "... [TRUNCATED FOR SECURITY]"
		}
	}

	result := input
	redactionCount := int64(0)
	for i := range patterns {
		originalLen := len(result)
		result = f.filterWithTimeout(result, patterns[i], timeout)
		// Track redactions (result changed)
		if len(result) != originalLen || result != input {
			redactionCount++
		}
		// Early exit if result becomes empty or redacted
		if result == "" || result == "[REDACTED]" {
			redactionCount++
			break
		}
	}

	// Update metrics
	f.totalFiltered.Add(1)
	if redactionCount > 0 {
		f.totalRedactions.Add(redactionCount)
	}
	latencyNs := time.Since(startTime).Nanoseconds()
	f.totalLatencyNs.Add(latencyNs)

	return result
}

// filterWithTimeout applies regex filtering with timeout protection for large inputs.
//
// The function uses a tiered approach based on input size:
// - Small inputs (< FastPathThreshold): Direct synchronous processing
// - Medium inputs (< FilterMediumInputThreshold): Synchronous chunked processing with context
// - Large inputs: Async processing with timeout and semaphore-based concurrency limiting
//
// For large inputs, a goroutine is spawned for regex processing. The context is passed
// to allow early termination on timeout. The semaphore limits concurrent goroutines
// to prevent resource exhaustion.
func (f *SensitiveDataFilter) filterWithTimeout(input string, pattern *regexp.Regexp, timeout time.Duration) string {
	inputLen := len(input)

	// Fast path for small inputs
	if inputLen < FastPathThreshold {
		return f.replaceWithPattern(input, pattern)
	}

	// For medium inputs, use synchronous chunked processing (no goroutine overhead)
	if inputLen < FilterMediumInputThreshold {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		result := f.filterInChunksWithContext(ctx, input, pattern)
		// Check if context timed out
		if ctx.Err() == context.DeadlineExceeded {
			f.totalTimeouts.Add(1)
		}
		return result
	}

	// Try to acquire semaphore with timeout to limit concurrent goroutines
	select {
	case f.semaphore <- struct{}{}:
		defer func() { <-f.semaphore }()
	case <-time.After(timeout / 2):
		// Could not acquire semaphore within half the timeout, return [REDACTED] for safety
		f.totalTimeouts.Add(1)
		return "[REDACTED]"
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	type result struct {
		output string
	}
	done := make(chan result, 1)

	f.activeGoroutines.Add(1)
	go func() {
		defer f.activeGoroutines.Add(-1)
		defer func() {
			if r := recover(); r != nil {
				select {
				case done <- result{output: "[REDACTED]"}:
				default:
				}
			}
		}()

		output := f.filterInChunksWithContext(ctx, input, pattern)
		select {
		case done <- result{output: output}:
		case <-ctx.Done():
		}
	}()

	select {
	case res := <-done:
		return res.output
	case <-ctx.Done():
		f.totalTimeouts.Add(1)
		return "[REDACTED]"
	}
}

// filterInChunksWithContext processes input in chunks with context support for early termination.
// Uses overlapping chunks to ensure sensitive data patterns spanning chunk boundaries are detected.
func (f *SensitiveDataFilter) filterInChunksWithContext(ctx context.Context, input string, pattern *regexp.Regexp) string {
	inputLen := len(input)

	// For inputs up to a reasonable size, process directly without chunking
	// This avoids the complexity of reassembling filtered chunks with different lengths
	if inputLen <= FilterDirectProcessThreshold {
		return f.replaceWithPattern(input, pattern)
	}

	// For very large inputs, use chunked processing with overlap for boundary detection
	overlap := ChunkOverlapSize

	var result strings.Builder
	result.Grow(inputLen)

	// Calculate effective step (chunk size minus overlap)
	step := FilterChunkSize - overlap
	if step <= 0 {
		step = FilterChunkSize / 2
		if step == 0 {
			step = 1
		}
	}

	lastWritten := 0

	for pos := 0; pos < inputLen; pos += step {
		select {
		case <-ctx.Done():
			// Write remaining unprocessed input
			if lastWritten < inputLen {
				result.WriteString(input[lastWritten:])
			}
			return result.String()
		default:
		}

		chunkStart := pos
		chunkEnd := min(pos+FilterChunkSize, inputLen)
		chunk := input[chunkStart:chunkEnd]

		// Filter the current chunk
		filtered := f.replaceWithPattern(chunk, pattern)

		if pos == 0 {
			// First chunk: write everything
			result.WriteString(filtered)
			lastWritten = chunkEnd
		} else {
			// Subsequent chunks: skip the overlap region that was already written
			// Only write the new portion (from overlap point to chunk end)
			overlapStart := overlap
			if overlapStart < len(filtered) {
				// Simple approach: write the non-overlap portion
				// The overlap ensures boundary patterns are caught in the previous chunk
				result.WriteString(filtered[overlapStart:])
			}
			lastWritten = chunkEnd
		}
	}

	// Final pass to ensure consistency and catch any remaining patterns
	return f.replaceWithPattern(result.String(), pattern)
}

func (f *SensitiveDataFilter) replaceWithPattern(input string, pattern *regexp.Regexp) string {
	if pattern.NumSubexp() > 0 {
		return pattern.ReplaceAllString(input, "$1[REDACTED]")
	}
	return pattern.ReplaceAllString(input, "[REDACTED]")
}

func (f *SensitiveDataFilter) FilterFieldValue(key string, value any) any {
	if f == nil || !f.enabled.Load() {
		return value
	}

	str, ok := value.(string)
	if !ok {
		return value
	}

	if internal.IsSensitiveKey(key) {
		return "[REDACTED]"
	}

	return f.Filter(str)
}

// FilterValueRecursive recursively filters sensitive data from nested structures.
// It processes maps, slices, arrays, and structs to filter sensitive values.
// Circular references are detected and replaced with "[CIRCULAR_REFERENCE]".
// Maximum recursion depth is limited to MaxRecursionDepth to prevent stack overflow.
func (f *SensitiveDataFilter) FilterValueRecursive(key string, value any) any {
	return f.filterValueRecursiveInternal(key, value, make(map[uintptr]bool), 0)
}

// filterValueRecursiveInternal is the internal implementation with circular reference detection.
func (f *SensitiveDataFilter) filterValueRecursiveInternal(key string, value any, visited map[uintptr]bool, depth int) any {
	if f == nil || !f.enabled.Load() {
		return value
	}

	// Check recursion depth to prevent stack overflow on deeply nested structures
	if depth > MaxRecursionDepth {
		return "[MAX_DEPTH_EXCEEDED]"
	}

	// Handle nil values
	if value == nil {
		return nil
	}

	// Check if the key itself is sensitive
	if internal.IsSensitiveKey(key) {
		return "[REDACTED]"
	}

	// Handle string values directly
	if str, ok := value.(string); ok {
		return f.Filter(str)
	}

	// Use reflection for complex types
	val := reflect.ValueOf(value)
	if !val.IsValid() {
		return value
	}

	kind := val.Kind()

	// Handle pointers - check for circular references
	if kind == reflect.Ptr {
		if val.IsNil() {
			return nil
		}
		ptr := val.Pointer()
		if visited[ptr] {
			return "[CIRCULAR_REFERENCE]"
		}
		visited[ptr] = true
		return f.filterValueRecursiveInternal(key, val.Elem().Interface(), visited, depth+1)
	}

	// Handle interfaces
	if kind == reflect.Interface {
		if val.IsNil() {
			return nil
		}
		return f.filterValueRecursiveInternal(key, val.Elem().Interface(), visited, depth+1)
	}

	// Handle slices and arrays
	if kind == reflect.Slice || kind == reflect.Array {
		if val.Len() == 0 {
			if kind == reflect.Slice {
				return []any{}
			}
			return value
		}
		// Check for circular reference in slice pointer
		if kind == reflect.Slice {
			ptr := val.Pointer()
			if visited[ptr] {
				return "[CIRCULAR_REFERENCE]"
			}
			visited[ptr] = true
		}
		result := make([]any, val.Len())
		for i := 0; i < val.Len(); i++ {
			result[i] = f.filterValueRecursiveInternal("", val.Index(i).Interface(), visited, depth+1)
		}
		return result
	}

	// Handle maps
	if kind == reflect.Map {
		if val.IsNil() {
			return nil
		}
		// Check for circular reference in map pointer
		ptr := val.Pointer()
		if visited[ptr] {
			return "[CIRCULAR_REFERENCE]"
		}
		visited[ptr] = true
		result := make(map[string]any, val.Len())
		for _, mapKey := range val.MapKeys() {
			keyStr := fmt.Sprintf("%v", mapKey.Interface())
			mapValue := val.MapIndex(mapKey).Interface()
			result[keyStr] = f.filterValueRecursiveInternal(keyStr, mapValue, visited, depth+1)
		}
		return result
	}

	// Handle structs
	if kind == reflect.Struct {
		result := make(map[string]any)
		typ := val.Type()
		for i := 0; i < val.NumField(); i++ {
			field := val.Field(i)
			fieldType := typ.Field(i)

			// Skip unexported fields
			if !field.CanInterface() {
				continue
			}

			fieldName := fieldType.Name
			// Check for json tag
			if tag := fieldType.Tag.Get("json"); tag != "" && tag != "-" {
				if commaIdx := strings.Index(tag, ","); commaIdx >= 0 {
					if tagName := tag[:commaIdx]; tagName != "" {
						fieldName = tagName
					}
				} else if tag != "" {
					fieldName = tag
				}
			}

			result[fieldName] = f.filterValueRecursiveInternal(fieldName, field.Interface(), visited, depth+1)
		}
		return result
	}

	// For other types, return as-is
	return value
}

type SecurityConfig struct {
	MaxMessageSize  int
	MaxWriters      int
	SensitiveFilter *SensitiveDataFilter
}

// Clone creates a copy of the SecurityConfig.
//
// Deep copy:
//   - SensitiveFilter (via SensitiveDataFilter.Clone())
//
// Returns nil if the receiver is nil.
func (sc *SecurityConfig) Clone() *SecurityConfig {
	if sc == nil {
		return nil
	}

	clone := &SecurityConfig{
		MaxMessageSize: sc.MaxMessageSize,
		MaxWriters:     sc.MaxWriters,
	}
	if sc.SensitiveFilter != nil {
		clone.SensitiveFilter = sc.SensitiveFilter.Clone()
	}
	return clone
}

// NewFullSensitiveDataFilter creates a filter with all built-in sensitive data patterns.
// This is an alias for NewSensitiveDataFilter() with a clearer name indicating
// that it includes all available patterns.
func NewFullSensitiveDataFilter() *SensitiveDataFilter {
	return NewSensitiveDataFilter()
}

func NewBasicSensitiveDataFilter() *SensitiveDataFilter {
	internal.InitPatterns()
	return newSensitiveDataFilterWithPatterns(internal.CompiledBasicPatterns, DefaultFilterTimeout)
}

// DefaultSecurityConfig returns a security config with basic sensitive data filtering enabled.
// This provides out-of-the-box protection for common sensitive data like passwords,
// API keys, credit cards, and phone numbers.
func DefaultSecurityConfig() *SecurityConfig {
	return &SecurityConfig{
		MaxMessageSize:  MaxMessageSize,
		MaxWriters:      MaxWriterCount,
		SensitiveFilter: NewBasicSensitiveDataFilter(),
	}
}

// DefaultSecurityConfigDisabled returns a security config with NO sensitive data filtering.
// This is the default configuration for maximum performance.
// Use this when you need raw logging without any filtering overhead.
func DefaultSecurityConfigDisabled() *SecurityConfig {
	return &SecurityConfig{
		MaxMessageSize:  MaxMessageSize,
		MaxWriters:      MaxWriterCount,
		SensitiveFilter: nil,
	}
}

// DefaultSecureConfig returns a security config with full sensitive data filtering enabled.
// This includes all patterns from basic filtering plus additional patterns for
// emails, IP addresses, JWT tokens, and database connection strings.
// Use this for maximum security in production environments.
func DefaultSecureConfig() *SecurityConfig {
	return &SecurityConfig{
		MaxMessageSize:  MaxMessageSize,
		MaxWriters:      MaxWriterCount,
		SensitiveFilter: NewSensitiveDataFilter(),
	}
}

// SecureConfig returns a security config with full sensitive data filtering enabled.
//
// Deprecated: Use DefaultSecureConfig() instead. This function is kept for backward
// compatibility and will be removed in a future major version.
func SecureConfig() *SecurityConfig {
	return DefaultSecureConfig()
}
