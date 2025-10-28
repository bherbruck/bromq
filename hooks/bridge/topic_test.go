package bridge

import (
	"testing"
)

func TestMatchTopic(t *testing.T) {
	tests := []struct {
		name     string
		topic    string
		pattern  string
		expected bool
	}{
		// Exact matches
		{"exact match", "sensor/temp", "sensor/temp", true},
		{"exact mismatch", "sensor/temp", "sensor/humidity", false},

		// Single-level wildcard (+)
		{"single wildcard match", "sensor/kitchen/temp", "sensor/+/temp", true},
		{"single wildcard mismatch levels", "sensor/kitchen", "sensor/+/temp", false},
		{"single wildcard multiple", "sensor/kitchen/temp", "sensor/+/+", true},
		{"single wildcard at end", "sensor/temp", "sensor/+", true},
		{"single wildcard at start", "sensor/temp", "+/temp", true},

		// Multi-level wildcard (#)
		{"multi-level match all", "sensor/kitchen/temp/value", "sensor/#", true},
		{"multi-level match one", "sensor/temp", "sensor/#", true},
		{"multi-level at root", "any/topic/here", "#", true},
		{"multi-level prefix mismatch", "other/topic", "sensor/#", false},

		// Combined wildcards
		{"combined wildcards", "sensor/kitchen/temp/value", "sensor/+/temp/#", true},
		{"combined mismatch", "sensor/kitchen/humidity", "sensor/+/temp/#", false},

		// Edge cases
		{"empty topic", "", "", true},
		{"empty pattern", "topic", "", false},
		{"too many levels", "a/b/c", "a/b", false},
		{"too few levels", "a/b", "a/b/c", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MatchTopic(tt.topic, tt.pattern)
			if result != tt.expected {
				t.Errorf("MatchTopic(%q, %q) = %v, want %v",
					tt.topic, tt.pattern, result, tt.expected)
			}
		})
	}
}

func TestTransformTopic(t *testing.T) {
	tests := []struct {
		name          string
		topic         string
		localPattern  string
		remotePattern string
		expected      string
	}{
		// Multi-level wildcard transformations
		{
			name:          "hash at end simple",
			topic:         "sensor/kitchen/temp",
			localPattern:  "sensor/#",
			remotePattern: "remote/sensor/#",
			expected:      "remote/sensor/kitchen/temp",
		},
		{
			name:          "hash at end single level",
			topic:         "data/value",
			localPattern:  "data/#",
			remotePattern: "cloud/data/#",
			expected:      "cloud/data/value",
		},
		{
			name:          "hash with prefix",
			topic:         "building/floor1/room2/temp",
			localPattern:  "building/#",
			remotePattern: "site-a/building/#",
			expected:      "site-a/building/floor1/room2/temp",
		},

		// Single-level wildcard transformations
		{
			name:          "single wildcard simple",
			topic:         "sensor/kitchen/temp",
			localPattern:  "sensor/+/temp",
			remotePattern: "remote/+/temperature",
			expected:      "remote/kitchen/temperature",
		},
		{
			name:          "single wildcard multiple",
			topic:         "building/floor1/room2",
			localPattern:  "building/+/+",
			remotePattern: "site/+/+",
			expected:      "site/floor1/room2",
		},

		// Exact transformations (no wildcards)
		{
			name:          "exact prefix change",
			topic:         "data/value",
			localPattern:  "data/value",
			remotePattern: "cloud/data/value",
			expected:      "cloud/data/value",
		},

		// Note: Mixed wildcards (+ and # together) are edge cases
		// and transformation behavior may vary. For production use,
		// prefer patterns with either + OR # but not both.
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := TransformTopic(tt.topic, tt.localPattern, tt.remotePattern)
			if result != tt.expected {
				t.Errorf("TransformTopic(%q, %q, %q) = %q, want %q",
					tt.topic, tt.localPattern, tt.remotePattern, result, tt.expected)
			}
		})
	}
}

// Benchmark tests
func BenchmarkMatchTopic(b *testing.B) {
	for i := 0; i < b.N; i++ {
		MatchTopic("sensor/kitchen/temp/value", "sensor/+/temp/#")
	}
}

func BenchmarkTransformTopic(b *testing.B) {
	for i := 0; i < b.N; i++ {
		TransformTopic("sensor/kitchen/temp", "sensor/#", "remote/sensor/#")
	}
}
