package verify

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

const (
	toleranceSeconds = 300
	secretPrefix     = "whsec_"
)

type SvixHeaders struct {
	ID        string
	Timestamp string
	Signature string
}

type VerifySvixSignatureParams struct {
	Headers SvixHeaders
	Body    []byte
	Secret  string
}

func VerifySvixSignature(params VerifySvixSignatureParams) error {
	if params.Headers.ID == "" || params.Headers.Timestamp == "" || params.Headers.Signature == "" {
		return fmt.Errorf("missing required Svix headers")
	}

	timestampInt, err := strconv.ParseInt(params.Headers.Timestamp, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid svix-timestamp: %w", err)
	}

	now := time.Now().Unix()
	if math.Abs(float64(now-timestampInt)) > toleranceSeconds {
		return fmt.Errorf("timestamp outside tolerance window")
	}

	secretBytes, err := decodeSecret(params.Secret)
	if err != nil {
		return fmt.Errorf("failed to decode secret: %w", err)
	}

	signedContent := fmt.Sprintf("%s.%s.%s", params.Headers.ID, params.Headers.Timestamp, string(params.Body))

	mac := hmac.New(sha256.New, secretBytes)
	mac.Write([]byte(signedContent))
	expectedSignature := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	expectedWithPrefix := "v1," + expectedSignature

	signatures := strings.Split(params.Headers.Signature, " ")
	for _, sig := range signatures {
		if hmac.Equal([]byte(strings.TrimSpace(sig)), []byte(expectedWithPrefix)) {
			return nil
		}
	}

	return fmt.Errorf("no matching signature found")
}

func decodeSecret(secret string) ([]byte, error) {
	trimmed := strings.TrimPrefix(secret, secretPrefix)
	return base64.StdEncoding.DecodeString(trimmed)
}
