package shared

import (
	"fmt"
	"time"
)

// FormatRelativeDuration renders d in compact time-duration form:
//
//	"5s", "10m", "1h", "2d", "1w", "3mo", "1y"
//
// Sub-second values render as "0s". Negative durations are treated as
// their absolute value (e.g. a future timestamp shows as elapsed). Each
// unit truncates toward zero — 90s is "1m", not "2m" — so the label
// reflects whole units passed.
func FormatRelativeDuration(d time.Duration) string {
	if d < 0 {
		d = -d
	}
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d/time.Second))
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d/time.Minute))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d/time.Hour))
	case d < 7*24*time.Hour:
		return fmt.Sprintf("%dd", int(d/(24*time.Hour)))
	case d < 30*24*time.Hour:
		return fmt.Sprintf("%dw", int(d/(7*24*time.Hour)))
	case d < 365*24*time.Hour:
		return fmt.Sprintf("%dmo", int(d/(30*24*time.Hour)))
	default:
		return fmt.Sprintf("%dy", int(d/(365*24*time.Hour)))
	}
}
