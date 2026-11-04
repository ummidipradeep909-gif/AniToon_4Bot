package moderation

import (
	"testing"
	"time"
)

func TestParseMetricsWindow(t *testing.T) {
	cases := []struct {
		args []string
		want time.Duration
		label string
		wantErr bool
	}{
		{nil, 7 * 24 * time.Hour, "7 days", false},
		{[]string{"week"}, 7 * 24 * time.Hour, "7 days", false},
		{[]string{"day"}, 24 * time.Hour, "24 hours", false},
		{[]string{"month"}, 30 * 24 * time.Hour, "30 days", false},
		{[]string{"year"}, 0, "", true},
		{[]string{"day", "week"}, 0, "", true},
	}
	for _, tc := range cases {
		got, label, err := parseMetricsWindow(tc.args)
		if (err != nil) != tc.wantErr {
			t.Fatalf("parseMetricsWindow(%v) err=%v wantErr=%v", tc.args, err, tc.wantErr)
		}
		if err == nil && (got != tc.want || label != tc.label) {
			t.Errorf("parseMetricsWindow(%v)=(%v,%q), want (%v,%q)", tc.args, got, label, tc.want, tc.label)
		}
	}
}
