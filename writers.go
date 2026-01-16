package dd

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
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

func NewFileWriter(path string, config FileWriterConfig) (*FileWriter, error) {
	securePath, err := validateAndSecurePath(path)
	if err != nil {
		return nil, err
	}

	if err := validateFileWriterConfig(&config); err != nil {
		return nil, err
	}

	if config.MaxAge > 0 && config.MaxBackups <= 0 {
		return nil, fmt.Errorf("MaxAge is set but MaxBackups is not configured, set MaxBackups to enable cleanup")
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

func validateAndSecurePath(path string) (string, error) {
	if path == "" {
		return "", ErrEmptyFilePath
	}

	if strings.Contains(path, "\x00") {
		return "", ErrNullByte
	}

	if len(path) > MaxPathLength {
		return "", fmt.Errorf("%w (max %d characters)", ErrPathTooLong, MaxPathLength)
	}

	cleanPath := filepath.Clean(path)
	if strings.Contains(cleanPath, "..") {
		return "", ErrPathTraversal
	}

	// Convert to absolute path
	// Note: Symlink checking is done AFTER opening the file in internal.OpenFile
	// to prevent TOCTOU (time-of-check-time-of-use) vulnerabilities
	absPath, err := filepath.Abs(cleanPath)
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrInvalidPath, err)
	}

	return absPath, nil
}

func validateFileWriterConfig(config *FileWriterConfig) error {
	maxSize := config.MaxSizeMB
	if maxSize <= 0 {
		maxSize = DefaultMaxSizeMB
	}
	maxAge := config.MaxAge
	if maxAge <= 0 {
		maxAge = DefaultMaxAge
	}
	maxBackups := config.MaxBackups
	if maxBackups < 0 {
		maxBackups = DefaultMaxBackups
	}

	if maxSize > MaxFileSizeMB {
		return fmt.Errorf("%w: maximum %dMB", ErrMaxSizeExceeded, MaxFileSizeMB)
	}
	if maxBackups > MaxBackupCount {
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
			internal.CleanupOldFiles(fw.path, fw.maxAge)
		}
	}
}

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

	if bw.buffer != nil {
		if err := bw.buffer.Flush(); err != nil {
			return err
		}
	}

	if closer, ok := bw.writer.(io.Closer); ok {
		return closer.Close()
	}

	return nil
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
			bw.mu.Lock()
			if bw.buffer.Buffered() > 0 && time.Since(bw.lastFlush) >= bw.flushTime {
				_ = bw.buffer.Flush()
				bw.lastFlush = time.Now()
			}
			bw.mu.Unlock()
		}
	}
}

type MultiWriter struct {
	writers []io.Writer
	mu      sync.RWMutex
}

func NewMultiWriter(writers ...io.Writer) *MultiWriter {
	var validWriters []io.Writer
	for _, w := range writers {
		if w != nil {
			validWriters = append(validWriters, w)
		}
	}

	return &MultiWriter{
		writers: validWriters,
	}
}

func (mw *MultiWriter) Write(p []byte) (int, error) {
	pLen := len(p)
	if pLen == 0 {
		return 0, nil
	}

	mw.mu.RLock()
	writerCount := len(mw.writers)
	if writerCount == 0 {
		mw.mu.RUnlock()
		return pLen, nil
	}

	// Fast path: single writer optimization
	if writerCount == 1 {
		w := mw.writers[0]
		mw.mu.RUnlock()
		return w.Write(p)
	}

	writers := make([]io.Writer, writerCount)
	copy(writers, mw.writers)
	mw.mu.RUnlock()

	var firstErr error
	successCount := 0

	for i := 0; i < writerCount; i++ {
		n, err := writers[i].Write(p)
		if err != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("writer[%d]: %w", i, err)
			}
			continue
		}
		if n != pLen {
			if firstErr == nil {
				firstErr = fmt.Errorf("writer[%d]: short write (%d/%d bytes)", i, n, pLen)
			}
			continue
		}
		successCount++
	}

	// If all writers failed, return error
	if successCount == 0 {
		return 0, firstErr
	}

	// If partial success, return bytes written but include error info
	if firstErr != nil {
		return pLen, fmt.Errorf("partial write failure (%d/%d succeeded): %w", successCount, writerCount, firstErr)
	}

	return pLen, nil
}

func (mw *MultiWriter) AddWriter(w io.Writer) {
	if w == nil || mw == nil {
		return
	}

	mw.mu.Lock()
	defer mw.mu.Unlock()

	// Check for duplicates and max limit
	for _, existing := range mw.writers {
		if existing == w {
			return // Already exists
		}
	}

	if len(mw.writers) >= MaxWriterCount {
		return // Silently ignore if max reached
	}

	mw.writers = append(mw.writers, w)
}

func (mw *MultiWriter) RemoveWriter(w io.Writer) {
	if mw == nil {
		return
	}

	mw.mu.Lock()
	defer mw.mu.Unlock()

	for i := 0; i < len(mw.writers); i++ {
		if mw.writers[i] == w {
			// Prevent memory leak by clearing reference
			mw.writers[i] = mw.writers[len(mw.writers)-1]
			mw.writers[len(mw.writers)-1] = nil
			mw.writers = mw.writers[:len(mw.writers)-1]
			break
		}
	}
}

func (mw *MultiWriter) Close() error {
	mw.mu.RLock()
	writers := make([]io.Writer, len(mw.writers))
	copy(writers, mw.writers)
	mw.mu.RUnlock()

	var lastErr error
	for _, w := range writers {
		if closer, ok := w.(io.Closer); ok {
			if err := closer.Close(); err != nil {
				lastErr = err
			}
		}
	}

	return lastErr
}
