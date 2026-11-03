package moderation

import (
	"testing"
	"time"
)

func TestParseMuteDuration(t *testing.T) {
	tests := []struct {
		input string
		want time.Duration
	}{
		{"30s", 30 * time.Second},
		{"15m", 15 * time.Minute},
		{"2h", 2 * time.Hour},
		{"3d", 72 * time.Hour},
		{"1w", 7 * 24 * time.Hour},
		{"1d12h", 36 * time.Hour},
		{" 10M ", 10 * time.Minute},
	}
	for _, tt := range tests {
		got, err := ParseMuteDuration(tt.input)
		if err != nil { t.Errorf("ParseMuteDuration(%q): %v", tt.input, err); continue }
		if got != tt.want { t.Errorf("ParseMuteDuration(%q) = %v, want %v", tt.input, got, tt.want) }
	}
}

func TestParseMuteDurationRejectsInvalidValues(t *testing.T) {
	for _, input := range []string{"", "0m", "29s", "10x", "2", "-5m", "367d", "1hxyz"} {
		if _, err := ParseMuteDuration(input); err == nil {
			t.Errorf("ParseMuteDuration(%q) should fail", input)
		}
	}
}

func TestParseMuteDurationRejectsOverflow(t *testing.T) {
	if _, err := ParseMuteDuration("999999999999999999999999d"); err == nil {
		t.Fatal("overflowing duration should fail")
	}
}
