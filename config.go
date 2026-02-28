package dd

import (
	"time"

	"github.com/cybergodev/dd/internal"
)

// JSONOptions configures JSON output format.
type JSONOptions = internal.JSONOptions

// JSONFieldNames configures custom field names for JSON output.
type JSONFieldNames = internal.JSONFieldNames

// DefaultJSONOptions returns default JSON options.
func DefaultJSONOptions() *JSONOptions {
	return &JSONOptions{
		PrettyPrint: false,
		Indent:      DefaultJSONIndent,
		FieldNames:  internal.DefaultJSONFieldNames(),
	}
}

// SamplingConfig configures log sampling for high-throughput scenarios.
// Sampling reduces log volume by only recording a subset of messages.
type SamplingConfig struct {
	// Enabled controls whether sampling is active.
	Enabled bool
	// Initial is the number of messages that are always logged before sampling begins.
	// This ensures visibility of initial burst traffic.
	Initial int
	// Thereafter is the sampling rate after Initial messages.
	// A value of 10 means log 1 out of every 10 messages.
	Thereafter int
	// Tick is the time interval after which counters are reset.
	// This allows sampling to restart periodically for burst handling.
	Tick time.Duration
}
