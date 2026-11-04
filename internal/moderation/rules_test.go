package moderation

import (
	"strings"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
)

func TestValidateGroupRules(t *testing.T) {
	if _, err := validateGroupRules("  "); err == nil { t.Fatal("empty rules accepted") }
	if _, err := validateGroupRules(strings.Repeat("x", maxGroupRulesRunes+1)); err == nil { t.Fatal("oversized rules accepted") }
	if _, err := validateGroupRules("rules\x00"); err == nil { t.Fatal("control character accepted") }
	got, err := validateGroupRules("  Be respectful.\nNo spam.  ")
	if err != nil || got != "Be respectful.\nNo spam." { t.Fatalf("unexpected validation: %q %v", got, err) }
}

func TestRulesCommandTextPreservesNewlines(t *testing.T) {
	m := &gotgbot.Message{Text: "/setrules@AniToonBot Be respectful.\nNo spam."}
	if got := rulesCommandText(m); got != "Be respectful.\nNo spam." { t.Fatalf("got %q", got) }
}

func TestFormatGroupRulesDoesNotInterpretHTML(t *testing.T) {
	rules := "<b>Respect members</b> & follow the rules"
	got := formatGroupRules(rules)
	if !strings.Contains(got, rules) { t.Fatalf("rules text was changed: %q", got) }
	if strings.Contains(got, "<b>") != true { t.Fatal("plain-text content unexpectedly altered") }
}
