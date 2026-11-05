package config

import (
	"encoding/base64"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/ummidipradeep909-gif/AniToon_4Bot/internal/secrets"
)

var webhookSecretPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,256}$`)

type Config struct {
	BotToken, DatabaseURL, RedisURL, CloneEncryptionKey, AIServiceToken string
	Mode, LogLevel, WebhookURL, WebhookPath, WebhookListenAddr, WebhookSecretToken string
	PolicyServiceURL string
	OwnerID int64
	DBPoolMaxConns int32
	DBPoolMinConns int32
	DBPoolMaxConnLifetime time.Duration
	DBPoolMaxConnIdleTime time.Duration
	DBPoolHealthCheckPeriod time.Duration
}

func Load() (Config, error) {
	var c Config
	var err error
	for _, item := range []struct{name string; dst *string; required bool}{
		{"BOT_TOKEN",&c.BotToken,true}, {"DATABASE_URL",&c.DatabaseURL,true},
		{"REDIS_URL",&c.RedisURL,true}, {"CLONE_ENCRYPTION_KEY",&c.CloneEncryptionKey,true},
		{"AI_SERVICE_TOKEN",&c.AIServiceToken,false},
	} {
		*item.dst,err=secrets.Load(item.name,item.required)
		if err!=nil { return Config{},err }
	}
	key,e:=base64.StdEncoding.DecodeString(c.CloneEncryptionKey)
	if e!=nil || len(key)!=32 { return Config{},fmt.Errorf("CLONE_ENCRYPTION_KEY must be the existing base64-encoded 32-byte AES key") }
	maxDefault := runtime.NumCPU() * 4
	if maxDefault < 4 { maxDefault = 4 }
	if maxDefault > 32 { maxDefault = 32 }
	maxConns, parseErr := boundedIntEnv("DB_POOL_MAX_CONNS", maxDefault, 2, 100)
	if parseErr != nil { return Config{}, parseErr }
	minConns, parseErr := boundedIntEnv("DB_POOL_MIN_CONNS", 0, 0, maxConns)
	if parseErr != nil { return Config{}, parseErr }
	maxLifetime, parseErr := boundedDurationEnv("DB_POOL_MAX_LIFETIME_SECONDS", 1800, 60, 86400)
	if parseErr != nil { return Config{}, parseErr }
	maxIdle, parseErr := boundedDurationEnv("DB_POOL_MAX_IDLE_SECONDS", 300, 0, 3600)
	if parseErr != nil { return Config{}, parseErr }
	healthPeriod, parseErr := boundedDurationEnv("DB_POOL_HEALTH_CHECK_SECONDS", 60, 5, 600)
	if parseErr != nil { return Config{}, parseErr }
	c.DBPoolMaxConns, c.DBPoolMinConns = int32(maxConns), int32(minConns)
	c.DBPoolMaxConnLifetime, c.DBPoolMaxConnIdleTime = maxLifetime, maxIdle
	c.DBPoolHealthCheckPeriod = healthPeriod

	ownerValue := strings.TrimSpace(os.Getenv("BOT_OWNER_ID"))
	if ownerValue != "" {
		c.OwnerID, err = strconv.ParseInt(ownerValue, 10, 64)
		if err != nil || c.OwnerID <= 0 {
			return Config{}, fmt.Errorf("BOT_OWNER_ID must be a positive Telegram user ID")
		}
	}
	if c.Mode=strings.ToLower(strings.TrimSpace(os.Getenv("BOT_MODE"))); c.Mode=="" { c.Mode="polling" }
	if c.Mode!="polling" && c.Mode!="webhook" { return Config{},fmt.Errorf("unsupported BOT_MODE; supported modes: polling, webhook") }
	if c.LogLevel=strings.ToLower(strings.TrimSpace(os.Getenv("LOG_LEVEL"))); c.LogLevel=="" { c.LogLevel="info" }
	switch c.LogLevel { case "debug","info","warn","error": default: return Config{},fmt.Errorf("invalid LOG_LEVEL") }
	if c.PolicyServiceURL=strings.TrimRight(strings.TrimSpace(os.Getenv("POLICY_SERVICE_URL")),"/"); c.PolicyServiceURL=="" { c.PolicyServiceURL="http://127.0.0.1:8000" }
	if c.WebhookURL=strings.TrimRight(strings.TrimSpace(os.Getenv("WEBHOOK_URL")),"/"); c.Mode=="webhook" {
		u,e:=url.Parse(c.WebhookURL)
		if e!=nil || u.Scheme!="https" || u.Host=="" || u.User!=nil || u.RawQuery!="" || u.Fragment!="" { return Config{},fmt.Errorf("WEBHOOK_URL must be an HTTPS base URL without credentials, query, or fragment") }
		c.WebhookPath=strings.TrimSpace(os.Getenv("WEBHOOK_PATH")); if c.WebhookPath=="" { c.WebhookPath="/telegram/webhook" }
		if !strings.HasPrefix(c.WebhookPath,"/") || strings.Contains(c.WebhookPath,"..") || strings.ContainsAny(c.WebhookPath,"?# ") { return Config{},fmt.Errorf("WEBHOOK_PATH must be an absolute URL path without traversal, query, or fragment") }
		c.WebhookListenAddr=strings.TrimSpace(os.Getenv("WEBHOOK_LISTEN_ADDR")); if c.WebhookListenAddr=="" { port:=strings.TrimSpace(os.Getenv("PORT")); if port=="" { port="8080" }; c.WebhookListenAddr=":"+port }
		c.WebhookSecretToken,err=secrets.Load("WEBHOOK_SECRET_TOKEN",true); if err!=nil { return Config{},err }
		if !webhookSecretPattern.MatchString(c.WebhookSecretToken) { return Config{},fmt.Errorf("WEBHOOK_SECRET_TOKEN contains unsupported characters") }
	}
	return c,nil
}

func boundedIntEnv(name string, fallback, min, max int) (int, error) {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" { return fallback, nil }
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < min || parsed > max {
		return 0, fmt.Errorf("%s must be an integer between %d and %d", name, min, max)
	}
	return parsed, nil
}

func boundedDurationEnv(name string, fallback, min, max int) (time.Duration, error) {
	seconds, err := boundedIntEnv(name, fallback, min, max)
	if err != nil { return 0, err }
	return time.Duration(seconds) * time.Second, nil
}
