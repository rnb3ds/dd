package dd

// Debug Visualization Functions
//
// SECURITY WARNING: These functions output directly to stdout WITHOUT sensitive
// data filtering. For production logging, use Logger methods (Info, Debug, etc.).
// Never use these with passwords, tokens, or other sensitive data.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/cybergodev/dd/internal"
)

// JSON outputs data as compact JSON to stdout with caller info for debugging.
func JSON(data ...any) {
	outputJSON(internal.GetCaller(DebugVisualizationDepth, false), data...)
}

// JSONF outputs formatted data as compact JSON to stdout with caller info for debugging.
func JSONF(format string, args ...any) {
	formatted := fmt.Sprintf(format, args...)
	outputJSON(internal.GetCaller(DebugVisualizationDepth, false), formatted)
}

// Text outputs data as pretty-printed format to stdout for debugging.
func Text(data ...any) {
	outputTextData(os.Stdout, data...)
}

// outputTextData is the shared implementation for Text output.
// It writes formatted data to the specified writer.
func outputTextData(w io.Writer, data ...any) {
	if len(data) == 0 {
		fmt.Fprintln(w)
		return
	}

	buf := newDebugBuffer()
	defer buf.Release()

	encoder := json.NewEncoder(buf)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")

	for i, item := range data {
		if isSimpleType(item) {
			output := formatSimpleValue(item)
			if i < len(data)-1 {
				fmt.Fprintf(w, "%s ", output)
			} else {
				fmt.Fprintf(w, "%s\n", output)
			}
			continue
		}

		buf.Reset()
		convertedItem := convertValue(item)

		if err := encoder.Encode(convertedItem); err != nil {
			fmt.Fprintf(w, "[%d] %v", i, item)
			if i < len(data)-1 {
				fmt.Fprint(w, " ")
			} else {
				fmt.Fprintln(w)
			}
			continue
		}

		out := buf.Bytes()
		if len(out) > 0 && out[len(out)-1] == '\n' {
			out = out[:len(out)-1]
		}

		if i < len(data)-1 {
			fmt.Fprintf(w, "%s ", out)
		} else {
			fmt.Fprintf(w, "%s\n", out)
		}
	}
}

// Textf outputs formatted data as pretty-printed format to stdout for debugging.
func Textf(format string, args ...any) {
	formatted := fmt.Sprintf(format, args...)
	fmt.Fprintln(os.Stdout, formatted)
}

// Exit outputs data as pretty-printed JSON to stdout and exits with code 0.
func Exit(data ...any) {
	outputText(internal.GetCaller(DebugVisualizationDepth, false), data...)
	os.Exit(0)
}

// Exitf outputs formatted data to stdout with caller info and exits with code 0.
func Exitf(format string, args ...any) {
	formatted := fmt.Sprintf(format, args...)
	fmt.Fprintf(os.Stdout, "%s %s\n", internal.GetCaller(DebugVisualizationDepth, false), formatted)
	os.Exit(0)
}

func isSimpleType(v any) bool {
	if v == nil {
		return true
	}

	if _, ok := v.(error); ok {
		return true
	}

	return !internal.IsComplexValue(v)
}

func formatSimpleValue(v any) string {
	if v == nil {
		return "nil"
	}

	if err, ok := v.(error); ok {
		if err == nil {
			return "nil"
		}
		return err.Error()
	}

	val := reflect.ValueOf(v)
	if val.Kind() == reflect.Ptr {
		if val.IsNil() {
			return "nil"
		}
		val = val.Elem()
	}

	return fmt.Sprintf("%v", val.Interface())
}

const MaxDebugBufferSize = 64 * 1024 // 64KB - maximum buffer size to return to pool

var (
	debugBufPool = sync.Pool{
		New: func() any {
			return &bytes.Buffer{}
		},
	}
)

// debugBuffer is a helper type that manages getting and returning a buffer from the pool.
type debugBuffer struct {
	*bytes.Buffer
}

func newDebugBuffer() *debugBuffer {
	return &debugBuffer{Buffer: debugBufPool.Get().(*bytes.Buffer)}
}

func (b *debugBuffer) Release() {
	if b.Buffer != nil {
		// Discard buffers that grew too large to prevent unbounded memory growth
		if b.Buffer.Cap() <= MaxDebugBufferSize {
			b.Reset()
			debugBufPool.Put(b.Buffer)
		}
		b.Buffer = nil
	}
}

// convertValue converts any value to a JSON-serializable format.
// Simplified version focused on debugging rather than comprehensive type handling.
func convertValue(v any) any {
	if v == nil {
		return nil
	}

	val := reflect.ValueOf(v)

	if !val.IsValid() {
		return nil
	}

	// Handle pointers
	if val.Kind() == reflect.Ptr {
		if val.IsNil() {
			return nil
		}
		val = val.Elem()
	}

	// Handle interfaces
	if val.Kind() == reflect.Interface {
		if val.IsNil() {
			return nil
		}
		return convertValue(val.Elem().Interface())
	}

	switch val.Kind() {
	case reflect.String, reflect.Bool,
		reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		return val.Interface()

	case reflect.Func:
		return fmt.Sprintf("<func:%s>", val.Type().String())

	case reflect.Chan:
		return fmt.Sprintf("<chan:%s>", val.Type().String())

	case reflect.Slice, reflect.Array:
		return convertSlice(val)

	case reflect.Map:
		return convertMap(val)

	case reflect.Struct:
		return convertStruct(val)

	default:
		// Handle special common types
		if val.CanInterface() {
			iface := val.Interface()
			switch v := iface.(type) {
			case error:
				if v == nil {
					return nil
				}
				return v.Error()
			case time.Time:
				return v.Format(time.RFC3339)
			case time.Duration:
				return v.String()
			case fmt.Stringer:
				return v.String()
			}
		}

		// Try JSON marshaling as fallback
		if val.CanInterface() {
			if data, err := json.Marshal(val.Interface()); err == nil {
				var result any
				if json.Unmarshal(data, &result) == nil {
					return result
				}
			}
		}

		return fmt.Sprintf("<%s:%v>", val.Type().String(), val)
	}
}

func convertSlice(val reflect.Value) any {
	length := val.Len()
	if length == 0 {
		return []any{}
	}

	result := make([]any, length)
	for i := 0; i < length; i++ {
		result[i] = convertValue(val.Index(i).Interface())
	}
	return result
}

func convertMap(val reflect.Value) any {
	if val.IsNil() {
		return nil
	}

	result := make(map[string]any)
	keys := val.MapKeys()

	for _, key := range keys {
		keyStr := fmt.Sprintf("%v", key.Interface())
		result[keyStr] = convertValue(val.MapIndex(key).Interface())
	}

	return result
}

func convertStruct(val reflect.Value) any {
	typ := val.Type()

	// Handle special types
	if val.CanInterface() {
		iface := val.Interface()
		switch v := iface.(type) {
		case error:
			if v == nil {
				return nil
			}
			return v.Error()
		case time.Time:
			return v.Format(time.RFC3339)
		case time.Duration:
			return v.String()
		case fmt.Stringer:
			return v.String()
		}
	}

	result := make(map[string]any)

	for i := 0; i < val.NumField(); i++ {
		field := val.Field(i)
		fieldType := typ.Field(i)

		if !field.CanInterface() && !fieldType.IsExported() {
			continue
		}

		fieldName := fieldType.Name
		if tag := fieldType.Tag.Get("json"); tag != "" && tag != "-" {
			tagName, _, found := strings.Cut(tag, ",")
			if found && tagName != "" {
				fieldName = tagName
			} else if !found && tag != "" {
				fieldName = tag
			}
			if fieldName == "" {
				fieldName = fieldType.Name
			}
		}

		if fieldName != "" {
			result[fieldName] = convertValue(field.Interface())
		}
	}

	return result
}

// formatJSONData formats data as JSON using intelligent type conversion.
func formatJSONData(data ...any) string {
	if len(data) == 0 {
		return "{}"
	}

	if len(data) == 1 {
		buf := newDebugBuffer()
		defer buf.Release()

		converted := convertValue(data[0])

		encoder := json.NewEncoder(buf)
		encoder.SetEscapeHTML(false)
		if err := encoder.Encode(converted); err != nil {
			if jsonData, err := json.Marshal(data[0]); err == nil {
				return string(jsonData)
			}
			return "{}"
		}

		result := buf.String()
		if len(result) > 0 && result[len(result)-1] == '\n' {
			result = result[:len(result)-1]
		}
		return result
	}

	// Multiple arguments: treat as key-value pairs
	result := make(map[string]any, len(data)/2)
	for i := 0; i < len(data); i += 2 {
		var key string
		var value any

		if i < len(data) {
			key = fmt.Sprintf("%v", convertValue(data[i]))
		}

		if i+1 < len(data) {
			value = convertValue(data[i+1])
		}

		if key != "" {
			result[key] = value
		}
	}

	buf := newDebugBuffer()
	defer buf.Release()

	encoder := json.NewEncoder(buf)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(result); err != nil {
		if jsonData, err := json.Marshal(result); err == nil {
			return string(jsonData)
		}
		return "{}"
	}

	output := buf.String()
	if len(output) > 0 && output[len(output)-1] == '\n' {
		output = output[:len(output)-1]
	}
	return output
}

func outputJSON(caller string, data ...any) {
	if len(data) == 0 {
		fmt.Fprintf(os.Stdout, "%s {}\n", caller)
		return
	}

	converted := formatJSONData(data...)
	fmt.Fprintf(os.Stdout, "%s %s\n", caller, converted)
}

func outputText(caller string, data ...any) {
	if len(data) == 0 {
		fmt.Fprintf(os.Stdout, "%s\n", caller)
		return
	}

	fmt.Fprint(os.Stdout, caller)

	buf := newDebugBuffer()
	defer buf.Release()

	encoder := json.NewEncoder(buf)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")

	for i, item := range data {
		if isSimpleType(item) {
			output := formatSimpleValue(item)
			if i < len(data)-1 {
				fmt.Fprintf(os.Stdout, " %s", output)
			} else {
				fmt.Fprintf(os.Stdout, " %s\n", output)
			}
			continue
		}

		buf.Reset()
		convertedItem := convertValue(item)

		if err := encoder.Encode(convertedItem); err != nil {
			fmt.Fprintf(os.Stdout, " [%d] %v", i, item)
			continue
		}

		output := buf.Bytes()
		if len(output) > 0 && output[len(output)-1] == '\n' {
			output = output[:len(output)-1]
		}

		if i < len(data)-1 {
			fmt.Fprintf(os.Stdout, " %s", output)
		} else {
			fmt.Fprintf(os.Stdout, " %s\n", output)
		}
	}
}
