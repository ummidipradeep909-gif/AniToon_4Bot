package moderation

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type forwardProtectionService struct {
	db *pgxpool.Pool
}

func RegisterForwardProtectionCommands(d *ext.Dispatcher, pool *pgxpool.Pool) {
	s := &forwardProtectionService{db: pool}
	d.AddHandler(handlers.NewCommand("antiforward", func(b *gotgbot.Bot, c *ext.Context) error {
		return s.configure(b, c)
	}))
	d.AddHandler(handlers.NewMessage(func(m *gotgbot.Message) bool {
		return m != nil &&
			(m.Chat.Type == "group" || m.Chat.Type == "supergroup") &&
			m.MessageId > 0 && m.ForwardOrigin != nil
	}, s.handleForward))
}

func (s *forwardProtectionService) configure(b *gotgbot.Bot, c *ext.Context) error {
	chatID, err := groupChat(c)
	if err != nil { return reply(b, c, err.Error()) }
	actor, err := actorID(c)
	if err != nil { return reply(b, c, err.Error()) }
	if err = RequireActorAdmin(b, chatID, actor); err != nil { return reply(b, c, err.Error()) }
	if s.db == nil { return reply(b, c, "Forward protection settings are unavailable: PostgreSQL is not configured.") }

	args := commandArgs(c.EffectiveMessage)
	if len(args) == 0 {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		var enabled bool
		err = s.db.QueryRow(ctx, `SELECT enabled FROM forward_policy_configs WHERE bot_id=$1 AND chat_id=$2`, b.User.Id, chatID).Scan(&enabled)
		if err == pgx.ErrNoRows { return reply(b, c, "Forward protection is disabled. Use /antiforward on to enable it.") }
		if err != nil { return reply(b, c, "I couldn't read this group's forward-protection settings.") }
		if enabled { return reply(b, c, "↪️ Forward protection is enabled. Forwarded messages from regular members are deleted.") }
		return reply(b, c, "Forward protection is disabled. Use /antiforward on to enable it.")
	}
	if len(args) != 1 || (!strings.EqualFold(args[0], "on") && !strings.EqualFold(args[0], "off")) {
		return reply(b, c, "Usage: /antiforward on or /antiforward off")
	}
	enabled := strings.EqualFold(args[0], "on")
	if enabled {
		if err = RequireBotDeletePermission(b, chatID); err != nil { return reply(b, c, err.Error()) }
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err = s.db.Exec(ctx, `INSERT INTO chats(chat_id) VALUES($1) ON CONFLICT(chat_id) DO NOTHING`, chatID)
	if err == nil {
		_, err = s.db.Exec(ctx, `INSERT INTO forward_policy_configs(bot_id,chat_id,enabled,updated_by)
			VALUES($1,$2,$3,$4)
			ON CONFLICT(bot_id,chat_id) DO UPDATE SET enabled=EXCLUDED.enabled,updated_by=EXCLUDED.updated_by,updated_at=NOW()`,
			b.User.Id, chatID, enabled, actor)
	}
	if err != nil { return reply(b, c, "I couldn't save forward-protection settings. Apply migration 008 and check PostgreSQL.") }
	if enabled { return reply(b, c, "↪️ Forward protection enabled. Forwarded messages from regular members will be deleted. Group administrators are exempt.") }
	return reply(b, c, "Forward protection disabled in this group.")
}

func (s *forwardProtectionService) handleForward(b *gotgbot.Bot, c *ext.Context) error {
	m := c.EffectiveMessage
	if m == nil || m.ForwardOrigin == nil || m.MessageId <= 0 || m.From == nil || m.From.IsBot || s.db == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	var enabled bool
	err := s.db.QueryRow(ctx, `SELECT enabled FROM forward_policy_configs WHERE bot_id=$1 AND chat_id=$2`, b.User.Id, m.Chat.Id).Scan(&enabled)
	if err != nil || !enabled { return nil } // Fail open if policy cannot be verified.
	member, err := b.GetChatMember(m.Chat.Id, m.From.Id, nil)
	if err != nil || member == nil { return nil } // Never delete when administrator status cannot be verified.
	if IsChatAdmin(member.GetStatus()) { return nil }
	_, err = b.DeleteMessage(m.Chat.Id, m.MessageId, nil)
	if err != nil {
		return fmt.Errorf("delete forwarded message %d in chat %d: %w", m.MessageId, m.Chat.Id, err)
	}
	return nil
}
