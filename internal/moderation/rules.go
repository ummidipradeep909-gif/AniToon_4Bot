package moderation

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const maxGroupRulesRunes = 3500

type rulesService struct { db *pgxpool.Pool }

func RegisterRulesCommands(d *ext.Dispatcher, pool *pgxpool.Pool) {
	s := &rulesService{db: pool}
	d.AddHandler(handlers.NewCommand("setrules", s.set))
	d.AddHandler(handlers.NewCommand("rules", s.show))
	d.AddHandler(handlers.NewCommand("privaterules", s.sendPrivate))
	d.AddHandler(handlers.NewCommand("clearrules", s.clear))
}

func rulesCommandText(m *gotgbot.Message) string {
	if m == nil || strings.TrimSpace(m.Text) == "" { return "" }
	fields := strings.Fields(m.Text)
	if len(fields) < 2 { return "" }
	return strings.TrimSpace(strings.TrimPrefix(m.Text, fields[0]))
}

func validateGroupRules(raw string) (string, error) {
	value := strings.TrimSpace(raw)
	if value == "" { return "", fmt.Errorf("provide the rules text after the command") }
	if len([]rune(value)) > maxGroupRulesRunes { return "", fmt.Errorf("rules are limited to 3500 characters") }
	for _, r := range value {
		if unicode.IsControl(r) && r != '\n' && r != '\t' {
			return "", fmt.Errorf("rules cannot contain control characters")
		}
	}
	return value, nil
}

func formatGroupRules(rules string) string {
	return "📜 Group rules\n\n" + rules
}

func (s *rulesService) requireAdmin(b *gotgbot.Bot, c *ext.Context) (int64, int64, error) {
	chatID, err := groupChat(c)
	if err != nil { return 0, 0, err }
	actor, err := actorID(c)
	if err != nil { return 0, 0, err }
	if err = RequireActorAdmin(b, chatID, actor); err != nil { return 0, 0, err }
	if s.db == nil { return 0, 0, fmt.Errorf("rules storage is unavailable because PostgreSQL is not configured") }
	return chatID, actor, nil
}

func (s *rulesService) ensureChat(ctx context.Context, chatID int64) error {
	_, err := s.db.Exec(ctx, `INSERT INTO chats(chat_id) VALUES($1) ON CONFLICT(chat_id) DO NOTHING`, chatID)
	return err
}

func (s *rulesService) set(b *gotgbot.Bot, c *ext.Context) error {
	chatID, actor, err := s.requireAdmin(b, c)
	if err != nil { return reply(b, c, err.Error()) }
	rules, err := validateGroupRules(rulesCommandText(c.EffectiveMessage))
	if err != nil { return reply(b, c, "Usage: /setrules <rules text>. " + err.Error()) }

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err = s.ensureChat(ctx, chatID); err == nil {
		_, err = s.db.Exec(ctx, `INSERT INTO group_rules(bot_id,chat_id,rules,updated_by)
			VALUES($1,$2,$3,$4)
			ON CONFLICT(bot_id,chat_id) DO UPDATE
			SET rules=EXCLUDED.rules,updated_by=EXCLUDED.updated_by,updated_at=NOW()`,
			b.User.Id, chatID, rules, actor)
	}
	if err != nil { return reply(b, c, "I couldn't save the rules. Apply migration 012 and check PostgreSQL.") }
	return reply(b, c, "Group rules saved for this bot. Use /rules to display them.")
}

func (s *rulesService) load(ctx context.Context, botID, chatID int64) (string, error) {
	var rules string
	err := s.db.QueryRow(ctx, `SELECT rules FROM group_rules WHERE bot_id=$1 AND chat_id=$2`, botID, chatID).Scan(&rules)
	return rules, err
}

func (s *rulesService) show(b *gotgbot.Bot, c *ext.Context) error {
	chatID, err := groupChat(c)
	if err != nil { return reply(b, c, err.Error()) }
	if s.db == nil { return reply(b, c, "Rules are unavailable because PostgreSQL is not configured.") }
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	rules, err := s.load(ctx, b.User.Id, chatID)
	if err == pgx.ErrNoRows { return reply(b, c, "No group rules have been set yet. An administrator can use /setrules.") }
	if err != nil { return reply(b, c, "I couldn't retrieve this group's rules.") }
	_, err = c.EffectiveMessage.Reply(b, formatGroupRules(rules), nil)
	return err
}

func (s *rulesService) sendPrivate(b *gotgbot.Bot, c *ext.Context) error {
	chatID, err := groupChat(c)
	if err != nil { return reply(b, c, err.Error()) }
	if c.EffectiveUser == nil || c.EffectiveUser.Id <= 0 {
		return reply(b, c, "I cannot identify your Telegram account to send the rules privately.")
	}
	if s.db == nil { return reply(b, c, "Rules are unavailable because PostgreSQL is not configured.") }
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	rules, err := s.load(ctx, b.User.Id, chatID)
	if err == pgx.ErrNoRows { return reply(b, c, "No group rules have been set yet.") }
	if err != nil { return reply(b, c, "I couldn't retrieve this group's rules.") }
	_, err = b.SendMessage(c.EffectiveUser.Id, formatGroupRules(rules), nil)
	if err != nil {
		return reply(b, c, "I couldn't message you privately. Open the bot's private chat and send /start, then try /privaterules again.")
	}
	return reply(b, c, "I've sent the group rules to your private chat.")
}

func (s *rulesService) clear(b *gotgbot.Bot, c *ext.Context) error {
	chatID, _, err := s.requireAdmin(b, c)
	if err != nil { return reply(b, c, err.Error()) }
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	tag, err := s.db.Exec(ctx, `DELETE FROM group_rules WHERE bot_id=$1 AND chat_id=$2`, b.User.Id, chatID)
	if err != nil { return reply(b, c, "I couldn't clear this group's rules.") }
	if tag.RowsAffected() == 0 { return reply(b, c, "No rules are currently configured for this bot in this group.") }
	return reply(b, c, "Group rules cleared for this bot.")
}
