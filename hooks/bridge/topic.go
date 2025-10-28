package bridge

import (
	"strings"
)

// MatchTopic checks if a topic matches an MQTT pattern with wildcards
// Supports single-level wildcard (+) and multi-level wildcard (#)
// Examples:
//   - "sensors/+/temp" matches "sensors/kitchen/temp" but not "sensors/kitchen/hum"
//   - "sensors/#" matches "sensors/kitchen/temp" and "sensors/living/hum/value"
func MatchTopic(topic, pattern string) bool {
	topicParts := strings.Split(topic, "/")
	patternParts := strings.Split(pattern, "/")

	// Multi-level wildcard must be last element
	if len(patternParts) > 0 && patternParts[len(patternParts)-1] == "#" {
		// Pattern ends with #, so it matches if topic starts with pattern prefix
		patternParts = patternParts[:len(patternParts)-1]
		if len(topicParts) < len(patternParts) {
			return false
		}
		for i, part := range patternParts {
			if part != "+" && part != topicParts[i] {
				return false
			}
		}
		return true
	}

	// Exact length match required for patterns without #
	if len(topicParts) != len(patternParts) {
		return false
	}

	// Check each part
	for i, part := range patternParts {
		if part == "+" {
			// Single-level wildcard matches any single level
			continue
		}
		if part != topicParts[i] {
			return false
		}
	}

	return true
}

// TransformTopic transforms a topic from local pattern to remote pattern
// Preserves wildcard-matched segments from source topic
// Examples:
//   - local="sensors/#", remote="edge/sensors/#", topic="sensors/temp/1"
//     → "edge/sensors/temp/1"
//   - local="data/+/value", remote="remote/+/data", topic="data/kitchen/value"
//     → "remote/kitchen/data"
func TransformTopic(topic, localPattern, remotePattern string) string {
	topicParts := strings.Split(topic, "/")
	localParts := strings.Split(localPattern, "/")
	remoteParts := strings.Split(remotePattern, "/")

	// Handle multi-level wildcard (#)
	if len(localParts) > 0 && localParts[len(localParts)-1] == "#" {
		// Extract the prefix and suffix
		localPrefix := localParts[:len(localParts)-1]

		// Check if remote also ends with #
		if len(remoteParts) > 0 && remoteParts[len(remoteParts)-1] == "#" {
			remotePrefix := remoteParts[:len(remoteParts)-1]

			// Get the wildcard-matched part from topic
			if len(topicParts) >= len(localPrefix) {
				suffix := topicParts[len(localPrefix):]
				result := append(remotePrefix, suffix...)
				return strings.Join(result, "/")
			}
		}
	}

	// Handle single-level wildcards (+)
	result := make([]string, len(remoteParts))
	localIndex := 0

	for i, remotePart := range remoteParts {
		if remotePart == "+" {
			// Find corresponding + in local pattern and use actual topic value
			for localIndex < len(localParts) && localParts[localIndex] != "+" {
				localIndex++
			}
			if localIndex < len(topicParts) {
				result[i] = topicParts[localIndex]
				localIndex++
			} else {
				result[i] = remotePart // Fallback
			}
		} else {
			result[i] = remotePart
		}
	}

	return strings.Join(result, "/")
}
