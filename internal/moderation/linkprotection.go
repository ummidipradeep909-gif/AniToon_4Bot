package moderation

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/ummidipradeep909-gif/AniToon_4Bot/internal/aiassistant"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type linkPolicyService struct {
	db *pgxpool.Pool
	mu sync.RWMutex
	adminCache map[string]linkAdminCacheEntry
	ai *aiassistant.Client
}

type linkAdminCacheEntry struct {
	admin bool
	expiresAt time.Time
}

var linkPattern = regexp.MustCompile(`(?i)(?:https?://|tg://|www\.)[^\s<>()]+|(?:t\.me|telegram\.me)/[^\s<>()]+|(?:[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z]{2,}(?:/[^\s<>()]*)?`)

func RegisterLinkProtectionCommands(d *ext.Dispatcher, pool *pgxpool.Pool, aiClient *aiassistant.Client) {
	s := &linkPolicyService{db: pool, adminCache: make(map[string]linkAdminCacheEntry), ai: aiClient}
	d.AddHandler(handlers.NewCommand("antilink", func(b *gotgbot.Bot, c *ext.Context) error { return s.configure(b, c) }))
	d.AddHandler(handlers.NewCommand("allowlink", func(b *gotgbot.Bot, c *ext.Context) error { return s.allowlistCommand(b, c) }))
	d.AddHandler(handlers.NewMessage(func(m *gotgbot.Message) bool {
		if m == nil || (m.Chat.Type != "group" && m.Chat.Type != "supergroup") || m.From == nil || m.From.IsBot || m.From.Id <= 0 {
			return false
		}
		text := strings.TrimSpace(m.Text)
		caption := strings.TrimSpace(m.Caption)
		return !strings.HasPrefix(text, "/") && !strings.HasPrefix(caption, "/")
	}, s.handleMessage))
}

func normalizeAllowedDomain(raw string) (string, error) {
	raw = strings.TrimSpace(strings.ToLower(raw))
	raw = strings.TrimPrefix(raw, "https://")
	raw = strings.TrimPrefix(raw, "http://")
	raw = strings.TrimPrefix(raw, "www.")
	raw = strings.TrimSuffix(raw, "/")
	if raw == "" || strings.ContainsAny(raw, "/?#@:*") || strings.ContainsAny(raw, " \t\r\n") {
		return "", fmt.Errorf("provide a domain only, for example example.com")
	}
	if net.ParseIP(raw) != nil || !strings.Contains(raw, ".") {
		return "", fmt.Errorf("please provide a valid public domain, for example example.com")
	}
	for _, label := range strings.Split(raw, ".") {
		if label == "" || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return "", fmt.Errorf("please provide a valid domain, for example example.com")
		}
		for _, r := range label {
			if !(r >= 'a' && r <= 'z') && !(r >= '0' && r <= '9') && r != '-' {
				return "", fmt.Errorf("please provide a valid domain using standard ASCII characters")
			}
		}
	}
	return raw, nil
}

func extractMessageLinks(m *gotgbot.Message) []string {
	if m == nil { return nil }
	var links []string
	collect := func(text string, entities []gotgbot.MessageEntity) {
		for _, entity := range entities {
			if entity.Type == "text_link" && entity.Url != "" {
				links = append(links, entity.Url)
			}
		}
		for _, match := range linkPattern.FindAllString(text, -1) {
			links = append(links, strings.TrimRight(match, ".,!?;:)]}"))
		}
	}
	collect(m.Text, m.Entities)
	collect(m.Caption, m.CaptionEntities)
	return uniqueStrings(links)
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]bool, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" || seen[value] { continue }
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func linkHost(raw string) (string, string, bool) {
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(strings.ToLower(raw), "tg://") {
		return "telegram.me", raw, true
	}
	candidate := raw
	if !strings.Contains(candidate, "://") {
		candidate = "https://" + candidate
	}
	parsed, err := url.Parse(candidate)
	if err != nil || parsed.Hostname() == "" { return "", "", false }
	host := strings.TrimSuffix(strings.ToLower(parsed.Hostname()), ".")
	return host, parsed.Path, true
}

func isTelegramInviteLink(raw string) bool {
	host, path, ok := linkHost(raw)
	if !ok { return false }
	if host != "t.me" && host != "telegram.me" && host != "www.t.me" && host != "www.telegram.me" {
		return false
	}
	lower := strings.ToLower(path)
	return strings.HasPrefix(lower, "/+") || strings.HasPrefix(lower, "/joinchat/") || strings.HasPrefix(lower, "/addlist/") || strings.HasPrefix(lower, "tg://join?invite=")
}

func domainAllowed(host string, allowed []string) bool {
	host = strings.TrimPrefix(strings.ToLower(host), "www.")
	for _, domain := range allowed {
		domain = strings.TrimPrefix(strings.ToLower(domain), "www.")
		if host == domain || strings.HasSuffix(host, "."+domain) { return true }
	}
	return false
}

func (s *linkPolicyService) configure(b *gotgbot.Bot, c *ext.Context) error {
	chatID, err := groupChat(c)
	if err != nil { return reply(b, c, err.Error()) }
	actor, err := actorID(c)
	if err != nil { return reply(b, c, err.Error()) }
	if err = RequireActorAdmin(b, chatID, actor); err != nil { return reply(b, c, err.Error()) }
	if s.db == nil { return reply(b, c, "Link-protection settings are unavailable: PostgreSQL is not configured.") }

	args := commandArgs(c.EffectiveMessage)
	if len(args) == 0 {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		var enabled bool
		err = s.db.QueryRow(ctx, `SELECT enabled FROM link_policy_configs WHERE bot_id=$1 AND chat_id=$2`, b.User.Id, chatID).Scan(&enabled)
		if err == pgx.ErrNoRows { return reply(b, c, "Link protection is disabled. Use /antilink on to enable it.") }
		if err != nil { return reply(b, c, "I couldn't read this group's link-protection settings.") }
		if enabled { return reply(b, c, "🔗 Link protection is enabled. Unapproved external links and Telegram invite links are deleted. Use /allowlink to manage trusted domains.") }
		return reply(b, c, "Link protection is disabled. Use /antilink on to enable it.")
	}
	if len(args) != 1 || (!strings.EqualFold(args[0], "on") && !strings.EqualFold(args[0], "off")) {
		return reply(b, c, "Usage: /antilink on or /antilink off")
	}
	enabled := strings.EqualFold(args[0], "on")
	if enabled {
		if err = RequireBotDeletePermission(b, chatID); err != nil { return reply(b, c, err.Error()) }
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err = s.db.Exec(ctx, `INSERT INTO chats(chat_id) VALUES($1) ON CONFLICT(chat_id) DO NOTHING`, chatID)
	if err == nil {
		_, err = s.db.Exec(ctx, `INSERT INTO link_policy_configs(bot_id,chat_id,enabled,updated_by)
			VALUES($1,$2,$3,$4)
			ON CONFLICT(bot_id,chat_id) DO UPDATE SET enabled=EXCLUDED.enabled,updated_by=EXCLUDED.updated_by,updated_at=NOW()`,
			b.User.Id, chatID, enabled, actor)
	}
	if err != nil { return reply(b, c, "I couldn't save link-protection settings. Apply migration 007 and check PostgreSQL.") }
	if enabled { return reply(b, c, "🔗 Link protection enabled. Unapproved external links and Telegram invite links will be deleted. Add trusted domains with /allowlink add example.com.") }
	return reply(b, c, "Link protection disabled in this group.")
}

func (s *linkPolicyService) allowlistCommand(b *gotgbot.Bot, c *ext.Context) error {
	chatID, err := groupChat(c)
	if err != nil { return reply(b, c, err.Error()) }
	actor, err := actorID(c)
	if err != nil { return reply(b, c, err.Error()) }
	if err = RequireActorAdmin(b, chatID, actor); err != nil { return reply(b, c, err.Error()) }
	if s.db == nil { return reply(b, c, "Link allowlist storage is unavailable: PostgreSQL is not configured.") }
	args := commandArgs(c.EffectiveMessage)
	if len(args) == 0 { return reply(b, c, "Usage: /allowlink add <domain>, /allowlink remove <domain>, or /allowlink list") }
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	switch strings.ToLower(args[0]) {
	case "add", "remove":
		if len(args) != 2 { return reply(b, c, "Usage: /allowlink "+strings.ToLower(args[0])+" <domain>") }
		domain, domainErr := normalizeAllowedDomain(args[1])
		if domainErr != nil { return reply(b, c, domainErr.Error()) }
		if strings.EqualFold(args[0], "add") {
			_, err = s.db.Exec(ctx, `INSERT INTO link_allowlist(bot_id,chat_id,domain,added_by) VALUES($1,$2,$3,$4) ON CONFLICT(bot_id,chat_id,domain) DO NOTHING`, b.User.Id, chatID, domain, actor)
			if err == nil { return reply(b, c, "Added trusted domain: "+domain) }
		} else {
			tag, deleteErr := s.db.Exec(ctx, `DELETE FROM link_allowlist WHERE bot_id=$1 AND chat_id=$2 AND domain=$3`, b.User.Id, chatID, domain)
			err = deleteErr
			if err == nil && tag.RowsAffected() == 0 { return reply(b, c, "That domain was not in this group's allowlist.") }
			if err == nil { return reply(b, c, "Removed trusted domain: "+domain) }
		}
		return reply(b, c, "I couldn't update the allowlist. Check PostgreSQL and migration 007.")
	case "list":
		if len(args) != 1 { return reply(b, c, "Usage: /allowlink list") }
		rows, queryErr := s.db.Query(ctx, `SELECT domain FROM link_allowlist WHERE bot_id=$1 AND chat_id=$2 ORDER BY domain LIMIT 100`, b.User.Id, chatID)
		if queryErr != nil { return reply(b, c, "I couldn't read the allowlist.") }
		defer rows.Close()
		var domains []string
		for rows.Next() { var domain string; if rows.Scan(&domain) == nil { domains = append(domains, domain) } }
		if rows.Err() != nil { return reply(b, c, "I couldn't read the complete allowlist.") }
		if len(domains) == 0 { return reply(b, c, "No trusted domains are configured for this group.") }
		return reply(b, c, "✅ Trusted domains (up to 100):\n"+strings.Join(domains, "\n"))
	default:
		return reply(b, c, "Usage: /allowlink add <domain>, /allowlink remove <domain>, or /allowlink list")
	}
}

func (s *linkPolicyService) isAdmin(b *gotgbot.Bot, chatID, userID int64) bool {
	key := fmt.Sprintf("%d:%d:%d", b.User.Id, chatID, userID)
	now := time.Now()
	s.mu.RLock()
	cached, ok := s.adminCache[key]
	s.mu.RUnlock()
	if ok && now.Before(cached.expiresAt) { return cached.admin }
	member, err := b.GetChatMember(chatID, userID, nil)
	if err != nil || member == nil { return true } // Fail open if administrator status cannot be verified.
	admin := IsChatAdmin(member.GetStatus())
	s.mu.Lock()
	if len(s.adminCache) > 4096 { s.adminCache = make(map[string]linkAdminCacheEntry) }
	s.adminCache[key] = linkAdminCacheEntry{admin: admin, expiresAt: now.Add(30*time.Second)}
	s.mu.Unlock()
	return admin
}

func (s *linkPolicyService) reviewCooldown(ctx context.Context, key string) (bool, error) {
	// Redis is optional for the base link policy. If it is unavailable, do not
	// issue AI calls repeatedly; return false and keep deterministic protection.
	return false, nil
}

func (s *linkPolicyService) handleMessage(b *gotgbot.Bot, c *ext.Context) error {
	m := c.EffectiveMessage
	if m == nil || m.From == nil || m.MessageId <= 0 || s.db == nil { return nil }
	links := extractMessageLinks(m)
	if len(links) == 0 { return nil }
	chatID, userID := m.Chat.Id, m.From.Id
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	var enabled bool
	err := s.db.QueryRow(ctx, `SELECT enabled FROM link_policy_configs WHERE bot_id=$1 AND chat_id=$2`, b.User.Id, chatID).Scan(&enabled)
	if err != nil || !enabled { return nil } // Fail open if settings cannot be verified.
	if s.isAdmin(b, chatID, userID) { return nil }
	rows, err := s.db.Query(ctx, `SELECT domain FROM link_allowlist WHERE bot_id=$1 AND chat_id=$2`, b.User.Id, chatID)
	if err != nil { return nil }
	defer rows.Close()
	var allowedDomains []string
	for rows.Next() { var domain string; if rows.Scan(&domain) != nil { return nil }; allowedDomains = append(allowedDomains, domain) }
	if rows.Err() != nil { return nil }
	blocked := false
	suspicious := make([]string, 0, len(links))
	for _, link := range links {
		if isTelegramInviteLink(link) { blocked = true; break }
		host, _, ok := linkHost(link)
		if !ok { continue }
		if !domainAllowed(host, allowedDomains) { blocked = true }
		suspicious = append(suspicious, host)
	}
	// AI checks only extracted hostnames and a redacted message snippet. It never
	// fetches a URL, and its advisory result cannot override the deterministic policy.
	if s.ai != nil && len(suspicious) > 0 {
		claimKey := fmt.Sprintf("anitoon4:ai-link-review:%d:%d:%d", b.User.Id, chatID, userID)
		if claimed, claimErr := s.reviewCooldown(ctx, claimKey); claimErr == nil && claimed {
			redacted := linkPattern.ReplaceAllString(m.Text+" "+m.Caption, "[link]")
			event := fmt.Sprintf("Extracted link hostnames: %s. Message context: %s", strings.Join(uniqueStrings(suspicious), ", "), strings.TrimSpace(redacted))
			aiCtx, aiCancel := context.WithTimeout(ctx, 3*time.Second)
			analysis, aiErr := s.ai.Analyze(aiCtx, chatID, userID, m.MessageId, "link_protection", event)
			aiCancel()
			if aiErr == nil && analysis.Recommendation == "admin_review" && analysis.Category == "phishing" {
				_, _ = b.SendMessage(chatID, fmt.Sprintf("🤖 AI link review: possible phishing indicators (confidence %.0f%%). %s AI did not take action.", analysis.Confidence*100, analysis.Rationale), nil)
			}
		}
	}
	if !blocked { return nil }
	_, deleteErr := b.DeleteMessage(chatID, m.MessageId, nil)
	return deleteErr
}
