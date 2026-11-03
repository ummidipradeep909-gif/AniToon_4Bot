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
	t.Setenv("BOT_MODE", "webhook")
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
