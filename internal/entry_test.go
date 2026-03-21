package internal

import (
	"testing"
)

// TestMergeFieldSlicesLarge tests the large field count merging path.
// This tests the internal mergeFieldSlicesLarge function which is triggered
// when newFields > 4 OR existingFields > 8.
func TestMergeFieldSlicesLarge(t *testing.T) {
	t.Run("trigger large path with many existing fields", func(t *testing.T) {
		// Create 10 existing fields (> 8 threshold)
		existing := make([]Field, 10)
		for i := range 10 {
			existing[i] = Field{Key: string(rune('a' + i)), Value: i}
		}

		// Create 2 new fields (<= 4 threshold, but existing > 8 triggers large path)
		newFields := []Field{
			{Key: "x", Value: "new_x"},
			{Key: "a", Value: "override_a"}, // Override existing
		}

		result := mergeFieldSlices(existing, newFields)

		// Verify override worked
		foundA := false
		for _, f := range result {
			if f.Key == "a" {
				foundA = true
				if f.Value != "override_a" {
					t.Errorf("Field 'a' should be overridden, got %v", f.Value)
				}
			}
		}
		if !foundA {
			t.Error("Field 'a' should be present")
		}

		// Verify new field added
		foundX := false
		for _, f := range result {
			if f.Key == "x" {
				foundX = true
			}
		}
		if !foundX {
			t.Error("Field 'x' should be present")
		}

		// Verify non-overridden fields still present
		foundB := false
		for _, f := range result {
			if f.Key == "b" {
				foundB = true
			}
		}
		if !foundB {
			t.Error("Field 'b' should still be present")
		}
	})

	t.Run("trigger large path with many new fields", func(t *testing.T) {
		// Create 5 existing fields (<= 8 threshold)
		existing := []Field{
			{Key: "a", Value: 1},
			{Key: "b", Value: 2},
			{Key: "c", Value: 3},
			{Key: "d", Value: 4},
			{Key: "e", Value: 5},
		}

		// Create 6 new fields (> 4 threshold triggers large path)
		newFields := []Field{
			{Key: "x", Value: "new_x"},
			{Key: "y", Value: "new_y"},
			{Key: "z", Value: "new_z"},
			{Key: "w", Value: "new_w"},
			{Key: "v", Value: "new_v"},
			{Key: "a", Value: "override_a"}, // Override existing
		}

		result := mergeFieldSlices(existing, newFields)

		// Verify count: 5 existing + 6 new - 1 override = 10
		if len(result) != 10 {
			t.Errorf("Expected 10 fields, got %d", len(result))
		}

		// Verify override worked
		for _, f := range result {
			if f.Key == "a" && f.Value != "override_a" {
				t.Errorf("Field 'a' should be overridden, got %v", f.Value)
			}
		}
	})

	t.Run("all fields overridden", func(t *testing.T) {
		existing := make([]Field, 10)
		for i := range 10 {
			existing[i] = Field{Key: string(rune('a' + i)), Value: "old"}
		}

		// Override all with new values
		newFields := make([]Field, 10)
		for i := range 10 {
			newFields[i] = Field{Key: string(rune('a' + i)), Value: "new"}
		}

		result := mergeFieldSlices(existing, newFields)

		// All should have "new" value
		for _, f := range result {
			if f.Value != "new" {
				t.Errorf("Field %q should have 'new' value, got %v", f.Key, f.Value)
			}
		}
	})
}

// mergeFieldSlices is duplicated here for testing since it's not exported.
// This tests the exact function from entry.go.
func mergeFieldSlices(existingFields, newFields []Field) []Field {
	// Fast path: no existing fields
	if len(existingFields) == 0 {
		return newFields
	}
	// Fast path: no new fields
	if len(newFields) == 0 {
		return existingFields
	}

	existingLen := len(existingFields)
	newLen := len(newFields)

	// For small field counts, use linear search
	if newLen <= 4 && existingLen <= 8 {
		return mergeFieldSlicesSmall(existingFields, newFields)
	}

	// For larger field counts, use map-based approach
	return mergeFieldSlicesLarge(existingFields, newFields)
}

func mergeFieldSlicesSmall(existingFields, newFields []Field) []Field {
	merged := make([]Field, 0, len(existingFields)+len(newFields))

	for _, existing := range existingFields {
		overridden := false
		for _, newF := range newFields {
			if newF.Key == existing.Key {
				overridden = true
				break
			}
		}
		if !overridden {
			merged = append(merged, existing)
		}
	}

	merged = append(merged, newFields...)
	return merged
}

func mergeFieldSlicesLarge(existingFields, newFields []Field) []Field {
	merged := make([]Field, 0, len(existingFields)+len(newFields))

	newKeys := make(map[string]struct{}, len(newFields))
	for _, f := range newFields {
		newKeys[f.Key] = struct{}{}
	}

	for _, f := range existingFields {
		if _, exists := newKeys[f.Key]; !exists {
			merged = append(merged, f)
		}
	}

	merged = append(merged, newFields...)
	return merged
}
