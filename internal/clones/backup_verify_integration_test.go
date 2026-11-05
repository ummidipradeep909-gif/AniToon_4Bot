package clones

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ummidipradeep909-gif/AniToon_4Bot/migrations"
)

func TestVerifyBackupCredentialsAuthenticatesCiphertext(t *testing.T) {
	dsn := os.Getenv("CLONE_TEST_DATABASE_URL")
	if dsn == "" { t.Skip("CLONE_TEST_DATABASE_URL is not configured") }
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	admin, err := pgxpool.New(ctx, dsn)
	if err != nil { t.Fatal(err) }
	defer admin.Close()
	schema := fmt.Sprintf("clone_backup_verify_%d", os.Getpid())
	if _, err := admin.Exec(ctx, "CREATE SCHEMA "+schema); err != nil { t.Fatal(err) }
	defer func() { _, _ = admin.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE") }()
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil { t.Fatal(err) }
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil { t.Fatal(err) }
	defer pool.Close()
	if err := migrations.Apply(ctx, pool); err != nil { t.Fatalf("apply schema: %v", err) }

	key := []byte("0123456789abcdef0123456789abcdef")
	encodedKey := base64.StdEncoding.EncodeToString(key)
	const botID int64 = 123456789
	const ownerID int64 = 987654321
	ciphertext, nonce, err := encryptToken(key, "123456789:synthetic_test_token", botID, ownerID)
	if err != nil { t.Fatal(err) }
	if _, err := pool.Exec(ctx, "INSERT INTO clone_credentials(bot_id, owner_id, encrypted_token, nonce, enabled) VALUES($1,$2,$3,$4,TRUE)", botID, ownerID, ciphertext, nonce); err != nil { t.Fatal(err) }

	count, err := VerifyBackupCredentials(ctx, pool, encodedKey)
	if err != nil { t.Fatalf("verify authentic credentials: %v", err) }
	if count != 1 { t.Fatalf("verified %d credentials, want 1", count) }

	if _, err := pool.Exec(ctx, "UPDATE clone_credentials SET encrypted_token = set_byte(encrypted_token, 0, get_byte(encrypted_token, 0) # 1) WHERE bot_id=$1", botID); err != nil { t.Fatal(err) }
	if _, err := VerifyBackupCredentials(ctx, pool, encodedKey); err == nil { t.Fatal("tampered ciphertext unexpectedly passed verification") }
}
