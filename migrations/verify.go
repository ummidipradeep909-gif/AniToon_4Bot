package migrations

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// VerifyLedger compares the restored database's migration ledger with the SQL
// embedded in this exact application revision. It is read-only and never applies
// migrations. It rejects missing, extra, renamed, or checksum-mismatched entries.
func VerifyLedger(ctx context.Context, pool *pgxpool.Pool) (latestVersion int64, appliedCount int, err error) {
	if pool == nil {
		return 0, 0, fmt.Errorf("migration verification requires PostgreSQL")
	}
	expected, err := loadMigrations(migrationFiles)
	if err != nil {
		return 0, 0, fmt.Errorf("load embedded migration set: %w", err)
	}

	rows, err := pool.Query(ctx, "SELECT version, filename, checksum FROM schema_migrations ORDER BY version")
	if err != nil {
		return 0, 0, fmt.Errorf("read restored migration ledger: %w", err)
	}
	defer rows.Close()

	actual := make(map[int64]migration, len(expected))
	for rows.Next() {
		var version int64
		var filename, checksum string
		if err := rows.Scan(&version, &filename, &checksum); err != nil {
			return 0, 0, fmt.Errorf("read restored migration ledger row: %w", err)
		}
		if _, duplicate := actual[version]; duplicate {
			return 0, 0, fmt.Errorf("restored migration ledger contains duplicate version %d", version)
		}
		actual[version] = migration{version: version, name: filename, checksum: checksum}
	}
	if err := rows.Err(); err != nil {
		return 0, 0, fmt.Errorf("iterate restored migration ledger: %w", err)
	}

	for _, item := range expected {
		got, exists := actual[item.version]
		if !exists {
			return 0, len(actual), fmt.Errorf("restored database is missing migration %d (%s)", item.version, item.name)
		}
		if got.name != item.name || got.checksum != item.checksum {
			return 0, len(actual), fmt.Errorf("restored migration %d does not match this application revision", item.version)
		}
	}
	if len(actual) != len(expected) {
		return 0, len(actual), fmt.Errorf("restored migration ledger contains %d entries; expected exactly %d", len(actual), len(expected))
	}
	if len(expected) > 0 {
		latestVersion = expected[len(expected)-1].version
	}
	return latestVersion, len(actual), nil
}
