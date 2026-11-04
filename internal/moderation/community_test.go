package moderation

import (
	"strings"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
)

func TestCommunitySplitNameAndContent(t *testing.T) {
	name, content, err := communitySplitNameAndContent("Rules Be respectful.\nNo spam.")
	if err != nil || name != "rules" || content != "Be respectful.\nNo spam." {
		t.Fatalf("got %q %q %v", name, content, err)
	}
	for _, input := range []string{"", "name", "bad name content", "bad/name content", "ok " + strings.Repeat("x", maxNoteContentUTF16+1)} {
		if _, _, err := communitySplitNameAndContent(input); err == nil {
			t.Errorf("expected invalid note input %q", input[:min(len(input), 40)])
		}
	}
}

func TestValidateFilterTrigger(t *testing.T) {
	if got, err := validateFilterTrigger("  HELLO  "); err != nil || got != "hello" {
		t.Fatalf("got %q %v", got, err)
	}
	for _, input := range []string{"", "/start", strings.Repeat("x", maxFilterTriggerRunes+1), "bad\ntrigger"} {
		if _, err := validateFilterTrigger(input); err == nil { t.Errorf("accepted trigger %q", input) }
	}
}

func TestValidateFilterResponse(t *testing.T) {
	if got, err := validateFilterResponse("  Hello there!  "); err != nil || got != "Hello there!" {
		t.Fatalf("got %q %v", got, err)
	}
	if _, err := validateFilterResponse(strings.Repeat("x", maxFilterResponseUTF16+1)); err == nil {
		t.Fatal("accepted oversized response")
	}
}

func TestFilterMatchIsExactAndCaseInsensitive(t *testing.T) {
	trigger := "hello"
	for _, tc := range []struct{text string; want bool}{
		{"HELLO", true}, {" hello ", true}, {"hello there", false}, {"say hello", false},
		{"/hello", false}, {"", false},
	} {
		got := tc.text != "" && !strings.HasPrefix(strings.TrimSpace(tc.text), "/") && strings.EqualFold(strings.TrimSpace(tc.text), trigger)
		if got != tc.want { t.Errorf("text %q: got %v want %v", tc.text, got, tc.want) }
	}
}

func TestCommunityCommandPayloadPreservesLines(t *testing.T) {
	m := &gotgbot.Message{Text: "/save rules Be kind.\nNo spam."}
	if got := communityCommandPayload(m); got != "rules Be kind.\nNo spam." { t.Fatalf("got %q", got) }
}
