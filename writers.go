package dd

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cybergodev/dd/internal"
)

type FileWriter struct {
	path       string
	maxSize    int64
	maxAge     time.Duration
	maxBackups int
	compress   bool

	mu          sync.Mutex
	file        *os.File
	currentSize atomic.Int64

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

type FileWriterConfig struct {
	MaxSizeMB  int
	MaxAge     time.Duration
	MaxBackups int
	Compress   bool
}

// DefaultFileWriterConfig returns FileWriterConfig with sensible defaults.
// Default values: MaxSizeMB=100, MaxAge=30 days, MaxBackups=10, Compress=false.
func DefaultFileWriterConfig() FileWriterConfig {
	return FileWriterConfig{
		MaxSizeMB:  DefaultMaxSizeMB,
		MaxAge:     DefaultMaxAge,
		MaxBackups: DefaultMaxBackups,
		Compress:   false,
	}
}

func NewFileWriter(path string, config FileWriterConfig) (*FileWriter, error) {
	securePath, err := internal.ValidateAndSecurePath(path, MaxPathLength, ErrEmptyFilePath, ErrNullByte, ErrPathTooLong, ErrPathTraversal, ErrInvalidPath)
	if err != nil {
		return nil, err
	}

	if err := validateFileWriterConfig(&config); err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())

	fw := &FileWriter{
		path:       securePath,
		maxSize:    int64(config.MaxSizeMB) * 1024 * 1024,
		maxAge:     config.MaxAge,
		maxBackups: config.MaxBackups,
		compress:   config.Compress,
		ctx:        ctx,
		cancel:     cancel,
	}

	dir := filepath.Dir(securePath)
	if err := os.MkdirAll(dir, DirPermissions); err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create directory: %w", err)
	}

	file, size, err := internal.OpenFile(securePath)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to open file %s: %w", securePath, err)
	}
	fw.file = file
	fw.currentSize.Store(size)

	if fw.maxAge > 0 && fw.maxBackups > 0 {
		fw.wg.Add(1)
		go fw.cleanupRoutine()
	}

	return fw, nil
}

func validateFileWriterConfig(config *FileWriterConfig) error {
	// Apply defaults for zero/negative MaxSizeMB
	if config.MaxSizeMB <= 0 {
		config.MaxSizeMB = DefaultMaxSizeMB
	}

	// Cleanup is enabled only when at least one cleanup parameter is configured.
	// Apply defaults based on what the user has specified:
	// - Both zero: use full defaults (MaxAge + MaxBackups)
	// - Only MaxBackups set: use count-based cleanup only (MaxAge = 0)
	// - Only MaxAge set: use time-based cleanup with default MaxBackups
	if config.MaxAge == 0 && config.MaxBackups == 0 {
		config.MaxAge = DefaultMaxAge
		config.MaxBackups = DefaultMaxBackups
	} else if config.MaxAge == 0 && config.MaxBackups > 0 {
		// User set MaxBackups but not MaxAge - disable time-based cleanup
		config.MaxAge = 0
	} else if config.MaxAge > 0 && config.MaxBackups == 0 {
		// User set MaxAge but not MaxBackups - use default MaxBackups
		config.MaxBackups = DefaultMaxBackups
	}

	// Validate limits
	if config.MaxSizeMB > MaxFileSizeMB {
		return fmt.Errorf("%w: maximum %dMB", ErrMaxSizeExceeded, MaxFileSizeMB)
	}
	if config.MaxBackups > MaxBackupCount {
		return fmt.Errorf("%w: maximum %d", ErrMaxBackupsExceeded, MaxBackupCount)
	}

	return nil
}

func (fw *FileWriter) Write(p []byte) (int, error) {
	pLen := len(p)
	if pLen == 0 {
		return 0, nil
	}

	fw.mu.Lock()
	defer fw.mu.Unlock()

	if internal.NeedsRotation(fw.currentSize.Load(), int64(pLen), fw.maxSize) {
		if err := fw.rotate(); err != nil {
			return 0, fmt.Errorf("rotation failed: %w", err)
		}
	}

	n, err := fw.file.Write(p)
	if err != nil {
		return n, fmt.Errorf("write failed: %w", err)
	}

	fw.currentSize.Add(int64(n))
	return n, nil
}

func (fw *FileWriter) Close() error {
	fw.cancel()
	fw.wg.Wait()

	fw.mu.Lock()
	defer fw.mu.Unlock()

	if fw.file != nil {
		err := fw.file.Close()
		fw.file = nil
		return err
	}
	return nil
}

func (fw *FileWriter) rotate() error {
	if fw.file != nil {
		if err := fw.file.Close(); err != nil {
			return fmt.Errorf("close file during rotation: %w", err)
		}
		fw.file = nil
	}

	nextIndex := internal.FindNextBackupIndex(fw.path, fw.compress)
	backupPath := internal.GetBackupPath(fw.path, nextIndex, false)

	if err := os.Rename(fw.path, backupPath); err != nil {
		file, size, reopenErr := internal.OpenFile(fw.path)
		if reopenErr != nil {
			return fmt.Errorf("rename to backup failed and cannot reopen file: rename=%w, reopen=%w", err, reopenErr)
		}
		fw.file = file
		fw.currentSize.Store(size)
		return fmt.Errorf("rename to backup: %w", err)
	}

	internal.RotateBackups(fw.path, fw.maxBackups, fw.compress)

	if fw.compress {
		fw.wg.Add(1)
		go fw.compressBackup(backupPath)
	}

	file, size, err := internal.OpenFile(fw.path)
	if err != nil {
		return fmt.Errorf("open new file: %w", err)
	}
	fw.file = file
	fw.currentSize.Store(size)

	return nil
}

func (fw *FileWriter) compressBackup(path string) {
	defer fw.wg.Done()
	if err := internal.CompressFile(path); err != nil {
		fmt.Fprintf(os.Stderr, "dd: compress backup %s: %v\n", path, err)
	}
}

func (fw *FileWriter) cleanupRoutine() {
	defer fw.wg.Done()

	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-fw.ctx.Done():
			return
		case <-ticker.C:
			if err := internal.CleanupOldFiles(fw.path, fw.maxAge); err != nil {
				// Log to stderr as fallback - cleanup errors should not be silent
				fmt.Fprintf(os.Stderr, "dd: cleanup old files %s: %v\n", fw.path, err)
			}
		}
	}
}

// BufferedWriter wraps an io.Writer with buffering capabilities.
// It automatically flushes when the buffer reaches a certain size or after a timeout.
//
// IMPORTANT: Always call Close() when done to ensure all buffered data is flushed.
// Failure to call Close() may result in data loss.
type BufferedWriter struct {
	writer    io.Writer
	buffer    *bufio.Writer
	flushSize int
	flushTime time.Duration

	mu        sync.Mutex
	ctx       context.Context
	cancel    context.CancelFunc
	lastFlush time.Time
	wg        sync.WaitGroup
	closed    atomic.Bool
}

// NewBufferedWriter creates a new BufferedWriter with the specified buffer size.
// The writer automatically flushes when the buffer is half full or every 100ms.
// Remember to call Close() to ensure all buffered data is written to the underlying writer.
func NewBufferedWriter(w io.Writer, bufferSize int) (*BufferedWriter, error) {
	if w == nil {
		return nil, ErrNilWriter
	}

	if bufferSize < DefaultBufferSizeKB*1024 {
		bufferSize = DefaultBufferSizeKB * 1024
	}
	if bufferSize > MaxBufferSizeKB*1024 {
		return nil, fmt.Errorf("%w: maximum %dMB", ErrBufferSizeTooLarge, MaxBufferSizeKB/1024)
	}

	ctx, cancel := context.WithCancel(context.Background())

	bw := &BufferedWriter{
		writer:    w,
		buffer:    bufio.NewWriterSize(w, bufferSize),
		flushSize: bufferSize / AutoFlushThreshold,
		flushTime: AutoFlushInterval,
		ctx:       ctx,
		cancel:    cancel,
		lastFlush: time.Now(),
	}

	bw.wg.Add(1)
	go bw.autoFlushRoutine()

	return bw, nil
}

func (bw *BufferedWriter) Write(p []byte) (int, error) {
	pLen := len(p)
	if pLen == 0 {
		return 0, nil
	}

	bw.mu.Lock()
	defer bw.mu.Unlock()

	n, err := bw.buffer.Write(p)
	if err != nil {
		return n, err
	}

	if bw.buffer.Buffered() >= bw.flushSize {
		if flushErr := bw.buffer.Flush(); flushErr != nil {
			return n, fmt.Errorf("auto-flush failed: %w", flushErr)
		}
		bw.lastFlush = time.Now()
	}

	return n, nil
}

func (bw *BufferedWriter) Flush() error {
	bw.mu.Lock()
	defer bw.mu.Unlock()

	err := bw.buffer.Flush()
	bw.lastFlush = time.Now()
	return err
}

func (bw *BufferedWriter) Close() error {
	if bw == nil {
		return nil
	}
	if !bw.closed.CompareAndSwap(false, true) {
		return nil
	}

	if bw.cancel != nil {
		bw.cancel()
	}

	bw.wg.Wait()

	bw.mu.Lock()
	defer bw.mu.Unlock()

	var errs []error

	if bw.buffer != nil {
		if err := bw.buffer.Flush(); err != nil {
			errs = append(errs, fmt.Errorf("flush: %w", err))
		}
	}

	if closer, ok := bw.writer.(io.Closer); ok {
		if err := closer.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close writer: %w", err))
		}
	}

	return errors.Join(errs...)
}

func (bw *BufferedWriter) autoFlushRoutine() {
	defer bw.wg.Done()

	ticker := time.NewTicker(bw.flushTime)
	defer ticker.Stop()

	for {
		select {
		case <-bw.ctx.Done():
			return
		case <-ticker.C:
			// Check if closed before attempting to flush
			if bw.closed.Load() {
				return
			}
			bw.mu.Lock()
			if bw.buffer.Buffered() > 0 && time.Since(bw.lastFlush) >= bw.flushTime {
				if err := bw.buffer.Flush(); err != nil {
					fmt.Fprintf(os.Stderr, "dd: autoflush error: %v\n", err)
				}
				bw.lastFlush = time.Now()
			}
			bw.mu.Unlock()
		}
	}
}

type MultiWriter struct {
	// writersPtr stores an immutable slice of writers using atomic pointer.
	// This eliminates slice copying during write operations (hot path).
	// The slice is replaced atomically when writers are added/removed.
	writersPtr atomic.Pointer[[]io.Writer]
	mu         sync.Mutex // protects AddWriter/RemoveWriter operations
}

func NewMultiWriter(writers ...io.Writer) *MultiWriter {
	var validWriters []io.Writer
	for _, w := range writers {
		if w != nil {
			validWriters = append(validWriters, w)
		}
	}

	mw := &MultiWriter{}
	mw.writersPtr.Store(&validWriters)
	return mw
}

func (mw *MultiWriter) Write(p []byte) (int, error) {
	pLen := len(p)
	if pLen == 0 {
		return 0, nil
	}

	// Fast path: atomic load of writers pointer (lock-free read)
	writersPtr := mw.writersPtr.Load()
	if writersPtr == nil || len(*writersPtr) == 0 {
		return pLen, nil
	}

	writers := *writersPtr
	writerCount := len(writers)

	// Fast path: single writer optimization
	if writerCount == 1 {
		return writers[0].Write(p)
	}

	// Iterate directly over the immutable slice - no copy needed
	var allErrors MultiWriterError
	successCount := 0

	for i := 0; i < writerCount; i++ {
		n, err := writers[i].Write(p)
		if err != nil {
			allErrors.AddError(i, writers[i], err)
			continue
		}
		if n != pLen {
			allErrors.AddError(i, writers[i], fmt.Errorf("short write (%d/%d bytes)", n, pLen))
			continue
		}
		successCount++
	}

	// If all writers failed, return error
	if successCount == 0 {
		return 0, &allErrors
	}

	// If partial success, return bytes written but include error info
	if allErrors.HasErrors() {
		return pLen, &allErrors
	}

	return pLen, nil
}

func (mw *MultiWriter) AddWriter(w io.Writer) error {
	if mw == nil {
		return fmt.Errorf("%w", ErrNilWriter)
	}
	if w == nil {
		return ErrNilWriter
	}

	mw.mu.Lock()
	defer mw.mu.Unlock()

	// Load current writers slice
	currentWriters := mw.writersPtr.Load()
	if currentWriters == nil {
		return ErrNilWriter
	}

	// Check for duplicates
	for _, existing := range *currentWriters {
		if existing == w {
			return nil // Already exists, not an error
		}
	}

	if len(*currentWriters) >= MaxWriterCount {
		return ErrMaxWritersExceeded
	}

	// Create new slice with the new writer added
	newWriters := make([]io.Writer, len(*currentWriters)+1)
	copy(newWriters, *currentWriters)
	newWriters[len(*currentWriters)] = w

	// Atomically swap the pointer
	mw.writersPtr.Store(&newWriters)
	return nil
}

func (mw *MultiWriter) RemoveWriter(w io.Writer) {
	if mw == nil {
		return
	}

	mw.mu.Lock()
	defer mw.mu.Unlock()

	// Load current writers slice
	currentWriters := mw.writersPtr.Load()
	if currentWriters == nil {
		return
	}

	writerCount := len(*currentWriters)
	for i := 0; i < writerCount; i++ {
		if (*currentWriters)[i] == w {
			// Create new slice without the removed writer
			newWriters := make([]io.Writer, writerCount-1)
			copy(newWriters, (*currentWriters)[:i])
			copy(newWriters[i:], (*currentWriters)[i+1:])

			// Atomically swap the pointer
			mw.writersPtr.Store(&newWriters)
			return
		}
	}
}

func (mw *MultiWriter) Close() error {
	// Load writers atomically
	writersPtr := mw.writersPtr.Load()
	if writersPtr == nil {
		return nil
	}
	writers := *writersPtr

	var errs []error
	for _, w := range writers {
		if closer, ok := w.(io.Closer); ok {
			if err := closer.Close(); err != nil {
				errs = append(errs, err)
			}
		}
	}

	return errors.Join(errs...)
}
