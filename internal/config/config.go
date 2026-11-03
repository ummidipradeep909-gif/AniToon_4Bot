package config

import (
	"fmt"
	"os"
	"strings"
)

type Config struct {
	BotToken string
	Mode string
	LogLevel string
}

func Load() (Config, error) {
	c := Config{
		BotToken: strings.TrimSpace(os.Getenv("BOT_TOKEN")),
		Mode: strings.ToLower(strings.TrimSpace(os.Getenv("BOT_MODE"))),
		LogLevel: strings.ToLower(strings.TrimSpace(os.Getenv("LOG_LEVEL"))),
	}
	if c.BotToken == "" {
		return Config{}, fmt.Errorf("BOT_TOKEN is required")
	}
	if c.Mode == "" {
		c.Mode = "polling"
	}
	if c.Mode != "polling" {
		return Config{}, fmt.Errorf("unsupported BOT_MODE %q; supported mode: polling", c.Mode)
	}
	if c.LogLevel == "" {
		c.LogLevel = "info"
	}
	switch c.LogLevel {
	case "debug", "info", "warn", "error":
	default:
		return Config{}, fmt.Errorf("invalid LOG_LEVEL %q", c.LogLevel)
	}
	return c, nil
}
