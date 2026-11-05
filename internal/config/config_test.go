package config

import (
 "encoding/base64"
 "os"
 "testing"
)

func requiredEnv(t *testing.T) {
 t.Helper()
 t.Setenv("BOT_TOKEN","123456:testing-token")
 t.Setenv("DATABASE_URL","postgres://user:pass@localhost/db")
 t.Setenv("REDIS_URL","redis://localhost:6379/0")
 t.Setenv("CLONE_ENCRYPTION_KEY",base64.StdEncoding.EncodeToString(make([]byte,32)))
 t.Setenv("AI_SERVICE_TOKEN","")
 t.Setenv("BOT_OWNER_ID","")
 t.Setenv("BOT_MODE","")
 t.Setenv("LOG_LEVEL","")
 t.Setenv("WEBHOOK_URL","")
 t.Setenv("WEBHOOK_PATH","")
 t.Setenv("WEBHOOK_LISTEN_ADDR","")
 t.Setenv("WEBHOOK_SECRET_TOKEN","")
 t.Setenv("PORT","")
}

func TestLoadRequiresSecrets(t *testing.T) {
 requiredEnv(t)
 t.Setenv("BOT_TOKEN","")
 if _,err:=Load(); err==nil { t.Fatal("missing bot token accepted") }
 requiredEnv(t)
 t.Setenv("DATABASE_URL","")
 if _,err:=Load(); err==nil { t.Fatal("missing database URL accepted") }
 requiredEnv(t)
 t.Setenv("REDIS_URL","")
 if _,err:=Load(); err==nil { t.Fatal("missing Redis URL accepted") }
 requiredEnv(t)
 t.Setenv("CLONE_ENCRYPTION_KEY","")
 if _,err:=Load(); err==nil { t.Fatal("missing clone encryption key accepted") }
}

func TestLoadDefaults(t *testing.T) {
 requiredEnv(t)
 got,err:=Load()
 if err!=nil { t.Fatalf("Load(): %v",err) }
 if got.Mode!="polling" || got.LogLevel!="info" { t.Fatalf("unexpected defaults: %#v",got) }
 if got.CloneEncryptionKey=="" { t.Fatal("clone key not loaded") }
}

func TestLoadRejectsUnsupportedModeAndLogLevel(t *testing.T) {
 requiredEnv(t); t.Setenv("BOT_MODE","push")
 if _,err:=Load(); err==nil { t.Fatal("unsupported mode accepted") }
 requiredEnv(t); t.Setenv("LOG_LEVEL","verbose")
 if _,err:=Load(); err==nil { t.Fatal("invalid log level accepted") }
}

func TestLoadWebhookValidation(t *testing.T) {
 requiredEnv(t); t.Setenv("BOT_MODE","webhook"); t.Setenv("WEBHOOK_URL","http://example.com"); t.Setenv("WEBHOOK_SECRET_TOKEN","valid-secret")
 if _,err:=Load(); err==nil { t.Fatal("non-HTTPS webhook accepted") }
 requiredEnv(t); t.Setenv("BOT_MODE","webhook"); t.Setenv("WEBHOOK_URL","https://example.com"); t.Setenv("WEBHOOK_SECRET_TOKEN","")
 if _,err:=Load(); err==nil { t.Fatal("missing webhook secret accepted") }
 requiredEnv(t); t.Setenv("BOT_MODE","webhook"); t.Setenv("WEBHOOK_URL","https://example.com"); t.Setenv("WEBHOOK_SECRET_TOKEN","valid-secret"); t.Setenv("WEBHOOK_PATH","/../secret")
 if _,err:=Load(); err==nil { t.Fatal("traversal webhook path accepted") }
}

func TestLoadDoesNotExposeSecretContents(t *testing.T) {
 requiredEnv(t)
 t.Setenv("BOT_TOKEN","")
 t.Setenv("BOT_TOKEN_FILE",os.TempDir()+"/anitoon-no-such-secret")
 _,err:=Load()
 if err==nil || err.Error()=="123456:testing-token" { t.Fatalf("unexpected error: %v",err) }
}

func TestLoadOwnerID(t *testing.T) { requiredEnv(t); t.Setenv("BOT_OWNER_ID","123456"); got,err:=Load(); if err!=nil || got.OwnerID!=123456 { t.Fatalf("owner ID: %#v %v",got,err) }; requiredEnv(t); t.Setenv("BOT_OWNER_ID","0"); if _,err:=Load(); err==nil { t.Fatal("zero owner ID accepted") } }

func TestLoadPostgresPoolDefaultsAndOverrides(t *testing.T) {
 requiredEnv(t)
 got, err := Load()
 if err != nil { t.Fatalf("Load(): %v", err) }
 if got.DBPoolMaxConns < 4 || got.DBPoolMaxConns > 32 || got.DBPoolMinConns != 0 {
  t.Fatalf("unexpected pool defaults: max=%d min=%d", got.DBPoolMaxConns, got.DBPoolMinConns)
 }
 if got.DBPoolMaxConnLifetime != 30*time.Minute || got.DBPoolMaxConnIdleTime != 5*time.Minute || got.DBPoolHealthCheckPeriod != time.Minute {
  t.Fatalf("unexpected pool durations: %#v", got)
 }
 requiredEnv(t)
 t.Setenv("DB_POOL_MAX_CONNS", "24")
 t.Setenv("DB_POOL_MIN_CONNS", "4")
 t.Setenv("DB_POOL_MAX_LIFETIME_SECONDS", "900")
 t.Setenv("DB_POOL_MAX_IDLE_SECONDS", "120")
 t.Setenv("DB_POOL_HEALTH_CHECK_SECONDS", "30")
 got, err = Load()
 if err != nil { t.Fatalf("Load() with overrides: %v", err) }
 if got.DBPoolMaxConns != 24 || got.DBPoolMinConns != 4 || got.DBPoolMaxConnLifetime != 15*time.Minute || got.DBPoolMaxConnIdleTime != 2*time.Minute || got.DBPoolHealthCheckPeriod != 30*time.Second {
  t.Fatalf("pool overrides not applied: %#v", got)
 }
}

func TestLoadRejectsInvalidPostgresPoolSettings(t *testing.T) {
 cases := []struct{name, value string}{
  {"DB_POOL_MAX_CONNS", "1"},
  {"DB_POOL_MAX_CONNS", "101"},
  {"DB_POOL_MAX_CONNS", "many"},
  {"DB_POOL_MIN_CONNS", "3"},
  {"DB_POOL_MAX_LIFETIME_SECONDS", "30"},
  {"DB_POOL_MAX_IDLE_SECONDS", "3601"},
  {"DB_POOL_HEALTH_CHECK_SECONDS", "0"},
 }
 for _, tc := range cases {
  t.Run(tc.name+"_"+tc.value, func(t *testing.T) {
   requiredEnv(t)
   if tc.name == "DB_POOL_MIN_CONNS" { t.Setenv("DB_POOL_MAX_CONNS", "2") }
   t.Setenv(tc.name, tc.value)
   if _, err := Load(); err == nil { t.Fatalf("invalid %s=%q accepted", tc.name, tc.value) }
  })
 }
}
