package config

import (
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strings"
)

var webhookSecretPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,256}$`)

type Config struct {
	BotToken string
	Mode string
	LogLevel string
	WebhookURL string
	WebhookPath string
	WebhookListenAddr string
	WebhookSecretToken string
}

func Load() (Config, error) {
	c := Config{
		BotToken: strings.TrimSpace(os.Getenv("BOT_TOKEN")),
		Mode: strings.ToLower(strings.TrimSpace(os.Getenv("BOT_MODE"))),
		LogLevel: strings.ToLower(strings.TrimSpace(os.Getenv("LOG_LEVEL"))),
		WebhookURL: strings.TrimRight(strings.TrimSpace(os.Getenv("WEBHOOK_URL")), "/"),
		WebhookPath: strings.TrimSpace(os.Getenv("WEBHOOK_PATH")),
		WebhookListenAddr: strings.TrimSpace(os.Getenv("WEBHOOK_LISTEN_ADDR")),
		WebhookSecretToken: strings.TrimSpace(os.Getenv("WEBHOOK_SECRET_TOKEN")),
	}
	if c.BotToken == "" {
		return Config{}, fmt.Errorf("BOT_TOKEN is required")
	}
	if c.Mode == "" {
		c.Mode = "polling"
	}
	if c.Mode != "polling" && c.Mode != "webhook" {
		return Config{}, fmt.Errorf("unsupported BOT_MODE %q; supported modes: polling, webhook", c.Mode)
	}
	if c.LogLevel == "" {
		c.LogLevel = "info"
	}
	switch c.LogLevel {
	case "debug", "info", "warn", "error":
	default:
		return Config{}, fmt.Errorf("invalid LOG_LEVEL %q", c.LogLevel)
	}

	if c.Mode == "webhook" {
		if c.WebhookURL == "" {
			return Config{}, fmt.Errorf("WEBHOOK_URL is required when BOT_MODE=webhook")
		}
		u, err := url.Parse(c.WebhookURL)
		if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
			return Config{}, fmt.Errorf("WEBHOOK_URL must be an HTTPS base URL without credentials, query, or fragment")
		}
		if c.WebhookPath == "" {
			c.WebhookPath = "/telegram/webhook"
		}
		if !strings.HasPrefix(c.WebhookPath, "/") || strings.Contains(c.WebhookPath, "..") || strings.ContainsAny(c.WebhookPath, "?# ") {
			return Config{}, fmt.Errorf("WEBHOOK_PATH must be an absolute URL path without traversal, query, or fragment")
		}
		if c.WebhookListenAddr == "" {
			c.WebhookListenAddr = ":8080"
		}
		if c.WebhookSecretToken == "" || !webhookSecretPattern.MatchString(c.WebhookSecretToken) {
			return Config{}, fmt.Errorf("WEBHOOK_SECRET_TOKEN is required and must contain 1–256 letters, digits, underscores, or hyphens")
		}
	}
	return c, nil
}
