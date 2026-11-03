package moderation

import "testing"

func TestParseSlowModeInterval(t *testing.T) {
	for _, tc := range []struct {
		args []string
		want int
		wantErr bool
	}{
		{[]string{"1"}, 1, false},
		{[]string{"10"}, 10, false},
		{[]string{"3600"}, 3600, false},
		{[]string{"OFF"}, 0, false},
		{[]string{"0"}, 0, true},
		{[]string{"3601"}, 0, true},
		{[]string{"abc"}, 0, true},
		{nil, 0, true},
		{[]string{"10", "20"}, 0, true},
	} {
		got, err := parseSlowModeInterval(tc.args)
		if (err != nil) != tc.wantErr { t.Fatalf("parseSlowModeInterval(%v) error=%v wantErr=%v", tc.args, err, tc.wantErr) }
		if err == nil && got != tc.want { t.Fatalf("parseSlowModeInterval(%v)=%d want %d", tc.args, got, tc.want) }
	}
}

func TestSlowModeKeysAreScopedToBotChatUserAndConfigVersion(t *testing.T) {
	base := slowModeCounterKey(1, 2, 3, 4)
	for _, other := range []string{
		slowModeCounterKey(9, 2, 3, 4),
		slowModeCounterKey(1, 8, 3, 4),
		slowModeCounterKey(1, 2, 4, 4),
		slowModeCounterKey(1, 2, 3, 5),
	} {
		if base == other { t.Fatal("slow-mode cooldowns must be isolated by bot, group, member and config version") }
	}
}
