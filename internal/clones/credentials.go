package clones

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

type credential struct {
	BotID int64
	OwnerID int64
	Ciphertext []byte
	Nonce []byte
}

type credentialStore struct { db *pgxpool.Pool }

func (s credentialStore) insert(ctx context.Context, row credential) error {
	if s.db == nil { return fmt.Errorf("clone credential database unavailable") }
	_, err := s.db.Exec(ctx, `INSERT INTO clone_credentials (bot_id, owner_id, encrypted_token, nonce) VALUES ($1,$2,$3,$4)`,
		row.BotID, row.OwnerID, row.Ciphertext, row.Nonce)
	if err != nil { return fmt.Errorf("could not persist clone credentials") }
	return nil
}

func (s credentialStore) delete(ctx context.Context, botID int64) error {
	if s.db == nil { return fmt.Errorf("clone credential database unavailable") }
	_, err := s.db.Exec(ctx, `DELETE FROM clone_credentials WHERE bot_id=$1`, botID)
	if err != nil { return fmt.Errorf("could not remove clone credentials") }
	return nil
}

func (s credentialStore) list(ctx context.Context) ([]credential, error) {
	if s.db == nil { return nil, fmt.Errorf("clone credential database unavailable") }
	rows, err := s.db.Query(ctx, `SELECT bot_id, owner_id, encrypted_token, nonce FROM clone_credentials ORDER BY created_at`)
	if err != nil { return nil, fmt.Errorf("could not load clone credentials") }
	defer rows.Close()
	var result []credential
	for rows.Next() {
		var row credential
		if err := rows.Scan(&row.BotID, &row.OwnerID, &row.Ciphertext, &row.Nonce); err != nil {
			return nil, fmt.Errorf("could not read clone credentials")
		}
		result = append(result, row)
	}
	if err := rows.Err(); err != nil { return nil, fmt.Errorf("could not read clone credentials") }
	return result, nil
}
