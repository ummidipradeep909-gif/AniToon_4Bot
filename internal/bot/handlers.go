package bot

import (
	"fmt"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/ummidipradeep909-gif/AniToon_4Bot/internal/modules"
)

func Start(b *gotgbot.Bot, ctx *ext.Context) error {
	name := "AniToon_4Bot"
	if b.User != nil && b.User.FirstName != "" {
		name = b.User.FirstName
	}
	_, err := ctx.EffectiveMessage.Reply(b,
		fmt.Sprintf("<b>Welcome to %s!</b>\nA modular Telegram group-management bot.\n\nUse /help to see available commands.", name),
		&gotgbot.SendMessageOpts{ParseMode: "HTML"})
	return err
}

func Help(b *gotgbot.Bot, ctx *ext.Context) error {
	_, err := ctx.EffectiveMessage.Reply(b, modules.HelpText, &gotgbot.SendMessageOpts{ParseMode: "HTML"})
	return err
}

func ID(b *gotgbot.Bot, ctx *ext.Context) error {
	var chatID, userID int64
	if ctx.EffectiveChat != nil { chatID = ctx.EffectiveChat.Id }
	if ctx.EffectiveUser != nil { userID = ctx.EffectiveUser.Id }
	_, err := ctx.EffectiveMessage.Reply(b,
		fmt.Sprintf("<b>Chat ID:</b> <code>%d</code>\n<b>Your ID:</b> <code>%d</code>", chatID, userID),
		&gotgbot.SendMessageOpts{ParseMode: "HTML"})
	return err
}
