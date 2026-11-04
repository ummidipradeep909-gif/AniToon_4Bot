package clones

import (
	"os"
	"testing"
)

func TestCloneQuotaFromEnv(t *testing.T) {
	tests := []struct {
		name string
		value string
		want int
	}{
		{name: "default", value: "", want: 100},
		{name: "configured", value: "25", want: 25},
		{name: "zero falls back", value: "0", want: 100},
		{name: "negative falls back", value: "-1", want: 100},
		{name: "too large falls back", value: "10001", want: 100},
		{name: "invalid falls back", value: "many", want: 100},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("CLONE_MAX_BOTS", tc.value)
			if got := cloneQuotaFromEnv(); got != tc.want {
				t.Fatalf("cloneQuotaFromEnv() = %d, want %d", got, tc.want)
			}
		})
	}
	_ = os.Unsetenv("CLONE_MAX_BOTS")
}
