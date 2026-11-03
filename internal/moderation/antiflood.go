package moderation

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

type floodConfig struct {
	Limit int
	WindowSeconds int
	Action string
}

type floodAdminCacheEntry struct {
	admin bool
	expiresAt time.Time
}

type floodService struct {
	db *pgxpool.Pool
	redis *redis.Client
	mu sync.RWMutex
	adminCache map[string]floodAdminCacheEntry
}

func RegisterAntiFloodCommands(d *ext.Dispatcher, pool *pgxpool.Pool, client *redis.Client) {
	s := &floodService{db: pool, redis: client, adminCache: make(map[string]floodAdminCacheEntry)}
	d.AddHandler(handlers.NewCommand("setflood", func(b *gotgbot.Bot, c *ext.Context) error { return s.setConfig(b,c) }))
	d.AddHandler(handlers.NewCommand("flood", func(b *gotgbot.Bot, c *ext.Context) error { return s.showConfig(b,c) }))
	d.AddHandler(handlers.NewCommand("clearflood", func(b *gotgbot.Bot, c *ext.Context) error { return s.clearConfig(b,c) }))
	d.AddHandler(handlers.NewMessage(func(m *gotgbot.Message) bool {
		return m != nil && (m.Chat.Type == gotgbot.ChatTypeGroup || m.Chat.Type == gotgbot.ChatTypeSupergroup) && m.From != nil && !m.From.IsBot() && m.From.Id > 0
	}, s.handleMessage))
}

func parseFloodConfig(args []string) (floodConfig, error) {
	if len(args) < 2 || len(args) > 3 {
		return floodConfig{}, fmt.Errorf("usage: /setflood <message_limit> <seconds> [delete|mute|ban]")
	}
	limit, err := strconv.Atoi(args[0])
	if err != nil || limit < 2 || limit > 50 {
		return floodConfig{}, fmt.Errorf("message limit must be between 2 and 50")
	}
	seconds, err := strconv.Atoi(args[1])
	if err != nil || seconds < 3 || seconds > 300 {
		return floodConfig{}, fmt.Errorf("time window must be between 3 and 300 seconds")
	}
	action := "delete"
	if len(args) == 3 { action = strings.ToLower(args[2]) }
	if action != "delete" && action != "mute" && action != "ban" {
		return floodConfig{}, fmt.Errorf("action must be delete, mute or ban")
	}
	return floodConfig{Limit: limit, WindowSeconds: seconds, Action: action}, nil
}

func (s *floodService) available() error {
	if s.db == nil { return fmt.Errorf("anti-flood storage is unavailable: PostgreSQL is not configured") }
	if s.redis == nil { return fmt.Errorf("anti-flood detection is unavailable: Redis is not configured") }
	return nil
}

func (s *floodService) setConfig(b *gotgbot.Bot, c *ext.Context) error {
	chatID, err := groupChat(c); if err != nil { return reply(b,c,err.Error()) }
	actor, err := actorID(c); if err != nil { return reply(b,c,err.Error()) }
	if err = RequireActorAdmin(b,chatID,actor); err != nil { return reply(b,c,err.Error()) }
	if err = s.available(); err != nil { return reply(b,c,err.Error()) }
	cfg, err := parseFloodConfig(commandArgs(c.EffectiveMessage))
	if err != nil { return reply(b,c,err.Error()) }
	if cfg.Action == "delete" {
		if err = RequireBotDeletePermission(b,chatID); err != nil { return reply(b,c,err.Error()) }
	} else {
		if err = RequireBotRestrictPermission(b,chatID); err != nil { return reply(b,c,err.Error()) }
	}
	ctx,cancel:=context.WithTimeout(context.Background(),5*time.Second); defer cancel()
	_,err=s.db.Exec(ctx,`INSERT INTO chats(chat_id) VALUES($1) ON CONFLICT(chat_id) DO NOTHING`,chatID)
	if err==nil {
		_,err=s.db.Exec(ctx,`INSERT INTO flood_configs(bot_id,chat_id,message_limit,window_seconds,action,updated_by)
			VALUES($1,$2,$3,$4,$5,$6)
			ON CONFLICT(bot_id,chat_id) DO UPDATE SET message_limit=EXCLUDED.message_limit,window_seconds=EXCLUDED.window_seconds,action=EXCLUDED.action,updated_by=EXCLUDED.updated_by,updated_at=NOW()`,
			b.User.Id,chatID,cfg.Limit,cfg.WindowSeconds,cfg.Action,actor)
	}
	if err != nil { return reply(b,c,"I couldn't save the anti-flood settings. Please check the database and migration.") }
	return reply(b,c,fmt.Sprintf("🛡️ Anti-flood enabled: more than %d messages in %d seconds triggers %s.",cfg.Limit,cfg.WindowSeconds,cfg.Action))
}

func (s *floodService) showConfig(b *gotgbot.Bot,c *ext.Context) error {
	chatID,err:=groupChat(c); if err!=nil{return reply(b,c,err.Error())}
	actor,err:=actorID(c); if err!=nil{return reply(b,c,err.Error())}
	if err=RequireActorAdmin(b,chatID,actor); err!=nil{return reply(b,c,err.Error())}
	if s.db==nil{return reply(b,c,"Anti-flood storage is unavailable.")}
	ctx,cancel:=context.WithTimeout(context.Background(),5*time.Second); defer cancel()
	var cfg floodConfig
	err=s.db.QueryRow(ctx,`SELECT message_limit,window_seconds,action FROM flood_configs WHERE bot_id=$1 AND chat_id=$2`,b.User.Id,chatID).Scan(&cfg.Limit,&cfg.WindowSeconds,&cfg.Action)
	if err==pgx.ErrNoRows{return reply(b,c,"Anti-flood is disabled in this group. Use /setflood 5 10 delete.")}
	if err!=nil{return reply(b,c,"I couldn't read this group's anti-flood settings.")}
	return reply(b,c,fmt.Sprintf("🛡️ Anti-flood is enabled: more than %d messages in %d seconds; action: %s.",cfg.Limit,cfg.WindowSeconds,cfg.Action))
}

func (s *floodService) clearConfig(b *gotgbot.Bot,c *ext.Context) error {
	chatID,err:=groupChat(c); if err!=nil{return reply(b,c,err.Error())}
	actor,err:=actorID(c); if err!=nil{return reply(b,c,err.Error())}
	if err=RequireActorAdmin(b,chatID,actor); err!=nil{return reply(b,c,err.Error())}
	if s.db==nil{return reply(b,c,"Anti-flood storage is unavailable.")}
	ctx,cancel:=context.WithTimeout(context.Background(),5*time.Second); defer cancel()
	_,err=s.db.Exec(ctx,`DELETE FROM flood_configs WHERE bot_id=$1 AND chat_id=$2`,b.User.Id,chatID)
	if err!=nil{return reply(b,c,"I couldn't clear this group's anti-flood settings.")}
	return reply(b,c,"Anti-flood has been disabled in this group.")
}

func floodCounterKey(botID,chatID,userID int64) string {
	return fmt.Sprintf("anitoon4:flood:%d:%d:%d",botID,chatID,userID)
}
func floodActionKey(botID,chatID,userID int64) string {
	return fmt.Sprintf("anitoon4:flood-action:%d:%d:%d",botID,chatID,userID)
}

func (s *floodService) getConfig(ctx context.Context,botID,chatID int64)(floodConfig,error) {
	var cfg floodConfig
	err:=s.db.QueryRow(ctx,`SELECT message_limit,window_seconds,action FROM flood_configs WHERE bot_id=$1 AND chat_id=$2`,botID,chatID).Scan(&cfg.Limit,&cfg.WindowSeconds,&cfg.Action)
	return cfg,err
}

func (s *floodService) isAdmin(b *gotgbot.Bot,chatID,userID int64) bool {
	key:=fmt.Sprintf("%d:%d:%d",b.User.Id,chatID,userID)
	now:=time.Now()
	s.mu.RLock(); cached,ok:=s.adminCache[key]; s.mu.RUnlock()
	if ok && now.Before(cached.expiresAt) { return cached.admin }
	member,err:=b.GetChatMember(chatID,userID,nil)
	if err!=nil || member == nil { return true } // fail open: never auto-punish when exemption cannot be verified
	admin:=IsChatAdmin(member.GetStatus())
	s.mu.Lock()
	if len(s.adminCache)>4096 { s.adminCache=make(map[string]floodAdminCacheEntry) }
	s.adminCache[key]=floodAdminCacheEntry{admin:admin,expiresAt:now.Add(30*time.Second)}
	s.mu.Unlock()
	return admin
}

var floodWindowScript=redis.NewScript(`
redis.call('ZREMRANGEBYSCORE', KEYS[1], '-inf', ARGV[1])
redis.call('ZADD', KEYS[1], ARGV[2], ARGV[3])
redis.call('PEXPIRE', KEYS[1], ARGV[4])
local count=redis.call('ZCARD', KEYS[1])
return count
`)

func (s *floodService) handleMessage(b *gotgbot.Bot,c *ext.Context) error {
	m:=c.EffectiveMessage
	if m==nil || m.From==nil || m.MessageId<=0 { return nil }
	if s.db==nil || s.redis==nil { return nil }
	chatID,userID:=m.Chat.Id,m.From.Id
	if s.isAdmin(b,chatID,userID) { return nil }
	ctx,cancel:=context.WithTimeout(context.Background(),2*time.Second); defer cancel()
	cfg,err:=s.getConfig(ctx,b.User.Id,chatID)
	if err==pgx.ErrNoRows { return nil }
	if err!=nil { return nil } // fail open on configuration/database errors
	now:=time.Now()
	count,err:=floodWindowScript.Run(ctx,s.redis,[]string{floodCounterKey(b.User.Id,chatID,userID)},
		now.Add(-time.Duration(cfg.WindowSeconds)*time.Second).UnixMilli(),now.UnixMilli(),
		fmt.Sprintf("%d:%d",m.MessageId,now.UnixNano()),cfg.WindowSeconds*1000).Int64()
	if err!=nil || count<=int64(cfg.Limit) { return nil }
	// Remove every over-limit message where permitted; apply mute/ban at most once per minute.
	_, deleteErr := b.DeleteMessage(chatID,m.MessageId,nil)
	if cfg.Action=="delete" { return deleteErr }
	claimed,err:=s.redis.SetNX(ctx,floodActionKey(b.User.Id,chatID,userID),"1",time.Minute).Result()
	if err!=nil || !claimed { return nil }
	switch cfg.Action {
	case "mute":
		until:=time.Now().Add(10*time.Minute).Unix()
		_,err=b.RestrictChatMember(chatID,userID,gotgbot.ChatPermissions{},&gotgbot.RestrictChatMemberOpts{UseIndependentChatPermissions:true,UntilDate:until})
	case "ban":
		_,err=b.BanChatMember(chatID,userID,&gotgbot.BanChatMemberOpts{RevokeMessages:false})
	}
	return err
}
