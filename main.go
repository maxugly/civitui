// civitui — Terminal UI for the CivitAI image generation orchestrator.
//
// Usage:
//
//	CIVITAI_API_KEY="your-key" civitui
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
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/m/civitui/internal/ui"
	"github.com/m/civitui/pkg/civit"
)

func main() {
	apiKey := os.Getenv("CIVITAI_API_KEY")
	if apiKey == "" {
		fmt.Fprintln(os.Stderr, "CIVITAI_API_KEY environment variable is required")
		fmt.Fprintln(os.Stderr, "Get your key at: https://civitai.com/user/account")
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
