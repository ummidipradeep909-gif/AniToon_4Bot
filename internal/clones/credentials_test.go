package clones

import (
	"context"
	"fmt"
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

	migration, err := os.ReadFile("../../migrations/010_clone_credentials.sql")
	if err != nil { t.Fatal(err) }
	tx, err := conn.Begin(ctx)
	if err != nil { t.Fatal(err) }
	defer tx.Rollback(ctx)
	schema := fmt.Sprintf("clone_credential_test_%d", os.Getpid())
	if _, err = tx.Exec(ctx, "CREATE SCHEMA "+schema); err != nil { t.Fatal(err) }
	if _, err = tx.Exec(ctx, "SET LOCAL search_path TO "+schema); err != nil { t.Fatal(err) }
	if _, err = tx.Exec(ctx, string(migration)); err != nil { t.Fatalf("execute clone credential migration: %v", err) }

	insert := `INSERT INTO clone_credentials (bot_id, owner_id, encrypted_token, nonce) VALUES ($1,$2,$3,$4)`
	if _, err = tx.Exec(ctx, insert, int64(100), int64(200), []byte("cipher"), make([]byte,12)); err != nil { t.Fatal(err) }
	if _, err = conn.Exec(ctx, insert, int64(101), int64(200), []byte("cipher"), make([]byte,12)); err == nil {
		t.Fatal("PostgreSQL accepted a second clone for the same owner")
	}
	if _, err = conn.Exec(ctx, insert, int64(100), int64(201), []byte("cipher"), make([]byte,12)); err == nil {
		t.Fatal("PostgreSQL accepted a duplicate clone bot ID")
	}
}
