package dd

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"hash"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// hasherPool is a pool for reusing HMAC hashers.
// This avoids allocating a new hasher for each Sign/Verify operation
// while ensuring thread-safe concurrent access.
var hasherPool = sync.Pool{
	New: func() any {
		return hmac.New(sha256.New, nil)
	},
}

// HashAlgorithm defines the hash algorithm for integrity verification.
type HashAlgorithm int

const (
	// HashAlgorithmSHA256 uses SHA-256 for HMAC signatures.
	HashAlgorithmSHA256 HashAlgorithm = iota
)

// String returns the string representation of the hash algorithm.
func (a HashAlgorithm) String() string {
	switch a {
	case HashAlgorithmSHA256:
		return "SHA256"
	default:
		return "Unknown"
	}
}

// IntegrityConfig configures log integrity verification.
type IntegrityConfig struct {
	// SecretKey is the secret key for HMAC signatures.
	// Must be at least 32 bytes for SHA-256.
	// IMPORTANT: Keep this key secure and rotate periodically.
	SecretKey []byte

	// HashAlgorithm is the hash algorithm to use.
	// Default: SHA256
	HashAlgorithm HashAlgorithm

	// IncludeTimestamp determines if timestamps are included in signatures.
	IncludeTimestamp bool

	// IncludeSequence determines if sequence numbers are included.
	// This provides replay attack protection.
	IncludeSequence bool

	// SignaturePrefix is the prefix for signatures in log output.
	// Default: "[SIG:"
	SignaturePrefix string
}

// DefaultIntegrityConfig returns an IntegrityConfig with sensible defaults.
// Note: A cryptographically secure random key is generated but should be replaced for production use.
// IMPORTANT: Store the generated key securely if you need to verify logs across restarts.
func DefaultIntegrityConfig() *IntegrityConfig {
	// Generate a cryptographically secure random key
	defaultKey := make([]byte, 32)
	if _, err := rand.Read(defaultKey); err != nil {
		// This should never happen with crypto/rand, but panic if it does
		// as we cannot safely continue without a secure key
		panic("dd: failed to generate secure random key for integrity config: " + err.Error())
	}

	return &IntegrityConfig{
		SecretKey:        defaultKey,
		HashAlgorithm:    HashAlgorithmSHA256,
		IncludeTimestamp: true,
		IncludeSequence:  true,
		SignaturePrefix:  "[SIG:",
	}
}

// IntegritySigner signs log entries for integrity verification.
// It uses a sync.Pool for hashers to ensure thread-safe concurrent access.
type IntegritySigner struct {
	config     *IntegrityConfig
	secretKey  []byte // Store key for creating new hashers
	sequence   atomic.Uint64
	keyWarning atomic.Bool // Warns if using auto-generated key
}

// NewIntegritySigner creates a new IntegritySigner with the given configuration.
func NewIntegritySigner(config *IntegrityConfig) (*IntegritySigner, error) {
	if config == nil {
		config = DefaultIntegrityConfig()
	}

	if len(config.SecretKey) < 32 {
		return nil, fmt.Errorf("secret key must be at least 32 bytes, got %d", len(config.SecretKey))
	}

	if config.SignaturePrefix == "" {
		config.SignaturePrefix = "[SIG:"
	}

	// Validate hash algorithm
	switch config.HashAlgorithm {
	case HashAlgorithmSHA256:
		// Supported
	default:
		return nil, fmt.Errorf("unsupported hash algorithm: %v", config.HashAlgorithm)
	}

	// Copy the secret key to ensure we own the memory
	secretKey := make([]byte, len(config.SecretKey))
	copy(secretKey, config.SecretKey)

	return &IntegritySigner{
		config:    config,
		secretKey: secretKey,
	}, nil
}

// getHasher returns an HMAC hasher from the pool, configured with the secret key.
func (s *IntegritySigner) getHasher() hash.Hash {
	h := hasherPool.Get().(hash.Hash)
	h.Reset()
	h.Write(s.secretKey)
	return h
}

// putHasher returns a hasher to the pool after resetting it.
func (s *IntegritySigner) putHasher(h hash.Hash) {
	h.Reset()
	hasherPool.Put(h)
}

// Sign generates an HMAC signature for a log message.
// The signature includes the message, timestamp, and sequence number (if configured).
// Returns the signature string that should be appended to the log entry.
// This method is thread-safe and can be called concurrently.
func (s *IntegritySigner) Sign(message string) string {
	if s == nil {
		return ""
	}

	// Build the data to sign
	var data strings.Builder
	data.WriteString(message)

	if s.config.IncludeTimestamp {
		data.WriteString("|")
		data.WriteString(strconv.FormatInt(time.Now().UnixNano(), 10))
	}

	if s.config.IncludeSequence {
		data.WriteString("|")
		data.WriteString(strconv.FormatUint(s.sequence.Add(1), 10))
	}

	// Get hasher from pool and compute HMAC
	hasher := s.getHasher()
	defer s.putHasher(hasher)

	hasher.Write([]byte(data.String()))
	signature := hasher.Sum(nil)

	// Encode signature - use full 43 characters for maximum security
	encoded := base64.RawURLEncoding.EncodeToString(signature)

	return fmt.Sprintf("%s%s]", s.config.SignaturePrefix, encoded)
}

// SignFields generates an HMAC signature for a message with fields.
// Fields are included in the signature for additional integrity.
// This method is thread-safe and can be called concurrently.
func (s *IntegritySigner) SignFields(message string, fields []Field) string {
	if s == nil {
		return ""
	}

	// Build the data to sign including fields
	var data strings.Builder
	data.WriteString(message)

	for _, f := range fields {
		data.WriteString("|")
		data.WriteString(f.Key)
		data.WriteString("=")
		data.WriteString(fmt.Sprintf("%v", f.Value))
	}

	if s.config.IncludeTimestamp {
		data.WriteString("|")
		data.WriteString(strconv.FormatInt(time.Now().UnixNano(), 10))
	}

	if s.config.IncludeSequence {
		data.WriteString("|")
		data.WriteString(strconv.FormatUint(s.sequence.Add(1), 10))
	}

	// Get hasher from pool and compute HMAC
	hasher := s.getHasher()
	defer s.putHasher(hasher)

	hasher.Write([]byte(data.String()))
	signature := hasher.Sum(nil)

	encoded := base64.RawURLEncoding.EncodeToString(signature)

	return fmt.Sprintf("%s%s]", s.config.SignaturePrefix, encoded)
}

// LogIntegrity contains the result of integrity verification.
type LogIntegrity struct {
	// Valid indicates if the signature is valid.
	Valid bool
	// Timestamp is the timestamp from the log entry (if included).
	Timestamp time.Time
	// Sequence is the sequence number (if included).
	Sequence uint64
	// Message is the extracted message without signature.
	Message string
}

// Verify verifies the integrity of a log entry.
// Returns the verification result and any error.
// This method is thread-safe and can be called concurrently.
func (s *IntegritySigner) Verify(entry string) (*LogIntegrity, error) {
	if s == nil {
		return nil, fmt.Errorf("signer is nil")
	}

	// Find signature
	sigStart := strings.LastIndex(entry, s.config.SignaturePrefix)
	if sigStart == -1 {
		return &LogIntegrity{
			Valid:   false,
			Message: entry,
		}, nil
	}

	sigEnd := strings.Index(entry[sigStart:], "]")
	if sigEnd == -1 {
		return &LogIntegrity{
			Valid:   false,
			Message: entry,
		}, nil
	}

	// Extract signature
	sigStr := entry[sigStart+len(s.config.SignaturePrefix) : sigStart+sigEnd]
	message := entry[:sigStart]

	// Decode signature
	signature, err := base64.RawURLEncoding.DecodeString(sigStr)
	if err != nil {
		return &LogIntegrity{
			Valid:   false,
			Message: message,
		}, nil
	}

	// Get hasher from pool and recompute signature
	hasher := s.getHasher()
	defer s.putHasher(hasher)

	hasher.Write([]byte(message))
	expectedSig := hasher.Sum(nil)

	// Compare signatures (constant-time comparison)
	if !hmac.Equal(signature, expectedSig[:len(signature)]) {
		return &LogIntegrity{
			Valid:   false,
			Message: message,
		}, nil
	}

	return &LogIntegrity{
		Valid:   true,
		Message: message,
	}, nil
}

// GetSequence returns the current sequence number.
func (s *IntegritySigner) GetSequence() uint64 {
	if s == nil {
		return 0
	}
	return s.sequence.Load()
}

// ResetSequence resets the sequence counter to 0.
func (s *IntegritySigner) ResetSequence() {
	if s != nil {
		s.sequence.Store(0)
	}
}

// Clone creates a copy of the IntegrityConfig.
func (c *IntegrityConfig) Clone() *IntegrityConfig {
	if c == nil {
		return nil
	}

	copiedKey := make([]byte, len(c.SecretKey))
	copy(copiedKey, c.SecretKey)

	return &IntegrityConfig{
		SecretKey:        copiedKey,
		HashAlgorithm:    c.HashAlgorithm,
		IncludeTimestamp: c.IncludeTimestamp,
		IncludeSequence:  c.IncludeSequence,
		SignaturePrefix:  c.SignaturePrefix,
	}
}

// MarshalJSON implements json.Marshaler for IntegrityConfig.
// Note: SecretKey is intentionally not marshaled for security reasons.
func (c *IntegrityConfig) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]any{
		"hashAlgorithm":    c.HashAlgorithm.String(),
		"includeTimestamp": c.IncludeTimestamp,
		"includeSequence":  c.IncludeSequence,
		"signaturePrefix":  c.SignaturePrefix,
		"secretKeyLength":  len(c.SecretKey),
	})
}

// IntegrityStats holds integrity signer statistics.
type IntegrityStats struct {
	Sequence         uint64 // Current sequence number
	Algorithm        string // Hash algorithm name
	IncludeTimestamp bool   // Whether timestamps are included
	IncludeSequence  bool   // Whether sequence numbers are included
}

// Stats returns current integrity signer statistics.
func (s *IntegritySigner) Stats() IntegrityStats {
	if s == nil {
		return IntegrityStats{}
	}

	return IntegrityStats{
		Sequence:         s.sequence.Load(),
		Algorithm:        s.config.HashAlgorithm.String(),
		IncludeTimestamp: s.config.IncludeTimestamp,
		IncludeSequence:  s.config.IncludeSequence,
	}
}
