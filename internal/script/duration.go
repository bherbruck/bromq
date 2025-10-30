package script

import (
	"fmt"
	"regexp"
	"strconv"
	"time"
)

// ParseDurationWithDays parses a duration string that supports days (e.g., "30d", "7d", "24h", "1h30m")
// Supports all standard Go duration units plus "d" for days (24 hours)
func ParseDurationWithDays(s string) (time.Duration, error) {
	// Check for days suffix
	dayRegex := regexp.MustCompile(`^(\d+)d$`)
	if matches := dayRegex.FindStringSubmatch(s); matches != nil {
		days, _ := strconv.Atoi(matches[1])
		return time.Duration(days) * 24 * time.Hour, nil
	}

	// Fall back to standard time.ParseDuration for other formats
	return time.ParseDuration(s)
}

// CalculateCleanupInterval calculates an appropriate cleanup interval based on retention period
// Strategy: Check every 1/10th of retention period, clamped between 1 hour and 24 hours
func CalculateCleanupInterval(retention time.Duration) time.Duration {
	if retention == 0 {
		return 0 // Cleanup disabled
	}

	// Calculate 1/10th of retention
	interval := retention / 10

	// Clamp to reasonable bounds
	const minInterval = 1 * time.Hour
	const maxInterval = 24 * time.Hour

	if interval < minInterval {
		return minInterval
	}
	if interval > maxInterval {
		return maxInterval
	}

	return interval
}

// FormatDuration formats a duration in a human-readable way with days
func FormatDuration(d time.Duration) string {
	if d == 0 {
		return "disabled"
	}

	// Convert to days if >= 24 hours and evenly divisible
	if d >= 24*time.Hour && d%(24*time.Hour) == 0 {
		days := d / (24 * time.Hour)
		return fmt.Sprintf("%dd", days)
	}

	// Otherwise use standard formatting
	return d.String()
}
