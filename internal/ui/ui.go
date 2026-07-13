// Package ui provides the Bubble Tea terminal user interface for civitui.
//
// The UI renders a config form and an inline job queue in a single-window
// layout (lazygit-style). Async API calls are dispatched as tea.Cmd and
// route back to the correct job via a jobID carried in every message.
//
// Form input uses Charm's bubbles/textinput for ergonomic text editing
// (cursor movement, home/end, backspace, selection) while preserving
// full layout control for split-pane rendering.
package ui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/m/civitui/pkg/civit"
)

// ── Job ───────────────────────────────────────────────────────────────────────

// JobStatus tracks the lifecycle of a single generation request.
type JobStatus int

const (
	JobPricing     JobStatus = iota // waiting for CalculatePrice
	JobConfirming                   // showing cost, awaiting y/n
	JobSubmitting                   // waiting for SubmitJob
	JobPolling                      // polling for completion
	JobDownloading                  // downloading images
	JobDone                         // completed successfully
	JobFailed                       // error at any stage
)

// String returns a human-readable job status label.
func (s JobStatus) String() string {
	switch s {
	case JobPricing:
		return "pricing..."
	case JobConfirming:
		return ""
	case JobSubmitting:
		return "submitting..."
	case JobPolling:
		return "polling..."
	case JobDownloading:
		return "downloading..."
	case JobDone:
		return "done"
	case JobFailed:
		return "failed"
	default:
		return "unknown"
	}
}

// Job represents a single generation request and its lifecycle.
// All pipeline state (cost, jobID, poll results, download paths) lives
// inside the job, not on the global Model.
type Job struct {
	ID     string // unique identifier (auto-incrementing counter as string)
	Prompt string // truncated prompt for display
	Cost   int    // buzz cost (0 until pricing completes)
	Status JobStatus
	ErrMsg string // error message if failed

	// Internal pipeline state
	req      civit.GenerationRequest
	apiJobID string
	pollResp *civit.WorkflowResponse
	pollTick int
	dlPaths  []string
	dlErrors []string
}

// numFormFields is the count of editable fields in the config form.
const numFormFields = 21

// Form field indices into the inputs slice.
const (
	fiPrompt         = iota // 0 — text
	fiNegativePrompt        // 1 — text
	fiModel                 // 2 — text (presets)
	fiFluxMode              // 3 — text (presets, Flux-only)
	fiSampler               // 4 — text (presets)
	fiScheduler             // 5 — text (presets, advanced, model-aware)
	fiAspectRatio           // 6 — text (presets, auto-fills width/height)
	fiWidth                 // 7 — int
	fiHeight                // 8 — int
	fiSteps                 // 9 — int
	fiCFGScale              // 10 — float
	fiQuantity              // 11 — int
	fiOutputFormat          // 12 — text (presets)
	fiSeed                  // 13 — int64 (nil if empty)
	fiDraft                 // 14 — boolean (presets: true/false)
	fiDenoise               // 15 — float (0.0–1.0, img2img only)
	fiClipSkip              // 16 — int (1–3, SD/SDXL only)
	fiUpscaleWidth          // 17 — int (320–3840)
	fiUpscaleHeight         // 18 — int (320–3840)
	fiExperimental          // 19 — boolean (presets, advanced)
	fiFluxUltraRaw          // 20 — boolean (presets, Flux Ultra only)
)

// isReplaceOnFocus reports whether the field at idx should clear on
// first keystroke. Applies to all numeric fields. Prompt, negative
// prompt, model, and sampler keep normal behavior.
func isReplaceOnFocus(idx int) bool {
	return (idx >= fiWidth && idx <= fiSeed) ||
		idx == fiDenoise || idx == fiClipSkip ||
		idx == fiUpscaleWidth || idx == fiUpscaleHeight
}

// isPresetsField reports whether the field at idx uses the right-pane
// presets selector (right arrow to browse, no free-form typing).
func isPresetsField(idx int) bool {
	return idx == fiModel || idx == fiFluxMode || idx == fiSampler ||
		idx == fiScheduler || idx == fiAspectRatio || idx == fiOutputFormat ||
		idx == fiDraft || idx == fiExperimental || idx == fiFluxUltraRaw
}

// ── Regex ────────────────────────────────────────────────────────────────────

// Character-validation regexps. Numeric inputs block any keystroke that
// doesn't match their pattern, so the user can't type letters at all.
var (
	intRegex   = regexp.MustCompile(`^[0-9]*$`)
	floatRegex = regexp.MustCompile(`^[0-9]*\.?[0-9]*$`)
	ansiRegex  = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)
)

// ── Model ────────────────────────────────────────────────────────────────────

// Model holds the complete UI state for the civitui Bubble Tea application.
type Model struct {
	// client is the headless API engine injected at startup.
	client *civit.Client

	// ── Configuration Panel (form fields) ──
	//
	// inputs holds all textinput models for the config form.
	// activeInput indexes the currently-focused field (0..numFormFields-1).
	inputs      []textinput.Model
	activeInput int

	// justFocused is true when the user has just tabbed into a numeric
	// field. The first printable keystroke clears the existing value so
	// the user can type over it instead of appending.
	justFocused bool

	// ── Right pane state ──
	//
	// inPresetsPane is true when the user has entered the model presets
	// list (right arrow on the Model field) and is navigating it with
	// up/down/enter rather than editing the text input.
	inPresetsPane bool
	activePreset  int // currently highlighted index in the preset list

	// ── Job queue ──
	//
	// jobs holds all submitted generation requests. Newest at the bottom.
	// Each job manages its own pipeline state (cost, API job ID, poll
	// results, download paths) independently. Multiple jobs can be in
	// different stages simultaneously.
	jobs      []*Job
	nextJobID int // auto-incrementing counter

	// ── Debug ──

	debug     bool     // verbose logging enabled (--debug flag)
	debugLog  []string // ring buffer of recent log entries
	debugFile string   // path to debug.log file

	// ── Terminal geometry ──

	termWidth  int
	termHeight int

	// ── Spinner ──

	spinnerFrame int

	// errMsg shows a transient error at the bottom of the config form.
	errMsg string
}

// spinnerFrames for the waiting animations.
var spinnerFrames = []string{"⣾", "⣽", "⣻", "⢿", "⡿", "⣟", "⣯", "⣷"}

// fieldLabels maps each form field index to its display label.
var fieldLabels = []string{
	"Prompt",
	"Negative Prompt",
	"Model",
	"Flux Mode",
	"Sampler",
	"Scheduler",
	"Aspect Ratio",
	"Width",
	"Height",
	"Steps",
	"CFG Scale",
	"Quantity",
	"Output Format",
	"Seed",
	"Draft Mode (fast preview)",
	"Denoise",
	"CLIP Skip",
	"Upscale Width",
	"Upscale Height",
	"Experimental Mode",
	"Flux Ultra Raw",
}

// ── Model Presets ────────────────────────────────────────────────────────────

// ModelPreset describes a selectable CivitAI model from the right pane.
type ModelPreset struct {
	Name        string
	Description string
	AIR         string
	Ecosystem   string // "flux1", "flux2", "sdxl", "sd1", "zimage"
}

var modelPresets = []ModelPreset{
	{
		Name:        "Flux.1 Dev (Standard)",
		Description: "Recommended. Balanced and high quality for general prompts.",
		AIR:         "air:flux1:checkpoint:civitai:618692@691639",
		Ecosystem:   "flux1",
	},
	{
		Name:        "Pony Diffusion V6 XL",
		Description: "Extremely popular SDXL checkpoint for expressive art.",
		AIR:         "air:pony-diffusion-v6:checkpoint:civitai:257204@290640",
		Ecosystem:   "sdxl",
	},
	{
		Name:        "SDXL 1.0 (Base)",
		Description: "Stability AI official base model. Fast and reliable.",
		AIR:         "air:sdxl:checkpoint:civitai:101055@128078",
		Ecosystem:   "sdxl",
	},
	{
		Name:        "Stable Diffusion 1.5",
		Description: "Classic model. Extremely fast and lightweight.",
		AIR:         "air:sd15:checkpoint:civitai:1102@11124",
		Ecosystem:   "sd1",
	},
}

// ── Sampler Presets ──────────────────────────────────────────────────────────

// SamplerPreset describes a selectable diffusion sampler from the right pane.
// ValSD, ValZImage, ValFlux2 hold the ecosystem-specific API values.
type SamplerPreset struct {
	Name        string
	Description string
	ValSD       string // Value for SD1/SDXL/Pony/Illustrious
	ValZImage   string // Value for ZImage
	ValFlux2    string // Value for Flux2Klein
}

var samplerPresets = []SamplerPreset{
	{
		Name:        "Euler a",
		Description: "Euler Ancestral. Fast, creative, slightly unstable at high steps.",
		ValSD:       "Euler a",
	},
	{
		Name:        "DPM++ 2M Karras",
		Description: "Recommended for SDXL. High quality, excellent convergence.",
		ValSD:       "DPM++ 2M Karras",
	},
	{
		Name:        "DPM++ SDE Karras",
		Description: "Highly realistic, great detail, but takes twice as long.",
		ValSD:       "DPM++ SDE Karras",
	},
	{
		Name:        "Euler",
		Description: "Classic simple ODE solver. Fast and consistent.",
		ValSD:       "Euler",
		ValZImage:   "euler",
		ValFlux2:    "euler",
	},
	{
		Name:        "Heun",
		Description: "Very high accuracy, but requires more compute per step.",
		ValSD:       "Heun",
		ValZImage:   "heun",
		ValFlux2:    "heun",
	},
	{
		Name:        "DPM++ 2M",
		Description: "DPM++ 2M. Good convergence.",
		ValSD:       "DPM++ 2M",
		ValFlux2:    "dpm++2m",
	},
	{
		Name:        "LCM",
		Description: "Latent Consistency Model. Fast generation with few steps.",
		ValSD:       "LCM",
		ValFlux2:    "lcm",
	},
}

// filteredSamplerPresets returns the sampler presets valid for the current model ecosystem.
func (m *Model) filteredSamplerPresets() []SamplerPreset {
	eco := m.currentEcosystem()
	var visible []SamplerPreset
	for _, p := range samplerPresets {
		switch eco {
		case "flux1":
			// Flux1 uses fluxMode instead; sampler is hidden
		case "flux2":
			if p.ValFlux2 != "" {
				visible = append(visible, p)
			}
		case "zimage":
			if p.ValZImage != "" {
				visible = append(visible, p)
			}
		case "sd1", "sdxl":
			if p.ValSD != "" {
				visible = append(visible, p)
			}
		default:
			if p.ValSD != "" {
				visible = append(visible, p)
			}
		}
	}
	return visible
}

// ── Aspect Ratio Presets ─────────────────────────────────────────────────────

// AspectRatioPreset pairs a display label and ratio string with the
// dimensions it resolves to. Selecting a preset fills width and height.
type AspectRatioPreset struct {
	Label  string // e.g. "16:9 — 1344×768"
	Ratio  string // e.g. "16:9"
	Width  int
	Height int
}

var aspectRatioPresets = []AspectRatioPreset{
	{"1:1 — 1024×1024", "1:1", 1024, 1024},
	{"3:2 — 1216×832", "3:2", 1216, 832},
	{"2:3 — 832×1216", "2:3", 832, 1216},
	{"16:9 — 1344×768", "16:9", 1344, 768},
	{"9:16 — 768×1344", "9:16", 768, 1344},
	{"4:3 — 1152×896", "4:3", 1152, 896},
	{"3:4 — 896×1152", "3:4", 896, 1152},
}

// ── Flux Mode Presets ────────────────────────────────────────────────────────

// FluxModePreset selects a Flux1 model variant.
type FluxModePreset struct {
	Name        string
	Description string
	URN         string
}

var fluxModePresets = []FluxModePreset{
	{
		Name:        "Draft",
		Description: "Fastest, lowest quality. Great for rapid iteration.",
		URN:         "urn:air:flux1:checkpoint:civitai:618692@699279",
	},
	{
		Name:        "Standard",
		Description: "Recommended. Balanced speed and quality. Default.",
		URN:         "urn:air:flux1:checkpoint:civitai:618692@691639",
	},
	{
		Name:        "Krea",
		Description: "Krea-tuned Flux variant. Creative outputs.",
		URN:         "urn:air:flux1:checkpoint:civitai:618692@2068000",
	},
	{
		Name:        "Pro 1.1",
		Description: "Higher quality, more detail. Slower than Standard.",
		URN:         "urn:air:flux1:checkpoint:civitai:618692@922358",
	},
	{
		Name:        "Ultra",
		Description: "Highest quality, highest resolution. Most expensive.",
		URN:         "urn:air:flux1:checkpoint:civitai:618692@1088507",
	},
}

// ── Scheduler Presets ────────────────────────────────────────────────────────
//
// The CivitAI API's Scheduler enum uses sampler algorithm names (EulerA, DPM2, etc.),
// not noise schedule names (simple, karras, etc.). All 23 sampler-based values
// are valid; we expose a recommended subset of 7. Verified against live API
// via integration tests (2026-07-13, bones).
//
// DEPRECATED (noise schedule names that cause API 400s):
//   simple, discrete, karras, exponential, ays

// SchedulerPreset selects a scheduler algorithm for the generation.
type SchedulerPreset struct {
	Name        string
	Description string
	Scheduler   string
}

var schedulerPresets = []SchedulerPreset{
	{
		Name:        "Euler Ancestral",
		Description: "Euler Ancestral (EulerA). Fast, creative, standard default.",
		Scheduler:   "EulerA",
	},
	{
		Name:        "Euler",
		Description: "Euler. Simple, fast, and reliable.",
		Scheduler:   "Euler",
	},
	{
		Name:        "Heun",
		Description: "Heun. High accuracy, double compute per step.",
		Scheduler:   "Heun",
	},
	{
		Name:        "DPM++ 2M",
		Description: "DPM++ 2M (DPM2M). Fast with good convergence.",
		Scheduler:   "DPM2M",
	},
	{
		Name:        "DPM++ 2M Karras",
		Description: "DPM++ 2M Karras (DPM2MKarras). Highly recommended for SDXL.",
		Scheduler:   "DPM2MKarras",
	},
	{
		Name:        "LCM",
		Description: "Latent Consistency Model (LCM). Fast generation with few steps.",
		Scheduler:   "LCM",
	},
	{
		Name:        "UniPC",
		Description: "UniPC. Unified Predictor-Corrector method, fast convergence.",
		Scheduler:   "UniPC",
	},
}

// ── Output Format Presets ─────────────────────────────────────────────────────

// OutputFormatPreset selects the output image encoding.
type OutputFormatPreset struct {
	Name        string
	Description string
	Format      string
}

var outputFormatPresets = []OutputFormatPreset{
	{
		Name:        "JPEG",
		Description: "Smaller file size, faster download. Default.",
		Format:      "jpeg",
	},
	{
		Name:        "PNG",
		Description: "Lossless quality, supports transparency. Larger files.",
		Format:      "png",
	},
}

// ── Draft Mode Presets ────────────────────────────────────────────────────────

// DraftPreset selects the draft mode value (true/false toggle).
type DraftPreset struct {
	Name        string
	Description string
	Value       bool
}

var draftPresets = []DraftPreset{
	{
		Name:        "false",
		Description: "Normal generation. Speed and quality are dictated by other parameters. Default.",
		Value:       false,
	},
	{
		Name:        "true",
		Description: "Draft Mode. Injects draft LoRAs (SD1: 424706, SDXL: 391999), reduces steps to 6-8, and sets CFG to 1. Speed over quality.",
		Value:       true,
	},
}

// ── Experimental Mode Presets ─────────────────────────────────────────────────

var experimentalPresets = []DraftPreset{
	{
		Name:        "false",
		Description: "Standard generation. Default.",
		Value:       false,
	},
	{
		Name:        "true",
		Description: "Experimental Mode. Enables SDCPP engine edge features for supported models (SD1, SDXL, Pony, Illustrious, NoobAI, Flux1, FluxKrea).",
		Value:       true,
	},
}

// ── Flux Ultra Raw Presets ────────────────────────────────────────────────────

var fluxUltraRawPresets = []DraftPreset{
	{
		Name:        "false",
		Description: "Standard Flux Ultra output. Default.",
		Value:       false,
	},
	{
		Name:        "true",
		Description: "Flux Ultra Raw Mode. Generates less-refined, more photorealistic results from Flux Ultra.",
		Value:       true,
	},
}

// ── Field Visibility ──────────────────────────────────────────────────────────

// isFieldVisible reports whether the field at idx should be shown given the
// current form state. Some fields are conditionally hidden (e.g. denoise
// only when a source image is attached, clipSkip only for SD/SDXL models).
func (m *Model) isFieldVisible(idx int) bool {
	switch idx {
	case fiDenoise:
		return false // hidden until img2img source image support is added
	case fiClipSkip:
		eco := m.currentEcosystem()
		return eco != "flux1" && eco != "flux2" && eco != "zimage"
	case fiFluxUltraRaw:
		return m.currentEcosystem() == "flux1" && m.inputs[fiFluxMode].Value() == "urn:air:flux1:checkpoint:civitai:618692@1088507"
	case fiFluxMode:
		return m.currentEcosystem() == "flux1"
	case fiSampler:
		return len(m.filteredSamplerPresets()) > 1
	default:
		return true
	}
}

// currentEcosystem returns the ecosystem of the currently selected model.
// Keys off the modelPreset's Ecosystem if matched, or parses the AIR string.
func (m *Model) currentEcosystem() string {
	return ecosystemForAIR(m.inputs[fiModel].Value())
}

// adjustFieldsForEcosystem resets fields that became invalid after a model change.
// Called whenever the model is changed (preset selection or custom AIR entry).
func (m *Model) adjustFieldsForEcosystem() {
	eco := m.currentEcosystem()

	// Reset sampler if stale
	if m.isFieldVisible(fiSampler) {
		filtered := m.filteredSamplerPresets()
		if len(filtered) > 0 {
			valid := false
			cur := m.inputs[fiSampler].Value()
			for _, p := range filtered {
				v := p.ValSD
				if eco == "flux2" {
					v = p.ValFlux2
				} else if eco == "zimage" {
					v = p.ValZImage
				}
				if v == cur {
					valid = true
					break
				}
			}
			if !valid {
				// Reset to first valid preset
				v := filtered[0].ValSD
				if eco == "flux2" {
					v = filtered[0].ValFlux2
				} else if eco == "zimage" {
					v = filtered[0].ValZImage
				}
				m.inputs[fiSampler].SetValue(v)
			}
		}
	} else {
		m.inputs[fiSampler].SetValue("")
	}

	// Reset scheduler to EulerA if the current value isn't a known preset.
	if cur := m.inputs[fiScheduler].Value(); cur != "" {
		valid := false
		for _, s := range schedulerPresets {
			if s.Scheduler == cur {
				valid = true
				break
			}
		}
		if !valid {
			m.inputs[fiScheduler].SetValue(schedulerPresets[0].Scheduler)
		}
	}

	// Reset Flux-specific fields when leaving Flux
	if eco != "flux1" {
		m.inputs[fiFluxMode].SetValue("")
		m.inputs[fiFluxUltraRaw].SetValue("false")
	}
}

// refocusIfHidden moves focus to the first visible field if the current
// active field has become hidden (e.g. after a model change).
func (m *Model) refocusIfHidden() {
	if m.isFieldVisible(m.activeInput) {
		return
	}
	m.inputs[m.activeInput].Blur()
	start := m.activeInput
	for {
		m.activeInput = (m.activeInput + 1) % numFormFields
		if m.isFieldVisible(m.activeInput) {
			break
		}
		if m.activeInput == start {
			break // safety exit: all fields hidden
		}
	}
	m.inputs[m.activeInput].Focus()
	m.justFocused = isReplaceOnFocus(m.activeInput)
}

// maxCFGScale returns the maximum CFG scale allowed for the given ecosystem.
func maxCFGScale(eco string) float64 {
	if eco == "flux1" || eco == "flux2" {
		return 7.0
	}
	return 30.0
}

// ecosystemForAIR parses an AIR string and returns the ecosystem identifier.
// Used by both currentEcosystem() and the cfgScale keystroke validator.
func ecosystemForAIR(air string) string {
	// Check predefined presets first
	for _, preset := range modelPresets {
		if preset.AIR == air {
			return preset.Ecosystem
		}
	}
	valLower := strings.ToLower(air)
	if strings.Contains(valLower, "flux2") || strings.Contains(valLower, "klein") {
		return "flux2"
	}
	if strings.Contains(valLower, "flux") {
		return "flux1"
	}
	if strings.Contains(valLower, "zimage") {
		return "zimage"
	}
	if strings.Contains(valLower, "sdxl") || strings.Contains(valLower, "pony") || strings.Contains(valLower, "illustrious") {
		return "sdxl"
	}
	if strings.Contains(valLower, "sd15") || strings.Contains(valLower, "sd1") {
		return "sd1"
	}
	return "sdxl"
}
// Index aligns with the fi* constants.
var fieldHelpText = []string{
	/* fiPrompt */ "Describe the image you want to generate. Be specific and descriptive. E.g. 'a photo of an astronaut on Mars'. (Required)",
	/* fiNegativePrompt */ "Describe things you want to avoid in the generated image. E.g. 'blurry, low quality, distorted'. (Optional)",
	/* fiModel */ "Civitai Model AIR. Represents the base generator model. Use Right Arrow key to select from popular presets.",
	/* fiFluxMode */ "Flux model variant. Only applies when using a Flux1 model. Use Right Arrow to select from presets.",
	/* fiSampler */ "Diffusion sampler algorithm. Controls noise schedule and convergence. Use Right Arrow key to select from presets.",
	/* fiScheduler */ "Scheduler algorithm. Uses sampler names (EulerA, DPM2, LCM, etc.). Default: EulerA. Use Right Arrow to select from presets.",
	/* fiAspectRatio */ "Image aspect ratio. Use Right Arrow key to select from presets — auto-fills Width and Height.",
	/* fiWidth */ "Width of the generated image in pixels. Range: 64–4096. Standard resolutions: 1024, 2048. Only digits allowed.",
	/* fiHeight */ "Height of the generated image in pixels. Range: 64–4096. Standard resolutions: 1024, 2048. Only digits allowed.",
	/* fiSteps */ "Number of denoising steps. Range: 1–100. Recommended: 20-30. More steps take longer but add detail.",
	/* fiCFGScale */ "Classifier Free Guidance scale. Range: 1.0–7.0 (Flux) / 1.0–30.0 (SD/SDXL). Higher values follow the prompt more strictly. Decimals allowed.",
	/* fiQuantity */ "Number of images to generate. Range: 1–20 (API limit). Each image is processed concurrently in the workflow.",
	/* fiOutputFormat */ "Output image format. JPEG is smaller; PNG is lossless. Use Right Arrow to select.",
	/* fiSeed */ "Random seed for generation consistency. Range: 1–4294967295. Leave empty for a random seed.",
	/* fiDraft */ "Enable Draft Mode for fast previews (speed over quality). Injects draft LoRAs, reduces steps, sets CFG to 1. Use Right Arrow to select.",
	/* fiDenoise */ "Denoising strength for image-to-image workflows. Range: 0.0–1.0. 0.0 means no change, 1.0 means full regeneration. Only shown when a source image is attached.",
	/* fiClipSkip */ "Skip last N layers of CLIP text encoder. Range: 1–3. Default 2. Higher values reduce prompt adherence. Only for SD/SDXL models.",
	/* fiUpscaleWidth */ "Target width for upscaling. Range: 320–3840. Leave empty to disable.",
	/* fiUpscaleHeight */ "Target height for upscaling. Range: 320–3840. Leave empty to disable.",
	/* fiExperimental */ "Enable SDCPP experimental engine features. Use Right Arrow to select.",
	/* fiFluxUltraRaw */ "Flux Ultra Raw Mode. Less refined, more photorealistic. Only shown when Flux Mode is Ultra. Use Right Arrow to select.",
}

// ── Constructor ──────────────────────────────────────────────────────────────

// newTextInput creates a textinput.Model with the given placeholder and value.
func newTextInput(placeholder, value string, width int) textinput.Model {
	ti := textinput.New()
	ti.Placeholder = placeholder
	ti.SetValue(value)
	ti.Width = width
	ti.Prompt = ""
	ti.CharLimit = 512
	ti.ShowSuggestions = false
	return ti
}

// NewModel creates a new UI model wired to the given API client.
// The config form starts with 11 textinput fields, with Prompt focused.
// Numeric fields carry character-validation callbacks so letters are
// rejected at the keystroke level.
func NewModel(client *civit.Client, debug bool) Model {
	// Compute input widths that fit inside the frame.
	// The left column has: 2-space indent + 18-char label + ": " + input.
	// With leftColWidth capped at 80, the input can be at most ~55 chars.
	// Prompt and negative prompt get wider inputs; others are shorter.
	textWidth := 55
	promptWidth := 65

	inputs := make([]textinput.Model, numFormFields)
	inputs[fiPrompt] = newTextInput("a majestic cat wearing a top hat", "", promptWidth)
	inputs[fiNegativePrompt] = newTextInput("optional — what to avoid", "", promptWidth)
	inputs[fiModel] = newTextInput("air:flux1:checkpoint:civitai:618692@691639", "air:flux1:checkpoint:civitai:618692@691639", textWidth)
	inputs[fiFluxMode] = newTextInput("Standard", "urn:air:flux1:checkpoint:civitai:618692@691639", textWidth)
	inputs[fiSampler] = newTextInput("Euler a", "Euler a", textWidth)
	inputs[fiScheduler] = newTextInput("EulerA", "EulerA", textWidth)
	inputs[fiAspectRatio] = newTextInput("1:1", "1:1", 14)
	inputs[fiWidth] = newTextInput("1024", "1024", 10)
	inputs[fiHeight] = newTextInput("1024", "1024", 10)
	inputs[fiSteps] = newTextInput("20", "20", 6)
	inputs[fiCFGScale] = newTextInput("7.0", "7.0", 8)
	inputs[fiQuantity] = newTextInput("4", "4", 6)
	inputs[fiOutputFormat] = newTextInput("jpeg", "jpeg", textWidth)
	inputs[fiSeed] = newTextInput("random", "", 16)
	inputs[fiDraft] = newTextInput("false", "false", textWidth)
	inputs[fiDenoise] = newTextInput("0.4", "", 8)
	inputs[fiClipSkip] = newTextInput("2", "2", 6)
	inputs[fiUpscaleWidth] = newTextInput("disabled", "", 10)
	inputs[fiUpscaleHeight] = newTextInput("disabled", "", 10)
	inputs[fiExperimental] = newTextInput("false", "false", textWidth)
	inputs[fiFluxUltraRaw] = newTextInput("false", "false", textWidth)

	// Character validation: block letters in numeric fields.
	// empty input is always allowed (resolves to default/zero).
	inputs[fiWidth].Validate = func(s string) error {
		if s == "" {
			return nil
		}
		if !intRegex.MatchString(s) {
			return fmt.Errorf("must be an integer")
		}
		val, err := strconv.Atoi(s)
		if err != nil {
			return nil // intermediate state
		}
		if val > 4096 {
			return fmt.Errorf("width max is 4096")
		}
		return nil
	}
	inputs[fiHeight].Validate = func(s string) error {
		if s == "" {
			return nil
		}
		if !intRegex.MatchString(s) {
			return fmt.Errorf("must be an integer")
		}
		val, err := strconv.Atoi(s)
		if err != nil {
			return nil
		}
		if val > 4096 {
			return fmt.Errorf("height max is 4096")
		}
		return nil
	}
	inputs[fiSteps].Validate = func(s string) error {
		if s == "" {
			return nil
		}
		if !intRegex.MatchString(s) {
			return fmt.Errorf("must be an integer")
		}
		val, err := strconv.Atoi(s)
		if err != nil {
			return nil
		}
		if val > 100 {
			return fmt.Errorf("steps max is 100")
		}
		return nil
	}
	inputs[fiCFGScale].Validate = func(s string) error {
		if s == "" {
			return nil // empty is fine — resolves to zero, caught by range check on submit
		}
		if !floatRegex.MatchString(s) {
			return fmt.Errorf("must be a decimal number")
		}
		// Allow intermediate typing states like "." or "7." that haven't
		// formed a complete number yet. Reject only when the parsed value
		// exceeds the max — this is phone-number-style input masking.
		val, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return nil // intermediate state (e.g. bare ".")
		}
		// Model-dependent max: 7.0 for Flux, 30.0 for SD/SDXL
		maxVal := maxCFGScale(ecosystemForAIR(inputs[fiModel].Value()))
		if val > maxVal {
			return fmt.Errorf("cfg scale max is %.1f", maxVal)
		}
		return nil
	}
	inputs[fiQuantity].Validate = func(s string) error {
		if s == "" {
			return nil
		}
		if !intRegex.MatchString(s) {
			return fmt.Errorf("must be an integer")
		}
		val, err := strconv.Atoi(s)
		if err != nil {
			return nil
		}
		if val > 20 {
			return fmt.Errorf("quantity max is 20")
		}
		return nil
	}
	inputs[fiSeed].Validate = func(s string) error {
		if s == "" {
			return nil
		}
		if !intRegex.MatchString(s) {
			return fmt.Errorf("must be an integer")
		}
		val, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return nil
		}
		if val > 4294967295 {
			return fmt.Errorf("seed max is 4294967295")
		}
		return nil
	}

	inputs[fiDenoise].Validate = func(s string) error {
		if s == "" {
			return nil
		}
		if !floatRegex.MatchString(s) {
			return fmt.Errorf("must be a decimal number")
		}
		val, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return nil
		}
		if val < 0.0 || val > 1.0 {
			return fmt.Errorf("denoise must be between 0.0 and 1.0")
		}
		return nil
	}
	inputs[fiClipSkip].Validate = func(s string) error {
		if s == "" {
			return nil
		}
		if !intRegex.MatchString(s) {
			return fmt.Errorf("must be an integer")
		}
		val, err := strconv.Atoi(s)
		if err != nil {
			return nil
		}
		if val < 1 || val > 3 {
			return fmt.Errorf("clip skip must be 1, 2, or 3")
		}
		return nil
	}
	inputs[fiUpscaleWidth].Validate = func(s string) error {
		if s == "" {
			return nil
		}
		if !intRegex.MatchString(s) {
			return fmt.Errorf("must be an integer")
		}
		val, err := strconv.Atoi(s)
		if err != nil {
			return nil
		}
		if val > 3840 {
			return fmt.Errorf("upscale width max is 3840")
		}
		return nil
	}
	inputs[fiUpscaleHeight].Validate = func(s string) error {
		if s == "" {
			return nil
		}
		if !intRegex.MatchString(s) {
			return fmt.Errorf("must be an integer")
		}
		val, err := strconv.Atoi(s)
		if err != nil {
			return nil
		}
		if val > 3840 {
			return fmt.Errorf("upscale height max is 3840")
		}
		return nil
	}
	// Focus the first field.
	inputs[fiPrompt].Focus()

	m := Model{
		client:      client,
		inputs:      inputs,
		activeInput: 0,
		debug:       debug,
	}
	if debug {
		m.debugFile = fileDebugLogPath()
		m.logDebug("debug mode enabled — raw request/response bodies below")
	}
	return m
}

// toRequest builds a GenerationRequest from the current form state.
// Values are parsed from the textinput models' string values.
func (m *Model) toRequest() civit.GenerationRequest {
	width, _ := strconv.Atoi(m.inputs[fiWidth].Value())
	height, _ := strconv.Atoi(m.inputs[fiHeight].Value())
	steps, _ := strconv.Atoi(m.inputs[fiSteps].Value())
	cfgScale, _ := strconv.ParseFloat(m.inputs[fiCFGScale].Value(), 64)
	quantity, _ := strconv.Atoi(m.inputs[fiQuantity].Value())

	var seed *int64
	if s := m.inputs[fiSeed].Value(); s != "" {
		if n, err := strconv.ParseInt(s, 10, 64); err == nil {
			seed = &n
		}
	}

	return civit.GenerationRequest{
		Prompt:         m.inputs[fiPrompt].Value(),
		NegativePrompt: m.inputs[fiNegativePrompt].Value(),
		Model:          m.inputs[fiModel].Value(),
		FluxMode: func() string {
			if !m.isFieldVisible(fiFluxMode) {
				return ""
			}
			return m.inputs[fiFluxMode].Value()
		}(),
		Sampler: func() string {
			if !m.isFieldVisible(fiSampler) {
				return ""
			}
			return m.inputs[fiSampler].Value()
		}(),
		Scheduler:      m.inputs[fiScheduler].Value(),
		AspectRatio:    m.inputs[fiAspectRatio].Value(),
		Width:          width,
		Height:         height,
		Steps:          steps,
		CFGScale:       cfgScale,
		Quantity:       quantity,
		OutputFormat:   m.inputs[fiOutputFormat].Value(),
		Seed:           seed,
		Draft:          m.inputs[fiDraft].Value() == "true",
		// Tier 2 — only set conditional fields when visible/active
		Denoise:       nilIf(!m.isFieldVisible(fiDenoise), parseFloatPtr(m.inputs[fiDenoise].Value())),
		ClipSkip:      nilIf(!m.isFieldVisible(fiClipSkip), parseIntPtr(m.inputs[fiClipSkip].Value())),
		UpscaleWidth:  parseIntPtr(m.inputs[fiUpscaleWidth].Value()),
		UpscaleHeight: parseIntPtr(m.inputs[fiUpscaleHeight].Value()),
		Experimental:  parseBoolPtr(m.inputs[fiExperimental].Value()),
		FluxUltraRaw:  nilIf(!m.isFieldVisible(fiFluxUltraRaw), parseBoolPtr(m.inputs[fiFluxUltraRaw].Value())),
	}
}

// parseFloatPtr parses a string to *float64. Empty string returns nil.
func parseFloatPtr(s string) *float64 {
	if s == "" {
		return nil
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return nil
	}
	return &v
}

// parseIntPtr parses a string to *int. Empty string returns nil.
func parseIntPtr(s string) *int {
	if s == "" {
		return nil
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return nil
	}
	return &v
}

// parseBoolPtr parses a string to *bool. Empty string returns nil.
func parseBoolPtr(s string) *bool {
	if s == "" {
		return nil
	}
	v := s == "true"
	return &v
}

// nilIf returns nil if cond is true, otherwise returns val. Used to
// suppress conditional tier-2 fields when the model/mode doesn't apply.
func nilIf[T any](cond bool, val *T) *T {
	if cond {
		return nil
	}
	return val
}

// refocusConfig re-focuses the active input and clears presets pane state.
// Called when returning to the config form from errors or navigation.
func (m *Model) refocusConfig() {
	m.inPresetsPane = false
	m.inputs[m.activeInput].Focus()
}

// validateNumericFields checks that every numeric/seed field parses correctly
// and falls within valid ranges. Returns the first error found, or nil.
// Empty numeric fields (which get zero values) pass — only non-empty junk fails.
func (m *Model) validateNumericFields() error {
	if v := m.inputs[fiWidth].Value(); v != "" {
		val, err := strconv.Atoi(v)
		if err != nil {
			return fmt.Errorf("width: invalid integer %q", v)
		}
		if val < 64 || val > 4096 {
			return fmt.Errorf("width must be between 64 and 4096")
		}
	}
	if v := m.inputs[fiHeight].Value(); v != "" {
		val, err := strconv.Atoi(v)
		if err != nil {
			return fmt.Errorf("height: invalid integer %q", v)
		}
		if val < 64 || val > 4096 {
			return fmt.Errorf("height must be between 64 and 4096")
		}
	}
	if v := m.inputs[fiSteps].Value(); v != "" {
		val, err := strconv.Atoi(v)
		if err != nil {
			return fmt.Errorf("steps: invalid integer %q", v)
		}
		if val < 1 || val > 100 {
			return fmt.Errorf("steps must be between 1 and 100")
		}
	}
	if v := m.inputs[fiCFGScale].Value(); v != "" {
		val, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return fmt.Errorf("cfg scale: invalid float %q", v)
		}
		maxVal := maxCFGScale(m.currentEcosystem())
		if val < 1.0 || val > maxVal {
			return fmt.Errorf("cfg scale must be between 1.0 and %.1f", maxVal)
		}
	}
	if v := m.inputs[fiQuantity].Value(); v != "" {
		val, err := strconv.Atoi(v)
		if err != nil {
			return fmt.Errorf("quantity: invalid integer %q", v)
		}
		if val < 1 || val > 20 {
			return fmt.Errorf("quantity must be between 1 and 20")
		}
	}
	if s := m.inputs[fiSeed].Value(); s != "" {
		val, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return fmt.Errorf("seed: invalid integer %q", s)
		}
		if val < 1 || val > 4294967295 {
			return fmt.Errorf("seed must be between 1 and 4294967295")
		}
	}
	// Tier 2
	if v := m.inputs[fiDenoise].Value(); v != "" {
		val, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return fmt.Errorf("denoise: invalid float %q", v)
		}
		if val < 0.0 || val > 1.0 {
			return fmt.Errorf("denoise must be between 0.0 and 1.0")
		}
	}
	if v := m.inputs[fiClipSkip].Value(); v != "" {
		val, err := strconv.Atoi(v)
		if err != nil {
			return fmt.Errorf("clip skip: invalid integer %q", v)
		}
		if val < 1 || val > 3 {
			return fmt.Errorf("clip skip must be 1, 2, or 3")
		}
	}
	if v := m.inputs[fiUpscaleWidth].Value(); v != "" {
		val, err := strconv.Atoi(v)
		if err != nil {
			return fmt.Errorf("upscale width: invalid integer %q", v)
		}
		if val < 320 || val > 3840 {
			return fmt.Errorf("upscale width must be between 320 and 3840")
		}
	}
	if v := m.inputs[fiUpscaleHeight].Value(); v != "" {
		val, err := strconv.Atoi(v)
		if err != nil {
			return fmt.Errorf("upscale height: invalid integer %q", v)
		}
		if val < 320 || val > 3840 {
			return fmt.Errorf("upscale height must be between 320 and 3840")
		}
	}
	return nil
}

// ── ANSI / Text Helpers ──────────────────────────────────────────────────────

// aspectRatioMatches checks whether the current width and height maintain
// the selected aspect ratio. Compares as floating-point ratios rather
// than exact preset dimensions — e.g. "16:9" matches 1344×768, 2688×1536,
// and 672×384. A small tolerance handles rounding.
func (m *Model) aspectRatioMatches() bool {
	ratio := m.inputs[fiAspectRatio].Value()
	if ratio == "" {
		return true
	}
	w, errW := strconv.Atoi(m.inputs[fiWidth].Value())
	h, errH := strconv.Atoi(m.inputs[fiHeight].Value())
	if errW != nil || errH != nil || h == 0 {
		return false
	}
	for _, p := range aspectRatioPresets {
		if p.Ratio == ratio {
			presetRatio := float64(p.Width) / float64(p.Height)
			currentRatio := float64(w) / float64(h)
			diff := presetRatio - currentRatio
			if diff < 0 {
				diff = -diff
			}
			return diff < 0.01
		}
	}
	return false
}

// stripANSI removes terminal escape sequences so text can be measured
// for alignment without invisible control characters.
func stripANSI(str string) string {
	return ansiRegex.ReplaceAllString(str, "")
}

// wrapText wraps a string to the given column limit, breaking on word
// boundaries. Returns one string per line (nil for empty input).
func wrapText(text string, limit int) []string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return nil
	}
	var lines []string
	var current strings.Builder
	current.WriteString(words[0])

	for _, word := range words[1:] {
		if current.Len()+1+len(word) > limit {
			lines = append(lines, current.String())
			current.Reset()
			current.WriteString(word)
		} else {
			current.WriteString(" " + word)
		}
	}
	lines = append(lines, current.String())
	return lines
}

// ── Bubble Tea Interface ─────────────────────────────────────────────────────

// Init returns the initial command for the Bubble Tea runtime.
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		textinput.Blink,
		tickCmd(),
	)
}

// Update handles incoming messages and returns the updated model plus
// an optional command to execute.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.termWidth = msg.Width
		m.termHeight = msg.Height

	case tea.KeyMsg:
		return m.handleKey(msg)

	case tickMsg:
		m.spinnerFrame = (m.spinnerFrame + 1) % len(spinnerFrames)
		// Always keep the spinner ticking — it's used by the job queue.
		return m, tickCmd()

	case priceResultMsg:
		job := m.findJob(msg.jobID)
		if job == nil {
			return m, nil
		}
		if msg.err != nil {
			job.Status = JobFailed
			job.ErrMsg = msg.err.Error()
			m.logDebug("job %s pricing FAILED: %s", msg.jobID, msg.err)
		} else {
			job.Cost = msg.cost
			job.Status = JobConfirming
			m.logDebug("job %s priced: %d buzz, awaiting confirmation", msg.jobID, msg.cost)
		}

	case submitResultMsg:
		job := m.findJob(msg.jobID)
		if job == nil {
			return m, nil
		}
		if msg.err != nil {
			job.Status = JobFailed
			job.ErrMsg = msg.err.Error()
			m.logDebug("job %s submit FAILED: %s", msg.jobID, msg.err)
		} else {
			job.apiJobID = msg.apiJobID
			job.pollTick = 0
			job.dlPaths = nil
			job.dlErrors = nil
			job.Status = JobPolling
			m.logDebug("job %s submitted, API job: %s, polling...", msg.jobID, msg.apiJobID)
			return m, pollCmd(m.client, msg.jobID, msg.apiJobID)
		}

	case pollResultMsg:
		job := m.findJob(msg.jobID)
		if job == nil {
			return m, nil
		}
		if msg.err != nil {
			job.Status = JobFailed
			job.ErrMsg = msg.err.Error()
			return m, nil
		}
		job.pollResp = msg.resp
		job.pollTick++

		switch msg.resp.Status {
		case "succeeded":
			job.dlPaths = nil
			job.dlErrors = nil
			job.Status = JobDownloading
			return m, downloadCmd(m.client, msg.jobID, msg.resp)
		case "failed", "cancelled":
			job.Status = JobFailed
			job.ErrMsg = fmt.Sprintf("job %s", msg.resp.Status)
			return m, nil
		default:
			return m, pollCmd(m.client, msg.jobID, job.apiJobID)
		}

	case downloadResultMsg:
		job := m.findJob(msg.jobID)
		if job == nil {
			return m, nil
		}
		if msg.err != nil {
			job.dlErrors = append(job.dlErrors, msg.err.Error())
		} else {
			job.dlPaths = append(job.dlPaths, msg.path)
		}
		expected := 0
		if job.pollResp != nil && len(job.pollResp.Steps) > 0 {
			expected = len(job.pollResp.Steps[0].Output.Images)
		}
		done := len(job.dlPaths) + len(job.dlErrors)
		if done >= expected && expected > 0 {
			job.Status = JobDone
		}
		return m, nil
	}

	// Route Blink and other internal messages to the active textinput.
	if !m.inPresetsPane {
		m.inputs[m.activeInput], cmd = m.inputs[m.activeInput].Update(msg)
		return m, cmd
	}

	return m, cmd
}

// findJob locates a job by its ID. Returns nil if not found.
func (m *Model) findJob(id string) *Job {
	for _, j := range m.jobs {
		if j.ID == id {
			return j
		}
	}
	return nil
}

// createJob builds a new Job from the current form state and assigns it a unique ID.
func (m *Model) createJob() *Job {
	id := fmt.Sprintf("%d", m.nextJobID)
	m.nextJobID++
	prompt := m.inputs[fiPrompt].Value()
	if len(prompt) > 40 {
		prompt = prompt[:37] + "..."
	}
	j := &Job{
		ID:     id,
		Prompt: prompt,
		Status: JobPricing,
		req:    m.toRequest(),
	}
	m.logDebug("job %s created: %q", id, prompt)
	return j
}

// ── Debug Logging ─────────────────────────────────────────────────────────────

// logDebug appends a formatted message to the debug ring buffer and writes it
// to the debug log file. No-op when debug mode is off.
func (m *Model) logDebug(format string, args ...interface{}) {
	if !m.debug {
		return
	}
	msg := fmt.Sprintf(format, args...)
	m.debugLog = append(m.debugLog, msg)
	// Keep last 200 entries.
	if len(m.debugLog) > 200 {
		m.debugLog = m.debugLog[len(m.debugLog)-200:]
	}
	if m.debugFile != "" {
		f, err := os.OpenFile(m.debugFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err == nil {
			fmt.Fprintf(f, "%s %s\n", time.Now().Format("15:04:05"), msg)
			f.Close()
		}
	}
}

// fileDebugLogPath returns the path to the debug log file.
func fileDebugLogPath() string {
	home, _ := os.UserHomeDir()
	dir := filepath.Join(home, ".local", "share", "civitui")
	os.MkdirAll(dir, 0755)
	return filepath.Join(dir, "debug.log")
}

// View renders the config form at the top and the job queue below,
// wrapped in a clean frame — like lazygit. Content is rendered raw,
// then the outer border and section dividers are applied.
func (m Model) View() string {
	var b strings.Builder

	// ── Header bar ──
	b.WriteString(headerStyle.Render(" civitui "))
	b.WriteString("  ")
	b.WriteString(phaseStyle.Render(" [CONFIG] "))
	b.WriteString("\n\n")

	// ── Config form ──
	b.WriteString(m.sectionDivider("Configure generation"))
	b.WriteString("\n")
	m.viewConfig(&b)

	// ── Queue ──
	if len(m.jobs) > 0 {
		b.WriteString("\n")
		b.WriteString(m.sectionDivider("Queue"))
		b.WriteString("\n")
		m.viewQueue(&b)
	}

	// ── Debug ──
	if m.debug && len(m.debugLog) > 0 {
		b.WriteString("\n")
		b.WriteString(m.sectionDivider("Debug"))
		b.WriteString("\n")
		start := len(m.debugLog) - 8
		if start < 0 {
			start = 0
		}
		for _, entry := range m.debugLog[start:] {
			b.WriteString(dimStyle.Render("  " + entry))
			b.WriteString("\n")
		}
	}

	// ── Error ──
	if m.errMsg != "" {
		b.WriteString("\n")
		b.WriteString(errorStyle.Render(" ✗ " + m.errMsg))
	}

	// ── Footer ──
	b.WriteString("\n\n")
	b.WriteString(m.sectionDivider("ctrl+c quit  •  tab/↑↓ navigate  •  enter generate"))
	b.WriteString("\n")

	// Wrap the whole output in a frame.
	raw := b.String()
	return m.frame(raw)
}

// sectionDivider returns a line that intersects the frame border, like:
//
//	├── Queue ──────────────────────────────────────────┤
//
// The label sits on the divider line itself (lazygit style).
func (m Model) sectionDivider(label string) string {
	width := max(m.termWidth, 40) - 2
	prefix := "── "
	suffix := " "
	totalLabel := prefix + label + suffix
	remaining := width - len(totalLabel)
	if remaining < 2 {
		remaining = 2
	}
	return "├" + dimStyle.Render(prefix) + dimStyle.Render(label) + dimStyle.Render(suffix) + dimStyle.Render(strings.Repeat("─", remaining)) + "┤"
}

// frame wraps text in a plain box-drawing-character border with rounded
// corners (lazygit style). No ANSI styling on the border characters.
// Lines that exceed the frame width are truncated so the right border
// stays aligned.
func (m Model) frame(text string) string {
	width := max(m.termWidth, 40) - 2
	var out strings.Builder

	// Top border — rounded corners.
	out.WriteString("╭")
	out.WriteString(strings.Repeat("─", max(0, width)))
	out.WriteString("╮")
	out.WriteString("\n")

	lines := strings.Split(text, "\n")
	for _, line := range lines {
		// Section divider lines already have ├/┤ borders — pass through as-is.
		if strings.HasPrefix(line, "├") {
			plain := stripANSI(line)
			pad := width - len(plain)
			if pad < 0 {
				pad = 0
			}
			out.WriteString(line)
			out.WriteString(strings.Repeat(" ", pad))
			out.WriteString("\n")
			continue
		}

		plain := stripANSI(line)
		if len(plain) > width {
			line = truncateToWidth(line, width)
			plain = stripANSI(line)
		}
		pad := width - len(plain)
		if pad < 0 {
			pad = 0
		}
		out.WriteString("│")
		out.WriteString(line)
		out.WriteString(strings.Repeat(" ", pad))
		out.WriteString("│")
		out.WriteString("\n")
	}

	// Bottom border — rounded corners.
	out.WriteString("╰")
	out.WriteString(strings.Repeat("─", max(0, width)))
	out.WriteString("╯")

	return out.String()
}

// truncateToWidth cuts a string (which may contain ANSI escape codes)
// to at most visibleWidth printable characters. ANSI codes are preserved.
func truncateToWidth(s string, visibleWidth int) string {
	var result strings.Builder
	visible := 0
	inEscape := false
	for _, r := range s {
		if inEscape {
			result.WriteRune(r)
			if r == 'm' || (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') {
				inEscape = false
			}
			continue
		}
		if r == '\x1b' {
			inEscape = true
			result.WriteRune(r)
			continue
		}
		if visible >= visibleWidth {
			break
		}
		result.WriteRune(r)
		visible++
	}
	return result.String()
}

// viewQueue renders each job in the queue as a single status line.
func (m *Model) viewQueue(b *strings.Builder) {
	for _, job := range m.jobs {
		// Status icon.
		switch job.Status {
		case JobDone:
			b.WriteString(successStyle.Render(" ✓ "))
		case JobFailed:
			b.WriteString(errorStyle.Render(" ✗ "))
		default:
			b.WriteString(loadingStyle.Render(" " + spinnerFrames[m.spinnerFrame] + " "))
		}

		// Prompt (truncated to 40 chars).
		display := job.Prompt
		if display == "" {
			display = "(no prompt)"
		}
		b.WriteString(fmt.Sprintf("%-42s", fmt.Sprintf("%q", display)))

		// Cost.
		if job.Cost > 0 {
			b.WriteString(costStyle.Render(fmt.Sprintf(" %dⓑ  ", job.Cost)))
		} else if job.Status == JobFailed {
			b.WriteString(dimStyle.Render("  -   "))
		} else {
			b.WriteString(dimStyle.Render(" ...  "))
		}

		// Status text.
		switch job.Status {
		case JobConfirming:
			b.WriteString(costStyle.Render(fmt.Sprintf("%dⓑ ", job.Cost)))
			b.WriteString(keyStyle.Render("generate? "))
			b.WriteString(keyStyle.Render("[y/n]"))
		case JobDone:
			if len(job.dlPaths) > 0 {
				b.WriteString(successStyle.Render(job.dlPaths[0]))
			} else {
				b.WriteString(successStyle.Render("done"))
			}
		case JobFailed:
			b.WriteString(errorStyle.Render(job.ErrMsg))
		default:
			b.WriteString(loadingStyle.Render(job.Status.String()))
		}

		b.WriteString("\n")
	}
}

// ── View Helpers ─────────────────────────────────────────────────────────────

func (m *Model) viewConfig(b *strings.Builder) {
	// Left-column width: label + cursor + input, padded for alignment.
	// Capped to fit inside the frame (termWidth minus border and right-pane space).
	leftColWidth := m.termWidth - 35
	if leftColWidth < 50 {
		leftColWidth = 50
	}
	if leftColWidth > 80 {
		leftColWidth = 80
	}

	// Right-column width: what's left after the left column and the gap.
	// Dynamically calculated so content fits inside the frame.
	rightColWidth := m.termWidth - 2 - leftColWidth - 2
	if rightColWidth < 15 {
		rightColWidth = 15
	}
	if rightColWidth > 45 {
		rightColWidth = 45
	}
	wrapAt := rightColWidth - 4 // indent for description lines

	// Render left-column lines into a slice.
	var leftLines []string

	for i, input := range m.inputs {
		// Skip conditionally hidden fields.
		if !m.isFieldVisible(i) {
			continue
		}
		// Label.
		label := fieldLabelStyle.Render(fmt.Sprintf("  %-16s", fieldLabels[i]+":"))
		line := label
		if i == m.activeInput && !m.inPresetsPane {
			line = cursorStyle.Render("▶ ") + label
		} else {
			line = "  " + label
		}
		line += textInputStyle.Render(input.View())

		// Dim the aspect ratio field when width/height have drifted
		// from the selected preset's dimensions.
		if i == fiAspectRatio && !m.aspectRatioMatches() {
			line = dimStyle.Render(stripANSI(line))
		}

		leftLines = append(leftLines, line)
	}

	// Right-column content: either model presets or context help.
	var rightLines []string

	if isPresetsField(m.activeInput) {
		// Model, sampler, or aspect ratio presets pane.
		var presetsList []struct {
			Name        string
			Description string
		}

		switch m.activeInput {
		case fiModel:
			rightLines = append(rightLines, dimStyle.Render("── Model Presets ──"))
			for _, preset := range modelPresets {
				presetsList = append(presetsList, struct {
					Name        string
					Description string
				}{preset.Name, preset.Description})
			}
		case fiFluxMode:
			rightLines = append(rightLines, dimStyle.Render("── Flux Mode ──"))
			for _, preset := range fluxModePresets {
				presetsList = append(presetsList, struct {
					Name        string
					Description string
				}{preset.Name, preset.Description})
			}
		case fiSampler:
			rightLines = append(rightLines, dimStyle.Render("── Sampler Presets ──"))
			for _, preset := range m.filteredSamplerPresets() {
				presetsList = append(presetsList, struct {
					Name        string
					Description string
				}{preset.Name, preset.Description})
			}
		case fiScheduler:
			rightLines = append(rightLines, dimStyle.Render("── Scheduler ──"))
			for _, preset := range m.filteredSchedulerPresets() {
				presetsList = append(presetsList, struct {
					Name        string
					Description string
				}{preset.Name, preset.Description})
			}
		case fiAspectRatio:
			rightLines = append(rightLines, dimStyle.Render("── Aspect Ratio ──"))
			for _, preset := range aspectRatioPresets {
				presetsList = append(presetsList, struct {
					Name        string
					Description string
				}{preset.Label, ""})
			}
		case fiOutputFormat:
			rightLines = append(rightLines, dimStyle.Render("── Output Format ──"))
			for _, preset := range outputFormatPresets {
				presetsList = append(presetsList, struct {
					Name        string
					Description string
				}{preset.Name, preset.Description})
			}
		case fiDraft:
			rightLines = append(rightLines, dimStyle.Render("── Draft Mode ──"))
			for _, preset := range draftPresets {
				presetsList = append(presetsList, struct {
					Name        string
					Description string
				}{preset.Name, preset.Description})
			}
		case fiExperimental:
			rightLines = append(rightLines, dimStyle.Render("── Experimental Mode ──"))
			for _, preset := range experimentalPresets {
				presetsList = append(presetsList, struct {
					Name        string
					Description string
				}{preset.Name, preset.Description})
			}
		case fiFluxUltraRaw:
			rightLines = append(rightLines, dimStyle.Render("── Flux Ultra Raw ──"))
			for _, preset := range fluxUltraRawPresets {
				presetsList = append(presetsList, struct {
					Name        string
					Description string
				}{preset.Name, preset.Description})
			}
		}
		for j, preset := range presetsList {
			marker := "  "
			if m.inPresetsPane && j == m.activePreset {
				marker = cursorStyle.Render("▶ ")
			}
			rightLines = append(rightLines, fmt.Sprintf("%s%s", marker, presetStyle.Render(preset.Name)))
			// Wrap description under the name.
			for _, descLine := range wrapText(preset.Description, wrapAt) {
				rightLines = append(rightLines, dimStyle.Render("    "+descLine))
			}
		}
		if m.inPresetsPane {
			rightLines = append(rightLines, "")
			rightLines = append(rightLines, dimStyle.Render("↑↓ select  enter apply  ← esc back"))
		} else {
			rightLines = append(rightLines, "")
			rightLines = append(rightLines, dimStyle.Render("Press → to browse presets"))
		}
	} else {
		// Context-sensitive help for the active field.
		rightLines = append(rightLines, dimStyle.Render("── Help ──"))
		if m.activeInput < len(fieldHelpText) {
			for _, helpLine := range wrapText(fieldHelpText[m.activeInput], rightColWidth) {
				rightLines = append(rightLines, helpStyle.Render(helpLine))
			}
		}
	}

	// Render side-by-side.
	maxRows := len(leftLines)
	if len(rightLines) > maxRows {
		maxRows = len(rightLines)
	}

	for row := 0; row < maxRows; row++ {
		left := ""
		if row < len(leftLines) {
			left = leftLines[row]
		}
		right := ""
		if row < len(rightLines) {
			right = rightLines[row]
		}

		// Pad left column so right column starts at a consistent position.
		leftPlain := stripANSI(left)
		pad := leftColWidth - len(leftPlain)
		if pad < 2 {
			pad = 2
		}
		b.WriteString(left)
		b.WriteString(strings.Repeat(" ", pad))
		b.WriteString(right)
		b.WriteString("\n")
	}
}

// Deprecated view helpers — replaced by viewQueue in the lazygit layout.
// Kept as no-ops for compilation; will be removed in cleanup pass.
func (m *Model) viewPricing(b *strings.Builder)     {}
func (m *Model) viewConfirm(b *strings.Builder)     {}
func (m *Model) viewSubmitting(b *strings.Builder)  {}
func (m *Model) viewPolling(b *strings.Builder)     {}
func (m *Model) viewDownloading(b *strings.Builder) {}
func (m *Model) viewDone(b *strings.Builder)        {}

// ── Key Handling ─────────────────────────────────────────────────────────────

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	switch key {
	case "ctrl+c":
		return m, tea.Quit
	case "esc":
		if m.inPresetsPane {
			m.inPresetsPane = false
			return m, nil
		}
		return m, tea.Quit
	}

	// Intercept y/n when a job is awaiting confirmation.
	if key == "y" || key == "Y" || key == "n" || key == "N" {
		for _, job := range m.jobs {
			if job.Status == JobConfirming {
				if key == "y" || key == "Y" {
					job.Status = JobSubmitting
					return m, submitCmd(m.client, job.ID, job.req)
				}
				job.Status = JobFailed
				job.ErrMsg = "cancelled"
				return m, nil
			}
		}
	}

	// All config navigation — tab, shift+tab, up, down, enter, presets.
	return m.handleConfigKey(msg)
}

func (m Model) handleConfigKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	// Right-pane preset controls — active when browsing model, sampler,
	// or aspect ratio presets.
	if isPresetsField(m.activeInput) && m.inPresetsPane {
		presetsLen := len(modelPresets)
		switch m.activeInput {
		case fiFluxMode:
			presetsLen = len(fluxModePresets)
		case fiSampler:
			presetsLen = len(m.filteredSamplerPresets())
		case fiScheduler:
			presetsLen = len(m.filteredSchedulerPresets())
		case fiAspectRatio:
			presetsLen = len(aspectRatioPresets)
		case fiOutputFormat:
			presetsLen = len(outputFormatPresets)
		case fiDraft:
			presetsLen = len(draftPresets)
		case fiExperimental:
			presetsLen = len(experimentalPresets)
		case fiFluxUltraRaw:
			presetsLen = len(fluxUltraRawPresets)
		}
		switch key {
		case "up", "k":
			m.activePreset = (m.activePreset - 1 + presetsLen) % presetsLen
			return m, nil
		case "down", "j":
			m.activePreset = (m.activePreset + 1) % presetsLen
			return m, nil
		case "left", "esc", "tab":
			m.inPresetsPane = false
			return m, nil
		case "enter":
			switch m.activeInput {
			case fiModel:
				m.inputs[fiModel].SetValue(modelPresets[m.activePreset].AIR)
				m.adjustFieldsForEcosystem()
				m.refocusIfHidden()
			case fiFluxMode:
				m.inputs[fiFluxMode].SetValue(fluxModePresets[m.activePreset].URN)
			case fiSampler:
				visible := m.filteredSamplerPresets()
				if m.activePreset < len(visible) {
					eco := m.currentEcosystem()
					var val string
					switch eco {
					case "flux2":
						val = visible[m.activePreset].ValFlux2
					case "zimage":
						val = visible[m.activePreset].ValZImage
					default:
						val = visible[m.activePreset].ValSD
					}
					m.inputs[fiSampler].SetValue(val)
				}
			case fiScheduler:
				visible := m.filteredSchedulerPresets()
				if m.activePreset < len(visible) {
					m.inputs[fiScheduler].SetValue(visible[m.activePreset].Scheduler)
				}
			case fiAspectRatio:
				p := aspectRatioPresets[m.activePreset]
				m.inputs[fiAspectRatio].SetValue(p.Ratio)
				m.inputs[fiWidth].SetValue(strconv.Itoa(p.Width))
				m.inputs[fiHeight].SetValue(strconv.Itoa(p.Height))
			case fiOutputFormat:
				m.inputs[fiOutputFormat].SetValue(outputFormatPresets[m.activePreset].Format)
			case fiDraft:
				m.inputs[fiDraft].SetValue(draftPresets[m.activePreset].Name)
			case fiExperimental:
				m.inputs[fiExperimental].SetValue(experimentalPresets[m.activePreset].Name)
			case fiFluxUltraRaw:
				m.inputs[fiFluxUltraRaw].SetValue(fluxUltraRawPresets[m.activePreset].Name)
			}
			m.inPresetsPane = false
			return m, nil
		default:
			return m, nil // block typing when in right panel
		}
	}

	// Right arrow on a presets field enters the presets pane.
	if isPresetsField(m.activeInput) && !m.inPresetsPane && key == "right" {
		m.inPresetsPane = true
		m.activePreset = 0
		return m, nil
	}

	// Presets fields are select-only — block free-form typing.
	// Only navigation keys (tab/up/down/enter) pass through; typing
	// is eaten. Use right arrow to browse and apply presets.
	if isPresetsField(m.activeInput) && !m.inPresetsPane {
		switch key {
		case "tab", "down", "shift+tab", "up", "enter":
			// navigation and form submit pass through
		default:
			return m, nil // eat the keystroke
		}
	}

	switch key {
	case "tab", "down":
		// Blur current, focus next visible field.
		m.inputs[m.activeInput].Blur()
		startInput := m.activeInput
		for {
			m.activeInput = (m.activeInput + 1) % numFormFields
			if m.isFieldVisible(m.activeInput) {
				break
			}
			if m.activeInput == startInput {
				break // safety exit: all fields hidden
			}
		}
		m.inputs[m.activeInput].Focus()
		m.justFocused = isReplaceOnFocus(m.activeInput)
		return m, nil

	case "shift+tab", "up":
		// Blur current, focus previous visible field.
		m.inputs[m.activeInput].Blur()
		startInput := m.activeInput
		for {
			m.activeInput = (m.activeInput - 1 + numFormFields) % numFormFields
			if m.isFieldVisible(m.activeInput) {
				break
			}
			if m.activeInput == startInput {
				break // safety exit
			}
		}
		m.inputs[m.activeInput].Focus()
		m.justFocused = isReplaceOnFocus(m.activeInput)
		return m, nil

	case "enter":
		// Blur current input, validate, transition.
		m.inputs[m.activeInput].Blur()

		if m.inputs[fiPrompt].Value() == "" {
			m.errMsg = "prompt is required"
			m.inputs[m.activeInput].Focus()
			return m, nil
		}
		// Block numeric junk before it hits the wire.
		if err := m.validateNumericFields(); err != nil {
			m.errMsg = err.Error()
			m.inputs[m.activeInput].Focus()
			return m, nil
		}
		m.errMsg = ""
		job := m.createJob()
		m.jobs = append(m.jobs, job)
		return m, priceCmd(m.client, job.ID, job.req)

	default:
		// Pass keystroke to active textinput.
		//
		// Type-to-replace: on a newly-focused numeric field, the first
		// printable keystroke clears the old value so you can type over
		// it instead of appending. Backspace/delete pass through so you
		// can edit the existing value if you prefer.
		if m.justFocused && len(msg.Runes) > 0 {
			m.inputs[m.activeInput].SetValue("")
			m.inputs[m.activeInput].SetCursor(0)
		}
		m.justFocused = false

		// textinput.Validate sets Err but does NOT reject keystrokes —
		// the rune always lands in the value. To actually block invalid
		// characters, we capture pre-update state and rollback if the
		// input's Validate callback produced an error.
		oldValue := m.inputs[m.activeInput].Value()
		oldPos := m.inputs[m.activeInput].Position()

		var cmd tea.Cmd
		m.inputs[m.activeInput], cmd = m.inputs[m.activeInput].Update(msg)

		if m.inputs[m.activeInput].Err != nil {
			m.errMsg = m.inputs[m.activeInput].Err.Error()
			m.inputs[m.activeInput].SetValue(oldValue)
			m.inputs[m.activeInput].SetCursor(oldPos)
			m.inputs[m.activeInput].Err = nil
		}

		// When width or height changes, check if the new dimensions
		// match any aspect ratio preset (by ratio, not exact pixels).
		// Auto-update the aspect ratio field so it "cheers up" when
		// the user types a valid ratio, even if it's different from
		// the one they originally selected.
		if m.activeInput == fiWidth || m.activeInput == fiHeight {
			m.syncAspectRatio()
		}

		return m, cmd
	}
}

// syncAspectRatio checks whether the current width/height maintain any
// known aspect ratio and updates the aspect ratio field to match.
func (m *Model) syncAspectRatio() {
	w, errW := strconv.Atoi(m.inputs[fiWidth].Value())
	h, errH := strconv.Atoi(m.inputs[fiHeight].Value())
	if errW != nil || errH != nil || h == 0 {
		return
	}
	currentRatio := float64(w) / float64(h)
	for _, p := range aspectRatioPresets {
		presetRatio := float64(p.Width) / float64(p.Height)
		diff := currentRatio - presetRatio
		if diff < 0 {
			diff = -diff
		}
		if diff < 0.01 {
			m.inputs[fiAspectRatio].SetValue(p.Ratio)
			return
		}
	}
}

// filteredSchedulerPresets returns all scheduler presets. The new sampler-based
// values are ecosystem-independent — all 23 valid schedulers work across all models.
// The model-aware filtering (zImageOK, flux2KleinOK) was for the deprecated noise
// schedule names and has been removed.
func (m *Model) filteredSchedulerPresets() []SchedulerPreset {
	return schedulerPresets
}

func (m Model) handleConfirmKey(key string) (tea.Model, tea.Cmd) {
	// Confirm phase removed — jobs auto-submit after pricing.
	// Left as no-op for backward compat; never called in lazygit layout.
	return m, nil
}

// ── Async Commands ───────────────────────────────────────────────────────────

// priceResultMsg is sent when CalculatePrice completes.
type priceResultMsg struct {
	jobID string
	cost  int
	err   error
}

func priceCmd(client *civit.Client, jobID string, req civit.GenerationRequest) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		cost, err := client.CalculatePrice(ctx, req)
		return priceResultMsg{jobID: jobID, cost: cost, err: err}
	}
}

// submitResultMsg is sent when SubmitJob completes.
type submitResultMsg struct {
	jobID    string
	apiJobID string
	err      error
}

func submitCmd(client *civit.Client, jobID string, req civit.GenerationRequest) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		apiJobID, err := client.SubmitJob(ctx, req)
		return submitResultMsg{jobID: jobID, apiJobID: apiJobID, err: err}
	}
}

// pollResultMsg is sent each time PollJobStatus completes.
type pollResultMsg struct {
	jobID string
	resp  *civit.WorkflowResponse
	err   error
}

func pollCmd(client *civit.Client, jobID string, apiJobID string) tea.Cmd {
	return func() tea.Msg {
		time.Sleep(2 * time.Second)
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		resp, err := client.PollJobStatus(ctx, apiJobID)
		return pollResultMsg{jobID: jobID, resp: resp, err: err}
	}
}

// downloadResultMsg is sent each time an image download completes.
type downloadResultMsg struct {
	jobID string
	path  string
	err   error
}

func downloadCmd(client *civit.Client, jobID string, resp *civit.WorkflowResponse) tea.Cmd {
	// Collect all image URLs from completed steps.
	var images []civit.Image
	for _, step := range resp.Steps {
		if step.Output != nil {
			images = append(images, step.Output.Images...)
		}
	}
	if len(images) == 0 {
		return func() tea.Msg {
			return downloadResultMsg{err: fmt.Errorf("no images in completed workflow")}
		}
	}

	// Dispatch each download as an independent concurrent tea.Cmd.
	// tea.Batch runs them in parallel and each completion delivers its
	// own downloadResultMsg back to Update — no more single-result stall.
	var cmds []tea.Cmd
	for i, img := range images {
		img := img // capture loop variable
		idx := i
		cmds = append(cmds, func() tea.Msg {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()
			dest := fmt.Sprintf("civitai_output_%s_%d.png", resp.ID[:min(8, len(resp.ID))], idx+1)
			err := client.DownloadImage(ctx, img.URL, dest)
			return downloadResultMsg{jobID: jobID, path: dest, err: err}
		})
	}
	return tea.Batch(cmds...)
}

// tickMsg is sent on every frame to drive the spinner animation.
type tickMsg struct{}

func tickCmd() tea.Cmd {
	return tea.Tick(time.Second/8, func(t time.Time) tea.Msg {
		return tickMsg{}
	})
}

// ── Styles ───────────────────────────────────────────────────────────────────

// Terminal escape code helpers for non-textinput styling.
// textinput models use lipgloss for their internal cursor/value styling.
type style string

func (s style) Render(text string) string {
	return string(s) + text + "\033[0m"
}

var (
	headerStyle     style = "\033[1;37;44m" // bold white on blue
	phaseStyle      style = "\033[1;30;47m" // bold black on white
	fieldLabelStyle style = "\033[37m"      // white
	cursorStyle     style = "\033[1;33m"    // bold yellow
	dimStyle        style = "\033[2;37m"    // dim white
	loadingStyle    style = "\033[33m"      // yellow
	costStyle       style = "\033[1;33m"    // bold yellow
	keyStyle        style = "\033[1;37m"    // bold white
	statusStyle     style = "\033[1;36m"    // bold cyan
	successStyle    style = "\033[1;32m"    // bold green
	errorStyle      style = "\033[1;31m"    // bold red
	footerStyle     style = "\033[2;37m"    // dim white
	canvasStyle     style = "\033[34m"      // blue
	helpStyle       style = "\033[36m"      // cyan
	presetStyle     style = "\033[1;35m"    // bold magenta

	// textInputStyle wraps the textinput's built-in rendering so it
	// inherits our terminal color scheme.
	textInputStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("15"))

	// LazyGit-style bordered panels.
	frameStyle   = lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(lipgloss.Color("24")).Padding(0, 1)
	sectionStyle = lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(lipgloss.Color("24")).Padding(0, 1)
)
