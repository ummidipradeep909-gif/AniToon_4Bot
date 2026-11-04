package clones

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ummidipradeep909-gif/AniToon_4Bot/internal/bot"
	"github.com/ummidipradeep909-gif/AniToon_4Bot/internal/moderation"
)

type Manager struct {
	mu sync.Mutex
	updater *ext.Updater
	primaryID int64
	pending map[int64]time.Time
	owners map[int64]int64
	active map[int64]bool
	logger *slog.Logger
	store credentialStore
	key []byte
}

func NewManager(updater *ext.Updater, primaryID int64, logger *slog.Logger, db *pgxpool.Pool, encodedKey string) *Manager {
	if logger == nil { logger = slog.Default() }
	key, keyErr := parseEncryptionKey(encodedKey)
	if keyErr != nil { key = nil }
	return &Manager{
		updater: updater, primaryID: primaryID, pending: map[int64]time.Time{},
		owners: map[int64]int64{}, active: map[int64]bool{}, logger: logger,
		store: credentialStore{db: db}, key: key,
	}
}

func (m *Manager) persistenceReady() bool { return m.store.db != nil && len(m.key) == 32 }

func (m *Manager) Register(d *ext.Dispatcher) {
	d.AddHandler(handlers.NewCommand("clone", m.Begin))
	d.AddHandler(handlers.NewCommand("cancel", m.Cancel))
	d.AddHandler(handlers.NewMessage(nil, m.ReceiveToken))
}

func (m *Manager) Begin(b *gotgbot.Bot, c *ext.Context) error {
	if b.User.Id != m.primaryID || c.EffectiveChat == nil || c.EffectiveUser == nil || c.EffectiveMessage == nil { return nil }
	if c.EffectiveChat.Type != "private" {
		_, e := c.EffectiveMessage.Reply(b, "For token safety, open my private chat and send /clone there.", nil)
		return e
	}
	if !m.persistenceReady() {
		_, e := c.EffectiveMessage.Reply(b, "Clone creation is temporarily unavailable because secure persistent storage is not configured. Please contact the bot operator.", nil)
		return e
	}
	id := c.EffectiveUser.Id
	m.mu.Lock()
	if m.owners[id] != 0 {
		m.mu.Unlock()
		_, e := c.EffectiveMessage.Reply(b, "You already have a clone connected. Only one clone per account is supported.", nil)
		return e
	}
	m.pending[id] = time.Now().Add(10*time.Minute)
	m.mu.Unlock()
	text := "<b>Create your own AniToon clone bot</b>\n\n1. Tap Open BotFather and send /newbot.\n2. Follow the steps to choose a bot name and username.\n3. Copy the API token BotFather gives you.\n4. Return here and send the token in this private chat.\n\nI will delete the token message immediately, verify it with Telegram, securely save it, and start your clone. Setup expires in 10 minutes.\n\n<b>Security:</b> never send your token in a group or to another person. Use /cancel to stop."
	kb := gotgbot.InlineKeyboardMarkup{InlineKeyboard: [][]gotgbot.InlineKeyboardButton{{{Text: "🤖 Open BotFather", Url: "https://t.me/BotFather"}}}}
	_, e := c.EffectiveMessage.Reply(b, text, &gotgbot.SendMessageOpts{ParseMode: "HTML", ReplyMarkup: kb})
	return e
}

func (m *Manager) Cancel(b *gotgbot.Bot, c *ext.Context) error {
	if c.EffectiveChat == nil || c.EffectiveUser == nil || c.EffectiveMessage == nil || c.EffectiveChat.Type != "private" { return nil }
	m.mu.Lock()
	delete(m.pending, c.EffectiveUser.Id)
	m.mu.Unlock()
	_, e := c.EffectiveMessage.Reply(b, "Clone setup cancelled.", nil)
	return e
}

func (m *Manager) ReceiveToken(b *gotgbot.Bot, c *ext.Context) error {
	msg := c.EffectiveMessage
	if b.User.Id != m.primaryID || msg == nil || c.EffectiveChat == nil || c.EffectiveUser == nil || c.EffectiveChat.Type != "private" { return nil }
	owner := c.EffectiveUser.Id
	m.mu.Lock()
	expiry, waiting := m.pending[owner]
	if waiting && time.Now().After(expiry) { delete(m.pending, owner); waiting = false }
	connected := m.owners[owner] != 0
	m.mu.Unlock()
	if !waiting { return nil }

	// Delete before inspecting, validating, or otherwise using the credential.
	deleted, deleteErr := b.DeleteMessage(c.EffectiveChat.Id, msg.MessageId, nil)
	if deleteErr != nil || !deleted {
		m.mu.Lock(); delete(m.pending, owner); m.mu.Unlock()
		_, e := b.SendMessage(c.EffectiveChat.Id, "I couldn't confirm deletion of that message, so I did not process the token. Revoke it with @BotFather before trying again.", nil)
		return e
	}
	token := strings.TrimSpace(msg.Text)
	if connected {
		_, e := b.SendMessage(c.EffectiveChat.Id, "You already have a clone connected.", nil)
		return e
	}
	if !m.persistenceReady() {
		m.mu.Lock(); delete(m.pending, owner); m.mu.Unlock()
		_, e := b.SendMessage(c.EffectiveChat.Id, "Secure persistent clone storage is unavailable. I deleted the message but did not process or save the token.", nil)
		return e
	}
	if strings.ContainsAny(token, " \t\r\n") || !strings.Contains(token, ":") || len(token) < 20 || len(token) > 256 {
		m.mu.Lock(); delete(m.pending, owner); m.mu.Unlock()
		_, e := b.SendMessage(c.EffectiveChat.Id, "That doesn't look like a BotFather token. I deleted the message. Send /clone to restart setup.", nil)
		return e
	}

	user, err := m.connectNew(token, owner)
	token = ""
	m.mu.Lock(); delete(m.pending, owner); m.mu.Unlock()
	if err != nil {
		_, e := b.SendMessage(c.EffectiveChat.Id, "I couldn't connect that bot. Check the token and make sure it isn't running on another host, then send /clone to try again. Your token message was deleted.", nil)
		return e
	}
	_, e := b.SendMessage(c.EffectiveChat.Id, fmt.Sprintf("<b>Your AniToon clone is online!</b>\n\nBot: @%s\n\nOpen it and send /start. Add it to your group and promote it with the permissions needed for moderation.\n\nYour encrypted credentials are saved for automatic recovery after restarts.", user.Username), &gotgbot.SendMessageOpts{ParseMode: "HTML"})
	return e
}

// connectNew validates the token first, commits encrypted credentials, and only then starts polling.
// The manager lock serializes owner and bot uniqueness checks with in-process starts.
func (m *Manager) connectNew(token string, owner int64) (*gotgbot.User, error) {
	clone, err := gotgbot.NewBot(token, nil)
	token = ""
	if err != nil || clone == nil || clone.User.Id == m.primaryID { return nil, fmt.Errorf("clone validation failed") }
	webhook, err := clone.GetWebhookInfo(nil)
	if err != nil || (webhook != nil && webhook.Url != "") { return nil, fmt.Errorf("clone webhook check failed") }

	m.mu.Lock()
	defer m.mu.Unlock()
	if m.owners[owner] != 0 || m.active[clone.User.Id] { return nil, fmt.Errorf("clone already registered") }

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ciphertext, nonce, err := encryptToken(m.key, clone.Token, clone.User.Id, owner)
	if err != nil { return nil, fmt.Errorf("clone encryption failed") }
	if err := m.store.insert(ctx, credential{BotID: clone.User.Id, OwnerID: owner, Ciphertext: ciphertext, Nonce: nonce}); err != nil {
		return nil, fmt.Errorf("clone persistence failed")
	}
	if err := m.startPolling(clone); err != nil {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		cleanupErr := m.store.delete(cleanupCtx, clone.User.Id)
		cleanupCancel()
		if cleanupErr != nil { m.logger.Error("failed to remove inactive clone credential after startup failure", "bot_id", clone.User.Id) }
		return nil, fmt.Errorf("clone polling failed")
	}
	m.active[clone.User.Id] = true
	m.owners[owner] = clone.User.Id
	m.logger.Info("clone bot connected", "bot_id", clone.User.Id, "owner_id", owner)
	u := clone.User
	return &u, nil
}

func (m *Manager) startPolling(clone *gotgbot.Bot) error {
	return m.updater.StartPolling(clone, &ext.PollingOpts{GetUpdatesOpts: &gotgbot.GetUpdatesOpts{
		Timeout: 30, AllowedUpdates: []string{"message", "chat_member"},
		RequestOpts: &gotgbot.RequestOpts{Timeout: 35*time.Second},
	}})
}

// Recover loads and independently validates every persisted clone. One bad record never blocks the primary bot.
func (m *Manager) Recover(ctx context.Context) {
	if !m.persistenceReady() {
		m.logger.Warn("clone recovery skipped; PostgreSQL or CLONE_ENCRYPTION_KEY is unavailable")
		return
	}
	rows, err := m.store.list(ctx)
	if err != nil {
		m.logger.Error("clone recovery could not load credentials; primary bot will continue")
		return
	}
	for _, row := range rows {
		if row.BotID == m.primaryID {
			m.logger.Error("clone recovery skipped a credential using the primary bot ID")
			continue
		}
		token, decryptErr := decryptToken(m.key, row.Ciphertext, row.Nonce, row.BotID, row.OwnerID)
		if decryptErr != nil {
			m.logger.Error("clone recovery failed authentication; primary bot will continue", "bot_id", row.BotID, "owner_id", row.OwnerID)
			continue
		}
		clone, botErr := gotgbot.NewBot(token, nil)
		token = ""
		if botErr != nil || clone == nil || clone.User.Id != row.BotID {
			m.logger.Error("clone recovery Telegram validation failed; primary bot will continue", "bot_id", row.BotID)
			continue
		}
		webhook, webhookErr := clone.GetWebhookInfo(nil)
		if webhookErr != nil || (webhook != nil && webhook.Url != "") {
			m.logger.Error("clone recovery webhook check failed; primary bot will continue", "bot_id", row.BotID)
			continue
		}
		m.mu.Lock()
		if m.owners[row.OwnerID] != 0 || m.active[row.BotID] {
			m.mu.Unlock()
			m.logger.Error("clone recovery skipped duplicate owner or bot record", "bot_id", row.BotID, "owner_id", row.OwnerID)
			continue
		}
		if startErr := m.startPolling(clone); startErr != nil {
			m.mu.Unlock()
			m.logger.Error("clone recovery polling failed; primary bot will continue", "bot_id", row.BotID)
			continue
		}
		m.owners[row.OwnerID] = row.BotID
		m.active[row.BotID] = true
		m.mu.Unlock()
		m.logger.Info("clone bot recovered", "bot_id", row.BotID, "owner_id", row.OwnerID)
	}
}

// Common commands are registered once; the shared updater dispatches updates from all bots.
func RegisterCommands(d *ext.Dispatcher) {
	d.AddHandler(handlers.NewCommand("start", bot.Start))
	d.AddHandler(handlers.NewCommand("help", bot.Help))
	d.AddHandler(handlers.NewCommand("id", bot.ID))
	d.AddHandler(handlers.NewCommand("ban", moderation.Ban))
	d.AddHandler(handlers.NewCommand("unban", moderation.Unban))
	d.AddHandler(handlers.NewCommand("mute", moderation.Mute))
	d.AddHandler(handlers.NewCommand("unmute", moderation.Unmute))
	d.AddHandler(handlers.NewCommand("kick", moderation.Kick))
}
