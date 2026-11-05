package clones

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"fmt"
	"strconv"

	"github.com/jackc/pgx/v5/pgxpool"
)

// VerifyBackupCredentials authenticates all saved clone ciphertexts using the
// existing stable key. It never returns or logs plaintext tokens.
func VerifyBackupCredentials(ctx context.Context, db *pgxpool.Pool, encodedKey string) (int, error) {
	if db == nil { return 0, fmt.Errorf("clone credential verification requires PostgreSQL") }
	key, err := parseEncryptionKey(encodedKey)
	if err != nil { return 0, fmt.Errorf("clone credential verification key is invalid") }
	block, err := aes.NewCipher(key)
	if err != nil { return 0, fmt.Errorf("clone credential verifier could not initialize cipher") }
	gcm, err := cipher.NewGCM(block)
	if err != nil { return 0, fmt.Errorf("clone credential verifier could not initialize cipher") }

	rows, err := db.Query(ctx, "SELECT bot_id, owner_id, encrypted_token, nonce FROM clone_credentials ORDER BY bot_id")
	if err != nil { return 0, fmt.Errorf("could not read saved clone credentials") }
	defer rows.Close()
	count := 0
	for rows.Next() {
		var botID, ownerID int64
		var ciphertext, nonce []byte
		if err := rows.Scan(&botID, &ownerID, &ciphertext, &nonce); err != nil {
			return count, fmt.Errorf("could not read a saved clone credential")
		}
		if botID <= 0 || ownerID <= 0 || len(nonce) != gcm.NonceSize() {
			return count, fmt.Errorf("saved clone credential %d has invalid metadata", botID)
		}
		plain, openErr := gcm.Open(nil, nonce, ciphertext, associatedData(botID, ownerID))
		if openErr != nil { return count, fmt.Errorf("saved clone credential %d failed authentication", botID) }
		separator := bytes.IndexByte(plain, ':')
		valid := separator > 0 && separator < len(plain)-1 &&
			bytes.Equal(plain[:separator], []byte(strconv.FormatInt(botID, 10)))
		for i := range plain { plain[i] = 0 }
		if !valid { return count, fmt.Errorf("saved clone credential %d has invalid token structure", botID) }
		count++
	}
	if err := rows.Err(); err != nil { return count, fmt.Errorf("could not finish reading saved clone credentials") }
	return count, nil
}
