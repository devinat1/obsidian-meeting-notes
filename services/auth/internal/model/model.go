// services/auth/internal/model/model.go
package model

import "time"

type User struct {
	ID         int       `json:"id"`
	Email      string    `json:"email"`
	APIKeyHash *string   `json:"-"`
	Verified   bool      `json:"verified"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type VerificationToken struct {
	ID        int       `json:"id"`
	UserID    int       `json:"user_id"`
	TokenHash string    `json:"-"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}

type RegisterRequest struct {
	Email string `json:"email"`
}

type RegisterResponse struct {
	Message string `json:"message"`
}

type ValidateResponse struct {
	UserID int `json:"user_id"`
}

type RotateKeyResponse struct {
	APIKey string `json:"api_key"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}
