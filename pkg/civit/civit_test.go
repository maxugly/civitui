package civit

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// ── Helpers ──────────────────────────────────────────────────────────────────

// newTestClient creates a Client pointed at the given httptest server.
func newTestClient(srv *httptest.Server) *Client {
	c := NewClient("test-api-key")
	c.APIBase = srv.URL
	return c
}

// ── CalculatePrice ───────────────────────────────────────────────────────────

func TestCalculatePrice_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify method and path
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/v2/consumer/workflows" {
			t.Errorf("expected /v2/consumer/workflows, got %s", r.URL.Path)
		}
		if r.URL.Query().Get("whatif") != "true" {
			t.Errorf("expected whatif=true, got %s", r.URL.RawQuery)
		}
		// Verify auth header
		if r.Header.Get("Authorization") != "Bearer test-api-key" {
			t.Errorf("expected auth header, got %s", r.Header.Get("Authorization"))
		}

		resp := WorkflowResponse{
			ID:        "wf-whatif-123",
			Status:    "whatif",
			CreatedAt: "2026-07-11T00:00:00Z",
			Cost:      &Cost{Total: 42, Base: 30},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	client := newTestClient(srv)
	ctx := context.Background()

	cost, err := client.CalculatePrice(ctx, GenerationRequest{
		Prompt: "a rusty piano in the rain",
		Model:  "air:flux1:checkpoint:civitai:618692@691639",
		Width:  1024,
		Height: 1024,
		Steps:  20,
		Quantity: 1,
	})
	if err != nil {
		t.Fatalf("CalculatePrice failed: %v", err)
	}
	if cost != 42 {
		t.Errorf("expected cost 42, got %d", cost)
	}
}

func TestCalculatePrice_NoCost(t *testing.T) {
	// Whatif response missing the cost field — should error.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := WorkflowResponse{
			ID:     "wf-nocost-1",
			Status: "whatif",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	client := newTestClient(srv)
	_, err := client.CalculatePrice(context.Background(), GenerationRequest{
		Prompt: "test",
		Model:  "air:test@1",
		Width:  512, Height: 512,
	})
	if err == nil {
		t.Fatal("expected error for missing cost, got nil")
	}
}

func TestCalculatePrice_BadRequest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":"invalid model"}`))
	}))
	defer srv.Close()

	client := newTestClient(srv)
	_, err := client.CalculatePrice(context.Background(), GenerationRequest{
		Prompt: "test",
		Model:  "bad-model",
		Width:  512, Height: 512,
	})
	if err == nil {
		t.Fatal("expected error for 400, got nil")
	}
}

func TestCalculatePrice_InvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{this is not json`))
	}))
	defer srv.Close()

	client := newTestClient(srv)
	_, err := client.CalculatePrice(context.Background(), GenerationRequest{
		Prompt: "test",
		Model:  "air:test@1",
		Width:  512, Height: 512,
	})
	if err == nil {
		t.Fatal("expected unmarshal error for invalid JSON, got nil")
	}
}

// ── SubmitJob ────────────────────────────────────────────────────────────────

func TestSubmitJob_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}

		resp := WorkflowResponse{
			ID:        "wf-abc123",
			Status:    "unassigned",
			CreatedAt: "2026-07-11T00:00:00Z",
			Steps: []Step{{
				Type:   "textToImage",
				Status: "unassigned",
			}},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	client := newTestClient(srv)
	id, err := client.SubmitJob(context.Background(), GenerationRequest{
		Prompt:   "a junkyard dog",
		Model:    "air:flux1:checkpoint:civitai:618692@691639",
		Width:    1024,
		Height:   1024,
		Steps:    20,
		CFGScale: 7.0,
		Quantity: 2,
	})
	if err != nil {
		t.Fatalf("SubmitJob failed: %v", err)
	}
	if id != "wf-abc123" {
		t.Errorf("expected wf-abc123, got %s", id)
	}
}

func TestSubmitJob_NoID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return a response with an empty ID
		resp := WorkflowResponse{Status: "error"}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	client := newTestClient(srv)
	_, err := client.SubmitJob(context.Background(), GenerationRequest{
		Prompt: "test",
		Model:  "air:test@1",
		Width:  512, Height: 512,
	})
	if err == nil {
		t.Fatal("expected error for empty workflow ID, got nil")
	}
}

func TestSubmitJob_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"orchestrator down"}`))
	}))
	defer srv.Close()

	client := newTestClient(srv)
	// Use a short timeout so the retry loop finishes quickly.
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	_, err := client.SubmitJob(ctx, GenerationRequest{
		Prompt: "test",
		Model:  "air:test@1",
		Width:  512, Height: 512,
	})
	if err == nil {
		t.Fatal("expected error for 500, got nil")
	}
}

// ── PollJobStatus ────────────────────────────────────────────────────────────

func TestPollJobStatus_Success(t *testing.T) {
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/v2/consumer/workflows/wf-running" {
			t.Errorf("expected /v2/consumer/workflows/wf-running, got %s", r.URL.Path)
		}

		resp := WorkflowResponse{
			ID:     "wf-running",
			Status: "succeeded",
			Steps: []Step{{
				Type:   "textToImage",
				Status: "succeeded",
				Output: &Output{
					Images: []Image{
						{ID: "img-1", URL: srv.URL + "/download/img-1.png", Available: true, Width: 1024, Height: 1024},
						{ID: "img-2", URL: srv.URL + "/download/img-2.png", Available: true, Width: 1024, Height: 1024},
					},
				},
			}},
			Cost: &Cost{Total: 84, Base: 60},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	client := newTestClient(srv)
	result, err := client.PollJobStatus(context.Background(), "wf-running")
	if err != nil {
		t.Fatalf("PollJobStatus failed: %v", err)
	}
	if result.Status != "succeeded" {
		t.Errorf("expected status succeeded, got %s", result.Status)
	}
	if len(result.Steps) != 1 {
		t.Fatalf("expected 1 step, got %d", len(result.Steps))
	}
	if len(result.Steps[0].Output.Images) != 2 {
		t.Errorf("expected 2 images, got %d", len(result.Steps[0].Output.Images))
	}
	if result.Steps[0].Output.Images[0].URL != srv.URL+"/download/img-1.png" {
		t.Errorf("unexpected image URL: %s", result.Steps[0].Output.Images[0].URL)
	}
	if result.Cost == nil || result.Cost.Total != 84 {
		t.Errorf("unexpected cost: %+v", result.Cost)
	}
}

func TestPollJobStatus_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"error":"workflow not found"}`))
	}))
	defer srv.Close()

	client := newTestClient(srv)
	_, err := client.PollJobStatus(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error for 404, got nil")
	}
}

// ── DownloadImage ────────────────────────────────────────────────────────────

func TestDownloadImage_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-api-key" {
			t.Errorf("expected auth header on download")
		}
		w.Header().Set("Content-Type", "image/png")
		w.Write([]byte("fake-image-bytes-here"))
	}))
	defer srv.Close()

	client := newTestClient(srv)
	dest := filepath.Join(t.TempDir(), "output.png")

	err := client.DownloadImage(context.Background(), srv.URL+"/image.png", dest)
	if err != nil {
		t.Fatalf("DownloadImage failed: %v", err)
	}

	data, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("reading downloaded file: %v", err)
	}
	if string(data) != "fake-image-bytes-here" {
		t.Errorf("unexpected file content: %s", string(data))
	}
}

func TestDownloadImage_EmptyBody(t *testing.T) {
	// Return a 200 with zero-length body — should error after retries.
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusOK)
		// Write nothing — empty body.
	}))
	defer srv.Close()

	client := newTestClient(srv)
	dest := filepath.Join(t.TempDir(), "empty.png")

	err := client.DownloadImage(context.Background(), srv.URL+"/empty.png", dest)
	if err == nil {
		t.Fatal("expected error for empty body download, got nil")
	}
	// Should have retried 3 times.
	if callCount != 3 {
		t.Errorf("expected 3 download attempts, got %d", callCount)
	}
}

func TestDownloadImage_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`access denied`))
	}))
	defer srv.Close()

	client := newTestClient(srv)
	dest := filepath.Join(t.TempDir(), "nope.png")

	err := client.DownloadImage(context.Background(), srv.URL+"/secret.png", dest)
	if err == nil {
		t.Fatal("expected error for 403 download, got nil")
	}
}

// ── JSON Round-Trip (Serialization) ──────────────────────────────────────────

func TestWorkflowResponse_UnmarshalFull(t *testing.T) {
	// Verify the full JSON shape a real orchestrator would return
	// unmarshals correctly into our structs.
	payload := []byte(`{
		"id": "wf-full-1",
		"status": "succeeded",
		"createdAt": "2026-07-11T12:34:56Z",
		"steps": [{
			"$type": "textToImage",
			"status": "succeeded",
			"output": {
				"images": [{
					"id": "img-001",
					"url": "https://cdn.civitai.com/images/001.png",
					"available": true,
					"width": 1344,
					"height": 768
				}]
			},
			"jobs": [{
				"id": "job-1",
				"status": "completed",
				"cost": 42
			}]
		}],
		"cost": {
			"total": 42,
			"base": 30
		}
	}`)

	var resp WorkflowResponse
	if err := json.Unmarshal(payload, &resp); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if resp.ID != "wf-full-1" {
		t.Errorf("ID: expected wf-full-1, got %s", resp.ID)
	}
	if resp.Status != "succeeded" {
		t.Errorf("Status: expected succeeded, got %s", resp.Status)
	}
	if resp.Cost.Total != 42 {
		t.Errorf("Cost.Total: expected 42, got %d", resp.Cost.Total)
	}
	if resp.Cost.Base != 30 {
		t.Errorf("Cost.Base: expected 30, got %d", resp.Cost.Base)
	}

	step := resp.Steps[0]
	if step.Type != "textToImage" {
		t.Errorf("Step.Type: expected textToImage, got %s", step.Type)
	}
	if step.Status != "succeeded" {
		t.Errorf("Step.Status: expected succeeded, got %s", step.Status)
	}

	img := step.Output.Images[0]
	if img.ID != "img-001" {
		t.Errorf("Image.ID: expected img-001, got %s", img.ID)
	}
	if img.Width != 1344 || img.Height != 768 {
		t.Errorf("Image dimensions: expected 1344x768, got %dx%d", img.Width, img.Height)
	}
	if !img.Available {
		t.Error("Image.Available: expected true")
	}

	job := step.Jobs[0]
	if job.ID != "job-1" || job.Status != "completed" || job.Cost != 42 {
		t.Errorf("Job: unexpected values: %+v", job)
	}
}

func TestGenerationRequest_Marshal(t *testing.T) {
	seed := int64(12345)
	req := GenerationRequest{
		Prompt:         "a neon sign flickering",
		NegativePrompt:  "ugly, blurry",
		Model:           "air:flux1:checkpoint:civitai:618692@691639",
		Width:           1024,
		Height:          768,
		Steps:           25,
		CFGScale:        7.5,
		Quantity:        4,
		Seed:            &seed,
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	// Round-trip back.
	var parsed GenerationRequest
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if parsed.Prompt != req.Prompt {
		t.Errorf("Prompt mismatch")
	}
	if parsed.NegativePrompt != req.NegativePrompt {
		t.Errorf("NegativePrompt mismatch")
	}
	if parsed.Model != req.Model {
		t.Errorf("Model mismatch")
	}
	if parsed.Width != 1024 || parsed.Height != 768 {
		t.Errorf("dimensions mismatch: %dx%d", parsed.Width, parsed.Height)
	}
	if parsed.Steps != 25 {
		t.Errorf("Steps mismatch: %d", parsed.Steps)
	}
	if parsed.CFGScale != 7.5 {
		t.Errorf("CFGScale mismatch: %f", parsed.CFGScale)
	}
	if parsed.Quantity != 4 {
		t.Errorf("Quantity mismatch: %d", parsed.Quantity)
	}
	if parsed.Seed == nil || *parsed.Seed != 12345 {
		t.Errorf("Seed mismatch")
	}
}

func TestGenerationRequest_MarshalOmitEmpty(t *testing.T) {
	// Seed and NegativePrompt should be omitted when zero/nil.
	req := GenerationRequest{
		Prompt: "minimal",
		Model:  "air:test@1",
		Width:  512,
		Height: 512,
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var raw map[string]interface{}
	json.Unmarshal(data, &raw)

	if _, ok := raw["seed"]; ok {
		t.Error("seed should be omitted when nil")
	}
	if _, ok := raw["negativePrompt"]; ok {
		t.Error("negativePrompt should be omitted when empty")
	}
	if raw["prompt"] != "minimal" {
		t.Errorf("prompt mismatch: %v", raw["prompt"])
	}
}

func TestGenerationRequest_DraftMarshal(t *testing.T) {
	// Draft has no omitempty, so false must serialize.
	req := GenerationRequest{
		Prompt: "test",
		Model:  "air:test@1",
		Width:  512,
		Height: 512,
		Draft:  true,
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal true failed: %v", err)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if v, ok := raw["draft"]; !ok {
		t.Error("draft key missing when true")
	} else if v != true {
		t.Errorf("draft expected true, got %v", v)
	}

	// Default (false) must still serialize — no omitempty.
	req.Draft = false
	data, err = json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal false failed: %v", err)
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal false: %v", err)
	}
	if v, ok := raw["draft"]; !ok {
		t.Error("draft key missing when false (no omitempty)")
	} else if v != false {
		t.Errorf("draft expected false, got %v", v)
	}
}

// ── Context Cancellation ─────────────────────────────────────────────────────

func TestContextCancel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate a slow server.
		time.Sleep(2 * time.Second)
	}))
	defer srv.Close()

	client := newTestClient(srv)
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	_, err := client.CalculatePrice(ctx, GenerationRequest{
		Prompt: "test",
		Model:  "air:test@1",
		Width:  512, Height: 512,
	})
	if err == nil {
		t.Fatal("expected context cancellation error, got nil")
	}
}
