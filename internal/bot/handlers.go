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
	text := fmt.Sprintf(
		"<b>Welcome to %s!</b>\nYour all-round Telegram group-management assistant.\n\n"+
			"Use /help to see available commands.\n\n"+
			"<b>Want a bot of your own?</b>\n"+
			"1. Tap <b>Create Your Own Bot</b> below.\n"+
			"2. Open BotFather and send /newbot.\n"+
			"3. Follow its steps to choose a bot name and username.\n"+
			"4. Keep the API token private. Never post it in a group or send it to strangers.\n\n"+
			"<i>This button opens Telegram's official BotFather. Connecting and hosting a personal AniToon clone will be added as a separate, secure setup flow.</i>",
		name,
	)
	keyboard := gotgbot.InlineKeyboardMarkup{
		InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
			{{Text: "🤖 Create Your Own Bot", Url: "https://t.me/BotFather"}},
		},
	}
	_, err := ctx.EffectiveMessage.Reply(b, text, &gotgbot.SendMessageOpts{
		ParseMode: "HTML",
		ReplyMarkup: keyboard,
	})
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
