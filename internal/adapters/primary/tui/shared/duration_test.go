package shared_test

import (
	"testing"
	"time"

	"github.com/dnlopes/overseer/internal/adapters/primary/tui/shared"
)

func TestFormatRelativeDuration(t *testing.T) {
	tests := []struct {
		name string
		d    time.Duration
		want string
	}{
		{"zero", 0, "0s"},
		{"sub_second_rounds_to_zero", 500 * time.Millisecond, "0s"},
		{"five_seconds", 5 * time.Second, "5s"},
		{"fifty_nine_seconds", 59 * time.Second, "59s"},
		{"one_minute", time.Minute, "1m"},
		{"ninety_seconds_truncates_to_one_minute", 90 * time.Second, "1m"},
		{"fifty_nine_minutes", 59 * time.Minute, "59m"},
		{"one_hour", time.Hour, "1h"},
		{"twenty_three_hours", 23 * time.Hour, "23h"},
		{"one_day", 24 * time.Hour, "1d"},
		{"six_days", 6 * 24 * time.Hour, "6d"},
		{"one_week", 7 * 24 * time.Hour, "1w"},
		{"three_weeks", 21 * 24 * time.Hour, "3w"},
		{"one_month", 30 * 24 * time.Hour, "1mo"},
		{"eleven_months", 11 * 30 * 24 * time.Hour, "11mo"},
		{"one_year", 365 * 24 * time.Hour, "1y"},
		{"five_years", 5 * 365 * 24 * time.Hour, "5y"},
		{"negative_is_treated_as_positive", -42 * time.Second, "42s"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shared.FormatRelativeDuration(tt.d); got != tt.want {
				t.Errorf("FormatRelativeDuration(%v) = %q, want %q", tt.d, got, tt.want)
			}
		})
	}
}
