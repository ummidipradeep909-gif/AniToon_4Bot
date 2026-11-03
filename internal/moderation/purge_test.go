package moderation

import (
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
)

func TestPurgeRangeByCountIncludesCommand(t *testing.T) {
	message := &gotgbot.Message{MessageId: 120}
	start, end, err := purgeRange(message, []string{"20"})
	if err != nil {
		t.Fatal(err)
	}
	if start != 101 || end != 120 {
		t.Fatalf("range = %d..%d, want 101..120", start, end)
	}
}

func TestPurgeRangeFromReplyIncludesBothEndpoints(t *testing.T) {
	message := &gotgbot.Message{
		MessageId:      120,
		ReplyToMessage: &gotgbot.Message{MessageId: 100},
	}
	start, end, err := purgeRange(message, nil)
	if err != nil {
		t.Fatal(err)
	}
	if start != 100 || end != 120 {
		t.Fatalf("range = %d..%d, want 100..120", start, end)
	}
}

func TestPurgeRangeRejectsMoreThan100(t *testing.T) {
	message := &gotgbot.Message{MessageId: 500}
	for _, args := range [][]string{{"101"}, {"0"}, {"-1"}, {"abc"}, {"10", "extra"}} {
		if _, _, err := purgeRange(message, args); err == nil {
			t.Fatalf("purgeRange(%v) should fail", args)
		}
	}
}

func TestPurgeRangeRejectsReplyRangeOver100(t *testing.T) {
	message := &gotgbot.Message{
		MessageId:      500,
		ReplyToMessage: &gotgbot.Message{MessageId: 400},
	}
	if _, _, err := purgeRange(message, nil); err == nil {
		t.Fatal("range longer than 100 should fail")
	}
}

func TestPurgeRangeRequiresReplyOrCount(t *testing.T) {
	message := &gotgbot.Message{MessageId: 20}
	if _, _, err := purgeRange(message, nil); err == nil {
		t.Fatal("missing reply/count should fail")
	}
}
