// services/gateway/internal/middleware/auth.go
package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/devinat1/obsidian-meeting-notes/services/gateway/internal/authclient"
	"github.com/devinat1/obsidian-meeting-notes/services/shared/httputil"
)

type contextKey string

const userIDKey contextKey = "userID"

func Auth(client *authclient.Client) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := r.Header.Get("Authorization")
			if !strings.HasPrefix(header, "Bearer ") {
				httputil.RespondError(w, http.StatusUnauthorized, "Missing or invalid Authorization header.")
				return
			}

			apiKey := strings.TrimPrefix(header, "Bearer ")
			userID, err := client.Validate(apiKey)
			if err != nil {
				httputil.RespondError(w, http.StatusUnauthorized, "Invalid API key.")
				return
			}

			ctx := context.WithValue(r.Context(), userIDKey, userID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func UserIDFromContext(ctx context.Context) int {
	userID, ok := ctx.Value(userIDKey).(int)
	if !ok {
		return 0
	}
	return userID
}
