package main

import (
	"log/slog"
	"os"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"

	"github.com/ummidipradeep909-gif/AniToon_4Bot/internal/bot"
	"github.com/ummidipradeep909-gif/AniToon_4Bot/internal/config"
	"github.com/ummidipradeep909-gif/AniToon_4Bot/internal/moderation"
)

func main() {
	cfg, err := config.Load()
	if err != nil { slog.Error("configuration error", "error", err); os.Exit(1) }
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	b, err := gotgbot.NewBot(cfg.BotToken, nil)
	if err != nil { logger.Error("could not initialise Telegram bot", "error", err); os.Exit(1) }
	dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{
		Error: func(_ *gotgbot.Bot, _ *ext.Context, err error) ext.DispatcherAction {
			logger.Error("update handler failed", "error", err)
			return ext.DispatcherActionNoop
		},
		MaxRoutines: ext.DefaultMaxRoutines,
		Logger: logger,
	})
	updater := ext.NewUpdater(dispatcher, &ext.UpdaterOpts{Logger: logger})
	dispatcher.AddHandler(handlers.NewCommand("start", bot.Start))
	dispatcher.AddHandler(handlers.NewCommand("help", bot.Help))
	dispatcher.AddHandler(handlers.NewCommand("id", bot.ID))
	dispatcher.AddHandler(handlers.NewCommand("ban", moderation.Ban))
	dispatcher.AddHandler(handlers.NewCommand("unban", moderation.Unban))
	logger.Info("AniToon_4Bot starting", "username", b.User.Username)
	if err := updater.StartPolling(b, &ext.PollingOpts{
		DropPendingUpdates: false,
		GetUpdatesOpts: &gotgbot.GetUpdatesOpts{
			Timeout: 30,
			RequestOpts: &gotgbot.RequestOpts{Timeout: 35 * time.Second},
		},
	}); err != nil { logger.Error("polling stopped", "error", err); os.Exit(1) }
	updater.Idle()
}
