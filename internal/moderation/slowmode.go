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

type slowModeConfig struct {
	IntervalSeconds int
	UpdatedAt time.Time
}

type slowModeAdminCacheEntry struct {
	admin bool
	expiresAt time.Time
}

type slowModeService struct {
	db *pgxpool.Pool
	redis *redis.Client
	mu sync.RWMutex
	adminCache map[string]slowModeAdminCacheEntry
}

// RegisterSlowModeCommands installs the setting commands and message limiter
// once on the shared dispatcher, so the primary bot and clones use the same code.
func RegisterSlowModeCommands(d *ext.Dispatcher, pool *pgxpool.Pool, client *redis.Client) {
	s := &slowModeService{db: pool, redis: client, adminCache: make(map[string]slowModeAdminCacheEntry)}
	d.AddHandler(handlers.NewCommand("slowmode", func(b *gotgbot.Bot, c *ext.Context) error { return s.configure(b, c) }))
	d.AddHandler(handlers.NewMessage(func(m *gotgbot.Message) bool {
		if m == nil || (m.Chat.Type != "group" && m.Chat.Type != "supergroup") || m.From == nil || m.From.IsBot || m.From.Id <= 0 {
			return false
		}
		// Commands remain usable and are not counted as ordinary chat messages.
		return !strings.HasPrefix(strings.TrimSpace(m.Text), "/")
	}, s.handleMessage))
}

func parseSlowModeInterval(args []string) (int, error) {
	if len(args) != 1 {
		return 0, fmt.Errorf("usage: /slowmode <1-3600 seconds|off>")
	}
	if strings.EqualFold(args[0], "off") {
		return 0, nil
	}
	seconds, err := strconv.Atoi(args[0])
	if err != nil || seconds < 1 || seconds > 3600 {
		return 0, fmt.Errorf("slow-mode interval must be between 1 and 3600 seconds, or use /slowmode off")
	}
	return seconds, nil
}

func slowModeCounterKey(botID, chatID, userID int64, version int64) string {
	return fmt.Sprintf("anitoon4:slowmode:%d:%d:%d:%d", botID, chatID, userID, version)
}

func (s *slowModeService) available() error {
	if s.db == nil {
		return fmt.Errorf("slow-mode settings are unavailable: PostgreSQL is not configured")
	}
	if s.redis == nil {
		return fmt.Errorf("slow-mode enforcement is unavailable: Redis is not configured")
	}
	return nil
}

func (s *slowModeService) configure(b *gotgbot.Bot, c *ext.Context) error {
	chatID, err := groupChat(c)
	if err != nil { return reply(b, c, err.Error()) }
	actor, err := actorID(c)
	if err != nil { return reply(b, c, err.Error()) }
	if err = RequireActorAdmin(b, chatID, actor); err != nil { return reply(b, c, err.Error()) }
	if s.db == nil { return reply(b, c, "Slow-mode settings are unavailable: PostgreSQL is not configured.") }

	args := commandArgs(c.EffectiveMessage)
	if len(args) == 0 {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		var seconds int
		err = s.db.QueryRow(ctx, `SELECT interval_seconds FROM slowmode_configs WHERE bot_id=$1 AND chat_id=$2`, b.User.Id, chatID).Scan(&seconds)
		if err == pgx.ErrNoRows { return reply(b, c, "Slow mode is disabled. Use /slowmode 10 to allow one message every 10 seconds.") }
		if err != nil { return reply(b, c, "I couldn't read this group's slow-mode settings.") }
		return reply(b, c, fmt.Sprintf("🐢 Slow mode is enabled: one message per member every %d seconds. Use /slowmode off to disable it.", seconds))
	}

	seconds, err := parseSlowModeInterval(args)
	if err != nil { return reply(b, c, err.Error()) }
	if seconds > 0 {
		if err = s.available(); err != nil { return reply(b, c, err.Error()) }
		if err = RequireBotDeletePermission(b, chatID); err != nil { return reply(b, c, err.Error()) }
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if seconds == 0 {
		_, err = s.db.Exec(ctx, `DELETE FROM slowmode_configs WHERE bot_id=$1 AND chat_id=$2`, b.User.Id, chatID)
		if err != nil { return reply(b, c, "I couldn't disable slow mode. Check the database.") }
		return reply(b, c, "Slow mode has been disabled in this group.")
	}

	_, err = s.db.Exec(ctx, `INSERT INTO chats(chat_id) VALUES($1) ON CONFLICT(chat_id) DO NOTHING`, chatID)
	if err == nil {
		_, err = s.db.Exec(ctx, `INSERT INTO slowmode_configs(bot_id,chat_id,interval_seconds,updated_by)
			VALUES($1,$2,$3,$4)
			ON CONFLICT(bot_id,chat_id) DO UPDATE SET interval_seconds=EXCLUDED.interval_seconds,
			updated_by=EXCLUDED.updated_by,updated_at=NOW()`, b.User.Id, chatID, seconds, actor)
	}
	if err != nil { return reply(b, c, "I couldn't save slow-mode settings. Apply migration 006 and check PostgreSQL.") }
	return reply(b, c, fmt.Sprintf("🐢 Slow mode enabled: each regular member may send one message every %d seconds. Excess messages are automatically deleted.", seconds))
}

func (s *slowModeService) isAdmin(b *gotgbot.Bot, chatID, userID int64) bool {
	key := fmt.Sprintf("%d:%d:%d", b.User.Id, chatID, userID)
	now := time.Now()
	s.mu.RLock()
	cached, ok := s.adminCache[key]
	s.mu.RUnlock()
	if ok && now.Before(cached.expiresAt) { return cached.admin }

	member, err := b.GetChatMember(chatID, userID, nil)
	if err != nil || member == nil { return true } // Fail open if admin status cannot be verified.
	admin := IsChatAdmin(member.GetStatus())
	s.mu.Lock()
	if len(s.adminCache) > 4096 { s.adminCache = make(map[string]slowModeAdminCacheEntry) }
	s.adminCache[key] = slowModeAdminCacheEntry{admin: admin, expiresAt: now.Add(30*time.Second)}
	s.mu.Unlock()
	return admin
}

func (s *slowModeService) getConfig(ctx context.Context, botID, chatID int64) (slowModeConfig, error) {
	var cfg slowModeConfig
	err := s.db.QueryRow(ctx, `SELECT interval_seconds,updated_at FROM slowmode_configs WHERE bot_id=$1 AND chat_id=$2`, botID, chatID).
		Scan(&cfg.IntervalSeconds, &cfg.UpdatedAt)
	return cfg, err
}

func (s *slowModeService) handleMessage(b *gotgbot.Bot, c *ext.Context) error {
	m := c.EffectiveMessage
	if m == nil || m.From == nil || m.MessageId <= 0 || s.db == nil || s.redis == nil { return nil }
	chatID, userID := m.Chat.Id, m.From.Id
	if s.isAdmin(b, chatID, userID) { return nil }

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	cfg, err := s.getConfig(ctx, b.User.Id, chatID)
	if err != nil { return nil } // No configuration or storage error: fail open.
	version := cfg.UpdatedAt.UnixNano()
	key := slowModeCounterKey(b.User.Id, chatID, userID, version)
	allowed, err := s.redis.SetNX(ctx, key, strconv.FormatInt(m.MessageId, 10), time.Duration(cfg.IntervalSeconds)*time.Second).Result()
	if err != nil || allowed { return nil }
	_, deleteErr := b.DeleteMessage(chatID, m.MessageId, nil)
	return deleteErr
}
