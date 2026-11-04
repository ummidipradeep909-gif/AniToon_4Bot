package clones

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

var errInvalidEncryptionKey = errors.New("CLONE_ENCRYPTION_KEY must be base64 encoding of exactly 32 bytes")

func parseEncryptionKey(encoded string) ([]byte, error) {
	key, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil || len(key) != 32 { return nil, errInvalidEncryptionKey }
	return key, nil
}

func associatedData(botID, ownerID int64) []byte {
	const prefix = "AniToon_4Bot:clone:v1"
	data := make([]byte, len(prefix)+16)
	copy(data, prefix)
	binary.BigEndian.PutUint64(data[len(prefix):len(prefix)+8], uint64(botID))
	binary.BigEndian.PutUint64(data[len(prefix)+8:], uint64(ownerID))
	return data
}

func encryptToken(key []byte, token string, botID, ownerID int64) (ciphertext, nonce []byte, err error) {
	if len(key) != 32 { return nil, nil, errInvalidEncryptionKey }
	block, err := aes.NewCipher(key)
	if err != nil { return nil, nil, err }
	gcm, err := cipher.NewGCM(block)
	if err != nil { return nil, nil, err }
	nonce = make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil { return nil, nil, err }
	ciphertext = gcm.Seal(nil, nonce, []byte(token), associatedData(botID, ownerID))
	return ciphertext, nonce, nil
}

func decryptToken(key, ciphertext, nonce []byte, botID, ownerID int64) (string, error) {
	if len(key) != 32 { return "", errInvalidEncryptionKey }
	block, err := aes.NewCipher(key)
	if err != nil { return "", err }
	gcm, err := cipher.NewGCM(block)
	if err != nil { return "", err }
	if len(nonce) != gcm.NonceSize() { return "", fmt.Errorf("invalid clone credential nonce") }
	plain, err := gcm.Open(nil, nonce, ciphertext, associatedData(botID, ownerID))
	if err != nil { return "", fmt.Errorf("clone credential authentication failed") }
	return string(plain), nil
}
