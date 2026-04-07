// services/calendar-service/internal/store/connection_test.go
package store

import (
	"testing"

	"github.com/devinat1/obsidian-meeting-notes/services/calendar-service/internal/encryption"
)

func TestConnectionStore_EncryptionRoundtrip(t *testing.T) {
	// This test validates the encrypt/decrypt path without a database.
	// Full integration tests require a running Postgres instance.
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}

	enc, err := encryption.NewEncryptor(key)
	if err != nil {
		t.Fatalf("Failed to create encryptor: %v.", err)
	}

	token := "1//0e-refresh-token-from-google"
	encrypted, err := enc.Encrypt(token)
	if err != nil {
		t.Fatalf("Encrypt failed: %v.", err)
	}

	decrypted, err := enc.Decrypt(encrypted)
	if err != nil {
		t.Fatalf("Decrypt failed: %v.", err)
	}

	if decrypted != token {
		t.Errorf("Roundtrip failed: got %q, want %q.", decrypted, token)
	}
}
