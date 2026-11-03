package moderation

import (
	"reflect"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
)

func TestNormalizeAllowedDomain(t *testing.T) {
	tests := []struct {
		input string
		want string
		wantErr bool
	}{
		{"Example.COM", "example.com", false},
		{"https://www.example.com/", "example.com", false},
		{"sub.example.com", "sub.example.com", false},
		{"", "", true},
		{"example.com/path", "", true},
		{"user@example.com", "", true},
		{"localhost", "", true},
		{"127.0.0.1", "", true},
		{"-bad.example", "", true},
		{"bad-.example", "", true},
	}
	for _, tc := range tests {
		got, err := normalizeAllowedDomain(tc.input)
		if (err != nil) != tc.wantErr {
			t.Errorf("normalizeAllowedDomain(%q) err=%v wantErr=%v", tc.input, err, tc.wantErr)
			continue
		}
		if err == nil && got != tc.want {
			t.Errorf("normalizeAllowedDomain(%q)=%q want %q", tc.input, got, tc.want)
		}
	}
}

func TestTelegramInviteDetection(t *testing.T) {
	for _, link := range []string{
		"https://t.me/+AbCd123",
		"t.me/joinchat/abc",
		"https://telegram.me/addlist/abc",
	} {
		if !isTelegramInviteLink(link) {
			t.Errorf("expected invite link to be blocked: %s", link)
		}
	}
	for _, link := range []string{
		"https://t.me/animechannel",
		"https://example.com/+invite",
		"https://t.me/someuser",
	} {
		if isTelegramInviteLink(link) {
			t.Errorf("unexpected invite link: %s", link)
		}
	}
}

func TestDomainAllowlistMatchesExactAndSubdomains(t *testing.T) {
	allowed := []string{"example.com"}
	for _, host := range []string{"example.com", "www.example.com", "docs.example.com"} {
		if !domainAllowed(host, allowed) {
			t.Errorf("expected %s to be allowed", host)
		}
	}
	for _, host := range []string{"notexample.com", "example.org"} {
		if domainAllowed(host, allowed) {
			t.Errorf("unexpected allowlist match for %s", host)
		}
	}
}

func TestExtractMessageLinksIncludesHiddenTextLinksAndCaptions(t *testing.T) {
	msg := &gotgbot.Message{
		Text: "See example.com",
		Entities: []gotgbot.MessageEntity{{Type: "text_link", URL: "https://hidden.example/path"}},
		Caption: "https://caption.example",
	}
	got := extractMessageLinks(msg)
	want := []string{"https://hidden.example/path", "example.com", "https://caption.example"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("extractMessageLinks()=%v want %v", got, want)
	}
}
