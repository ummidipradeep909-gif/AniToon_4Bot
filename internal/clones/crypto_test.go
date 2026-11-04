package clones

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"os"
	"strings"
	"testing"
)

func testKey() []byte { key := make([]byte, 32); if _, err := rand.Read(key); err != nil { panic(err) }; return key }

func TestEncryptDecryptRoundTrip(t *testing.T) {
	key := testKey()
	ciphertext, nonce, err := encryptToken(key, "123456:secret-token", 1001, 2002)
	if err != nil { t.Fatal(err) }
	got, err := decryptToken(key, ciphertext, nonce, 1001, 2002)
	if err != nil { t.Fatal(err) }
	if got != "123456:secret-token" { t.Fatalf("unexpected plaintext %q", got) }
	if bytes.Contains(ciphertext, []byte("secret-token")) { t.Fatal("ciphertext contains plaintext") }
}

func TestEncryptionUsesUniqueNonces(t *testing.T) {
	key := testKey()
	_, n1, err := encryptToken(key, "token", 1, 2); if err != nil { t.Fatal(err) }
	_, n2, err := encryptToken(key, "token", 1, 2); if err != nil { t.Fatal(err) }
	if bytes.Equal(n1, n2) { t.Fatal("nonce was reused") }
}

func TestParseEncryptionKeyRequiresExactly32Bytes(t *testing.T) {
	for _, size := range []int{0, 16, 31, 33, 64} {
		_, err := parseEncryptionKey(base64.StdEncoding.EncodeToString(make([]byte, size)))
		if err == nil { t.Fatalf("expected %d-byte key to be rejected", size) }
	}
	if _, err := parseEncryptionKey("not-base64"); err == nil { t.Fatal("expected malformed base64 to fail") }
	if _, err := parseEncryptionKey(base64.StdEncoding.EncodeToString(make([]byte, 32))); err != nil { t.Fatal(err) }
	if _, _, err := encryptToken(make([]byte, 16), "token", 1, 2); err == nil { t.Fatal("invalid AES key accepted") }
}

func TestWrongKeysTamperingAndNonceAreRejected(t *testing.T) {
	key := testKey()
	ciphertext, nonce, err := encryptToken(key, "token-value", 10, 20); if err != nil { t.Fatal(err) }
	if _, err := decryptToken(testKey(), ciphertext, nonce, 10, 20); err == nil { t.Fatal("wrong key accepted") }
	tampered := append([]byte(nil), ciphertext...); tampered[0] ^= 0xff
	if _, err := decryptToken(key, tampered, nonce, 10, 20); err == nil { t.Fatal("tampered ciphertext accepted") }
	if _, err := decryptToken(key, ciphertext, nonce[:len(nonce)-1], 10, 20); err == nil { t.Fatal("invalid nonce accepted") }
}

func TestAssociatedDataAuthenticatesBotAndOwner(t *testing.T) {
	key := testKey()
	ciphertext, nonce, err := encryptToken(key, "token-value", 10, 20); if err != nil { t.Fatal(err) }
	for _, identity := range [][2]int64{{11,20},{10,21}} {
		if _, err := decryptToken(key, ciphertext, nonce, identity[0], identity[1]); err == nil {
			t.Fatalf("accepted mismatched identity bot=%d owner=%d", identity[0], identity[1])
		}
	}
}

func TestCloneCredentialMigrationEnforcesBothUniqueIdentities(t *testing.T) {
	migration, err := os.ReadFile("../../migrations/010_clone_credentials.sql")
	if err != nil { t.Fatal(err) }
	sql := string(migration)
	for _, fragment := range []string{"bot_id BIGINT PRIMARY KEY", "owner_id BIGINT NOT NULL UNIQUE", "encrypted_token BYTEA NOT NULL", "nonce BYTEA NOT NULL"} {
		if !strings.Contains(sql, fragment) { t.Fatalf("migration missing constraint/column %q", fragment) }
	}
}
