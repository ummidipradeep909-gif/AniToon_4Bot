package clones

import (
 "fmt"
 "log/slog"
 "strings"
 "sync"
 "time"

 "github.com/PaulSonOfLars/gotgbot/v2"
 "github.com/PaulSonOfLars/gotgbot/v2/ext"
 "github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
 "github.com/ummidipradeep909-gif/AniToon_4Bot/internal/bot"
 "github.com/ummidipradeep909-gif/AniToon_4Bot/internal/moderation"
)

// Manager holds clone credentials only inside live Bot objects in memory.
// It deliberately does not persist tokens or include them in logs/replies.
type Manager struct {
 mu sync.Mutex
 updater *ext.Updater
 primaryID int64
 pending map[int64]time.Time
 owners map[int64]int64
 active map[int64]bool
 logger *slog.Logger
}

func NewManager(updater *ext.Updater, primaryID int64, logger *slog.Logger) *Manager {
 if logger == nil { logger = slog.Default() }
 return &Manager{updater:updater, primaryID:primaryID, pending:map[int64]time.Time{}, owners:map[int64]int64{}, active:map[int64]bool{}, logger:logger}
}

func (m *Manager) Register(d *ext.Dispatcher) {
 d.AddHandler(handlers.NewCommand("clone", m.Begin))
 d.AddHandler(handlers.NewCommand("cancel", m.Cancel))
 d.AddHandler(handlers.NewMessage(nil, m.ReceiveToken))
}

func (m *Manager) Begin(b *gotgbot.Bot, c *ext.Context) error {
 if b.User.Id != m.primaryID || c.EffectiveChat == nil || c.EffectiveUser == nil || c.EffectiveMessage == nil { return nil }
 if c.EffectiveChat.Type != "private" {
  _,e:=c.EffectiveMessage.Reply(b,"For token safety, open my private chat and send /clone there.",nil); return e
 }
 id:=c.EffectiveUser.Id
 m.mu.Lock()
 if m.owners[id]!=0 { m.mu.Unlock(); _,e:=c.EffectiveMessage.Reply(b,"You already have a clone connected. Only one clone per account is supported for now.",nil); return e }
 m.pending[id]=time.Now().Add(10*time.Minute)
 m.mu.Unlock()
 text:="<b>Create your own AniToon clone bot</b>\n\n1. Tap Open BotFather and send /newbot.\n2. Follow the steps to choose a bot name and username.\n3. Copy the API token BotFather gives you.\n4. Return here and send the token in this private chat.\n\nI will delete the token message immediately, verify it with Telegram, and start your clone if valid. Setup expires in 10 minutes.\n\n<b>Security:</b> never send your token in a group or to another person. Use /cancel to stop."
 kb:=gotgbot.InlineKeyboardMarkup{InlineKeyboard:[][]gotgbot.InlineKeyboardButton{{{Text:"🤖 Open BotFather",Url:"https://t.me/BotFather"}}}}
 _,e:=c.EffectiveMessage.Reply(b,text,&gotgbot.SendMessageOpts{ParseMode:"HTML",ReplyMarkup:kb}); return e
}

func (m *Manager) Cancel(b *gotgbot.Bot,c *ext.Context) error {
 if c.EffectiveChat==nil || c.EffectiveUser==nil || c.EffectiveMessage==nil || c.EffectiveChat.Type!="private" { return nil }
 m.mu.Lock(); delete(m.pending,c.EffectiveUser.Id); m.mu.Unlock()
 _,e:=c.EffectiveMessage.Reply(b,"Clone setup cancelled.",nil); return e
}

func (m *Manager) ReceiveToken(b *gotgbot.Bot,c *ext.Context) error {
 msg:=c.EffectiveMessage
 if b.User.Id!=m.primaryID || msg==nil || c.EffectiveChat==nil || c.EffectiveUser==nil || c.EffectiveChat.Type!="private" { return nil }
 owner:=c.EffectiveUser.Id
 m.mu.Lock()
 expiry,waiting:=m.pending[owner]
 if waiting && time.Now().After(expiry) { delete(m.pending,owner); waiting=false }
 connected:=m.owners[owner]!=0
 m.mu.Unlock()
 if !waiting { return nil }
 // Delete the message before token validation. Never echo or log its contents.
 _,_=b.DeleteMessage(c.EffectiveChat.Id,msg.MessageId,nil)
 token:=strings.TrimSpace(msg.Text)
 if connected { _,e:=b.SendMessage(c.EffectiveChat.Id,"You already have a clone connected.",nil); return e }
 if strings.ContainsAny(token," \t\r\n") || !strings.Contains(token,":") || len(token)<20 || len(token)>256 {
  _,e:=b.SendMessage(c.EffectiveChat.Id,"That doesn't look like a BotFather token. I deleted the message. Send /clone to restart setup.",nil); return e
 }
 user,err:=m.start(token,owner)
 token=""
 if err!=nil {
  m.mu.Lock(); delete(m.pending,owner); m.mu.Unlock()
  _,e:=b.SendMessage(c.EffectiveChat.Id,"I couldn't connect that bot. Check the token and make sure it isn't running on another host, then send /clone to try again. Your token message was deleted.",nil); return e
 }
 m.mu.Lock(); delete(m.pending,owner); m.mu.Unlock()
 _,e:=b.SendMessage(c.EffectiveChat.Id,fmt.Sprintf("<b>Your AniToon clone is online!</b>\n\nBot: @%s\n\nOpen it and send /start. Add it to your group and promote it with the permissions needed for moderation.\n\nThe token is held in memory only, not saved to the database or logs. If this service restarts, reconnect with /clone.",user.Username),&gotgbot.SendMessageOpts{ParseMode:"HTML"})
 return e
}

func (m *Manager) start(token string,owner int64)(*gotgbot.User,error) {
 clone,err:=gotgbot.NewBot(token,nil) // Telegram getMe validates the token.
 token=""
 if err!=nil { return nil,fmt.Errorf("invalid token") }
 if clone.User.Id==m.primaryID { return nil,fmt.Errorf("cannot clone the primary bot") }
 m.mu.Lock(); defer m.mu.Unlock()
 if m.active[clone.User.Id] { return nil,fmt.Errorf("clone already connected") }
 if m.owners[owner]!=0 { return nil,fmt.Errorf("owner already has a clone") }
 webhook,err:=clone.GetWebhookInfo(nil)
 if err!=nil { return nil,fmt.Errorf("could not check webhook") }
 if webhook!=nil && webhook.Url!="" { return nil,fmt.Errorf("remove existing webhook/hosting before connecting") }
 err=m.updater.StartPolling(clone,&ext.PollingOpts{GetUpdatesOpts:&gotgbot.GetUpdatesOpts{Timeout:30,RequestOpts:&gotgbot.RequestOpts{Timeout:35*time.Second}}})
 if err!=nil { return nil,fmt.Errorf("clone polling failed") }
 m.active[clone.User.Id]=true
 m.owners[owner]=clone.User.Id
 m.logger.Info("clone bot connected","bot_id",clone.User.Id,"owner_id",owner)
 u:=clone.User; return &u,nil
}

// Common commands are registered once; the shared updater dispatches updates
// from the primary bot and each clone through the same dispatcher.
func RegisterCommands(d *ext.Dispatcher) {
 d.AddHandler(handlers.NewCommand("start",bot.Start))
 d.AddHandler(handlers.NewCommand("help",bot.Help))
 d.AddHandler(handlers.NewCommand("id",bot.ID))
 d.AddHandler(handlers.NewCommand("ban",moderation.Ban))
 d.AddHandler(handlers.NewCommand("unban",moderation.Unban))
 d.AddHandler(handlers.NewCommand("mute",moderation.Mute))
 d.AddHandler(handlers.NewCommand("unmute",moderation.Unmute))
 d.AddHandler(handlers.NewCommand("kick",moderation.Kick))
}
