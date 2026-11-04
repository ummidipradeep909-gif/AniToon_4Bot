package migrations

import (
	"context"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"fmt"
	"io/fs"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed *.sql
var migrationFiles embed.FS

const advisoryLockID int64 = 721904260411

var migrationName = regexp.MustCompile(`^(\d{3,})_([a-zA-Z0-9_-]+)\.sql$`)

type migration struct {
	version int64
	name    string
	sql     []byte
	checksum string
}

// Apply runs embedded SQL migrations in numeric order. Each migration and its
// history record commit atomically. A session advisory lock serializes runners
// across processes, not merely goroutines in this process.
func Apply(ctx context.Context, pool *pgxpool.Pool) error {
	if pool == nil {
		return fmt.Errorf("migration runner requires a PostgreSQL connection pool")
	}
	return applyFS(ctx, pool, migrationFiles)
}

func applyFS(ctx context.Context, pool *pgxpool.Pool, source fs.FS) error {
	files, err := loadMigrations(source)
	if err != nil {
		return err
	}
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("acquire PostgreSQL connection for migrations: %w", err)
	}
	defer conn.Release()

	if _, err = conn.Exec(ctx, "SELECT pg_advisory_lock($1)", advisoryLockID); err != nil {
		return fmt.Errorf("lock PostgreSQL migrations: %w", err)
	}
	defer func() {
		unlockCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if _, unlockErr := conn.Exec(unlockCtx, "SELECT pg_advisory_unlock($1)", advisoryLockID); unlockErr != nil {
			// Releasing the pooled connection closes the session if it cannot be unlocked.
			conn.Hijack().Close(unlockCtx)
		}
	}()

	tx, err := conn.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin migration metadata transaction: %w", err)
	}
	_, err = tx.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		version BIGINT PRIMARY KEY,
		filename TEXT NOT NULL,
		checksum TEXT NOT NULL,
		applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`)
	if err != nil {
		_ = tx.Rollback(ctx)
		return fmt.Errorf("create migration history table: %w", err)
	}
	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit migration history table: %w", err)
	}

	for _, item := range files {
		if err = applyOne(ctx, conn, item); err != nil {
			return err
		}
	}
	return nil
}

func applyOne(ctx context.Context, conn *pgxpool.Conn, item migration) error {
	tx, err := conn.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin migration %s: %w", item.name, err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	var existingName, existingChecksum string
	err = tx.QueryRow(ctx, "SELECT filename, checksum FROM schema_migrations WHERE version=$1", item.version).
		Scan(&existingName, &existingChecksum)
	switch {
	case err == nil:
		if existingName != item.name || existingChecksum != item.checksum {
			return fmt.Errorf("migration %d checksum/name mismatch: database has %s (%s), source has %s (%s); restore the original migration or create a new migration", item.version, existingName, existingChecksum, item.name, item.checksum)
		}
		return tx.Commit(ctx)
	case err != pgx.ErrNoRows:
		return fmt.Errorf("read migration history for %s: %w", item.name, err)
	}

	if _, err = tx.Exec(ctx, string(item.sql), pgx.QueryExecModeSimpleProtocol); err != nil {
		return fmt.Errorf("apply migration %s (transaction rolled back): %w", item.name, err)
	}
	if _, err = tx.Exec(ctx, "INSERT INTO schema_migrations(version, filename, checksum) VALUES($1,$2,$3)", item.version, item.name, item.checksum); err != nil {
		return fmt.Errorf("record migration %s (transaction rolled back): %w", item.name, err)
	}
	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit migration %s: %w", item.name, err)
	}
	return nil
}

func loadMigrations(source fs.FS) ([]migration, error) {
	entries, err := fs.ReadDir(source, ".")
	if err != nil {
		return nil, fmt.Errorf("read embedded migrations: %w", err)
	}
	var result []migration
	seen := make(map[int64]struct{})
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		match := migrationName.FindStringSubmatch(entry.Name())
		if match == nil {
			return nil, fmt.Errorf("invalid migration filename %q: expected NNN_description.sql", entry.Name())
		}
		version, parseErr := strconv.ParseInt(match[1], 10, 64)
		if parseErr != nil || version < 1 {
			return nil, fmt.Errorf("invalid migration version in %q", entry.Name())
		}
		if _, exists := seen[version]; exists {
			return nil, fmt.Errorf("duplicate migration version %d", version)
		}
		seen[version] = struct{}{}
		body, readErr := fs.ReadFile(source, entry.Name())
		if readErr != nil {
			return nil, fmt.Errorf("read migration %s: %w", entry.Name(), readErr)
		}
		sum := sha256.Sum256(body)
		result = append(result, migration{version: version, name: entry.Name(), sql: body, checksum: hex.EncodeToString(sum[:])})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].version < result[j].version })
	return result, nil
}
