package moderation

import (
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
)

func TestParseMuteTargetByID(t *testing.T) {
	msg := &gotgbot.Message{Text: "/mute 12345 2h repeated spam"}
	id, duration, reason, err := parseMuteTarget(msg, commandArgs(msg))
	if err != nil { t.Fatal(err) }
	if id != 12345 { t.Fatalf("id = %d, want 12345", id) }
	if duration != 2*time.Hour { t.Fatalf("duration = %v, want 2h", duration) }
	if reason != "repeated spam" { t.Fatalf("reason = %q", reason) }
}

func TestParseMuteTargetReplyPermanent(t *testing.T) {
	msg := &gotgbot.Message{
		Text: "/mute repeated spam",
		ReplyToMessage: &gotgbot.Message{From: &gotgbot.User{Id: 777}},
	}
	id, duration, reason, err := parseMuteTarget(msg, commandArgs(msg))
	if err != nil { t.Fatal(err) }
	if id != 777 { t.Fatalf("id = %d, want 777", id) }
	if duration != 0 { t.Fatalf("duration = %v, want permanent", duration) }
	if reason != "repeated spam" { t.Fatalf("reason = %q", reason) }
}

func TestParseMuteTargetRejectsInvalidDuration(t *testing.T) {
	msg := &gotgbot.Message{Text: "/mute 12345 10x spam"}
	if _, _, _, err := parseMuteTarget(msg, commandArgs(msg)); err == nil {
		t.Fatal("invalid duration should fail")
	}
}

func TestParseMuteTargetRejectsMissingTarget(t *testing.T) {
	msg := &gotgbot.Message{Text: "/mute"}
	if _, _, _, err := parseMuteTarget(msg, commandArgs(msg)); err == nil {
		t.Fatal("missing target should fail")
	}
}
