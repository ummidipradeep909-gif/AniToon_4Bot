package clones

import (
	"fmt"
	"sync"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

// Manager starts user-owned clone bots on the same dispatcher as the main bot.
// Tokens remain only in process memory; durable encrypted storage is a later
// requirement before this can be treated as a production multi-tenant service.
type Manager struct {
	mu sync.Mutex
	updater *ext.Updater
	primaryBotID int64
	pending map[int64]time.Time
	active map[int64]int64 // clone bot ID -> owner Telegram user ID
}

func NewManager(updater *ext.Updater, primaryBotID int64) *Manager {
	return &Manager{
		updater: updater,
		primaryBotID: primaryBotID,
		pending: make(map[int64]time.Time),
		active: make(map[int64]int64),
	}
}

func (m *Manager) Begin(userID int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.pending[userID] = time.Now().Add(10 * time.Minute)
}

func (m *Manager) Cancel(userID int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.pending, userID)
}

func (m *Manager) IsPending(userID int64) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	expiry, ok := m.pending[userID]
	if !ok { return false }
	if time.Now().After(expiry) {
		delete(m.pending, userID)
		return false
	}
	return true
}

// Start validates the supplied token via getMe and starts polling it using
// the application's shared dispatcher. It never returns or logs the token.
func (m *Manager) Start(token string, ownerID int64) (*gotgbot.User, error) {
	cloneBot, err := gotgbot.NewBot(token, nil)
	if err != nil {
		return nil, fmt.Errorf("Telegram could not verify that token")
	}
	if cloneBot.User.Id == m.primaryBotID {
		return nil, fmt.Errorf("the main AniToon_4Bot token cannot be used as a clone")
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.active[cloneBot.User.Id]; exists {
		return nil, fmt.Errorf("this bot is already connected to AniToon_4Bot")
	}

	webhook, err := cloneBot.GetWebhookInfo(nil)
	if err != nil {
		return nil, fmt.Errorf("could not check the clone bot's webhook configuration")
	}
	if webhook != nil && webhook.Url != "" {
		return nil, fmt.Errorf("this bot currently has a webhook configured; stop its existing hosting and remove its webhook before connecting it here")
	}

	err = m.updater.StartPolling(cloneBot, &ext.PollingOpts{
		DropPendingUpdates: false,
		GetUpdatesOpts: &gotgbot.GetUpdatesOpts{
			Timeout: 30,
			RequestOpts: &gotgbot.RequestOpts{Timeout: 35 * time.Second},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("could not start this clone bot; check whether another service is polling it")
	}
	m.active[cloneBot.User.Id] = ownerID
	delete(m.pending, ownerID)
	user := cloneBot.User
	return &user, nil
}
