// services/calendar-service/internal/oauth/google.go
package oauth

import (
	"context"
	"fmt"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	calendarapi "google.golang.org/api/calendar/v3"
)

type GoogleOAuth struct {
	config *oauth2.Config
}

func NewGoogleOAuth(params NewGoogleOAuthParams) *GoogleOAuth {
	return &GoogleOAuth{
		config: &oauth2.Config{
			ClientID:     params.ClientID,
			ClientSecret: params.ClientSecret,
			RedirectURL:  params.RedirectURI,
			Scopes:       []string{calendarapi.CalendarReadonlyScope},
			Endpoint:     google.Endpoint,
		},
	}
}

type NewGoogleOAuthParams struct {
	ClientID     string
	ClientSecret string
	RedirectURI  string
}

// AuthURL generates the Google OAuth consent URL. State parameter should include the user_id.
func (g *GoogleOAuth) AuthURL(state string) string {
	return g.config.AuthCodeURL(state, oauth2.AccessTypeOffline, oauth2.ApprovalForce)
}

// Exchange exchanges an authorization code for tokens.
func (g *GoogleOAuth) Exchange(ctx context.Context, code string) (*oauth2.Token, error) {
	token, err := g.config.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("Failed to exchange OAuth code: %w.", err)
	}

	if token.RefreshToken == "" {
		return nil, fmt.Errorf("No refresh token received. User may need to re-authorize with consent prompt.")
	}

	return token, nil
}

// TokenSource returns an oauth2.TokenSource that automatically refreshes the access token.
func (g *GoogleOAuth) TokenSource(ctx context.Context, refreshToken string) oauth2.TokenSource {
	token := &oauth2.Token{
		RefreshToken: refreshToken,
	}
	return g.config.TokenSource(ctx, token)
}

// Config returns the underlying oauth2.Config for advanced use.
func (g *GoogleOAuth) Config() *oauth2.Config {
	return g.config
}
