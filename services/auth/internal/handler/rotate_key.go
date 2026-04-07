// services/auth/internal/handler/rotate_key.go
package handler

import (
	"log"
	"net/http"
	"strings"

	"github.com/devinat1/obsidian-meeting-notes/services/shared/httputil"
	"github.com/jackc/pgx/v5/pgxpool"
)

type RotateKeyHandler struct {
	db *pgxpool.Pool
}

type RotateKeyHandlerParams struct {
	DB *pgxpool.Pool
}

func NewRotateKeyHandler(params RotateKeyHandlerParams) *RotateKeyHandler {
	return &RotateKeyHandler{db: params.DB}
}

func (h *RotateKeyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	header := r.Header.Get("Authorization")
	if !strings.HasPrefix(header, "Bearer ") {
		httputil.RespondError(w, http.StatusUnauthorized, "Missing or invalid Authorization header.")
		return
	}

	currentKey := strings.TrimPrefix(header, "Bearer ")
	currentHash := hashString(currentKey)
	ctx := r.Context()

	var userID int
	err := h.db.QueryRow(ctx,
		"SELECT id FROM users WHERE api_key_hash = $1 AND verified = TRUE",
		currentHash,
	).Scan(&userID)

	if err != nil {
		httputil.RespondError(w, http.StatusUnauthorized, "Invalid API key.")
		return
	}

	newKey, err := generateAPIKey()
	if err != nil {
		log.Printf("Error generating new API key: %v", err)
		httputil.RespondError(w, http.StatusInternalServerError, "Failed to generate new API key.")
		return
	}

	newHash := hashString(newKey)

	_, err = h.db.Exec(ctx,
		"UPDATE users SET api_key_hash = $1, updated_at = NOW() WHERE id = $2",
		newHash, userID,
	)
	if err != nil {
		log.Printf("Error rotating API key: %v", err)
		httputil.RespondError(w, http.StatusInternalServerError, "Failed to rotate API key.")
		return
	}

	httputil.RespondJSON(w, http.StatusOK, map[string]string{
		"api_key": newKey,
	})
}
