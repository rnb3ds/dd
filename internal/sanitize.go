package internal

// HexChars is a package-level constant for hex digit conversion.
// Avoids allocation in SanitizeControlChars hot path.
const HexChars = "0123456789abcdef"

// SanitizeControlChars replaces dangerous control characters with visible escape sequences.
// This preserves debugging information while preventing log injection attacks.
//
// Allowed control characters: \n (newline), \r (carriage return), \t (tab)
// Null bytes (\x00) and DEL (127) are removed entirely for security.
// Other control characters (0x01-0x1F except \n, \r, \t) are replaced with \xNN format.
// ANSI escape sequences (starting with ESC \x1b) are removed entirely for security.
func SanitizeControlChars(message string) string {
	msgLen := len(message)
	if msgLen == 0 {
		return message
	}

	// Fast path: check if sanitization is needed using string indexing
	// Avoids []byte allocation when no sanitization is needed
	needsSanitization := false
	for i := 0; i < msgLen; i++ {
		b := message[i]
		// 0x1b is ESC character (start of ANSI escape sequences)
		if b == 0x00 || b == 0x1b || (b < 32 && b != '\n' && b != '\r' && b != '\t') || b == 127 {
			needsSanitization = true
			break
		}
	}

	if !needsSanitization {
		return message
	}

	// Slow path: replace control characters and remove ANSI sequences
	// Now we need the byte slice for manipulation
	msgBytes := []byte(message)

	// First pass: calculate result size and identify ANSI sequences
	resultSize := 0
	for i := 0; i < len(msgBytes); i++ {
		b := msgBytes[i]
		if b == 0x00 || b == 127 {
			// These are removed
			continue
		} else if b == 0x1b {
			// Skip ANSI escape sequence
			i += skipAnsiSequence(msgBytes, i+1)
			continue
		} else if b < 32 && b != '\n' && b != '\r' && b != '\t' {
			resultSize += 4 // \xNN is 4 bytes
		} else {
			resultSize++
		}
	}

	result := make([]byte, 0, resultSize)

	// Second pass: build the result
	for i := 0; i < len(msgBytes); i++ {
		b := msgBytes[i]
		switch {
		case b == 0x00:
			// Null bytes are removed entirely for security (prevent log truncation)
			continue
		case b == 127:
			// DEL character is removed
			continue
		case b == 0x1b:
			// ESC character - skip the entire ANSI escape sequence
			i += skipAnsiSequence(msgBytes, i+1)
			continue
		case b < 32 && b != '\n' && b != '\r' && b != '\t':
			// Replace other control characters with visible escape sequence \xNN
			result = append(result, '\\', 'x', HexChars[b>>4], HexChars[b&0x0f])
		default:
			result = append(result, b)
		}
	}

	return string(result)
}

// skipAnsiSequence skips an ANSI escape sequence starting after the ESC character.
// Returns the number of bytes to skip (not including the ESC character itself).
// ANSI escape sequences follow these patterns:
//   - CSI: ESC [ ... final byte (0x40-0x7E)
//   - OSC: ESC ] ... BEL or ESC ] ... ST (ESC \)
//   - Other: ESC followed by a single intermediate/final byte
func skipAnsiSequence(data []byte, start int) int {
	if start >= len(data) {
		return 0
	}

	b := data[start]

	// CSI (Control Sequence Introducer): ESC [
	if b == '[' {
		skip := 1 // for the '['
		for i := start + 1; i < len(data); i++ {
			c := data[i]
			// Parameter bytes: 0x30-0x3F
			// Intermediate bytes: 0x20-0x2F
			// Final byte: 0x40-0x7E
			if c >= 0x40 && c <= 0x7E {
				return skip + 1 // include the final byte
			}
			skip++
		}
		return skip // reached end of data
	}

	// OSC (Operating System Command): ESC ]
	if b == ']' {
		skip := 1 // for the ']'
		for i := start + 1; i < len(data); i++ {
			c := data[i]
			if c == 0x07 { // BEL terminates OSC
				return skip + 1
			}
			if c == 0x1b && i+1 < len(data) && data[i+1] == '\\' {
				// ST (String Terminator): ESC \ terminates OSC
				return skip + 2
			}
			skip++
		}
		return skip // reached end of data
	}

	// Other single-character sequences (ESC followed by one char)
	// Includes: ESC (, ESC ), ESC *, ESC +, ESC -, ESC ., ESC /, ESC #, ESC _, ESC ^
	// These are typically 2-byte sequences
	if (b >= 0x20 && b <= 0x2F) || // Intermediate bytes
		(b >= 0x30 && b <= 0x3F) || // Parameter bytes range
		(b >= 0x40 && b <= 0x5F) { // Final byte range for non-CSI
		return 1
	}

	// Unknown sequence, just skip the ESC and the next byte
	return 1
}
