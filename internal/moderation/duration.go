package moderation

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// ParseMuteDuration accepts compact durations such as 30s, 15m, 2h, 3d,
// 1w, and combinations such as 1d12h. Zero is not a duration: omit the
// argument to create a permanent mute. Telegram requires temporary
// restrictions to be at least 30 seconds and no more than 366 days.
func ParseMuteDuration(value string) (time.Duration, error) {
	s := strings.TrimSpace(strings.ToLower(value))
	if s == "" {
		return 0, fmt.Errorf("duration is empty")
	}
	var total time.Duration
	for len(s) > 0 {
		i := 0
		for i < len(s) && s[i] >= '0' && s[i] <= '9' {
			i++
		}
		if i == 0 || i == len(s) {
			return 0, fmt.Errorf("invalid duration; use values such as 30s, 15m, 2h, 3d or 1w")
		}
		n, err := strconv.ParseInt(s[:i], 10, 64)
		if err != nil || n <= 0 {
			return 0, fmt.Errorf("duration values must be positive whole numbers")
		}
		var unit time.Duration
		switch s[i] {
		case 's':
			unit = time.Second
		case 'm':
			unit = time.Minute
		case 'h':
			unit = time.Hour
		case 'd':
			unit = 24 * time.Hour
		case 'w':
			unit = 7 * 24 * time.Hour
		default:
			return 0, fmt.Errorf("unknown duration unit; use s, m, h, d or w")
		}
		if n > int64((366*24*time.Hour-total)/unit) {
			return 0, fmt.Errorf("mute duration cannot exceed 366 days")
		}
		total += time.Duration(n) * unit
		s = s[i+1:]
	}
	if total < 30*time.Second {
		return 0, fmt.Errorf("temporary mute must be at least 30 seconds; omit the duration for a permanent mute")
	}
	if total > 366*24*time.Hour {
		return 0, fmt.Errorf("mute duration cannot exceed 366 days")
	}
	return total, nil
}
