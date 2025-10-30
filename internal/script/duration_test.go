package script

import (
	"testing"
	"time"
)

func TestParseDurationWithDays(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    time.Duration
		wantErr bool
	}{
		// Days
		{"1 day", "1d", 24 * time.Hour, false},
		{"7 days", "7d", 7 * 24 * time.Hour, false},
		{"30 days", "30d", 30 * 24 * time.Hour, false},
		{"90 days", "90d", 90 * 24 * time.Hour, false},

		// Standard durations
		{"1 hour", "1h", 1 * time.Hour, false},
		{"30 minutes", "30m", 30 * time.Minute, false},
		{"1 hour 30 minutes", "1h30m", 1*time.Hour + 30*time.Minute, false},
		{"24 hours", "24h", 24 * time.Hour, false},

		// Edge cases
		{"0 days", "0d", 0, false},
		{"invalid", "invalid", 0, true},
		{"negative", "-1d", 0, true},
		{"empty", "", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseDurationWithDays(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseDurationWithDays(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("ParseDurationWithDays(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestCalculateCleanupInterval(t *testing.T) {
	tests := []struct {
		name      string
		retention time.Duration
		want      time.Duration
	}{
		// Disabled
		{"disabled", 0, 0},

		// Short retention (< 10h) - clamp to 1h minimum
		{"6 hours", 6 * time.Hour, 1 * time.Hour},
		{"1 hour", 1 * time.Hour, 1 * time.Hour},

		// Medium retention - 1/10th
		{"12 hours", 12 * time.Hour, 72 * time.Minute}, // 1.2h
		{"24 hours", 24 * time.Hour, 144 * time.Minute}, // 2.4h
		{"7 days", 7 * 24 * time.Hour, 16*time.Hour + 48*time.Minute}, // 16.8h

		// Long retention (> 10 days) - clamp to 24h maximum
		{"30 days", 30 * 24 * time.Hour, 24 * time.Hour}, // clamped to 24h
		{"90 days", 90 * 24 * time.Hour, 24 * time.Hour}, // clamped to 24h
		{"365 days", 365 * 24 * time.Hour, 24 * time.Hour}, // clamped to 24h
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CalculateCleanupInterval(tt.retention)
			if got != tt.want {
				t.Errorf("CalculateCleanupInterval(%v) = %v, want %v", tt.retention, got, tt.want)
			}
		})
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		name  string
		input time.Duration
		want  string
	}{
		{"disabled", 0, "disabled"},
		{"1 day", 24 * time.Hour, "1d"},
		{"7 days", 7 * 24 * time.Hour, "7d"},
		{"30 days", 30 * 24 * time.Hour, "30d"},
		{"1 hour", 1 * time.Hour, "1h0m0s"},
		{"1h30m", 1*time.Hour + 30*time.Minute, "1h30m0s"},
		{"25 hours (not evenly divisible)", 25 * time.Hour, "25h0m0s"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatDuration(tt.input)
			if got != tt.want {
				t.Errorf("FormatDuration(%v) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestCleanupIntervalExamples(t *testing.T) {
	// Test practical examples to verify the algorithm makes sense
	examples := []struct {
		retention string
		wantCheck string
	}{
		{"30d", "24h"},   // 30 days → check daily
		{"7d", "16h48m"}, // 7 days → check every 16.8h
		{"1d", "2h24m"},  // 1 day → check every 2.4h
		{"12h", "1h12m"}, // 12 hours → check every 1.2h
		{"6h", "1h"},     // 6 hours → clamped to 1h minimum
		{"0d", "0s"},     // disabled → 0
	}

	for _, ex := range examples {
		t.Run(ex.retention, func(t *testing.T) {
			retention, _ := ParseDurationWithDays(ex.retention)
			interval := CalculateCleanupInterval(retention)

			wantInterval, _ := time.ParseDuration(ex.wantCheck)
			if interval != wantInterval {
				t.Errorf("retention %s: got check interval %v, want %v",
					ex.retention, interval, wantInterval)
			}
		})
	}
}
