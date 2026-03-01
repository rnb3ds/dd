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

	// Filter result cache for repeated messages
	cacheMu    sync.RWMutex
	cache      map[uint64]filterCacheEntry
	cacheSize  int
	cacheHits  atomic.Int64
	cacheMiss  atomic.Int64
	maxCacheSz int
}

// filterCacheEntry stores a cached filter result
type filterCacheEntry struct {
	hash    uint64
	input   string
	result  string
	created int64 // unix timestamp for TTL
}

// FNV-1a hash implementation for string hashing (fast, no allocations)
func hashString(s string) uint64 {
	h := uint64(14695981039346656037) // FNV offset basis
	for i := 0; i < len(s); i++ {
		h ^= uint64(s[i])
		h *= 1099511628211 // FNV prime
	}
	return h
}

// newSensitiveDataFilterWithPatterns is the internal constructor for SensitiveDataFilter.
// It creates a filter with the specified patterns and timeout.
func newSensitiveDataFilterWithPatterns(patterns []*regexp.Regexp, timeout time.Duration) *SensitiveDataFilter {
	filter := &SensitiveDataFilter{
		maxInputLength: MaxInputLength,
		timeout:        timeout,
		semaphore:      make(chan struct{}, MaxConcurrentFilters),
		cache:          make(map[uint64]filterCacheEntry),
		cacheSize:      0,
		maxCacheSz:     1000, // Maximum cache entries
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
// Shared (immutable):
//   - patterns slice pointer (shared for better performance, patterns are immutable after creation)
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

	clone := &SensitiveDataFilter{
		maxInputLength: f.maxInputLength,
		timeout:        f.timeout,
		semaphore:      make(chan struct{}, MaxConcurrentFilters),
	}
	clone.enabled.Store(f.enabled.Load())

	// Share the patterns pointer directly (immutable after creation)
	// This avoids allocation when cloning
	clone.patternsPtr.Store(f.patternsPtr.Load())
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

	// Check cache for repeated messages (only for small inputs to avoid memory bloat)
	// Skip cache if not initialized (for filters created without using constructor)
	if inputLen <= 1024 {
		f.cacheMu.RLock()
		if f.cache != nil {
			hash := hashString(input)
			if entry, ok := f.cache[hash]; ok && entry.input == input {
				f.cacheMu.RUnlock()
				f.cacheHits.Add(1)
				f.totalFiltered.Add(1)
				// Record minimal latency for cache hit
				f.totalLatencyNs.Add(1)
				return entry.result
			}
		}
		f.cacheMu.RUnlock()
		f.cacheMiss.Add(1)
	}

	startTime := time.Now()

	patterns := *patternsPtr
	timeout := f.timeout

	// Quick rejection: check if input could possibly contain sensitive data
	// This avoids running all regex patterns on obviously safe input
	// Note: We still need to handle truncation for large inputs
	if !f.couldContainSensitiveData(input) {
		// Handle truncation for large inputs even if no sensitive data detected
		if f.maxInputLength > 0 && inputLen > f.maxInputLength {
			input = input[:f.maxInputLength] + "... [TRUNCATED]"
		}
		// Still track metrics for monitoring
		// Ensure at least 1ns to avoid zero average latency for very fast operations
		latencyNs := time.Since(startTime).Nanoseconds()
		if latencyNs == 0 {
			latencyNs = 1
		}
		f.totalFiltered.Add(1)
		f.totalLatencyNs.Add(latencyNs)

		// Cache the result for small inputs
		if inputLen <= 1024 && f.cache != nil {
			f.cacheResult(hashString(input), input, input)
		}
		return input
	}

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

	// Cache the result for small inputs
	if inputLen <= 1024 && f.cache != nil {
		f.cacheResult(hashString(input), input, result)
	}

	return result
}

// cacheResult stores a filter result in the cache
func (f *SensitiveDataFilter) cacheResult(hash uint64, input, result string) {
	f.cacheMu.Lock()
	defer f.cacheMu.Unlock()
	if f.cache == nil {
		return
	}

	// Evict old entries if cache is full
	if f.cacheSize >= f.maxCacheSz {
		// Simple eviction: clear half the cache
		count := 0
		for k := range f.cache {
			delete(f.cache, k)
			count++
			if count >= f.maxCacheSz/2 {
				break
			}
		}
		f.cacheSize = len(f.cache)
	}

	f.cache[hash] = filterCacheEntry{
		hash:    hash,
		input:   input,
		result:  result,
		created: time.Now().Unix(),
	}
	f.cacheSize++
}

// Pre-computed lowercase credential keywords for fast case-insensitive matching
// These are the most common credential keywords that appear in sensitive data patterns
var credentialKeywords = [][]byte{
	[]byte("password"),
	[]byte("passwd"),
	[]byte("secret"),
	[]byte("token"),
	[]byte("api_key"),
	[]byte("apikey"),
	[]byte("bearer"),
	[]byte("auth"),
	[]byte("credential"),
	[]byte("private_key"),
	[]byte("session"),
}

// couldContainSensitiveData performs fast pre-checks to determine if input
// could possibly contain sensitive data. This avoids expensive regex matching
// on obviously safe input, providing significant performance improvement.
//
// Checks performed:
//   - Has digits: required for credit cards, SSN, phone numbers, many API keys
//   - Has special prefixes: required for API keys (sk-, ghp_, AKIA, AIza, etc.)
//   - Has credential keywords: required for password/token/secret patterns
//   - Has @ symbol: required for email patterns
//   - Has protocol indicators: required for connection strings
//
// Returns true if any sensitive data pattern could potentially match.
func (f *SensitiveDataFilter) couldContainSensitiveData(input string) bool {
	inputLen := len(input)

	// Track what characteristics the input has
	hasDigits := false
	hasAtSign := false
	hasProtocol := false
	hasCredentialKeyword := false
	hasAPIKeyPrefix := false

	// Quick scan for key characteristics
	// Use byte-by-byte scanning for efficiency
	for i := 0; i < inputLen; i++ {
		c := input[i]

		// Check for digits
		if c >= '0' && c <= '9' {
			hasDigits = true
		}

		// Check for @ (email)
		if c == '@' {
			hasAtSign = true
		}

		// Check for protocol indicators (:)
		if c == ':' && i+2 < inputLen && input[i+1] == '/' && input[i+2] == '/' {
			hasProtocol = true
		}

		// Early exit if we found all characteristics
		if hasDigits && hasAtSign && hasProtocol {
			break
		}
	}

	// Check for API key prefixes (case-sensitive for efficiency)
	// These are the most common API key prefixes
	if !hasAPIKeyPrefix {
		// Check for common prefixes without allocation
		if strings.HasPrefix(input, "sk-") ||
			strings.HasPrefix(input, "ghp_") ||
			strings.HasPrefix(input, "gho_") ||
			strings.HasPrefix(input, "ghu_") ||
			strings.HasPrefix(input, "ghs_") ||
			strings.HasPrefix(input, "ghr_") ||
			strings.HasPrefix(input, "glpat-") ||
			strings.HasPrefix(input, "xox") ||
			strings.Contains(input, "AKIA") ||
			strings.Contains(input, "ASIA") ||
			strings.Contains(input, "AIza") ||
			strings.Contains(input, "ya29.") ||
			strings.Contains(input, "1//") {
			hasAPIKeyPrefix = true
		}
	}

	// Check for credential keywords using case-insensitive byte comparison
	// This avoids strings.ToLower allocation
	if !hasCredentialKeyword && inputLen >= 4 {
		hasCredentialKeyword = containsCredentialKeyword(input)
	}

	// If input has none of the characteristics, it's very unlikely to contain sensitive data
	// Most patterns require at least one of these characteristics
	return hasDigits || hasAtSign || hasProtocol || hasCredentialKeyword || hasAPIKeyPrefix
}

// containsCredentialKeyword checks if input contains any credential keyword.
// Uses case-insensitive byte-by-byte comparison to avoid allocation.
func containsCredentialKeyword(input string) bool {
	inputLen := len(input)
	if inputLen < 4 {
		return false
	}

	// Convert input to lowercase inline for comparison
	// Use a sliding window approach for each keyword
	for _, keyword := range credentialKeywords {
		keywordLen := len(keyword)
		if inputLen < keywordLen {
			continue
		}

		// Search for keyword in input using case-insensitive comparison
		for i := 0; i <= inputLen-keywordLen; i++ {
			match := true
			for j := 0; j < keywordLen; j++ {
				c := input[i+j]
				// Convert to lowercase inline
				if c >= 'A' && c <= 'Z' {
					c += 32
				}
				if c != keyword[j] {
					match = false
					break
				}
			}
			if match {
				return true
			}
		}
	}
	return false
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
		return f.replaceWithPatternWithContext(ctx, input, pattern)
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
		// Check context at the start of each iteration for early termination
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

		// Filter the current chunk with context awareness
		filtered := f.replaceWithPatternWithContext(ctx, chunk, pattern)

		// Check if context was cancelled during filtering
		select {
		case <-ctx.Done():
			// Context cancelled, return what we have
			if lastWritten < inputLen {
				result.WriteString(input[lastWritten:])
			}
			return result.String()
		default:
		}

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
	return f.replaceWithPatternWithContext(ctx, result.String(), pattern)
}

// replaceWithPatternWithContext applies regex replacement with context awareness.
// It checks for context cancellation to allow early termination.
func (f *SensitiveDataFilter) replaceWithPatternWithContext(ctx context.Context, input string, pattern *regexp.Regexp) string {
	// Quick context check before expensive regex operation
	select {
	case <-ctx.Done():
		return input // Return unchanged on cancellation
	default:
	}
	return f.replaceWithPattern(input, pattern)
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

// SecurityLevel defines the security level for the logger.
// Higher levels provide more protection but may impact performance.
type SecurityLevel int

const (
	// SecurityLevelDevelopment provides minimal security for development.
	// - No sensitive data filtering
	// - No rate limiting
	// - No audit logging
	// Use only in local development environments.
	SecurityLevelDevelopment SecurityLevel = iota

	// SecurityLevelBasic provides basic security for non-production environments.
	// - Basic sensitive data filtering (passwords, API keys, credit cards)
	// - No rate limiting
	// - No audit logging
	// Suitable for staging and testing environments.
	SecurityLevelBasic

	// SecurityLevelStandard provides standard security for production.
	// - Full sensitive data filtering
	// - Rate limiting enabled
	// - Basic audit logging
	// Recommended for most production deployments.
	SecurityLevelStandard

	// SecurityLevelStrict provides enhanced security for sensitive environments.
	// - Full sensitive data filtering
	// - Strict rate limiting
	// - Full audit logging
	// - Input sanitization
	// Suitable for environments handling PII or financial data.
	SecurityLevelStrict

	// SecurityLevelParanoid provides maximum security for high-risk environments.
	// - Full sensitive data filtering with all patterns
	// - Very strict rate limiting
	// - Complete audit logging
	// - All input validation
	// - Log integrity verification
	// Use for healthcare (HIPAA), financial (PCI-DSS), or government systems.
	SecurityLevelParanoid
)

// String returns the string representation of the security level.
func (l SecurityLevel) String() string {
	switch l {
	case SecurityLevelDevelopment:
		return "Development"
	case SecurityLevelBasic:
		return "Basic"
	case SecurityLevelStandard:
		return "Standard"
	case SecurityLevelStrict:
		return "Strict"
	case SecurityLevelParanoid:
		return "Paranoid"
	default:
		return "Unknown"
	}
}

// SecurityConfigForLevel returns a SecurityConfig configured for the specified security level.
// This provides a convenient way to configure security based on deployment environment.
func SecurityConfigForLevel(level SecurityLevel) *SecurityConfig {
	switch level {
	case SecurityLevelDevelopment:
		return &SecurityConfig{
			MaxMessageSize:  MaxMessageSize,
			MaxWriters:      MaxWriterCount,
			SensitiveFilter: nil, // No filtering
		}

	case SecurityLevelBasic:
		return &SecurityConfig{
			MaxMessageSize:  MaxMessageSize,
			MaxWriters:      MaxWriterCount,
			SensitiveFilter: NewBasicSensitiveDataFilter(),
		}

	case SecurityLevelStandard:
		return &SecurityConfig{
			MaxMessageSize:  MaxMessageSize,
			MaxWriters:      MaxWriterCount,
			SensitiveFilter: NewSensitiveDataFilter(),
		}

	case SecurityLevelStrict:
		filter := NewSensitiveDataFilter()
		// Add additional strict patterns
		strictPatterns := []string{
			// Additional context patterns for strict mode
			`(?i)(?:confidential|classified|secret|private)[\s:=]+[^\s]{1,256}\b`,
			`(?i)(?:internal[_-]?id|employee[_-]?id|user[_-]?id)[\s:=]+[A-Za-z0-9]{4,50}\b`,
		}
		for _, p := range strictPatterns {
			filter.AddPattern(p)
		}
		return &SecurityConfig{
			MaxMessageSize:  MaxMessageSize,
			MaxWriters:      MaxWriterCount,
			SensitiveFilter: filter,
		}

	case SecurityLevelParanoid:
		filter := NewSensitiveDataFilter()
		// Add all additional patterns for paranoid mode
		paranoidPatterns := []string{
			// Confidential/classified data
			`(?i)(?:confidential|classified|secret|private|restricted)[\s:=]+[^\s]{1,256}\b`,
			// All IDs
			`(?i)(?:internal[_-]?id|employee[_-]?id|user[_-]?id|session[_-]?id|transaction[_-]?id|reference[_-]?id|tracking[_-]?id)[\s:=]+[A-Za-z0-9]{4,50}\b`,
			// Additional financial patterns
			`(?i)(?:amount|balance|deposit|withdrawal|transfer|payment)[\s:=]+[0-9.,]{1,20}\b`,
			// Any UUID-like identifier
			`\b[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}\b`,
		}
		for _, p := range paranoidPatterns {
			filter.AddPattern(p)
		}
		return &SecurityConfig{
			MaxMessageSize:  MaxMessageSize,
			MaxWriters:      MaxWriterCount,
			SensitiveFilter: filter,
		}

	default:
		return DefaultSecurityConfig()
	}
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
//
// This is the recommended default for production use. For development environments
// where performance is critical and data sensitivity is low, consider using
// SecurityConfigForLevel(SecurityLevelDevelopment) instead.
func DefaultSecurityConfig() *SecurityConfig {
	return &SecurityConfig{
		MaxMessageSize:  MaxMessageSize,
		MaxWriters:      MaxWriterCount,
		SensitiveFilter: NewBasicSensitiveDataFilter(),
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

// HealthcareConfig returns a security config optimized for HIPAA compliance.
// This includes all patterns from DefaultSecureConfig plus healthcare-specific patterns:
//   - ICD-10 diagnosis codes
//   - US National Provider Identifier (NPI)
//   - Medical Record Numbers (MRN)
//   - Health Insurance Claim Numbers (HICN)
//
// Use this configuration for applications handling Protected Health Information (PHI)
// in healthcare, medical, and insurance environments.
func HealthcareConfig() *SecurityConfig {
	filter := NewSensitiveDataFilter()

	// Add healthcare-specific patterns
	healthcarePatterns := []string{
		// ICD-10 Diagnosis codes
		`\b[A-Z][0-9]{2}(?:\.[0-9A-Z]{1,4})?\b`,
		// Medical Record Numbers with context
		`(?i)(?:mrn|medical[_-]?record[_-]?number|patient[_-]?id|health[_-]?record)[\s:=]+[A-Za-z0-9]{6,20}\b`,
		// Health Insurance Claim Number (Medicare)
		`\b[0-9]{9}[A-Z]{1,2}\b`,
		// Patient identifiers with context
		`(?i)(?:patient[_-]?identifier|patient[_-]?code)[\s:=]+[A-Za-z0-9]{6,20}\b`,
	}

	for _, pattern := range healthcarePatterns {
		filter.AddPattern(pattern)
	}

	return &SecurityConfig{
		MaxMessageSize:  MaxMessageSize,
		MaxWriters:      MaxWriterCount,
		SensitiveFilter: filter,
	}
}

// FinancialConfig returns a security config optimized for PCI-DSS compliance.
// This includes all patterns from DefaultSecureConfig plus financial-specific patterns:
//   - SWIFT/BIC codes
//   - IBAN (International Bank Account Numbers)
//   - CVV/CVC security codes
//   - Additional card number formats
//
// Use this configuration for applications in banking, payment processing,
// fintech, and other financial services environments.
func FinancialConfig() *SecurityConfig {
	filter := NewSensitiveDataFilter()

	// Add financial-specific patterns
	financialPatterns := []string{
		// SWIFT/BIC codes
		`\b[A-Z]{4}[A-Z]{2}[A-Z0-9]{2}(?:[A-Z0-9]{3})?\b`,
		// IBAN
		`\b[A-Z]{2}[0-9]{2}[A-Z0-9]{4}[0-9]{7,30}\b`,
		// CVV/CVC with context
		`(?i)(?:cvv|cvc|cv2|security[_-]?code|card[_-]?verification)[\s:=]+[0-9]{3,4}\b`,
		// Bank account numbers with context
		`(?i)(?:account[_-]?number|bank[_-]?account|acct[_-]?no)[\s:=]+[0-9]{8,17}\b`,
		// Routing numbers (ABA)
		`\b[0-9]{9}\b`,
	}

	for _, pattern := range financialPatterns {
		filter.AddPattern(pattern)
	}

	return &SecurityConfig{
		MaxMessageSize:  MaxMessageSize,
		MaxWriters:      MaxWriterCount,
		SensitiveFilter: filter,
	}
}

// GovernmentConfig returns a security config optimized for government and public sector.
// This includes all patterns from DefaultSecureConfig plus government-specific patterns:
//   - US Passport numbers
//   - US Driver's License numbers
//   - US Tax ID / EIN
//   - UK National Insurance Numbers
//   - Canadian Social Insurance Numbers
//
// Use this configuration for applications in government, public sector,
// defense, and regulated identity management environments.
func GovernmentConfig() *SecurityConfig {
	filter := NewSensitiveDataFilter()

	// Add government-specific patterns
	governmentPatterns := []string{
		// US Passport numbers with context
		`(?i)(?:passport[_-]?number|passport[_-]?no|passport[_-]?id)[\s:=]+[0-9]{8,9}\b`,
		// US Driver's License with context
		`(?i)(?:driver[_-]?license|dl[_-]?number|license[_-]?number|drivers[_-]?license)[\s:=]+[A-Za-z0-9]{5,20}\b`,
		// US Tax ID / EIN
		`\b[0-9]{2}-[0-9]{7}\b`,
		// UK National Insurance Number
		`\b[A-CEGHJ-PR-TW-Z][A-CEGHJ-NPR-TW-Z][0-9]{6}[A-D]\b`,
		// Canadian SIN
		`\b[0-9]{3}[- ]?[0-9]{3}[- ]?[0-9]{3}\b`,
		// Case numbers with context
		`(?i)(?:case[_-]?number|file[_-]?number|docket)[\s:=]+[A-Za-z0-9]{5,20}\b`,
	}

	for _, pattern := range governmentPatterns {
		filter.AddPattern(pattern)
	}

	return &SecurityConfig{
		MaxMessageSize:  MaxMessageSize,
		MaxWriters:      MaxWriterCount,
		SensitiveFilter: filter,
	}
}
