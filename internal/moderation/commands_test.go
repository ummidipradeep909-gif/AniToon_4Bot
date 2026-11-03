package moderation

import (
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
)

func TestResolveTargetFromNumericArgument(t *testing.T) {
	msg := &gotgbot.Message{Text: "/ban 123456 reason here"}
	id, reason, err := resolveTarget(msg, commandArgs(msg))
	if err != nil { t.Fatal(err) }
	if id != 123456 { t.Fatalf("id = %d, want 123456", id) }
	if reason != "reason here" { t.Fatalf("reason = %q", reason) }
}

func TestResolveTargetRejectsUsername(t *testing.T) {
	msg := &gotgbot.Message{Text: "/ban @someone"}
	if _, _, err := resolveTarget(msg, commandArgs(msg)); err == nil {
		t.Fatal("username should not be accepted as a numeric ID")
	}
}

func TestResolveTargetRequiresTarget(t *testing.T) {
	msg := &gotgbot.Message{Text: "/ban"}
	if _, _, err := resolveTarget(msg, commandArgs(msg)); err == nil {
		t.Fatal("missing target should fail")
	}
}

func TestResolveTargetFromReply(t *testing.T) {
	msg := &gotgbot.Message{
		Text: "/ban reason here",
		ReplyToMessage: &gotgbot.Message{From: &gotgbot.User{Id: 777}},
	}
	id, reason, err := resolveTarget(msg, commandArgs(msg))
	if err != nil { t.Fatal(err) }
	if id != 777 { t.Fatalf("id = %d, want 777", id) }
	if reason != "reason here" { t.Fatalf("reason = %q", reason) }
}
