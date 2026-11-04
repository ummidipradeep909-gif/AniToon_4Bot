package migrations

import (
	"context"
	"fmt"
	"os"
	"testing"
	"testing/fstest"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestApplyFSPostgresIntegration(t *testing.T) {
	dsn := os.Getenv("CLONE_TEST_DATABASE_URL")
	if dsn == "" { t.Skip("CLONE_TEST_DATABASE_URL is not configured") }
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	admin, err := pgxpool.New(ctx, dsn)
	if err != nil { t.Fatal(err) }
	defer admin.Close()
	schema := fmt.Sprintf("migration_test_%d", os.Getpid())
	if _, err = admin.Exec(ctx, "CREATE SCHEMA "+schema); err != nil { t.Fatal(err) }
	defer func() { _, _ = admin.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE") }()

	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil { t.Fatal(err) }
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil { t.Fatal(err) }
	defer pool.Close()

	source := fstest.MapFS{
		"001_first.sql": &fstest.MapFile{Data: []byte("CREATE TABLE migration_probe (id INTEGER PRIMARY KEY);")},
		"002_second.sql": &fstest.MapFile{Data: []byte("ALTER TABLE migration_probe ADD COLUMN label TEXT NOT NULL DEFAULT '';")},
	}
	if err = applyFS(ctx, pool, source); err != nil { t.Fatal(err) }
	if err = applyFS(ctx, pool, source); err != nil { t.Fatalf("second apply should be a no-op: %v", err) }
	var applied int
	if err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM schema_migrations").Scan(&applied); err != nil { t.Fatal(err) }
	if applied != 2 { t.Fatalf("got %d applied migrations, want 2", applied) }

	changed := fstest.MapFS{
		"001_first.sql": &fstest.MapFile{Data: []byte("CREATE TABLE migration_probe (id BIGINT PRIMARY KEY);")},
		"002_second.sql": &fstest.MapFile{Data: []byte("ALTER TABLE migration_probe ADD COLUMN label TEXT NOT NULL DEFAULT '';")},
	}
	if err = applyFS(ctx, pool, changed); err == nil { t.Fatal("expected checksum mismatch after editing an applied migration") }
}

func TestApplyFSRollsBackFailedMigration(t *testing.T) {
	dsn := os.Getenv("CLONE_TEST_DATABASE_URL")
	if dsn == "" { t.Skip("CLONE_TEST_DATABASE_URL is not configured") }
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	admin, err := pgxpool.New(ctx, dsn)
	if err != nil { t.Fatal(err) }
	defer admin.Close()
	schema := fmt.Sprintf("migration_failure_test_%d", os.Getpid())
	if _, err = admin.Exec(ctx, "CREATE SCHEMA "+schema); err != nil { t.Fatal(err) }
	defer func() { _, _ = admin.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE") }()
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil { t.Fatal(err) }
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil { t.Fatal(err) }
	defer pool.Close()

	source := fstest.MapFS{
		"001_first.sql": &fstest.MapFile{Data: []byte("CREATE TABLE migration_probe (id INTEGER PRIMARY KEY);")},
		"002_broken.sql": &fstest.MapFile{Data: []byte("THIS IS NOT VALID SQL;")},
	}
	if err = applyFS(ctx, pool, source); err == nil { t.Fatal("expected invalid migration to fail") }
	var applied int
	if err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM schema_migrations").Scan(&applied); err != nil { t.Fatal(err) }
	if applied != 1 { t.Fatalf("got %d recorded migrations, want only first migration", applied) }
	var exists bool
	if err = pool.QueryRow(ctx, "SELECT to_regclass($1) IS NOT NULL", schema+".migration_probe").Scan(&exists); err != nil { t.Fatal(err) }
	if !exists { t.Fatal("first migration should remain committed") }
}
