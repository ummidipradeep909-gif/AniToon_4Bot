package moderation

import (
	"strings"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
)

func TestFormatUserInfo(t *testing.T) {
	user := gotgbot.User{Id: 123, FirstName: "Ada", LastName: "Lovelace", Username: "ada", IsBot: false, LanguageCode: "en"}
	got := formatUserInfo(user, "member")
	for _, want := range []string{"Ada Lovelace", "123", "member", "@ada", "User", "en"} {
		if !strings.Contains(got, want) {
			t.Errorf("formatUserInfo missing %q in %q", want, got)
		}
	}
}

func TestFormatAdminList(t *testing.T) {
	admins := []gotgbot.ChatMember{
		gotgbot.ChatMemberOwner{User: gotgbot.User{Id: 10, FirstName: "Group", LastName: "Owner"}},
		gotgbot.ChatMemberAdministrator{User: gotgbot.User{Id: 20, FirstName: "Mod", Username: "moderator"}, CustomTitle: "Chief Mod"},
	}
	got := formatAdminList(admins)
	for _, want := range []string{"Group Owner", "Owner", "Mod (@moderator)", "Administrator (Chief Mod)", "10", "20"} {
		if !strings.Contains(got, want) {
			t.Errorf("formatAdminList missing %q in %q", want, got)
		}
	}
}

func TestFormatAdminListEmpty(t *testing.T) {
	if got := formatAdminList(nil); got != "No administrators were returned by Telegram." {
		t.Fatalf("unexpected empty admin list: %q", got)
	}
}
