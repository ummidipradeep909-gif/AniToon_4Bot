package migrations

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestEmbeddedMigrationsApplyTwiceOnFreshSchema(t *testing.T) {
	dsn := os.Getenv("CLONE_TEST_DATABASE_URL")
	if dsn == "" { t.Skip("CLONE_TEST_DATABASE_URL is not configured") }
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	admin, err := pgxpool.New(ctx, dsn)
	if err != nil { t.Fatal(err) }
	defer admin.Close()
	schema := fmt.Sprintf("embedded_migrations_test_%d", os.Getpid())
	if _, err = admin.Exec(ctx, "CREATE SCHEMA "+schema); err != nil { t.Fatal(err) }
	defer func() { _, _ = admin.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE") }()
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil { t.Fatal(err) }
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil { t.Fatal(err) }
	defer pool.Close()

	if err = Apply(ctx, pool); err != nil { t.Fatalf("apply embedded migrations: %v", err) }
	if err = Apply(ctx, pool); err != nil { t.Fatalf("reapply embedded migrations: %v", err) }
	var applied int
	if err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM schema_migrations").Scan(&applied); err != nil { t.Fatal(err) }
	if applied != 17 { t.Fatalf("got %d embedded migrations, want 17", applied) }
	version, verified, err := VerifyLedger(ctx, pool)
	if err != nil { t.Fatalf("verify restored migration ledger: %v", err) }
	if version != 17 || verified != applied { t.Fatalf("verified version/count = %d/%d, want 17/%d", version, verified, applied) }
	if _, err = pool.Exec(ctx, "UPDATE schema_migrations SET checksum = $1 WHERE version = 1", "tampered"); err != nil { t.Fatal(err) }
	if _, _, err = VerifyLedger(ctx, pool); err == nil { t.Fatal("expected checksum mismatch to fail verification") }
	var enabledColumn bool
	if err = pool.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema=$1 AND table_name='clone_credentials' AND column_name='enabled')", schema).Scan(&enabledColumn); err != nil { t.Fatal(err) }
	if !enabledColumn { t.Fatal("clone lifecycle migration did not add enabled column") }
}
