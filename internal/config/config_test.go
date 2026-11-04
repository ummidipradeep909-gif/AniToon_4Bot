package config

import (
	"os"
	"testing"
)

func TestLoadRequiresToken(t *testing.T) {
	t.Setenv("BOT_TOKEN", "")
	t.Setenv("BOT_MODE", "")
	t.Setenv("LOG_LEVEL", "")
	if _, err := Load(); err == nil {
		t.Fatal("expected missing BOT_TOKEN to fail")
	}
}

func TestLoadDefaults(t *testing.T) {
	t.Setenv("BOT_TOKEN", "123456:testing-token")
	t.Setenv("BOT_MODE", "")
	t.Setenv("LOG_LEVEL", "")
	got, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got.Mode != "polling" || got.LogLevel != "info" {
		t.Fatalf("unexpected defaults: %#v", got)
	}
}

func TestLoadRejectsUnsupportedMode(t *testing.T) {
	t.Setenv("BOT_TOKEN", "123456:testing-token")
	t.Setenv("BOT_MODE", "push")
	t.Setenv("LOG_LEVEL", "")
	if _, err := Load(); err == nil {
		t.Fatal("expected unsupported mode to fail")
	}
}

func TestLoadRejectsInvalidLogLevel(t *testing.T) {
	t.Setenv("BOT_TOKEN", "123456:testing-token")
	t.Setenv("BOT_MODE", "polling")
	t.Setenv("LOG_LEVEL", "verbose")
	if _, err := Load(); err == nil {
		t.Fatal("expected invalid log level to fail")
	}
}

func TestEnvironmentIsIsolated(t *testing.T) {
	// Keep this package's tests independent of any shell-provided token.
	for _, key := range []string{"BOT_TOKEN", "BOT_MODE", "LOG_LEVEL"} {
		old, ok := os.LookupEnv(key)
		t.Cleanup(func() {
			if ok { _ = os.Setenv(key, old) } else { _ = os.Unsetenv(key) }
		})
	}
}

func TestLoadWebhookDefaultsAndValidation(t *testing.T) {
	t.Setenv("BOT_TOKEN", "123456:testing-token")
	t.Setenv("BOT_MODE", "webhook")
	t.Setenv("LOG_LEVEL", "info")
	t.Setenv("WEBHOOK_URL", "https://bot.example.com")
	t.Setenv("WEBHOOK_PATH", "")
	t.Setenv("WEBHOOK_LISTEN_ADDR", "")
	t.Setenv("WEBHOOK_SECRET_TOKEN", "secret_token-123")
	got, err := Load()
	if err != nil { t.Fatalf("Load() error = %v", err) }
	if got.WebhookPath != "/telegram/webhook" || got.WebhookListenAddr != ":8080" {
		t.Fatalf("unexpected webhook defaults: %#v", got)
	}
	if got.WebhookURL+got.WebhookPath != "https://bot.example.com/telegram/webhook" {
		t.Fatalf("unexpected public webhook URL: %s%s", got.WebhookURL, got.WebhookPath)
	}
}

func TestLoadWebhookRequiresHTTPSAndSecret(t *testing.T) {
	base := func() {
		t.Setenv("BOT_TOKEN", "123456:testing-token")
		t.Setenv("BOT_MODE", "webhook")
		t.Setenv("LOG_LEVEL", "info")
		t.Setenv("WEBHOOK_PATH", "/telegram/webhook")
		t.Setenv("WEBHOOK_LISTEN_ADDR", ":8080")
		t.Setenv("WEBHOOK_SECRET_TOKEN", "secret_token-123")
	}
	base()
	t.Setenv("WEBHOOK_URL", "http://bot.example.com")
	if _, err := Load(); err == nil { t.Fatal("expected non-HTTPS webhook URL to fail") }
	base()
	t.Setenv("WEBHOOK_URL", "https://bot.example.com")
	t.Setenv("WEBHOOK_SECRET_TOKEN", "")
	if _, err := Load(); err == nil { t.Fatal("expected missing webhook secret to fail") }
	base()
	t.Setenv("WEBHOOK_URL", "https://bot.example.com")
	t.Setenv("WEBHOOK_PATH", "/../secret")
	if _, err := Load(); err == nil { t.Fatal("expected path traversal to fail") }
}

func TestLoadWebhookUsesPlatformPort(t *testing.T) {
	t.Setenv("BOT_TOKEN", "123456:testing-token")
	t.Setenv("BOT_MODE", "webhook")
	t.Setenv("LOG_LEVEL", "info")
	t.Setenv("WEBHOOK_URL", "https://bot.example.com")
	t.Setenv("WEBHOOK_PATH", "")
	t.Setenv("WEBHOOK_LISTEN_ADDR", "")
	t.Setenv("PORT", "9000")
	t.Setenv("WEBHOOK_SECRET_TOKEN", "secret_token-123")
	got, err := Load()
	if err != nil { t.Fatalf("Load() error = %v", err) }
	if got.WebhookListenAddr != ":9000" { t.Fatalf("listener = %q, want :9000", got.WebhookListenAddr) }
}
