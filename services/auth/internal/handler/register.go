// services/auth/internal/handler/register.go
package handler

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"time"

	"github.com/devinat1/obsidian-meeting-notes/services/auth/internal/email"
	"github.com/devinat1/obsidian-meeting-notes/services/shared/httputil"
	"github.com/jackc/pgx/v5/pgxpool"
)

var emailRegexp = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

type RegisterHandler struct {
	db          *pgxpool.Pool
	emailSender *email.Sender
	baseURL     string
}

type RegisterHandlerParams struct {
	DB          *pgxpool.Pool
	EmailSender *email.Sender
	BaseURL     string
}

func NewRegisterHandler(params RegisterHandlerParams) *RegisterHandler {
	return &RegisterHandler{
		db:          params.DB,
		emailSender: params.EmailSender,
		baseURL:     params.BaseURL,
	}
}

func (h *RegisterHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	type request struct {
		Email string `json:"email"`
	}
	var req request
	if err := httputil.ParseJSON(r, &req); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "Invalid request body.")
		return
	}

	if req.Email == "" {
		httputil.RespondError(w, http.StatusBadRequest, "Email is required.")
		return
	}

	if !emailRegexp.MatchString(req.Email) {
		httputil.RespondError(w, http.StatusBadRequest, "Invalid email format.")
		return
	}

	ctx := r.Context()

	var existingUserID int
	var existingVerified bool
	err := h.db.QueryRow(ctx,
		"SELECT id, verified FROM users WHERE email = $1",
		req.Email,
	).Scan(&existingUserID, &existingVerified)

	if err == nil && existingVerified {
		httputil.RespondError(w, http.StatusConflict, "Email already registered and verified.")
		return
	}

	var userID int
	if err == nil {
		userID = existingUserID
		h.db.Exec(ctx, "DELETE FROM verification_tokens WHERE user_id = $1", userID)
	} else {
		err = h.db.QueryRow(ctx,
			"INSERT INTO users (email) VALUES ($1) ON CONFLICT (email) DO UPDATE SET updated_at = NOW() RETURNING id",
			req.Email,
		).Scan(&userID)
		if err != nil {
			log.Printf("Error inserting user: %v", err)
			httputil.RespondError(w, http.StatusInternalServerError, "Failed to create user.")
			return
		}
	}

	token, err := generateToken()
	if err != nil {
		log.Printf("Error generating verification token: %v", err)
		httputil.RespondError(w, http.StatusInternalServerError, "Failed to generate verification token.")
		return
	}

	tokenHash := hashString(token)
	expiresAt := time.Now().Add(24 * time.Hour)

	_, err = h.db.Exec(ctx,
		"INSERT INTO verification_tokens (user_id, token_hash, expires_at) VALUES ($1, $2, $3)",
		userID, tokenHash, expiresAt,
	)
	if err != nil {
		log.Printf("Error inserting verification token: %v", err)
		httputil.RespondError(w, http.StatusInternalServerError, "Failed to store verification token.")
		return
	}

	go func() {
		if err := h.emailSender.SendVerificationEmail(email.SendVerificationEmailParams{
			To:      req.Email,
			Token:   token,
			BaseURL: h.baseURL,
		}); err != nil {
			log.Printf("Error sending verification email to %s: %v", req.Email, err)
		}
	}()

	httputil.RespondJSON(w, http.StatusCreated, map[string]string{
		"message": "Verification email sent. Check your inbox.",
	})
}

func generateToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("generating random bytes: %w", err)
	}
	return hex.EncodeToString(bytes), nil
}

func generateAPIKey() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("generating random bytes: %w", err)
	}
	return hex.EncodeToString(bytes), nil
}

func hashString(s string) string {
	hash := sha256.Sum256([]byte(s))
	return hex.EncodeToString(hash[:])
}

func contextWithTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, 5*time.Second)
}
