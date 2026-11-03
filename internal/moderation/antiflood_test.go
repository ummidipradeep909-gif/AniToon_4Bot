package moderation

import "testing"

func TestParseFloodConfig(t *testing.T) {
	tests := []struct {
		args []string
		want floodConfig
		wantErr bool
	}{
		{[]string{"5","10"}, floodConfig{Limit:5,WindowSeconds:10,Action:"delete"}, false},
		{[]string{"20","60","MUTE"}, floodConfig{Limit:20,WindowSeconds:60,Action:"mute"}, false},
		{[]string{"2","3","ban"}, floodConfig{Limit:2,WindowSeconds:3,Action:"ban"}, false},
		{[]string{"1","10"}, floodConfig{}, true},
		{[]string{"51","10"}, floodConfig{}, true},
		{[]string{"5","2"}, floodConfig{}, true},
		{[]string{"5","301"}, floodConfig{}, true},
		{[]string{"5","10","kick"}, floodConfig{}, true},
		{[]string{"5"}, floodConfig{}, true},
		{[]string{"5","10","mute","extra"}, floodConfig{}, true},
	}
	for _, tc := range tests {
		got, err := parseFloodConfig(tc.args)
		if (err != nil) != tc.wantErr {
			t.Fatalf("parseFloodConfig(%v) error=%v, wantErr=%v", tc.args, err, tc.wantErr)
		}
		if err == nil && got != tc.want {
			t.Fatalf("parseFloodConfig(%v) = %+v, want %+v", tc.args, got, tc.want)
		}
	}
}

func TestFloodCounterKeysAreScopedToBotChatAndUser(t *testing.T) {
	base:=floodCounterKey(1,2,3)
	if base==floodCounterKey(9,2,3) || base==floodCounterKey(1,8,3) || base==floodCounterKey(1,2,4) {
		t.Fatal("flood counters must be isolated by bot, chat and user")
	}
	if floodActionKey(1,2,3)==floodActionKey(1,2,4) { t.Fatal("action cooldown must be per user") }
}
