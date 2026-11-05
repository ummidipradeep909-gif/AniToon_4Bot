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
