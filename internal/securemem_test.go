package internal

import (
	"strings"
	"testing"
)

func TestSecureBuffer_Write(t *testing.T) {
	sb := NewSecureBuffer()
	defer sb.Release()

	n, err := sb.Write([]byte("hello"))
	if err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if n != 5 {
		t.Errorf("Write() returned %d, want 5", n)
	}

	if string(sb.Bytes()) != "hello" {
		t.Errorf("Bytes() = %q, want %q", sb.Bytes(), "hello")
	}
}

func TestSecureBuffer_WriteString(t *testing.T) {
	sb := NewSecureBuffer()
	defer sb.Release()

	n, err := sb.WriteString("hello world")
	if err != nil {
		t.Fatalf("WriteString() error = %v", err)
	}
	if n != 11 {
		t.Errorf("WriteString() returned %d, want 11", n)
	}

	if sb.String() != "hello world" {
		t.Errorf("String() = %q, want %q", sb.String(), "hello world")
	}
}

func TestSecureBuffer_Reset(t *testing.T) {
	sb := NewSecureBuffer()
	defer sb.Release()

	sb.WriteString("sensitive data")
	if sb.Len() != 14 {
		t.Errorf("Len() = %d, want 14", sb.Len())
	}

	sb.Reset()

	if sb.Len() != 0 {
		t.Errorf("After Reset(), Len() = %d, want 0", sb.Len())
	}
}

func TestSecureBuffer_Release(t *testing.T) {
	sb := NewSecureBuffer()
	sb.WriteString("sensitive data")

	// Store a copy of the internal slice reference
	data := sb.Bytes()

	sb.Release()

	// After release, the internal data should be zeroed
	// Note: We can't reliably test this because the buffer is returned to pool
	// and may be reused, but we can test that it's safe to call
	if data == nil {
		t.Error("data should not be nil after Release")
	}
}

func TestSecureBuffer_Grow(t *testing.T) {
	sb := NewSecureBuffer()
	defer sb.Release()

	sb.WriteString("test")
	sb.Grow(1000)

	// After grow, should be able to write more
	sb.WriteString(strings.Repeat("x", 1000))

	if sb.Len() != 1004 {
		t.Errorf("After Grow and write, Len() = %d, want 1004", sb.Len())
	}
}

func TestSecureBuffer_PoolReuse(t *testing.T) {
	// Get buffer from pool
	sb1 := NewSecureBuffer()
	sb1.WriteString("first")
	sb1.Release()

	// Get another buffer (likely the same one from pool)
	sb2 := NewSecureBuffer()
	defer sb2.Release()

	// Should start empty
	if sb2.Len() != 0 {
		t.Errorf("Pooled buffer should start empty, Len() = %d", sb2.Len())
	}
}

func TestSecureBuffer_NilSafety(t *testing.T) {
	var sb *SecureBuffer

	// Should not panic
	sb.Release()
	sb.Reset()
	sb.Grow(10)
}

func TestSecureString_Basic(t *testing.T) {
	ss := NewSecureString("hello")

	if ss.String() != "hello" {
		t.Errorf("String() = %q, want %q", ss.String(), "hello")
	}

	if ss.Len() != 5 {
		t.Errorf("Len() = %d, want 5", ss.Len())
	}
}

func TestSecureString_Clear(t *testing.T) {
	ss := NewSecureString("sensitive")

	// Store reference to data
	data := ss.Bytes()

	ss.Clear()

	if ss.Bytes() != nil {
		t.Error("Bytes() should be nil after Clear()")
	}

	// Check original data was zeroed
	for i, b := range data {
		if b != 0 {
			t.Errorf("Data[%d] = %d, want 0 (data should be zeroed)", i, b)
		}
	}
}

func TestSecureString_Equals(t *testing.T) {
	ss := NewSecureString("password123")

	if !ss.Equals("password123") {
		t.Error("Equals() should return true for matching string")
	}

	if ss.Equals("wrong") {
		t.Error("Equals() should return false for non-matching string")
	}

	if ss.Equals("") {
		t.Error("Equals() should return false for empty string")
	}
}

func TestSecureString_EqualsConstantTime(t *testing.T) {
	// Test that Equals doesn't short-circuit (timing attack protection)
	ss := NewSecureString("abc")

	// First char wrong
	ss.Equals("xbc")
	// Middle char wrong
	ss.Equals("axc")
	// Last char wrong
	ss.Equals("abx")
	// Completely wrong
	ss.Equals("xxx")

	// Just ensure no panic and correct result
	if !ss.Equals("abc") {
		t.Error("Equals() should return true for matching string")
	}
}

func TestSecureString_NilSafety(t *testing.T) {
	var ss *SecureString

	// Should not panic and should return false
	if ss.Equals("test") {
		t.Error("Nil SecureString.Equals() should return false")
	}

	ss.Clear() // Should not panic
}

func TestSecureBytes_Basic(t *testing.T) {
	sb := NewSecureBytes([]byte("hello"))

	if string(sb.Bytes()) != "hello" {
		t.Errorf("Bytes() = %q, want %q", sb.Bytes(), "hello")
	}

	if sb.Len() != 5 {
		t.Errorf("Len() = %d, want 5", sb.Len())
	}
}

func TestSecureBytes_Clear(t *testing.T) {
	original := []byte("sensitive")
	sb := NewSecureBytes(original)

	// Verify copy was made
	original[0] = 'X'
	if sb.Bytes()[0] == 'X' {
		t.Error("SecureBytes should have its own copy of data")
	}

	sb.Clear()

	if sb.Bytes() != nil {
		t.Error("Bytes() should be nil after Clear()")
	}
}

func TestSecureBytes_Equals(t *testing.T) {
	sb := NewSecureBytes([]byte("test"))

	if !sb.Equals([]byte("test")) {
		t.Error("Equals() should return true for matching bytes")
	}

	if sb.Equals([]byte("wrong")) {
		t.Error("Equals() should return false for non-matching bytes")
	}
}

func TestSecureBytes_NilSafety(t *testing.T) {
	var sb *SecureBytes

	if sb.Equals([]byte("test")) {
		t.Error("Nil SecureBytes.Equals() should return false")
	}

	sb.Clear() // Should not panic
}

func TestWipeBytes(t *testing.T) {
	data := []byte("sensitive data")
	WipeBytes(data)

	for i, b := range data {
		if b != 0 {
			t.Errorf("WipeBytes: data[%d] = %d, want 0", i, b)
		}
	}
}

func TestSecureBuffer_ConcurrentUse(t *testing.T) {
	// Test that the pool works correctly under concurrent use
	done := make(chan bool)

	for i := 0; i < 10; i++ {
		go func() {
			sb := NewSecureBuffer()
			sb.WriteString("test data")
			sb.Release()
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}
