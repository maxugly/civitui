// civitui — Terminal UI for the CivitAI image generation orchestrator.
//
// Usage:
//
//	civitui
//
// The application walks through a 6-phase pipeline:
//
//	config  — fill in generation parameters
//	pricing — calculate buzz cost (whatif)
//	confirm — review cost, confirm or cancel
//	submit  — dispatch the workflow
//	poll    — wait for completion
//	done    — view downloaded images
//
// All API communication is handled by pkg/civit (headless engine).
// The UI layer (internal/ui) is a Bubble Tea state machine with no
// direct HTTP knowledge.
package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/m/civitui/internal/ui"
	"github.com/m/civitui/pkg/civit"
)

func main() {
	apiKey, err := resolveAPIKey()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		fmt.Fprintln(os.Stderr, "Generate an API key at: https://civitai.com/user/account")
		os.Exit(1)
	}

	// Initialize the headless API engine.
	client := civit.NewClient(apiKey)

	// Create the UI model and start the Bubble Tea runtime.
	model := ui.NewModel(client)
	program := tea.NewProgram(model, tea.WithAltScreen())

	if _, err := program.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "civitui: %v\n", err)
		os.Exit(1)
	}
}

// resolveAPIKey loads the API key from the environment variable,
// ~/.config/civitui/civitui.conf, or ~/.config/civitai/config.yaml.
func resolveAPIKey() (string, error) {
	// 1. Check environment variable override
	if key := os.Getenv("CIVITAI_API_KEY"); key != "" {
		return key, nil
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("unable to find user home directory: %w", err)
	}

	// 2. Check primary config path
	primaryPath := filepath.Join(homeDir, ".config", "civitui", "civitui.conf")
	if key := parseAPIKeyFromConfig(primaryPath); key != "" {
		return key, nil
	}

	// 3. Check legacy config path
	legacyPath := filepath.Join(homeDir, ".config", "civitai", "config.yaml")
	if key := parseAPIKeyFromConfig(legacyPath); key != "" {
		return key, nil
	}

	return "", fmt.Errorf("civitai api key not found in CIVITAI_API_KEY env var, %s, or %s", primaryPath, legacyPath)
}

// parseAPIKeyFromConfig reads a file and parses the api_key value.
func parseAPIKeyFromConfig(path string) string {
	file, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		// Skip comments and empty lines
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Support split by ':' or '='
		var sepIdx int
		colonIdx := strings.Index(line, ":")
		equalIdx := strings.Index(line, "=")

		if colonIdx != -1 && (equalIdx == -1 || colonIdx < equalIdx) {
			sepIdx = colonIdx
		} else if equalIdx != -1 {
			sepIdx = equalIdx
		} else {
			continue
		}

		key := strings.TrimSpace(line[:sepIdx])
		if key != "api_key" {
			continue
		}

		val := strings.TrimSpace(line[sepIdx+1:])
		// Remove quotes
		val = strings.Trim(val, `"'`)
		return val
	}
	return ""
}
