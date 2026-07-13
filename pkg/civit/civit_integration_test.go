//go:build integration
// +build integration

// Package civit_test contains integration tests that validate request
// payloads against the live CivitAI orchestrator API.
//
// Run with:
//
//	go test -tags=integration -run Integration ./pkg/civit/ -count=1 -timeout 60s
//
// These tests call the real API's ?whatif=true endpoint (no buzz spent).
// They exist because httptest mocks are blind to API schema drift —
// the mock server accepts whatever we send, but the real API doesn't.
package civit_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/m/civitui/pkg/civit"
)

// resolveIntegrationAPIKey reads the API key from config files (same as main.go).
func resolveIntegrationAPIKey(t *testing.T) string {
	t.Helper()

	if key := os.Getenv("CIVITAI_API_KEY"); key != "" {
		return key
	}

	home, err := os.UserHomeDir()
	if err != nil {
		t.Skipf("cannot find home dir: %v", err)
	}

	for _, p := range []string{
		filepath.Join(home, ".config", "civitui", "civitui.conf"),
		filepath.Join(home, ".config", "civitai", "config.yaml"),
	} {
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			// Match "api_key" key followed by separator and value.
			after := ""
			if strings.HasPrefix(line, "api_key:") {
				after = strings.TrimPrefix(line, "api_key:")
			} else if strings.HasPrefix(line, "api_key") && strings.Contains(line, "=") {
				parts := strings.SplitN(line, "=", 2)
				after = parts[1]
			}
			if after != "" {
				return strings.Trim(strings.TrimSpace(after), `"'`)
			}
		}
	}

	t.Skip("no API key found; set CIVITAI_API_KEY or configure ~/.config/civitai/config.yaml")
	return ""
}

// whatif calls CalculatePrice with ?whatif=true and returns nil on success.
func whatif(t *testing.T, client *civit.Client, req civit.GenerationRequest) error {
	t.Helper()
	ctx := context.Background()
	_, err := client.CalculatePrice(ctx, req)
	return err
}

// TestIntegration_DefaultRequest validates the default form values.
func TestIntegration_DefaultRequest(t *testing.T) {
	key := resolveIntegrationAPIKey(t)
	client := civit.NewClient(key)

	req := civit.GenerationRequest{
		Prompt:       "test — integration validation",
		Model:        "air:flux1:checkpoint:civitai:618692@691639",
		FluxMode:     "urn:air:flux1:checkpoint:civitai:618692@691639",
		Sampler:      "Euler a",
		AspectRatio:  "1:1",
		Width:        1024,
		Height:       1024,
		Steps:        20,
		CFGScale:     7.0,
		Quantity:     1,
		OutputFormat: "jpeg",
		Draft:        false,
	}

	if err := whatif(t, client, req); err != nil {
		t.Fatalf("default request rejected: %v", err)
	}
}

// TestIntegration_SamplerPresets validates every sampler value the TUI offers.
func TestIntegration_SamplerPresets(t *testing.T) {
	key := resolveIntegrationAPIKey(t)
	client := civit.NewClient(key)

	// These are the sampler options shown in the TUI presets.
	// If any of these fail, the preset list is wrong.
	samplers := []string{
		"Euler a",
		"Euler",
		"Heun",
		"DPM++ 2S a",
		"DPM++ 2M",
		"DPM++ 2Mv2",
		"IPNDM",
		"IPNDM_V",
		"LCM",
	}

	base := civit.GenerationRequest{
		Prompt:       "test — sampler validation",
		Model:        "air:pony-diffusion-v6:checkpoint:civitai:257204@290640", // SDXL-based, exposes sampler
		AspectRatio:  "1:1",
		Width:        1024,
		Height:       1024,
		Steps:        20,
		CFGScale:     7.0,
		Quantity:     1,
		OutputFormat: "jpeg",
		Draft:        false,
	}

	for _, sampler := range samplers {
		req := base
		req.Sampler = sampler
		t.Run(sampler, func(t *testing.T) {
			if err := whatif(t, client, req); err != nil {
				t.Errorf("sampler %q rejected: %v", sampler, err)
			}
		})
	}
}

// TestIntegration_SchedulerPresets validates every scheduler value the TUI offers.
func TestIntegration_SchedulerPresets(t *testing.T) {
	key := resolveIntegrationAPIKey(t)
	client := civit.NewClient(key)

	// These are the scheduler options shown in the TUI presets.
	// The API's Scheduler enum uses sampler algorithm names, not noise schedule names.
	schedulers := []string{
		"EulerA",
		"Euler",
		"Heun",
		"DPM2",
		"DPM2A",
		"DPM2SA",
		"DPM2M",
		"DPMSDE",
		"DPMFast",
		"DPMAdaptive",
		"LMSKarras",
		"DPM2Karras",
		"DPM2AKarras",
		"DPM2SAKarras",
		"DPM2MKarras",
		"DPMSDEKarras",
		"DDIM",
		"PLMS",
		"UniPC",
		"LCM",
		"DDPM",
		"DEIS",
		"LMS",
	}

	base := civit.GenerationRequest{
		Prompt:       "test — scheduler validation",
		Model:        "air:pony-diffusion-v6:checkpoint:civitai:257204@290640", // SDXL-based, exposes scheduler
		AspectRatio:  "1:1",
		Width:        1024,
		Height:       1024,
		Steps:        20,
		CFGScale:     7.0,
		Quantity:     1,
		OutputFormat: "jpeg",
		Draft:        false,
	}

	for _, scheduler := range schedulers {
		req := base
		req.Scheduler = scheduler
		t.Run(scheduler, func(t *testing.T) {
			if err := whatif(t, client, req); err != nil {
				t.Errorf("scheduler %q rejected: %v", scheduler, err)
			}
		})
	}
}

// TestIntegration_AspectRatioPresets validates aspect ratio values.
func TestIntegration_AspectRatioPresets(t *testing.T) {
	key := resolveIntegrationAPIKey(t)
	client := civit.NewClient(key)

	ratios := []string{
		"1:1", "3:2", "2:3", "16:9", "9:16", "4:3", "3:4",
	}

	base := civit.GenerationRequest{
		Prompt:       "test — aspect ratio validation",
		Model:        "air:flux1:checkpoint:civitai:618692@691639",
		AspectRatio:  "1:1",
		Width:        1024,
		Height:       1024,
		Steps:        20,
		CFGScale:     7.0,
		Quantity:     1,
		OutputFormat: "jpeg",
		Draft:        false,
	}

	for _, ratio := range ratios {
		req := base
		req.AspectRatio = ratio
		t.Run(ratio, func(t *testing.T) {
			if err := whatif(t, client, req); err != nil {
				t.Errorf("aspect ratio %q rejected: %v", ratio, err)
			}
		})
	}
}

// TestIntegration_ErrorBodyDump prints the actual API error body on failure
// so the developer can see exactly what the API said — not a truncated message.
func TestIntegration_ErrorBodyDump(t *testing.T) {
	key := resolveIntegrationAPIKey(t)
	client := civit.NewClient(key)

	// Deliberately bad payload to exercise error formatting.
	req := civit.GenerationRequest{
		Prompt:    "test",
		Model:     "air:flux1:checkpoint:civitai:618692@691639",
		Scheduler: "karras", // known bad value
		Width:     1024,
		Height:    1024,
		Steps:     20,
		CFGScale:  7.0,
		Quantity:  1,
		Draft:     false,
	}

	err := whatif(t, client, req)
	if err == nil {
		t.Error("expected error for bad scheduler value, got nil")
	} else {
		// Print full error body for inspection.
		fmt.Fprintf(os.Stderr, "\n--- Full API error body ---\n%v\n---\n", err)
	}
}
