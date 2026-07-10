// Package civit provides a client for the CivitAI orchestrator API (v2).
//
// This package handles HTTP interactions only — no UI, no terminal logic.
// The UI layer (internal/ui) consumes this package via the Client methods
// and chains them together: whatif → confirm → submit → poll → download.
package civit

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// Default API base URL for the CivitAI orchestrator.
const DefaultAPIBase = "https://orchestration.civitai.com"

// ── Client ──────────────────────────────────────────────────────────────────

// Client handles authenticated communication with the CivitAI orchestrator.
// It owns an http.Client with sensible timeouts and an API key for auth.
type Client struct {
	apiKey  string
	http    *http.Client
	APIBase string
}

// NewClient creates a CivitAI API client with the given API key.
// The underlying http.Client uses a 30s timeout; per-call deadlines
// are enforced via context.
func NewClient(apiKey string) *Client {
	return &Client{
		apiKey: apiKey,
		http: &http.Client{
			Timeout: 30 * time.Second,
		},
		APIBase: DefaultAPIBase,
	}
}

// ── Data Models ─────────────────────────────────────────────────────────────

// GenerationRequest holds the parameters for a text-to-image generation.
type GenerationRequest struct {
	Prompt        string `json:"prompt"`
	NegativePrompt string `json:"negativePrompt,omitempty"`
	Model         string `json:"model"`          // AIR format, e.g. "air:flux1:checkpoint:civitai:618692@691639"
	Width         int    `json:"width"`
	Height        int    `json:"height"`
	Steps         int    `json:"steps"`
	CFGScale      float64 `json:"cfgScale"`
	Quantity      int    `json:"quantity"`
	Seed          *int64 `json:"seed,omitempty"` // nil means random
}

// workflowStep is the internal JSON shape for a single step in the workflow.
type workflowStep struct {
	Type  string              `json:"$type"`
	Input *GenerationRequest  `json:"input"`
}

// workflowPayload is the top-level JSON body sent to POST /v2/consumer/workflows.
type workflowPayload struct {
	Steps []workflowStep `json:"steps"`
}

// WorkflowResponse is the JSON returned by the workflow endpoints.
type WorkflowResponse struct {
	ID        string `json:"id"`
	Status    string `json:"status"`
	CreatedAt string `json:"createdAt"`
	Steps     []Step `json:"steps"`
	Cost      *Cost  `json:"cost,omitempty"`
}

// Step represents one step inside a workflow (currently only textToImage).
type Step struct {
	Type   string  `json:"$type"`
	Status string  `json:"status"`
	Output *Output `json:"output,omitempty"`
	Jobs   []Job   `json:"jobs,omitempty"`
}

// Output contains the generated images for a completed step.
type Output struct {
	Images []Image `json:"images"`
}

// Image describes a generated image with its download URL.
type Image struct {
	ID        string `json:"id"`
	URL       string `json:"url"`
	Available bool   `json:"available"`
	Width     int    `json:"width"`
	Height    int    `json:"height"`
}

// Job represents a single compute job within a step.
type Job struct {
	ID     string `json:"id"`
	Status string `json:"status"`
	Cost   int    `json:"cost"`
}

// Cost is the top-level cost summary in a whatif or completed response.
type Cost struct {
	Total   int `json:"total"`
	Base    int `json:"base"`
}

// ── API Methods ─────────────────────────────────────────────────────────────

// doJSON performs an HTTP request with JSON serialization and deserialization.
// Retries on 5xx server errors up to maxRetries times.
func (c *Client) doJSON(ctx context.Context, method, path string, body interface{}, params map[string]string, result interface{}) error {
	url := strings.TrimRight(c.APIBase, "/") + path

	// Build query string
	if len(params) > 0 {
		q := make([]string, 0, len(params))
		for k, v := range params {
			q = append(q, k+"="+v)
		}
		url += "?" + strings.Join(q, "&")
	}

	// Marshal request body
	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal request: %w", err)
		}
		reqBody = bytes.NewReader(data)
	}

	const maxRetries = 3
	var lastErr error

	for attempt := 0; attempt < maxRetries; attempt++ {
		req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
		if err != nil {
			return fmt.Errorf("create request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")

		// Reset reader for retries
		if body != nil {
			data, _ := json.Marshal(body)
			req.Body = io.NopCloser(bytes.NewReader(data))
		}

		resp, err := c.http.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("request failed: %w", err)
			if attempt < maxRetries-1 {
				time.Sleep(time.Duration(attempt+1) * 3 * time.Second)
				continue
			}
			return lastErr
		}

		respBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		// Retry on 5xx
		if resp.StatusCode >= 500 && attempt < maxRetries-1 {
			lastErr = fmt.Errorf("API error %d: %s", resp.StatusCode, string(respBody))
			time.Sleep(time.Duration(attempt+1) * 3 * time.Second)
			continue
		}

		if resp.StatusCode >= 400 {
			return fmt.Errorf("API error %d: %s", resp.StatusCode, string(respBody))
		}

		if result != nil && len(respBody) > 0 {
			if err := json.Unmarshal(respBody, result); err != nil {
				return fmt.Errorf("unmarshal response: %w\nbody: %s", err, string(respBody)[:500])
			}
		}
		return nil
	}

	return lastErr
}

// CalculatePrice runs a whatif request and returns the estimated buzz cost.
// This does NOT spend any buzz — the UI uses this to prompt for confirmation.
func (c *Client) CalculatePrice(ctx context.Context, req GenerationRequest) (int, error) {
	payload := workflowPayload{
		Steps: []workflowStep{{
			Type:  "textToImage",
			Input: &req,
		}},
	}

	var result WorkflowResponse
	if err := c.doJSON(ctx, http.MethodPost, "/v2/consumer/workflows", payload,
		map[string]string{"whatif": "true"}, &result); err != nil {
		return 0, fmt.Errorf("calculate price: %w", err)
	}

	if result.Cost == nil {
		return 0, fmt.Errorf("no cost information in whatif response")
	}
	return result.Cost.Total, nil
}

// SubmitJob dispatches a generation workflow and returns the workflow ID.
// Use PollJobStatus to wait for completion.
func (c *Client) SubmitJob(ctx context.Context, req GenerationRequest) (string, error) {
	payload := workflowPayload{
		Steps: []workflowStep{{
			Type:  "textToImage",
			Input: &req,
		}},
	}

	var result WorkflowResponse
	if err := c.doJSON(ctx, http.MethodPost, "/v2/consumer/workflows", payload, nil, &result); err != nil {
		return "", fmt.Errorf("submit job: %w", err)
	}

	if result.ID == "" {
		return "", fmt.Errorf("no workflow ID in response")
	}
	return result.ID, nil
}

// PollJobStatus checks the status of a submitted workflow.
// Returns the status string: "succeeded", "failed", "cancelled", or
// intermediate states like "unassigned", "scheduled", "processing".
func (c *Client) PollJobStatus(ctx context.Context, jobID string) (*WorkflowResponse, error) {
	var result WorkflowResponse
	if err := c.doJSON(ctx, http.MethodGet, "/v2/consumer/workflows/"+jobID, nil, nil, &result); err != nil {
		return nil, fmt.Errorf("poll status: %w", err)
	}
	return &result, nil
}

// DownloadImage fetches the image at url and writes it to destPath.
// Retries on transient failures (empty body, network errors) up to 3 times.
// The URL should come from Image.URL in a completed WorkflowResponse.
func (c *Client) DownloadImage(ctx context.Context, url, destPath string) error {
	const maxRetries = 3
	var lastErr error

	for attempt := 0; attempt < maxRetries; attempt++ {
		if err := c.downloadOnce(ctx, url, destPath); err != nil {
			lastErr = err
			if attempt < maxRetries-1 {
				time.Sleep(time.Duration(attempt+1) * 2 * time.Second)
				continue
			}
			return fmt.Errorf("download failed after %d attempts: %w", maxRetries, lastErr)
		}
		return nil
	}
	return lastErr
}

func (c *Client) downloadOnce(ctx context.Context, url, destPath string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("create download request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("download request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("download HTTP %d: %s", resp.StatusCode, string(body)[:200])
	}

	f, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("create output file: %w", err)
	}
	defer f.Close()

	written, err := io.Copy(f, resp.Body)
	if err != nil {
		return fmt.Errorf("write download: %w", err)
	}
	if written == 0 {
		return fmt.Errorf("empty response body (zero bytes downloaded)")
	}

	return nil
}
