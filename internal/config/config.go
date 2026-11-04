package config

import (
	"encoding/base64"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strings"

	"github.com/ummidipradeep909-gif/AniToon_4Bot/internal/secrets"
)

var webhookSecretPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,256}$`)

type Config struct {
	BotToken, DatabaseURL, RedisURL, CloneEncryptionKey, AIServiceToken string
	Mode, LogLevel, WebhookURL, WebhookPath, WebhookListenAddr, WebhookSecretToken string
	PolicyServiceURL string
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
