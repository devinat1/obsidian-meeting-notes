// services/auth/internal/handler/verify.go
package handler

import (
	"log"
	"net/http"
	"time"

	"github.com/devinat1/obsidian-meeting-notes/services/shared/httputil"
	"github.com/jackc/pgx/v5/pgxpool"
)

type VerifyHandler struct {
	db *pgxpool.Pool
}

type VerifyHandlerParams struct {
	DB *pgxpool.Pool
}

func NewVerifyHandler(params VerifyHandlerParams) *VerifyHandler {
	return &VerifyHandler{db: params.DB}
}

func (h *VerifyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		httputil.RespondError(w, http.StatusBadRequest, "Token is required.")
		return
	}

	tokenHash := hashString(token)
	ctx := r.Context()

	var userID int
	var expiresAt time.Time
	err := h.db.QueryRow(ctx,
		"SELECT user_id, expires_at FROM verification_tokens WHERE token_hash = $1",
		tokenHash,
	).Scan(&userID, &expiresAt)

	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "Invalid or expired token.")
		return
	}

	if time.Now().After(expiresAt) {
		h.db.Exec(ctx, "DELETE FROM verification_tokens WHERE token_hash = $1", tokenHash)
		httputil.RespondError(w, http.StatusBadRequest, "Token has expired. Please register again.")
		return
	}

	apiKey, err := generateAPIKey()
	if err != nil {
		log.Printf("Error generating API key: %v", err)
		httputil.RespondError(w, http.StatusInternalServerError, "Failed to generate API key.")
		return
	}

	apiKeyHash := hashString(apiKey)

	_, err = h.db.Exec(ctx,
		"UPDATE users SET api_key_hash = $1, verified = TRUE, updated_at = NOW() WHERE id = $2",
		apiKeyHash, userID,
	)
	if err != nil {
		log.Printf("Error updating user verification: %v", err)
		httputil.RespondError(w, http.StatusInternalServerError, "Failed to verify user.")
		return
	}

	h.db.Exec(ctx, "DELETE FROM verification_tokens WHERE user_id = $1", userID)

	http.Redirect(w, r, "/static/verify.html#key="+apiKey, http.StatusFound)
}
