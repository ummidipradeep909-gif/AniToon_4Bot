package moderation

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf16"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	maxGroupNotes = 100
	maxNoteNameRunes = 32
	maxNoteContentUTF16 = 4000
	maxCustomFilters = 100
	maxFilterTriggerRunes = 64
	maxFilterResponseUTF16 = 4000
)

var noteNamePattern = regexp.MustCompile(`^[\p{L}\p{N}_-]+$`)

type communityService struct { db *pgxpool.Pool }

func RegisterCommunityCommands(d *ext.Dispatcher, pool *pgxpool.Pool) {
	s := &communityService{db: pool}
	d.AddHandler(handlers.NewCommand("save", s.saveNote))
	d.AddHandler(handlers.NewCommand("get", s.getNote))
	d.AddHandler(handlers.NewCommand("notes", s.listNotes))
	d.AddHandler(handlers.NewCommand("clear", s.clearNote))
	d.AddHandler(handlers.NewCommand("filter", s.filterCommand))
	d.AddHandler(handlers.NewCommand("filters", s.listFilters))
	d.AddHandler(handlers.NewMessage(nil, s.handleFilter))
}

func communityCommandPayload(m *gotgbot.Message) string {
	if m == nil || strings.TrimSpace(m.Text) == "" { return "" }
	fields := strings.Fields(m.Text)
	if len(fields) < 2 { return "" }
	return strings.TrimSpace(strings.TrimPrefix(m.Text, fields[0]))
}

func communitySplitNameAndContent(raw string) (string, string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" { return "", "", fmt.Errorf("provide a name and some content") }
	parts := strings.SplitN(raw, " ", 2)
	if len(parts) != 2 || strings.TrimSpace(parts[1]) == "" {
		return "", "", fmt.Errorf("provide a name followed by content")
	}
	name := strings.ToLower(strings.TrimSpace(parts[0]))
	if len([]rune(name)) > maxNoteNameRunes || !noteNamePattern.MatchString(name) {
		return "", "", fmt.Errorf("names may contain letters, numbers, underscores or hyphens (maximum 32 characters)")
	}
	content := strings.TrimSpace(parts[1])
	if len(utf16.Encode([]rune(content))) > maxNoteContentUTF16 {
		return "", "", fmt.Errorf("content cannot exceed 4000 UTF-16 units")
	}
	for _, r := range content {
		if unicode.IsControl(r) && r != '\n' && r != '\t' { return "", "", fmt.Errorf("content cannot contain control characters") }
	}
	return name, content, nil
}

func validateFilterTrigger(raw string) (string, error) {
	trigger := strings.ToLower(strings.TrimSpace(raw))
	if trigger == "" || len([]rune(trigger)) > maxFilterTriggerRunes { return "", fmt.Errorf("trigger must contain 1–64 characters") }
	if strings.HasPrefix(trigger, "/") { return "", fmt.Errorf("filter triggers cannot be slash commands") }
	for _, r := range trigger {
		if unicode.IsControl(r) { return "", fmt.Errorf("trigger cannot contain control characters") }
	}
	return trigger, nil
}

func validateFilterResponse(raw string) (string, error) {
	response := strings.TrimSpace(raw)
	if response == "" { return "", fmt.Errorf("provide a response") }
	if len(utf16.Encode([]rune(response))) > maxFilterResponseUTF16 { return "", fmt.Errorf("response cannot exceed 4000 UTF-16 units") }
	for _, r := range response {
		if unicode.IsControl(r) && r != '\n' && r != '\t' { return "", fmt.Errorf("response cannot contain control characters") }
	}
	return response, nil
}

func (s *communityService) requireAdmin(b *gotgbot.Bot, c *ext.Context) (int64, int64, error) {
	chatID, err := groupChat(c); if err != nil { return 0, 0, err }
	actor, err := actorID(c); if err != nil { return 0, 0, err }
	if err = RequireActorAdmin(b, chatID, actor); err != nil { return 0, 0, err }
	if s.db == nil { return 0, 0, fmt.Errorf("notes and custom filters require PostgreSQL") }
	return chatID, actor, nil
}

func (s *communityService) ensureChat(ctx context.Context, chatID int64) error {
	_, err := s.db.Exec(ctx, `INSERT INTO chats(chat_id) VALUES($1) ON CONFLICT(chat_id) DO NOTHING`, chatID)
	return err
}

func (s *communityService) saveNote(b *gotgbot.Bot, c *ext.Context) error {
	chatID, actor, err := s.requireAdmin(b,c); if err != nil { return reply(b,c,err.Error()) }
	name, content, err := communitySplitNameAndContent(communityCommandPayload(c.EffectiveMessage))
	if err != nil { return reply(b,c,"Usage: /save <name> <content>. "+err.Error()) }
	ctx,cancel:=context.WithTimeout(context.Background(),5*time.Second); defer cancel()
	if err=s.ensureChat(ctx,chatID); err==nil {
		var count int
		err=s.db.QueryRow(ctx,`SELECT COUNT(*) FROM group_notes WHERE bot_id=$1 AND chat_id=$2`,b.User.Id,chatID).Scan(&count)
		if err==nil {
			var exists bool
			err=s.db.QueryRow(ctx,`SELECT EXISTS(SELECT 1 FROM group_notes WHERE bot_id=$1 AND chat_id=$2 AND name=$3)`,b.User.Id,chatID,name).Scan(&exists)
			if err==nil && !exists && count>=maxGroupNotes { return reply(b,c,"This group has reached the 100-note limit for this bot. Remove a note before saving another.") }
		}
	}
	if err==nil {
		_,err=s.db.Exec(ctx,`INSERT INTO group_notes(bot_id,chat_id,name,content,created_by,updated_by)
			VALUES($1,$2,$3,$4,$5,$5)
			ON CONFLICT(bot_id,chat_id,name) DO UPDATE SET content=EXCLUDED.content,updated_by=EXCLUDED.updated_by,updated_at=NOW()`,
			b.User.Id,chatID,name,content,actor)
	}
	if err!=nil { return reply(b,c,"I couldn't save that note. Apply migration 014 and check PostgreSQL.") }
	return reply(b,c,fmt.Sprintf("Saved note '%s' for this bot in this group. Use /get %s to retrieve it.",name,name))
}

func (s *communityService) getNote(b *gotgbot.Bot, c *ext.Context) error {
	chatID,err:=groupChat(c); if err!=nil { return reply(b,c,err.Error()) }
	if s.db==nil { return reply(b,c,"Notes require PostgreSQL.") }
	args:=commandArgs(c.EffectiveMessage)
	if len(args)!=1 { return reply(b,c,"Usage: /get <name>") }
	name:=strings.ToLower(args[0])
	ctx,cancel:=context.WithTimeout(context.Background(),5*time.Second); defer cancel()
	var content string
	err=s.db.QueryRow(ctx,`SELECT content FROM group_notes WHERE bot_id=$1 AND chat_id=$2 AND name=$3`,b.User.Id,chatID,name).Scan(&content)
	if err==pgx.ErrNoRows { return reply(b,c,"That note was not found in this group for this bot.") }
	if err!=nil { return reply(b,c,"I couldn't retrieve that note.") }
	_,err=b.SendMessage(chatID,"📝 "+name+"\n\n"+content,nil)
	return err
}

func (s *communityService) listNotes(b *gotgbot.Bot, c *ext.Context) error {
	chatID,err:=groupChat(c); if err!=nil { return reply(b,c,err.Error()) }
	if s.db==nil { return reply(b,c,"Notes require PostgreSQL.") }
	ctx,cancel:=context.WithTimeout(context.Background(),5*time.Second); defer cancel()
	rows,err:=s.db.Query(ctx,`SELECT name FROM group_notes WHERE bot_id=$1 AND chat_id=$2 ORDER BY name LIMIT 100`,b.User.Id,chatID)
	if err!=nil { return reply(b,c,"I couldn't list this group's notes.") }
	defer rows.Close()
	names:=[]string{}
	for rows.Next(){ var name string; if rows.Scan(&name)!=nil{return reply(b,c,"I couldn't list this group's notes.")}; names=append(names,name) }
	if rows.Err()!=nil{return reply(b,c,"I couldn't list this group's notes.")}
	if len(names)==0{return reply(b,c,"No notes have been saved for this bot in this group.")}
	return reply(b,c,"📝 Saved notes:\n"+strings.Join(names,"\n"))
}

func (s *communityService) clearNote(b *gotgbot.Bot, c *ext.Context) error {
	chatID,_,err:=s.requireAdmin(b,c); if err!=nil{return reply(b,c,err.Error())}
	args:=commandArgs(c.EffectiveMessage)
	if len(args)!=1{return reply(b,c,"Usage: /clear <name> (administrator only)")}
	name:=strings.ToLower(args[0])
	ctx,cancel:=context.WithTimeout(context.Background(),5*time.Second);defer cancel()
	tag,err:=s.db.Exec(ctx,`DELETE FROM group_notes WHERE bot_id=$1 AND chat_id=$2 AND name=$3`,b.User.Id,chatID,name)
	if err!=nil{return reply(b,c,"I couldn't remove that note.")}
	if tag.RowsAffected()==0{return reply(b,c,"That note was not found.")}
	return reply(b,c,"Removed note '"+name+"'.")
}

func (s *communityService) filterCommand(b *gotgbot.Bot, c *ext.Context) error {
	chatID,actor,err:=s.requireAdmin(b,c);if err!=nil{return reply(b,c,err.Error())}
	raw:=communityCommandPayload(c.EffectiveMessage)
	parts:=strings.SplitN(raw," ",3)
	if len(parts)!=3{return reply(b,c,"Usage: /filter add <trigger> <response> or /filter remove <trigger>")}
	action:=strings.ToLower(parts[0])
	trigger,err:=validateFilterTrigger(parts[1]);if err!=nil{return reply(b,c,err.Error())}
	ctx,cancel:=context.WithTimeout(context.Background(),5*time.Second);defer cancel()
	if action=="remove" {
		tag,e:=s.db.Exec(ctx,`DELETE FROM chat_filters WHERE bot_id=$1 AND chat_id=$2 AND lower(trigger)=lower($3)`,b.User.Id,chatID,trigger)
		if e!=nil{return reply(b,c,"I couldn't remove that filter.")}
		if tag.RowsAffected()==0{return reply(b,c,"That filter was not found.")}
		return reply(b,c,"Removed custom filter for '"+trigger+"'.")
	}
	if action!="add" {return reply(b,c,"Usage: /filter add <trigger> <response> or /filter remove <trigger>")}
	response,err:=validateFilterResponse(parts[2]);if err!=nil{return reply(b,c,err.Error())}
	if err=s.ensureChat(ctx,chatID);err==nil {
		var count int
		err=s.db.QueryRow(ctx,`SELECT COUNT(*) FROM chat_filters WHERE bot_id=$1 AND chat_id=$2`,b.User.Id,chatID).Scan(&count)
		if err==nil {
			var exists bool
			err=s.db.QueryRow(ctx,`SELECT EXISTS(SELECT 1 FROM chat_filters WHERE bot_id=$1 AND chat_id=$2 AND lower(trigger)=lower($3))`,b.User.Id,chatID,trigger).Scan(&exists)
			if err==nil&&!exists&&count>=maxCustomFilters{return reply(b,c,"This group has reached the 100-filter limit for this bot.")}
		}
	}
	if err==nil {
		_,err=s.db.Exec(ctx,`INSERT INTO chat_filters(bot_id,chat_id,trigger,response,created_by) VALUES($1,$2,$3,$4,$5)
			ON CONFLICT(bot_id,chat_id,lower(trigger)) DO UPDATE SET response=EXCLUDED.response,created_by=EXCLUDED.created_by,created_at=NOW()`,
			b.User.Id,chatID,trigger,response,actor)
	}
	if err!=nil{return reply(b,c,"I couldn't save that filter. Apply migration 014 and check PostgreSQL.")}
	return reply(b,c,"Custom filter saved for '"+trigger+"'. It replies to exact, case-insensitive message or caption matches.")
}

func (s *communityService) listFilters(b *gotgbot.Bot, c *ext.Context) error {
	chatID,_,err:=s.requireAdmin(b,c);if err!=nil{return reply(b,c,err.Error())}
	ctx,cancel:=context.WithTimeout(context.Background(),5*time.Second);defer cancel()
	rows,err:=s.db.Query(ctx,`SELECT trigger FROM chat_filters WHERE bot_id=$1 AND chat_id=$2 ORDER BY trigger LIMIT 100`,b.User.Id,chatID)
	if err!=nil{return reply(b,c,"I couldn't list this group's custom filters.")}
	defer rows.Close()
	triggers:=[]string{}
	for rows.Next(){var trigger string;if rows.Scan(&trigger)!=nil{return reply(b,c,"I couldn't list this group's custom filters.")};triggers=append(triggers,trigger)}
	if rows.Err()!=nil{return reply(b,c,"I couldn't list this group's custom filters.")}
	if len(triggers)==0{return reply(b,c,"No custom filters are configured for this bot in this group.")}
	return reply(b,c,"🔎 Custom filters:\n"+strings.Join(triggers,"\n"))
}

func (s *communityService) handleFilter(b *gotgbot.Bot,c *ext.Context) error {
	m:=c.EffectiveMessage
	if m==nil||m.From==nil||m.From.IsBot||m.Chat==nil||(m.Chat.Type!="group"&&m.Chat.Type!="supergroup")||s.db==nil{return nil}
	text:=strings.TrimSpace(m.Text)
	if text==""{text=strings.TrimSpace(m.Caption)}
	if text==""||strings.HasPrefix(text,"/"){return nil}
	ctx,cancel:=context.WithTimeout(context.Background(),2*time.Second);defer cancel()
	rows,err:=s.db.Query(ctx,`SELECT trigger,response FROM chat_filters WHERE bot_id=$1 AND chat_id=$2 ORDER BY length(trigger) DESC LIMIT 100`,b.User.Id,m.Chat.Id)
	if err!=nil{return nil}
	defer rows.Close()
	for rows.Next(){
		var trigger,response string
		if rows.Scan(&trigger,&response)!=nil{return nil}
		if strings.EqualFold(strings.TrimSpace(text),trigger){
			_,_ = b.SendMessage(m.Chat.Id,response,nil)
			return nil
		}
	}
	return nil
}
