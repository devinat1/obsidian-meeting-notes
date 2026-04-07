package verify

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"strconv"
	"testing"
	"time"
)

type generateTestSignatureParams struct {
	SecretRaw string
	ID        string
	Timestamp string
	Body      []byte
}

func generateTestSignature(params generateTestSignatureParams) string {
	secretBytes, _ := base64.StdEncoding.DecodeString(params.SecretRaw)
	signedContent := fmt.Sprintf("%s.%s.%s", params.ID, params.Timestamp, string(params.Body))
	mac := hmac.New(sha256.New, secretBytes)
	mac.Write([]byte(signedContent))
	return "v1," + base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

func TestVerifySvixSignature_ValidSignature(t *testing.T) {
	secretRaw := base64.StdEncoding.EncodeToString([]byte("test-secret-key-1234"))
	secret := "whsec_" + secretRaw
	body := []byte(`{"event":"bot.status_change","data":{"bot_id":"abc123"}}`)
	msgID := "msg_abc123"
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	signature := generateTestSignature(generateTestSignatureParams{
		SecretRaw: secretRaw,
		ID:        msgID,
		Timestamp: timestamp,
		Body:      body,
	})

	err := VerifySvixSignature(VerifySvixSignatureParams{
		Headers: SvixHeaders{
			ID:        msgID,
			Timestamp: timestamp,
			Signature: signature,
		},
		Body:   body,
		Secret: secret,
	})
	if err != nil {
		t.Fatalf("expected valid signature, got error: %v", err)
	}
}

func TestVerifySvixSignature_InvalidSignature(t *testing.T) {
	secret := "whsec_" + base64.StdEncoding.EncodeToString([]byte("test-secret-key-1234"))
	body := []byte(`{"event":"bot.status_change"}`)
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)

	err := VerifySvixSignature(VerifySvixSignatureParams{
		Headers: SvixHeaders{
			ID:        "msg_abc123",
			Timestamp: timestamp,
			Signature: "v1,invalidsignaturehere",
		},
		Body:   body,
		Secret: secret,
	})
	if err == nil {
		t.Fatal("expected error for invalid signature, got nil")
	}
}

func TestVerifySvixSignature_ExpiredTimestamp(t *testing.T) {
	secretRaw := base64.StdEncoding.EncodeToString([]byte("test-secret-key-1234"))
	secret := "whsec_" + secretRaw
	body := []byte(`{"event":"bot.status_change"}`)
	msgID := "msg_abc123"
	expiredTimestamp := strconv.FormatInt(time.Now().Add(-10*time.Minute).Unix(), 10)
	signature := generateTestSignature(generateTestSignatureParams{
		SecretRaw: secretRaw,
		ID:        msgID,
		Timestamp: expiredTimestamp,
		Body:      body,
	})

	err := VerifySvixSignature(VerifySvixSignatureParams{
		Headers: SvixHeaders{
			ID:        msgID,
			Timestamp: expiredTimestamp,
			Signature: signature,
		},
		Body:   body,
		Secret: secret,
	})
	if err == nil {
		t.Fatal("expected error for expired timestamp, got nil")
	}
}

func TestVerifySvixSignature_MissingHeaders(t *testing.T) {
	err := VerifySvixSignature(VerifySvixSignatureParams{
		Headers: SvixHeaders{},
		Body:    []byte("test"),
		Secret:  "whsec_dGVzdA==",
	})
	if err == nil {
		t.Fatal("expected error for missing headers, got nil")
	}
}

func TestVerifySvixSignature_MultipleSignatures(t *testing.T) {
	secretRaw := base64.StdEncoding.EncodeToString([]byte("test-secret-key-1234"))
	secret := "whsec_" + secretRaw
	body := []byte(`{"event":"bot.status_change"}`)
	msgID := "msg_abc123"
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	validSig := generateTestSignature(generateTestSignatureParams{
		SecretRaw: secretRaw,
		ID:        msgID,
		Timestamp: timestamp,
		Body:      body,
	})
	multiSig := "v1,oldsignature " + validSig

	err := VerifySvixSignature(VerifySvixSignatureParams{
		Headers: SvixHeaders{
			ID:        msgID,
			Timestamp: timestamp,
			Signature: multiSig,
		},
		Body:   body,
		Secret: secret,
	})
	if err != nil {
		t.Fatalf("expected valid with multiple signatures, got error: %v", err)
	}
}
