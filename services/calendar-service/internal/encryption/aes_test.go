// services/calendar-service/internal/encryption/aes_test.go
package encryption

import (
	"crypto/rand"
	"testing"
)

func testKey(t *testing.T) []byte {
	t.Helper()
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatalf("Failed to generate test key: %v.", err)
	}
	return key
}

func TestEncryptDecrypt_Roundtrip(t *testing.T) {
	enc, err := NewEncryptor(testKey(t))
	if err != nil {
		t.Fatalf("Failed to create encryptor: %v.", err)
	}

	original := "ya29.a0AfH6SMBx-some-refresh-token-value"
	encrypted, err := enc.Encrypt(original)
	if err != nil {
		t.Fatalf("Encrypt failed: %v.", err)
	}

	if encrypted == original {
		t.Error("Encrypted value should differ from original.")
	}

	decrypted, err := enc.Decrypt(encrypted)
	if err != nil {
		t.Fatalf("Decrypt failed: %v.", err)
	}

	if decrypted != original {
		t.Errorf("Roundtrip failed: got %q, want %q.", decrypted, original)
	}
}

func TestEncrypt_ProducesDifferentCiphertexts(t *testing.T) {
	enc, err := NewEncryptor(testKey(t))
	if err != nil {
		t.Fatalf("Failed to create encryptor: %v.", err)
	}

	plaintext := "same-token-value"
	enc1, err := enc.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("First encrypt failed: %v.", err)
	}
	enc2, err := enc.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Second encrypt failed: %v.", err)
	}

	if enc1 == enc2 {
		t.Error("Two encryptions of same plaintext should produce different ciphertexts due to random nonce.")
	}
}

func TestDecrypt_WrongKey(t *testing.T) {
	enc1, err := NewEncryptor(testKey(t))
	if err != nil {
		t.Fatalf("Failed to create encryptor 1: %v.", err)
	}
	enc2, err := NewEncryptor(testKey(t))
	if err != nil {
		t.Fatalf("Failed to create encryptor 2: %v.", err)
	}

	encrypted, err := enc1.Encrypt("secret-token")
	if err != nil {
		t.Fatalf("Encrypt failed: %v.", err)
	}

	_, err = enc2.Decrypt(encrypted)
	if err == nil {
		t.Error("Decrypt with wrong key should fail.")
	}
}

func TestNewEncryptor_InvalidKeyLength(t *testing.T) {
	_, err := NewEncryptor([]byte("too-short"))
	if err == nil {
		t.Error("Expected error for invalid key length.")
	}
}

func TestDecrypt_InvalidBase64(t *testing.T) {
	enc, err := NewEncryptor(testKey(t))
	if err != nil {
		t.Fatalf("Failed to create encryptor: %v.", err)
	}

	_, err = enc.Decrypt("not-valid-base64!!!")
	if err == nil {
		t.Error("Expected error for invalid base64.")
	}
}
