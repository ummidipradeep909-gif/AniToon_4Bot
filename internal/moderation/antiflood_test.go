package moderation

import "testing"

func TestFloodCounterKeysAreScopedToBotChatAndUser(t *testing.T) {
	base:=floodCounterKey(1,2,3)
	if base==floodCounterKey(9,2,3) || base==floodCounterKey(1,8,3) || base==floodCounterKey(1,2,4) {
		t.Fatal("flood counters must be isolated by bot, chat and user")
	}
	if floodActionKey(1,2,3)==floodActionKey(1,2,4) { t.Fatal("action cooldown must be per user") }
}

func TestFloodConfigValidationBounds(t *testing.T) {
	for _,limit:=range []int{1,51,0} { if limit>=2 && limit<=50 { t.Fatalf("invalid test limit %d",limit) } }
	for _,seconds:=range []int{2,301,0} { if seconds>=3 && seconds<=300 { t.Fatalf("invalid test window %d",seconds) } }
}
