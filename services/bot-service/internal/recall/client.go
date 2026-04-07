package recall

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"strconv"
	"time"
)

type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

type NewClientParams struct {
	Region string
	APIKey string
}

func NewClient(params NewClientParams) *Client {
	return &Client{
		baseURL: fmt.Sprintf("https://%s.recall.ai/api/v1", params.Region),
		apiKey:  params.APIKey,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

type CreateBotRequest struct {
	MeetingURL       string            `json:"meeting_url"`
	BotName          string            `json:"bot_name,omitempty"`
	TranscriptConfig *TranscriptConfig `json:"transcript"`
}

type TranscriptConfig struct {
	Provider           string `json:"provider"`
	PrioritizeAccuracy bool   `json:"prioritize_accuracy"`
	Language           string `json:"language"`
	Diarization        bool   `json:"diarization"`
}

type CreateBotResponse struct {
	ID     string `json:"id"`
	Status string `json:"status_changes"`
}

type CreateBotParams struct {
	MeetingURL string
	BotName    string
}

const (
	maxRetries507        = 10
	retryInterval507     = 30 * time.Second
	defaultRetryAfter429 = 5 * time.Second
)

func (c *Client) CreateBot(ctx context.Context, params CreateBotParams) (*CreateBotResponse, error) {
	reqBody := CreateBotRequest{
		MeetingURL: params.MeetingURL,
		BotName:    params.BotName,
		TranscriptConfig: &TranscriptConfig{
			Provider:           "recallai_streaming",
			PrioritizeAccuracy: true,
			Language:           "auto",
			Diarization:        true,
		},
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal create bot request: %w", err)
	}

	for attempt := 0; attempt <= maxRetries507; attempt++ {
		if attempt > 0 {
			log.Printf("Retrying create bot (attempt %d/%d) after 507.", attempt, maxRetries507)
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(retryInterval507):
			}
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/bot/", bytes.NewReader(bodyBytes))
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Token "+c.apiKey)

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("create bot request failed: %w", err)
		}

		respBody, err := io.ReadAll(resp.Body)
		resp.Body.Close()

		if err != nil {
			return nil, fmt.Errorf("failed to read create bot response: %w", err)
		}

		switch resp.StatusCode {
		case http.StatusCreated, http.StatusOK:
			var result CreateBotResponse
			if err := json.Unmarshal(respBody, &result); err != nil {
				return nil, fmt.Errorf("failed to parse create bot response: %w", err)
			}
			return &result, nil

		case http.StatusTooManyRequests:
			retryAfter := parseRetryAfter(resp.Header.Get("Retry-After"))
			jitter := time.Duration(rand.Intn(1000)) * time.Millisecond
			waitTime := retryAfter + jitter
			log.Printf("Rate limited (429), waiting %v before retry.", waitTime)
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(waitTime):
			}
			continue

		case http.StatusInsufficientStorage:
			log.Printf("Recall returned 507 (insufficient capacity), will retry.")
			continue

		default:
			return nil, fmt.Errorf("create bot failed with status %d: %s", resp.StatusCode, string(respBody))
		}
	}

	return nil, fmt.Errorf("create bot failed after %d retries due to 507", maxRetries507)
}

type DeleteBotParams struct {
	BotID string
}

func (c *Client) DeleteBot(ctx context.Context, params DeleteBotParams) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, c.baseURL+"/bot/"+params.BotID+"/", nil)
	if err != nil {
		return fmt.Errorf("failed to create delete request: %w", err)
	}
	req.Header.Set("Authorization", "Token "+c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("delete bot request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNoContent || resp.StatusCode == http.StatusOK {
		return nil
	}

	body, _ := io.ReadAll(resp.Body)
	return fmt.Errorf("delete bot failed with status %d: %s", resp.StatusCode, string(body))
}

func parseRetryAfter(header string) time.Duration {
	if header == "" {
		return defaultRetryAfter429
	}
	seconds, err := strconv.Atoi(header)
	if err != nil {
		return defaultRetryAfter429
	}
	return time.Duration(seconds) * time.Second
}
