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
	"github.com/redis/go-redis/v9"
	"github.com/ummidipradeep909-gif/AniToon_4Bot/internal/aiassistant"
)

const (
	blacklistModeDelete = "delete"
	blacklistModeWarn = "warn"
	maxBlacklistRules = 100
	maxBlacklistPhraseLength = 80
)

type blacklistService struct {
	db *pgxpool.Pool
	redis *redis.Client
	ai *aiassistant.Client
}

func RegisterBlacklistCommands(d *ext.Dispatcher, pool *pgxpool.Pool, redisClient *redis.Client, aiClient *aiassistant.Client) {
	s := &blacklistService{db: pool, redis: redisClient, ai: aiClient}
	d.AddHandler(handlers.NewCommand("addblacklist", func(b *gotgbot.Bot, c *ext.Context) error { return s.add(b,c) }))
	d.AddHandler(handlers.NewCommand("unblacklist", func(b *gotgbot.Bot, c *ext.Context) error { return s.remove(b,c) }))
	d.AddHandler(handlers.NewCommand("blacklist", func(b *gotgbot.Bot, c *ext.Context) error { return s.list(b,c) }))
	d.AddHandler(handlers.NewCommand("blacklistmode", func(b *gotgbot.Bot, c *ext.Context) error { return s.setMode(b,c) }))
	d.AddHandler(handlers.NewMessage(func(m *gotgbot.Message) bool {
		return m != nil && (m.Chat.Type == "group" || m.Chat.Type == "supergroup") &&
			m.MessageId > 0 && m.From != nil && !m.From.IsBot &&
			(strings.TrimSpace(m.Text) != "" || strings.TrimSpace(m.Caption) != "")
	}, s.handleMessage))
}

func normalizeBlacklistPhrase(raw string) (string,error) {
	phrase := strings.Join(strings.Fields(strings.TrimSpace(raw)), " ")
	if len([]rune(phrase)) < 2 { return "", fmt.Errorf("a blacklist phrase must contain at least 2 characters") }
	if len([]rune(phrase)) > maxBlacklistPhraseLength { return "", fmt.Errorf("a blacklist phrase cannot exceed 80 characters") }
	for _, r := range phrase {
		if unicode.IsControl(r) { return "", fmt.Errorf("the phrase cannot contain control characters") }
	}
	return strings.ToLower(phrase), nil
}

func containsBlacklistPhrase(text, phrase string) bool {
	return phrase != "" && strings.Contains(strings.ToLower(text), phrase)
}

func (s *blacklistService) requireAdmin(b *gotgbot.Bot,c *ext.Context) (int64,int64,error) {
	chatID,err:=groupChat(c); if err!=nil{return 0,0,err}
	actor,err:=actorID(c); if err!=nil{return 0,0,err}
	if err=RequireActorAdmin(b,chatID,actor); err!=nil{return 0,0,err}
	if s.db==nil{return 0,0,fmt.Errorf("blacklist storage is unavailable: PostgreSQL is not configured")}
	return chatID,actor,nil
}

func blacklistCommandPhrase(m *gotgbot.Message) string {
	if m==nil || m.Text=="" { return "" }
	fields:=strings.Fields(m.Text)
	if len(fields)<2{return ""}
	// Remove the command token (including an optional @botname) and preserve
	// the phrase as one normalized string.
	idx:=strings.Index(m.Text,fields[0])
	if idx<0{return ""}
	return strings.TrimSpace(m.Text[idx+len(fields[0]):])
}

func (s *blacklistService) add(b *gotgbot.Bot,c *ext.Context) error {
	chatID,actor,err:=s.requireAdmin(b,c); if err!=nil{return reply(b,c,err.Error())}
	phrase,err:=normalizeBlacklistPhrase(blacklistCommandPhrase(c.EffectiveMessage))
	if err!=nil{return reply(b,c,"Usage: /addblacklist <word or phrase>. "+err.Error())}
	ctx,cancel:=context.WithTimeout(context.Background(),5*time.Second); defer cancel()
	_,err=s.db.Exec(ctx,`INSERT INTO chats(chat_id) VALUES($1) ON CONFLICT(chat_id) DO NOTHING`,chatID)
	if err==nil {
		var count int
		err=s.db.QueryRow(ctx,`SELECT COUNT(*) FROM keyword_blacklist WHERE bot_id=$1 AND chat_id=$2`,b.User.Id,chatID).Scan(&count)
		if err==nil && count>=maxBlacklistRules {
			return reply(b,c,"This group has reached the 100-phrase blacklist limit. Remove a phrase before adding another.")
		}
	}
	if err==nil {
		_,err=s.db.Exec(ctx,`INSERT INTO keyword_blacklist(bot_id,chat_id,phrase,added_by) VALUES($1,$2,$3,$4) ON CONFLICT(bot_id,chat_id,phrase) DO NOTHING`,b.User.Id,chatID,phrase,actor)
	}
	if err!=nil{return reply(b,c,"I couldn't save that blacklist phrase. Check PostgreSQL and apply migration 009.")}
	return reply(b,c,"Added blacklist phrase: "+phrase)
}

func (s *blacklistService) remove(b *gotgbot.Bot,c *ext.Context) error {
	chatID,_,err:=s.requireAdmin(b,c); if err!=nil{return reply(b,c,err.Error())}
	phrase,err:=normalizeBlacklistPhrase(blacklistCommandPhrase(c.EffectiveMessage))
	if err!=nil{return reply(b,c,"Usage: /unblacklist <word or phrase>. "+err.Error())}
	ctx,cancel:=context.WithTimeout(context.Background(),5*time.Second); defer cancel()
	tag,err:=s.db.Exec(ctx,`DELETE FROM keyword_blacklist WHERE bot_id=$1 AND chat_id=$2 AND phrase=$3`,b.User.Id,chatID,phrase)
	if err!=nil{return reply(b,c,"I couldn't update the blacklist. Check PostgreSQL.")}
	if tag.RowsAffected()==0{return reply(b,c,"That phrase is not in this group's blacklist.")}
	return reply(b,c,"Removed blacklist phrase: "+phrase)
}

func (s *blacklistService) list(b *gotgbot.Bot,c *ext.Context) error {
	chatID,_,err:=s.requireAdmin(b,c); if err!=nil{return reply(b,c,err.Error())}
	ctx,cancel:=context.WithTimeout(context.Background(),5*time.Second); defer cancel()
	rows,err:=s.db.Query(ctx,`SELECT phrase FROM keyword_blacklist WHERE bot_id=$1 AND chat_id=$2 ORDER BY phrase LIMIT 100`,b.User.Id,chatID)
	if err!=nil{return reply(b,c,"I couldn't read this group's blacklist.")}
	defer rows.Close()
	var phrases []string
	for rows.Next(){var p string;if rows.Scan(&p)!=nil{return reply(b,c,"I couldn't read this group's blacklist.")};phrases=append(phrases,p)}
	if rows.Err()!=nil{return reply(b,c,"I couldn't read this group's blacklist.")}
	if len(phrases)==0{return reply(b,c,"This group's blacklist is empty. Add a phrase with /addblacklist <phrase>.")}
	return reply(b,c,"🚫 Blacklisted phrases (up to 100):\n"+strings.Join(phrases,"\n"))
}

func (s *blacklistService) setMode(b *gotgbot.Bot,c *ext.Context) error {
	chatID,actor,err:=s.requireAdmin(b,c); if err!=nil{return reply(b,c,err.Error())}
	args:=commandArgs(c.EffectiveMessage)
	if len(args)==0 {
		ctx,cancel:=context.WithTimeout(context.Background(),5*time.Second); defer cancel()
		var mode string
		err=s.db.QueryRow(ctx,`SELECT mode FROM keyword_policy_configs WHERE bot_id=$1 AND chat_id=$2`,b.User.Id,chatID).Scan(&mode)
		if err==pgx.ErrNoRows{return reply(b,c,"Blacklist action is delete. Use /blacklistmode warn to delete matching messages and record a warning.")}
		if err!=nil{return reply(b,c,"I couldn't read the blacklist action setting.")}
		return reply(b,c,"Blacklist action: "+mode+".")
	}
	if len(args)!=1 || (args[0]!=blacklistModeDelete && args[0]!=blacklistModeWarn) {
		return reply(b,c,"Usage: /blacklistmode delete or /blacklistmode warn")
	}
	if err=RequireBotDeletePermission(b,chatID);err!=nil{return reply(b,c,err.Error())}
	ctx,cancel:=context.WithTimeout(context.Background(),5*time.Second);defer cancel()
	_,err=s.db.Exec(ctx,`INSERT INTO chats(chat_id) VALUES($1) ON CONFLICT(chat_id) DO NOTHING`,chatID)
	if err==nil {
		_,err=s.db.Exec(ctx,`INSERT INTO keyword_policy_configs(bot_id,chat_id,mode,updated_by) VALUES($1,$2,$3,$4)
			ON CONFLICT(bot_id,chat_id) DO UPDATE SET mode=EXCLUDED.mode,updated_by=EXCLUDED.updated_by,updated_at=NOW()`,b.User.Id,chatID,args[0],actor)
	}
	if err!=nil{return reply(b,c,"I couldn't save the blacklist action. Apply migration 009 and check PostgreSQL.")}
	if args[0]==blacklistModeWarn{return reply(b,c,"Blacklist action set to warn: matching messages are deleted and a warning is recorded.")}
	return reply(b,c,"Blacklist action set to delete: matching messages are deleted without a warning.")
}

func normalizeBlacklistForReview(value string) string {
	return strings.Map(func(r rune) rune {
		switch unicode.ToLower(r) {
		case '0':
			return 'o'
		case '1', '!':
			return 'i'
		case '3':
			return 'e'
		case '4', '@':
			return 'a'
		case '5', '$':
			return 's'
		case '7':
			return 't'
		}
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return unicode.ToLower(r)
		}
		return -1
	}, value)
}

// blacklistReviewCandidate is advisory-only: it detects common spacing,
// punctuation and leetspeak variants but never changes deterministic matching.
func blacklistReviewCandidate(text, phrase string) bool {
	normalizedPhrase := normalizeBlacklistForReview(phrase)
	if len([]rune(normalizedPhrase)) < 2 {
		return false
	}
	return strings.Contains(normalizeBlacklistForReview(text), normalizedPhrase)
}

func blacklistAIReviewKey(botID, chatID, userID int64) string {
	return fmt.Sprintf("anitoon4:ai-blacklist-review:%d:%d:%d", botID, chatID, userID)
}

func (s *blacklistService) reviewCooldown(ctx context.Context, key string) (bool, error) {
	if s.redis == nil {
		return false, nil
	}
	return s.redis.SetNX(ctx, key, "1", time.Minute).Result()
}

func (s *blacklistService) handleMessage(b *gotgbot.Bot, c *ext.Context) error {
	m := c.EffectiveMessage
	if m == nil || m.From == nil || m.From.IsBot || s.db == nil {
		return nil
	}
	messageText := strings.TrimSpace(m.Text + "\n" + m.Caption)
	if messageText == "" {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	var mode string
	err := s.db.QueryRow(ctx, `SELECT mode FROM keyword_policy_configs WHERE bot_id=$1 AND chat_id=$2`, b.User.Id, m.Chat.Id).Scan(&mode)
	if err == pgx.ErrNoRows {
		mode = blacklistModeDelete
	} else if err != nil {
		return nil // Fail open if policy cannot be verified.
	}

	rows, err := s.db.Query(ctx, `SELECT phrase FROM keyword_blacklist WHERE bot_id=$1 AND chat_id=$2 ORDER BY length(phrase) DESC LIMIT 100`, b.User.Id, m.Chat.Id)
	if err != nil {
		return nil
	}
	var matched string
	var reviewPhrase string
	for rows.Next() {
		var phrase string
		if rows.Scan(&phrase) != nil {
			rows.Close()
			return nil
		}
		if containsBlacklistPhrase(messageText, phrase) {
			matched = phrase
			reviewPhrase = phrase
			break
		}
		if reviewPhrase == "" && blacklistReviewCandidate(messageText, phrase) {
			reviewPhrase = phrase
		}
	}
	if rows.Err() != nil {
		rows.Close()
		return nil
	}
	rows.Close()

	if matched == "" && reviewPhrase == "" {
		return nil
	}
	member, err := b.GetChatMember(m.Chat.Id, m.From.Id, nil)
	if err != nil || member == nil || IsChatAdmin(member.GetStatus()) {
		return nil
	}

	// Only exact, original substring matches reach the existing enforcement path.
	// Similar/obfuscated candidates are sent to AI for advice and are never acted on.
	if matched != "" {
		if err = RequireBotDeletePermission(b, m.Chat.Id); err != nil {
			return nil
		}
		if _, err = b.DeleteMessage(m.Chat.Id, m.MessageId, nil); err != nil {
			return err
		}
		if mode == blacklistModeWarn {
			_, err = s.db.Exec(ctx, `INSERT INTO chat_warnings(chat_id,user_id,actor_user_id,reason) VALUES($1,$2,$3,$4)`, m.Chat.Id, m.From.Id, b.User.Id, "Blacklisted phrase: "+matched)
			if err != nil {
				return fmt.Errorf("message deleted for blacklist phrase %q, but warning could not be recorded: %w", matched, err)
			}
			_, _ = b.SendMessage(m.Chat.Id, fmt.Sprintf("⚠️ User %d received a warning for a blacklisted phrase.", m.From.Id), nil)
		}
	}

	// AI is advisory only. Redis is required so unavailable cooldown storage
	// cannot accidentally turn every matching message into a provider request.
	if s.ai == nil || s.redis == nil || reviewPhrase == "" {
		return nil
	}
	cooldownCtx, cooldownCancel := context.WithTimeout(context.Background(), time.Second)
	claimed, claimErr := s.reviewCooldown(cooldownCtx, blacklistAIReviewKey(b.User.Id, m.Chat.Id, m.From.Id))
	cooldownCancel()
	if claimErr != nil || !claimed {
		return nil
	}

	matchType := "obfuscated or formatting variant (advisory candidate only)"
	if matched != "" {
		matchType = "exact deterministic substring match"
	}
	event := fmt.Sprintf("Configured blacklist phrase: %q. Match type: %s. Message text: %s", reviewPhrase, matchType, messageText)
	aiCtx, aiCancel := context.WithTimeout(context.Background(), 3*time.Second)
	analysis, aiErr := s.ai.Analyze(aiCtx, m.Chat.Id, m.From.Id, m.MessageId, "keyword_blacklist", event)
	aiCancel()
	if aiErr != nil {
		return nil // AI failures never affect deterministic moderation.
	}

	// A likely-benign result is still useful context for an exact match; for
	// near-matches, only send a note when AI sees credible evasion/abuse signals.
	if analysis.Recommendation == "admin_review" || matched != "" {
		contextNote := "AI suggests admin review"
		if analysis.Recommendation == "allow" {
			contextNote = "AI context assessment is likely benign; consider reviewing the blacklist rule"
		}
		_, _ = b.SendMessage(m.Chat.Id, fmt.Sprintf("🤖 Keyword blacklist review: %s (category: %s, confidence: %.0f%%). %s AI did not delete, ban, mute, or punish anyone.", contextNote, analysis.Category, analysis.Confidence*100, analysis.Rationale), nil)
	}
	return nil
}
