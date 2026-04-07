package recall

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/devinat1/obsidian-meeting-notes/services/transcript-service/internal/model"
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
			Timeout: 60 * time.Second,
		},
	}
}

type TranscriptMetadata struct {
	ID          string `json:"id"`
	Status      string `json:"status"`
	DownloadURL string `json:"download_url"`
}

type FetchTranscriptMetadataParams struct {
	TranscriptID string
}

func (c *Client) FetchTranscriptMetadata(ctx context.Context, params FetchTranscriptMetadataParams) (*TranscriptMetadata, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/transcript/"+params.TranscriptID+"/", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create transcript metadata request: %w", err)
	}
	req.Header.Set("Authorization", "Token "+c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("transcript metadata request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("transcript metadata request returned %d: %s", resp.StatusCode, string(body))
	}

	var metadata TranscriptMetadata
	if err := json.NewDecoder(resp.Body).Decode(&metadata); err != nil {
		return nil, fmt.Errorf("failed to decode transcript metadata: %w", err)
	}

	return &metadata, nil
}

type DownloadTranscriptParams struct {
	DownloadURL string
}

func (c *Client) DownloadTranscript(ctx context.Context, params DownloadTranscriptParams) ([]model.RawSegment, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, params.DownloadURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create transcript download request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("transcript download request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("transcript download returned %d: %s", resp.StatusCode, string(body))
	}

	var segments []model.RawSegment
	if err := json.NewDecoder(resp.Body).Decode(&segments); err != nil {
		return nil, fmt.Errorf("failed to decode transcript data: %w", err)
	}

	return segments, nil
}
