package bot

import (
 "fmt"
 "html"
 "strings"

 "github.com/PaulSonOfLars/gotgbot/v2"
 "github.com/PaulSonOfLars/gotgbot/v2/ext"
 "github.com/ummidipradeep909-gif/AniToon_4Bot/internal/modules"
)

var primaryBotID int64

// SetPrimaryBotID distinguishes the master bot from user-owned clones in /start.
func SetPrimaryBotID(id int64) { primaryBotID = id }

func Start(b *gotgbot.Bot,ctx *ext.Context) error {
 name:="AniToon_4Bot"
 if b.User.FirstName!="" { name=b.User.FirstName }
 text:=fmt.Sprintf("<b>Welcome to %s!</b>\nYour Telegram group-management assistant.\n\nUse /help to see available commands.",name)
 opts:=&gotgbot.SendMessageOpts{ParseMode:"HTML"}
 if b.User.Id==primaryBotID {
  text+="\n\n<b>Want your own independent AniToon clone?</b>\n1. Tap the button below and create a bot with @BotFather using /newbot.\n2. Return here and send /clone.\n3. Send the API token from BotFather in this private chat.\n4. I will delete your token message, verify it, and start your clone.\n\n<i>Never send your token in a group or share it with anyone.</i>"
  opts.ReplyMarkup=gotgbot.InlineKeyboardMarkup{InlineKeyboard:[][]gotgbot.InlineKeyboardButton{{{Text:"🤖 Create Your Own Clone Bot",Url:"https://t.me/BotFather"}}}}
 }
 _,err:=ctx.EffectiveMessage.Reply(b,text,opts); return err
}

func Help(b *gotgbot.Bot,ctx *ext.Context) error {
 _,err:=ctx.EffectiveMessage.Reply(b,modules.HelpText,&gotgbot.SendMessageOpts{ParseMode:"HTML"}); return err
}

func ID(b *gotgbot.Bot, ctx *ext.Context) error {
 var chatID, userID int64
 chatType, chatTitle := "unknown", ""
 if ctx.EffectiveChat != nil {
  chatID, chatType, chatTitle = ctx.EffectiveChat.Id, ctx.EffectiveChat.Type, ctx.EffectiveChat.Title
 }
 if ctx.EffectiveUser != nil { userID = ctx.EffectiveUser.Id }
 targetLine := fmt.Sprintf("<b>Your ID:</b> <code>%d</code>", userID)
 if ctx.EffectiveMessage != nil && ctx.EffectiveMessage.ReplyToMessage != nil && ctx.EffectiveMessage.ReplyToMessage.From != nil {
  target := ctx.EffectiveMessage.ReplyToMessage.From
  targetLine = fmt.Sprintf("<b>Replied member:</b> %s (<code>%d</code>)", html.EscapeString(compactDisplayName(target)), target.Id)
 }
 message := fmt.Sprintf("<b>Telegram IDs</b>\n<b>Chat ID:</b> <code>%d</code>\n<b>Chat type:</b> %s\n<b>Chat title:</b> %s\n%s", chatID, html.EscapeString(chatType), html.EscapeString(chatTitle), targetLine)
 _, err := ctx.EffectiveMessage.Reply(b, message, &gotgbot.SendMessageOpts{ParseMode: "HTML"})
 return err
}

func compactDisplayName(user *gotgbot.User) string {
 if user == nil { return "Unknown" }
 name := strings.Join(strings.Fields(strings.TrimSpace(user.FirstName+" "+user.LastName)), " ")
 if name == "" { return "Unknown" }
 return name
}
