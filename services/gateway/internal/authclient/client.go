// services/gateway/internal/authclient/client.go
package authclient

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type Client struct {
	authServiceURL string
	httpClient     *http.Client
}

type NewParams struct {
	AuthServiceURL string
}

func New(params NewParams) *Client {
	return &Client{
		authServiceURL: params.AuthServiceURL,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

type validateResponse struct {
	UserID int `json:"user_id"`
}

func (c *Client) Validate(apiKey string) (int, error) {
	req, err := http.NewRequest(http.MethodGet, c.authServiceURL+"/internal/auth/validate", nil)
	if err != nil {
		return 0, fmt.Errorf("creating validate request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("calling auth service: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("auth service returned status %d", resp.StatusCode)
	}

	var result validateResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, fmt.Errorf("decoding validate response: %w", err)
	}

	return result.UserID, nil
}
