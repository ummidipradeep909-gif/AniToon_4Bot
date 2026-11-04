package moderation

import (
	"fmt"
	"strings"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
)

const maxAdminListEntries = 40

func RegisterUserInfoCommands(d *ext.Dispatcher) {
	d.AddHandler(handlers.NewCommand("info", UserInfo))
	d.AddHandler(handlers.NewCommand("admins", AdminList))
}

func compactName(user gotgbot.User) string {
	name := strings.Join(strings.Fields(strings.TrimSpace(user.FirstName+" "+user.LastName)), " ")
	if name == "" {
		name = "Unknown name"
	}
	return name
}

func formatUserInfo(user gotgbot.User, status string) string {
	var out strings.Builder
	fmt.Fprintf(&out, "👤 Member information\nName: %s\nUser ID: %d\nStatus: %s", compactName(user), user.Id, status)
	if user.Username != "" {
		fmt.Fprintf(&out, "\nUsername: @%s", user.Username)
	}
	if user.IsBot {
		out.WriteString("\nAccount type: Bot")
	} else {
		out.WriteString("\nAccount type: User")
	}
	if user.LanguageCode != "" {
		fmt.Fprintf(&out, "\nLanguage: %s", user.LanguageCode)
	}
	return out.String()
}

func formatAdminList(admins []gotgbot.ChatMember) string {
	if len(admins) == 0 {
		return "No administrators were returned by Telegram."
	}
	var out strings.Builder
	fmt.Fprintf(&out, "👮 Group administrators (%d)", len(admins))
	shown := 0
	for _, member := range admins {
		if shown >= maxAdminListEntries {
			break
		}
		user := member.GetUser()
		role := "Administrator"
		if member.GetStatus() == StatusCreator {
			role = "Owner"
		}
		name := compactName(user)
		if user.Username != "" {
			name += " (@" + user.Username + ")"
		}
		line := fmt.Sprintf("\n%d. %s — %s — ID: %d", shown+1, name, role, user.Id)
		if out.Len()+len(line) > 3500 {
			break
		}
		out.WriteString(line)
		shown++
	}
	if shown < len(admins) {
		fmt.Fprintf(&out, "\nShowing %d of %d administrators.", shown, len(admins))
	}
	return out.String()
}

// UserInfo returns Telegram's current member record. Use a reply or numeric ID;
// usernames are intentionally not treated as resolvable IDs.
func UserInfo(b *gotgbot.Bot, c *ext.Context) error {
	chatID, err := groupChat(c)
	if err != nil {
		return reply(b, c, err.Error())
	}
	targetID, extra, err := resolveTarget(c.EffectiveMessage, commandArgs(c.EffectiveMessage))
	if err != nil || strings.TrimSpace(extra) != "" {
		return reply(b, c, "Usage: reply with /info or use /info <numeric_user_id>. "+targetError(err))
	}
	member, err := b.GetChatMember(chatID, targetID, nil)
	if err != nil {
		return reply(b, c, "I couldn't retrieve that member. To inspect other members, make me a group administrator and try again.")
	}
	return reply(b, c, formatUserInfo(member.GetUser(), member.GetStatus()))
}

func targetError(err error) string {
	if err == nil {
		return "Do not add extra arguments."
	}
	return err.Error()
}

// AdminList fetches the live administrator list from Telegram, not a local cache.
func AdminList(b *gotgbot.Bot, c *ext.Context) error {
	chatID, err := groupChat(c)
	if err != nil {
		return reply(b, c, err.Error())
	}
	admins, err := b.GetChatAdministrators(chatID, nil)
	if err != nil {
		return reply(b, c, "I couldn't retrieve the administrator list. Check that I am a member of this group and try again.")
	}
	return reply(b, c, formatAdminList(admins))
}
