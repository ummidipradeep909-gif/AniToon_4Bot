package clones

import (
	"context"
	"fmt"
	"errors"

	"github.com/jackc/pgx/v5/pgxpool"
)

var errCloneQuotaReached = errors.New("clone quota reached")

type credential struct {
	BotID int64
	OwnerID int64
	Ciphertext []byte
	Nonce []byte
	Enabled bool
}

type credentialStore struct { db *pgxpool.Pool }

func (s credentialStore) insert(ctx context.Context, row credential, maxClones int) error {
	if s.db == nil { return fmt.Errorf("clone credential database unavailable") }
	tx, err := s.db.Begin(ctx)
	if err != nil { return fmt.Errorf("could not begin clone credential transaction") }
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(704202611)`); err != nil { return fmt.Errorf("could not lock clone quota") }
	var count int
	if err = tx.QueryRow(ctx, `SELECT COUNT(*) FROM clone_credentials`).Scan(&count); err != nil { return fmt.Errorf("could not check clone quota") }
	if maxClones > 0 && count >= maxClones { return errCloneQuotaReached }
	_, err = tx.Exec(ctx, `INSERT INTO clone_credentials (bot_id, owner_id, encrypted_token, nonce, enabled) VALUES ($1,$2,$3,$4,TRUE)`,
		row.BotID, row.OwnerID, row.Ciphertext, row.Nonce)
	if err != nil { return fmt.Errorf("could not persist clone credentials") }
	if err = tx.Commit(ctx); err != nil { return fmt.Errorf("could not commit clone credentials") }
	return nil
}

func (s credentialStore) delete(ctx context.Context, botID int64) error {
	if s.db == nil { return fmt.Errorf("clone credential database unavailable") }
	_, err := s.db.Exec(ctx, `DELETE FROM clone_credentials WHERE bot_id=$1`, botID)
	if err != nil { return fmt.Errorf("could not remove clone credentials") }
	return nil
}

func (s credentialStore) getByOwner(ctx context.Context, ownerID int64) (credential, error) {
	if s.db == nil { return credential{}, fmt.Errorf("clone credential database unavailable") }
	var row credential
	err := s.db.QueryRow(ctx, `SELECT bot_id, owner_id, encrypted_token, nonce, enabled FROM clone_credentials WHERE owner_id=$1`, ownerID).Scan(&row.BotID,&row.OwnerID,&row.Ciphertext,&row.Nonce,&row.Enabled)
	if err != nil { return credential{}, fmt.Errorf("could not load clone credentials") }
	return row,nil
}

func (s credentialStore) setEnabled(ctx context.Context, botID, ownerID int64, enabled bool) error {
	if s.db == nil { return fmt.Errorf("clone credential database unavailable") }
	tag,err:=s.db.Exec(ctx,`UPDATE clone_credentials SET enabled=$3 WHERE bot_id=$1 AND owner_id=$2`,botID,ownerID,enabled)
	if err!=nil || tag.RowsAffected()!=1 { return fmt.Errorf("could not update clone state") }
	return nil
}

func (s credentialStore) list(ctx context.Context) ([]credential, error) {
	if s.db == nil { return nil, fmt.Errorf("clone credential database unavailable") }
	rows, err := s.db.Query(ctx, `SELECT bot_id, owner_id, encrypted_token, nonce, enabled FROM clone_credentials ORDER BY created_at`)
	if err != nil { return nil, fmt.Errorf("could not load clone credentials") }
	defer rows.Close()
	var result []credential
	for rows.Next() {
		var row credential
		if err := rows.Scan(&row.BotID, &row.OwnerID, &row.Ciphertext, &row.Nonce, &row.Enabled); err != nil {
			return nil, fmt.Errorf("could not read clone credentials")
		}
		result = append(result, row)
	}
	if err := rows.Err(); err != nil { return nil, fmt.Errorf("could not read clone credentials") }
	return result, nil
}
