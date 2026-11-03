package moderation

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

type raidConfig struct {
	JoinLimit int
	WindowSeconds int
	QuarantineMinutes int
}

type antiRaidService struct {
	db *pgxpool.Pool
	redis *redis.Client
}

func RegisterAntiRaidCommands(d *ext.Dispatcher, pool *pgxpool.Pool, client *redis.Client) {
	s := &antiRaidService{db: pool, redis: client}
	d.AddHandler(handlers.NewCommand("setraid", func(b *gotgbot.Bot, c *ext.Context) error { return s.setConfig(b,c) }))
	d.AddHandler(handlers.NewCommand("raid", func(b *gotgbot.Bot, c *ext.Context) error { return s.showConfig(b,c) }))
	d.AddHandler(handlers.NewCommand("clearraid", func(b *gotgbot.Bot, c *ext.Context) error { return s.clearConfig(b,c) }))
	d.AddHandler(handlers.NewChatMember(nil, s.handleJoin))
}

func parseRaidConfig(args []string) (raidConfig, error) {
	if len(args) < 2 || len(args) > 3 {
		return raidConfig{}, fmt.Errorf("usage: /setraid <join_limit> <seconds> [quarantine_minutes]")
	}
	limit, err := strconv.Atoi(args[0])
	if err != nil || limit < 3 || limit > 100 {
		return raidConfig{}, fmt.Errorf("join limit must be between 3 and 100")
	}
	seconds, err := strconv.Atoi(args[1])
	if err != nil || seconds < 5 || seconds > 300 {
		return raidConfig{}, fmt.Errorf("time window must be between 5 and 300 seconds")
	}
	minutes := 10
	if len(args) == 3 {
		minutes, err = strconv.Atoi(args[2])
		if err != nil || minutes < 1 || minutes > 60 {
			return raidConfig{}, fmt.Errorf("quarantine duration must be between 1 and 60 minutes")
		}
	}
	return raidConfig{JoinLimit: limit, WindowSeconds: seconds, QuarantineMinutes: minutes}, nil
}

func raidGroupChat(c *ext.Context) (int64, error) {
	chatID, err := groupChat(c)
	if err != nil { return 0, err }
	if c.EffectiveChat.Type != "supergroup" {
		return 0, fmt.Errorf("anti-raid quarantine requires a Telegram supergroup")
	}
	return chatID, nil
}

func isNewMemberJoin(oldStatus, newStatus string) bool {
	return (oldStatus == "left" || oldStatus == "kicked") && newStatus == "member"
}

func raidCounterKey(botID, chatID int64) string {
	return fmt.Sprintf("anitoon4:raid:%d:%d", botID, chatID)
}

func raidAlertKey(botID, chatID int64) string {
	return fmt.Sprintf("anitoon4:raid-alert:%d:%d", botID, chatID)
}

func raidQuarantineKey(botID, chatID, userID int64) string {
	return fmt.Sprintf("anitoon4:raid-quarantine:%d:%d:%d", botID, chatID, userID)
}

func raidQuarantinePermissions() map[string]any {
	return map[string]any{
		"can_send_messages": false, "can_send_audios": false, "can_send_documents": false,
		"can_send_photos": false, "can_send_videos": false, "can_send_video_notes": false,
		"can_send_voice_notes": false, "can_send_polls": false, "can_send_other_messages": false,
		"can_add_web_page_previews": false, "can_react_to_messages": false,
		"can_change_info": false, "can_invite_users": false, "can_pin_messages": false,
		"can_manage_topics": false, "can_edit_tag": false,
	}
}

func (s *antiRaidService) available() error {
	if s.db == nil { return fmt.Errorf("anti-raid settings are unavailable: PostgreSQL is not configured") }
	if s.redis == nil { return fmt.Errorf("anti-raid detection is unavailable: Redis is not configured") }
	return nil
}

func (s *antiRaidService) setConfig(b *gotgbot.Bot, c *ext.Context) error {
	chatID, err := raidGroupChat(c)
	if err != nil { return reply(b,c,err.Error()) }
	actor, err := actorID(c)
	if err != nil { return reply(b,c,err.Error()) }
	if err = RequireActorAdmin(b,chatID,actor); err != nil { return reply(b,c,err.Error()) }
	if err = s.available(); err != nil { return reply(b,c,err.Error()) }
	cfg, err := parseRaidConfig(commandArgs(c.EffectiveMessage))
	if err != nil { return reply(b,c,err.Error()) }
	if err = RequireBotRestrictPermission(b,chatID); err != nil { return reply(b,c,err.Error()) }

	ctx,cancel := context.WithTimeout(context.Background(),5*time.Second)
	defer cancel()
	_,err = s.db.Exec(ctx,`INSERT INTO chats(chat_id) VALUES($1) ON CONFLICT(chat_id) DO NOTHING`,chatID)
	if err == nil {
		_,err = s.db.Exec(ctx,`INSERT INTO raid_configs(bot_id,chat_id,join_limit,window_seconds,quarantine_minutes,updated_by)
			VALUES($1,$2,$3,$4,$5,$6)
			ON CONFLICT(bot_id,chat_id) DO UPDATE SET join_limit=EXCLUDED.join_limit,window_seconds=EXCLUDED.window_seconds,
			quarantine_minutes=EXCLUDED.quarantine_minutes,updated_by=EXCLUDED.updated_by,updated_at=NOW()`,
			b.User.Id,chatID,cfg.JoinLimit,cfg.WindowSeconds,cfg.QuarantineMinutes,actor)
	}
	if err != nil { return reply(b,c,"I couldn't save the anti-raid settings. Check the database migration.") }
	return reply(b,c,fmt.Sprintf("🛡️ Anti-raid enabled: %d new members within %d seconds triggers a %d-minute quarantine for the join wave.",cfg.JoinLimit,cfg.WindowSeconds,cfg.QuarantineMinutes))
}

func (s *antiRaidService) showConfig(b *gotgbot.Bot,c *ext.Context) error {
	chatID,err := raidGroupChat(c)
	if err != nil { return reply(b,c,err.Error()) }
	actor,err := actorID(c)
	if err != nil { return reply(b,c,err.Error()) }
	if err = RequireActorAdmin(b,chatID,actor); err != nil { return reply(b,c,err.Error()) }
	if s.db == nil { return reply(b,c,"Anti-raid storage is unavailable.") }
	ctx,cancel := context.WithTimeout(context.Background(),5*time.Second)
	defer cancel()
	var cfg raidConfig
	err = s.db.QueryRow(ctx,`SELECT join_limit,window_seconds,quarantine_minutes FROM raid_configs WHERE bot_id=$1 AND chat_id=$2`,b.User.Id,chatID).
		Scan(&cfg.JoinLimit,&cfg.WindowSeconds,&cfg.QuarantineMinutes)
	if err == pgx.ErrNoRows { return reply(b,c,"Anti-raid is disabled. Use /setraid 5 10 10 to enable it.") }
	if err != nil { return reply(b,c,"I couldn't read this group's anti-raid settings.") }
	return reply(b,c,fmt.Sprintf("🛡️ Anti-raid is enabled: %d joins in %d seconds; quarantine: %d minutes.",cfg.JoinLimit,cfg.WindowSeconds,cfg.QuarantineMinutes))
}

func (s *antiRaidService) clearConfig(b *gotgbot.Bot,c *ext.Context) error {
	chatID,err := raidGroupChat(c)
	if err != nil { return reply(b,c,err.Error()) }
	actor,err := actorID(c)
	if err != nil { return reply(b,c,err.Error()) }
	if err = RequireActorAdmin(b,chatID,actor); err != nil { return reply(b,c,err.Error()) }
	if s.db == nil { return reply(b,c,"Anti-raid storage is unavailable.") }
	ctx,cancel := context.WithTimeout(context.Background(),5*time.Second)
	defer cancel()
	_,err = s.db.Exec(ctx,`DELETE FROM raid_configs WHERE bot_id=$1 AND chat_id=$2`,b.User.Id,chatID)
	if err != nil { return reply(b,c,"I couldn't clear this group's anti-raid settings.") }
	if s.redis != nil { _ = s.redis.Del(ctx,raidCounterKey(b.User.Id,chatID),raidAlertKey(b.User.Id,chatID)).Err() }
	return reply(b,c,"Anti-raid has been disabled in this group. Existing Telegram quarantine timers are not cancelled.")
}

var raidWindowScript = redis.NewScript(`
redis.call('ZREMRANGEBYSCORE', KEYS[1], '-inf', ARGV[1])
redis.call('ZADD', KEYS[1], ARGV[2], ARGV[3])
redis.call('PEXPIRE', KEYS[1], ARGV[4])
return redis.call('ZCARD', KEYS[1])
`)

func (s *antiRaidService) getConfig(ctx context.Context,botID,chatID int64) (raidConfig,error) {
	var cfg raidConfig
	err := s.db.QueryRow(ctx,`SELECT join_limit,window_seconds,quarantine_minutes FROM raid_configs WHERE bot_id=$1 AND chat_id=$2`,botID,chatID).
		Scan(&cfg.JoinLimit,&cfg.WindowSeconds,&cfg.QuarantineMinutes)
	return cfg,err
}

func (s *antiRaidService) handleJoin(b *gotgbot.Bot,c *ext.Context) error {
	cm := c.ChatMember
	if cm == nil || (cm.Chat.Type != "group" && cm.Chat.Type != "supergroup") || !isNewMemberJoin(cm.OldChatMember.GetStatus(),cm.NewChatMember.GetStatus()) {
		return nil
	}
	if s.db == nil || s.redis == nil { return nil }
	user := cm.NewChatMember.GetUser()
	if user.Id <= 0 || user.IsBot { return nil }
	chatID,botID := cm.Chat.Id,b.User.Id
	ctx,cancel := context.WithTimeout(context.Background(),5*time.Second)
	defer cancel()
	cfg,err := s.getConfig(ctx,botID,chatID)
	if err == pgx.ErrNoRows || err != nil { return nil }
	if cm.From.IsBot || cm.From.Id <= 0 { return nil }
	// Joins performed by a verified administrator are deliberate invitations,
	// not a raid signal. If the inviter cannot be verified, fail open.
	inviter,inviterErr := b.GetChatMember(chatID,cm.From.Id,nil)
	if inviterErr != nil || inviter == nil || IsChatAdmin(inviter.GetStatus()) { return nil }
	now := time.Now()
	key := raidCounterKey(botID,chatID)
	count,err := raidWindowScript.Run(ctx,s.redis,[]string{key},
		now.Add(-time.Duration(cfg.WindowSeconds)*time.Second).UnixMilli(),
		now.UnixMilli(),fmt.Sprintf("%d:%d",user.Id,now.UnixNano()),cfg.WindowSeconds*1000).Int64()
	if err != nil || count < int64(cfg.JoinLimit) { return nil }

	// A join wave is quarantined, not automatically banned. Each user is
	// restricted at most once per raid window; Telegram expires the restriction.
	members,err := s.redis.ZRange(ctx,key,0,-1).Result()
	if err != nil { return nil }
	quarantined := 0
	ttl := time.Duration(cfg.QuarantineMinutes)*time.Minute + time.Minute
	for _,rawID := range members {
		parts := strings.SplitN(rawID,":",2)
		if len(parts) != 2 { continue }
		userID,parseErr := strconv.ParseInt(parts[0],10,64)
		if parseErr != nil || userID <= 0 { continue }
		member,lookupErr := b.GetChatMember(chatID,userID,nil)
		if lookupErr != nil || member == nil || member.GetStatus() != "member" { continue }
		marker := raidQuarantineKey(botID,chatID,userID)
		claimed,claimErr := s.redis.SetNX(ctx,marker,"1",ttl).Result()
		if claimErr != nil || !claimed { continue }
		restrictParams := map[string]any{
			"chat_id": chatID, "user_id": userID,
			"permissions": raidQuarantinePermissions(),
			"use_independent_chat_permissions": true,
			"until_date": now.Add(time.Duration(cfg.QuarantineMinutes)*time.Minute).Unix(),
		}
		_,restrictErr := b.Request("restrictChatMember",restrictParams,nil)
		if restrictErr != nil {
			_ = s.redis.Del(ctx,marker).Err()
			continue
		}
		quarantined++
	}
	alerted,alertErr := s.redis.SetNX(ctx,raidAlertKey(botID,chatID),"1",time.Duration(cfg.WindowSeconds)*time.Second).Result()
	if alertErr == nil && alerted && quarantined > 0 {
		_,_ = b.SendMessage(chatID,fmt.Sprintf("🚨 Possible raid detected: %d members joined within %d seconds. %d new members have been temporarily restricted for %d minutes.",count,cfg.WindowSeconds,quarantined,cfg.QuarantineMinutes),nil)
	}
	return nil
}

