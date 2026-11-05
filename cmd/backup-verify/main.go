package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ummidipradeep909-gif/AniToon_4Bot/internal/clones"
	"github.com/ummidipradeep909-gif/AniToon_4Bot/internal/secrets"
	"github.com/ummidipradeep909-gif/AniToon_4Bot/migrations"
)

type result struct {
	LatestMigrationVersion int64 `json:"latest_migration_version"`
	AppliedMigrationCount int `json:"applied_migration_count"`
	LedgerVerified bool `json:"ledger_verified"`
	CloneCredentials int `json:"clone_credentials"`
	CloneCredentialsVerified bool `json:"clone_credentials_verified"`
}

func run() error {
	dsn, err := secrets.Load("DATABASE_URL", true)
	if err != nil {
		return fmt.Errorf("database configuration is missing or invalid")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return fmt.Errorf("could not connect to restored database")
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		return fmt.Errorf("restored database is unavailable")
	}
	version, count, err := migrations.VerifyLedger(ctx, pool)
	if err != nil {
		return fmt.Errorf("restored migration ledger verification failed: %w", err)
	}
	key, err := secrets.Load("CLONE_ENCRYPTION_KEY", true)
	if err != nil {
		return fmt.Errorf("clone encryption key configuration is missing")
	}
	cloneCount, err := clones.VerifyBackupCredentials(ctx, pool, key)
	if err != nil {
		return fmt.Errorf("restored clone credentials verification failed: %w", err)
	}
	return json.NewEncoder(os.Stdout).Encode(result{
		LatestMigrationVersion: version,
		AppliedMigrationCount: count,
		LedgerVerified: true,
		CloneCredentials: cloneCount,
		CloneCredentialsVerified: true,
	})
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "backup verification failed:", err.Error())
		os.Exit(1)
	}
}
