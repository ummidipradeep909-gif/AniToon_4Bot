package moderation

import (
	"context"
	"fmt"
	"html"
	"strings"
	"time"
	"unicode"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const maxGreetingTemplateRunes = 1000

const defaultWelcomeTemplate = "👋 Welcome {mention} to {chatname}!\nPlease read the group rules."
const defaultGoodbyeTemplate = "👋 {fullname} has left {chatname}."

type greetingService struct { db *pgxpool.Pool }

func RegisterGreetingCommands(d *ext.Dispatcher, pool *pgxpool.Pool) {
	s := &greetingService{db: pool}
	d.AddHandler(handlers.NewCommand("welcome", func(b *gotgbot.Bot,c *ext.Context) error { return s.toggle(b,c,true) }))
	d.AddHandler(handlers.NewCommand("goodbye", func(b *gotgbot.Bot,c *ext.Context) error { return s.toggle(b,c,false) }))
	d.AddHandler(handlers.NewCommand("setwelcome", func(b *gotgbot.Bot,c *ext.Context) error { return s.setTemplate(b,c,true) }))
	d.AddHandler(handlers.NewCommand("setgoodbye", func(b *gotgbot.Bot,c *ext.Context) error { return s.setTemplate(b,c,false) }))
	d.AddHandler(handlers.NewCommand("resetwelcome", func(b *gotgbot.Bot,c *ext.Context) error { return s.resetTemplate(b,c,true) }))
	d.AddHandler(handlers.NewCommand("resetgoodbye", func(b *gotgbot.Bot,c *ext.Context) error { return s.resetTemplate(b,c,false) }))
	d.AddHandler(handlers.NewChatMember(nil,s.handleMembership))
}

func (s *greetingService) requireAdmin(b *gotgbot.Bot,c *ext.Context) (int64,int64,error) {
	chatID,err:=groupChat(c); if err!=nil{return 0,0,err}
	actor,err:=actorID(c); if err!=nil{return 0,0,err}
	if err=RequireActorAdmin(b,chatID,actor); err!=nil{return 0,0,err}
	if s.db==nil{return 0,0,fmt.Errorf("welcome and goodbye settings require PostgreSQL")}
	return chatID,actor,nil
}

func greetingCommandText(m *gotgbot.Message) string {
	if m==nil || strings.TrimSpace(m.Text)=="" { return "" }
	fields:=strings.Fields(m.Text); if len(fields)<2{return ""}
	return strings.TrimSpace(strings.TrimPrefix(m.Text,fields[0]))
}

func validateGreetingTemplate(raw string) (string,error) {
	value:=strings.TrimSpace(raw)
	if value=="" { return "",fmt.Errorf("provide a message template") }
	if len([]rune(value))>maxGreetingTemplateRunes { return "",fmt.Errorf("templates are limited to 1000 characters") }
	for _,r:=range value { if unicode.IsControl(r) && r!='\n' && r!='\t' { return "",fmt.Errorf("templates cannot contain control characters") } }
	return value,nil
}

func (s *greetingService) ensureChat(ctx context.Context,chatID int64) error {
	_,err:=s.db.Exec(ctx,`INSERT INTO chats(chat_id) VALUES($1) ON CONFLICT(chat_id) DO NOTHING`,chatID)
	return err
}

func (s *greetingService) toggle(b *gotgbot.Bot,c *ext.Context,welcome bool) error {
	chatID,actor,err:=s.requireAdmin(b,c); if err!=nil{return reply(b,c,err.Error())}
	command:="welcome"; column:="welcome_enabled"; label:="Welcome"
	if !welcome { command="goodbye"; column="goodbye_enabled"; label="Goodbye" }
	args:=commandArgs(c.EffectiveMessage)
	ctx,cancel:=context.WithTimeout(context.Background(),5*time.Second); defer cancel()
	if len(args)==0 {
		var enabled bool
		err=s.db.QueryRow(ctx,"SELECT "+column+" FROM greeting_configs WHERE bot_id=$1 AND chat_id=$2",b.User.Id,chatID).Scan(&enabled)
		if err==pgx.ErrNoRows { return reply(b,c,label+" messages are disabled. Use /"+command+" on to enable them.") }
		if err!=nil{return reply(b,c,"I couldn't read this group's "+strings.ToLower(label)+" settings.")}
		if enabled{return reply(b,c,label+" messages are enabled.")}
		return reply(b,c,label+" messages are disabled.")
	}
	if len(args)!=1 || (!strings.EqualFold(args[0],"on") && !strings.EqualFold(args[0],"off")) {
		return reply(b,c,"Usage: /"+command+" on or /"+command+" off")
	}
	enabled:=strings.EqualFold(args[0],"on")
	if err=s.ensureChat(ctx,chatID); err==nil {
		_,err=s.db.Exec(ctx,"INSERT INTO greeting_configs(bot_id,chat_id,"+column+",updated_by) VALUES($1,$2,$3,$4) ON CONFLICT(bot_id,chat_id) DO UPDATE SET "+column+"=EXCLUDED."+column+",updated_by=EXCLUDED.updated_by,updated_at=NOW()",b.User.Id,chatID,enabled,actor)
	}
	if err!=nil{return reply(b,c,"I couldn't save the setting. Apply migration 011 and check PostgreSQL.")}
	if enabled{return reply(b,c,label+" messages enabled for this bot in this group.")}
	return reply(b,c,label+" messages disabled for this bot in this group.")
}

func (s *greetingService) setTemplate(b *gotgbot.Bot,c *ext.Context,welcome bool) error {
	chatID,actor,err:=s.requireAdmin(b,c); if err!=nil{return reply(b,c,err.Error())}
	template,err:=validateGreetingTemplate(greetingCommandText(c.EffectiveMessage))
	if err!=nil {
		command:="/setwelcome"; if !welcome {command="/setgoodbye"}
		return reply(b,c,"Usage: "+command+" <message>. "+err.Error())
	}
	column:="welcome_template"; if !welcome {column="goodbye_template"}
	ctx,cancel:=context.WithTimeout(context.Background(),5*time.Second); defer cancel()
	if err=s.ensureChat(ctx,chatID); err==nil {
		_,err=s.db.Exec(ctx,"INSERT INTO greeting_configs(bot_id,chat_id,"+column+",updated_by) VALUES($1,$2,$3,$4) ON CONFLICT(bot_id,chat_id) DO UPDATE SET "+column+"=EXCLUDED."+column+",updated_by=EXCLUDED.updated_by,updated_at=NOW()",b.User.Id,chatID,template,actor)
	}
	if err!=nil{return reply(b,c,"I couldn't save the message template. Apply migration 011 and check PostgreSQL.")}
	name:="Welcome"; if !welcome {name="Goodbye"}
	return reply(b,c,name+" template saved. Use /"+strings.ToLower(name)+" on to enable it.")
}

func (s *greetingService) resetTemplate(b *gotgbot.Bot,c *ext.Context,welcome bool) error {
	chatID,actor,err:=s.requireAdmin(b,c); if err!=nil{return reply(b,c,err.Error())}
	column:="welcome_template"; value:=defaultWelcomeTemplate; name:="Welcome"
	if !welcome {column="goodbye_template";value=defaultGoodbyeTemplate;name="Goodbye"}
	ctx,cancel:=context.WithTimeout(context.Background(),5*time.Second); defer cancel()
	if err=s.ensureChat(ctx,chatID); err==nil {
		_,err=s.db.Exec(ctx,"INSERT INTO greeting_configs(bot_id,chat_id,"+column+",updated_by) VALUES($1,$2,$3,$4) ON CONFLICT(bot_id,chat_id) DO UPDATE SET "+column+"=EXCLUDED."+column+",updated_by=EXCLUDED.updated_by,updated_at=NOW()",b.User.Id,chatID,value,actor)
	}
	if err!=nil{return reply(b,c,"I couldn't reset the template. Check PostgreSQL and migration 011.")}
	return reply(b,c,name+" template restored to its default.")
}

func renderGreeting(template string, user gotgbot.User, chat gotgbot.Chat) string {
	first,last,username:=user.FirstName,user.LastName,user.Username
	full:=strings.TrimSpace(first+" "+last)
	if full=="" {full=fmt.Sprintf("User %d",user.Id)}
	if first=="" {first=full}
	if username!="" {username="@"+username}
	mention:=fmt.Sprintf("<a href=\"tg://user?id=%d\">%s</a>",user.Id,html.EscapeString(full))
	escaped:=html.EscapeString(template)
	replacer:=strings.NewReplacer(
		"{first}",html.EscapeString(first),"{last}",html.EscapeString(last),
		"{fullname}",html.EscapeString(full),"{username}",html.EscapeString(username),
		"{mention}",mention,"{id}",fmt.Sprintf("%d",user.Id),
		"{chatname}",html.EscapeString(chat.Title),
	)
	return replacer.Replace(escaped)
}

func isGreetingJoin(oldStatus,newStatus string) bool {
	return (oldStatus=="left" || oldStatus=="kicked") && (newStatus=="member" || newStatus=="administrator" || newStatus=="creator")
}
func isGreetingLeave(oldStatus,newStatus string) bool {
	wasMember:=oldStatus=="member" || oldStatus=="administrator" || oldStatus=="creator" || oldStatus=="restricted"
	isGone:=newStatus=="left" || newStatus=="kicked"
	return wasMember && isGone
}

func (s *greetingService) handleMembership(b *gotgbot.Bot,c *ext.Context) error {
	update:=c.ChatMember
	if update==nil || (update.Chat.Type!="group" && update.Chat.Type!="supergroup") || s.db==nil {return nil}
	oldStatus,newStatus:=update.OldChatMember.GetStatus(),update.NewChatMember.GetStatus()
	kind:=""
	column:=""
	if isGreetingJoin(oldStatus,newStatus) {kind="welcome";column="welcome_enabled"} else if isGreetingLeave(oldStatus,newStatus) {kind="goodbye";column="goodbye_enabled"} else {return nil}
	member:=update.NewChatMember.GetUser()
	if member.Id<=0 || member.IsBot {return nil}
	ctx,cancel:=context.WithTimeout(context.Background(),4*time.Second); defer cancel()
	var enabled bool
	var template string
	err:=s.db.QueryRow(ctx,"SELECT "+column+", "+kind+"_template FROM greeting_configs WHERE bot_id=$1 AND chat_id=$2",b.User.Id,update.Chat.Id).Scan(&enabled,&template)
	if err!=nil || !enabled {return nil}
	if strings.TrimSpace(template)=="" {
		if kind=="welcome" {template=defaultWelcomeTemplate} else {template=defaultGoodbyeTemplate}
	}
	message:=renderGreeting(template,member,update.Chat)
	_,err=b.SendMessage(update.Chat.Id,message,&gotgbot.SendMessageOpts{ParseMode:"HTML",LinkPreviewOptions:&gotgbot.LinkPreviewOptions{IsDisabled:true}})
	return err
}
