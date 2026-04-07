// services/auth/internal/handler/validate.go
package handler

import (
	"net/http"
	"strings"

	"github.com/devinat1/obsidian-meeting-notes/services/shared/httputil"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ValidateHandler struct {
	db *pgxpool.Pool
}

type ValidateHandlerParams struct {
	DB *pgxpool.Pool
}

func NewValidateHandler(params ValidateHandlerParams) *ValidateHandler {
	return &ValidateHandler{db: params.DB}
}

func (h *ValidateHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	header := r.Header.Get("Authorization")
	if !strings.HasPrefix(header, "Bearer ") {
		httputil.RespondError(w, http.StatusUnauthorized, "Missing or invalid Authorization header.")
		return
	}

	apiKey := strings.TrimPrefix(header, "Bearer ")
	apiKeyHash := hashString(apiKey)
	ctx := r.Context()

	var userID int
	err := h.db.QueryRow(ctx,
		"SELECT id FROM users WHERE api_key_hash = $1 AND verified = TRUE",
		apiKeyHash,
	).Scan(&userID)

	if err != nil {
		httputil.RespondError(w, http.StatusUnauthorized, "Invalid API key.")
		return
	}

	httputil.RespondJSON(w, http.StatusOK, map[string]int{
		"user_id": userID,
	})
}
