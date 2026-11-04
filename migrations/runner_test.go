package migrations

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"
	"testing/fstest"
)

func TestLoadMigrationsSortsAndChecksums(t *testing.T) {
	source := fstest.MapFS{
		"002_second.sql": &fstest.MapFile{Data: []byte("SELECT 2;")},
		"001_first.sql": &fstest.MapFile{Data: []byte("SELECT 1;")},
	}
	got, err := loadMigrations(source)
	if err != nil { t.Fatal(err) }
	if len(got) != 2 || got[0].version != 1 || got[1].version != 2 {
		t.Fatalf("unexpected migration order: %#v", got)
	}
	sum := sha256.Sum256([]byte("SELECT 1;"))
	if got[0].checksum != hex.EncodeToString(sum[:]) {
		t.Fatalf("unexpected checksum: %s", got[0].checksum)
	}
}

func TestLoadMigrationsRejectsInvalidNames(t *testing.T) {
	for _, name := range []string{"bad.sql", "000_zero.sql"} {
		_, err := loadMigrations(fstest.MapFS{name: &fstest.MapFile{Data: []byte("SELECT 1;")}})
		if err == nil { t.Errorf("expected %q to be rejected", name) }
	}
}

func TestLoadMigrationsRejectsDuplicateVersions(t *testing.T) {
	source := fstest.MapFS{
		"001_first.sql": &fstest.MapFile{Data: []byte("SELECT 1;")},
		"001_second.sql": &fstest.MapFile{Data: []byte("SELECT 2;")},
	}
	if _, err := loadMigrations(source); err == nil {
		t.Fatal("expected duplicate version error")
	}
}
