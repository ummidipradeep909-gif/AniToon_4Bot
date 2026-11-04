package clones

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

func TestPostgresCloneCredentialUniqueness(t *testing.T) {
	dsn := os.Getenv("CLONE_TEST_DATABASE_URL")
	if dsn == "" { t.Skip("CLONE_TEST_DATABASE_URL is not configured") }
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil { t.Fatalf("connect PostgreSQL test service: %v", err) }
	defer conn.Close(context.Background())

	_, err = conn.Exec(ctx, `CREATE TEMP TABLE clone_credentials_test (
		bot_id BIGINT PRIMARY KEY,
		owner_id BIGINT NOT NULL UNIQUE,
		encrypted_token BYTEA NOT NULL,
		nonce BYTEA NOT NULL
	)`)
	if err != nil { t.Fatal(err) }
	insert := `INSERT INTO clone_credentials_test (bot_id, owner_id, encrypted_token, nonce) VALUES ($1,$2,$3,$4)`
	if _, err = conn.Exec(ctx, insert, int64(100), int64(200), []byte("cipher"), make([]byte,12)); err != nil { t.Fatal(err) }
	if _, err = conn.Exec(ctx, insert, int64(101), int64(200), []byte("cipher"), make([]byte,12)); err == nil {
		t.Fatal("PostgreSQL accepted a second clone for the same owner")
	}
	if _, err = conn.Exec(ctx, insert, int64(100), int64(201), []byte("cipher"), make([]byte,12)); err == nil {
		t.Fatal("PostgreSQL accepted a duplicate clone bot ID")
	}
}
