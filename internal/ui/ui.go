// Package ui provides the Bubble Tea terminal user interface for civitui.
//
// The UI implements a 7-phase state machine that walks the user through
// the CivitAI generation pipeline:
//
//	config → pricing → confirm → submitting → polling → downloading → done
//
// Each phase has its own view and input handling. Async API calls are
// dispatched as tea.Cmd and results arrive as typed messages.
//
// Form input uses Charm's bubbles/textinput for ergonomic text editing
// (cursor movement, home/end, backspace, selection) while preserving
// full layout control for split-pane rendering.
package ui

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/m/civitui/pkg/civit"
)

// ── Phase ────────────────────────────────────────────────────────────────────

// Phase is a step in the generation pipeline state machine.
type Phase int

const (
	PhaseConfig      Phase = iota // user fills in generation parameters
	PhasePricing                  // waiting for CalculatePrice
	PhaseConfirm                  // showing cost, awaiting y/n
	PhaseSubmitting               // waiting for SubmitJob
	PhasePolling                  // polling job status
	PhaseDownloading              // downloading completed images
	PhaseDone                     // showing results
)

// String returns a human-readable phase label.
func (p Phase) String() string {
	switch p {
	case PhaseConfig:
		return "CONFIG"
	case PhasePricing:
		return "PRICING"
	case PhaseConfirm:
		return "CONFIRM"
	case PhaseSubmitting:
		return "SUBMITTING"
	case PhasePolling:
		return "POLLING"
	case PhaseDownloading:
		return "DOWNLOADING"
	case PhaseDone:
		return "DONE"
	default:
		return "UNKNOWN"
	}
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

	// phase tracks the current step in the generation pipeline.
	phase Phase

	// ── Configuration Panel (form fields) ──
	//
	// inputs holds all 11 textinput models for the config form.
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

	// ── Pipeline state ──

	cost     int                    // buzz cost from whatif
	jobID    string                 // workflow ID from SubmitJob
	pollResp *civit.WorkflowResponse // latest poll result
	pollTick int                    // poll iteration counter

	// ── Results ──

	downloadPaths  []string
	downloadErrors []string
	errMsg         string

	// ── Terminal geometry ──

	termWidth  int
	termHeight int

	// ── Spinner ──

	spinnerFrame int
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

// SchedulerPreset selects a noise schedule. Model-aware: ZImage only
// supports simple+discrete; Flux2Klein supports all except ays.
type SchedulerPreset struct {
	Name         string
	Description  string
	Scheduler    string
	zImageOK     bool
	flux2KleinOK bool
}

var schedulerPresets = []SchedulerPreset{
	{
		Name:         "Simple",
		Description:  "Standard uniform schedule. Works everywhere. Default.",
		Scheduler:    "simple",
		zImageOK:     true,
		flux2KleinOK: true,
	},
	{
		Name:         "Discrete",
		Description:  "Discrete timesteps. Compatible with ZImage and Flux2Klein.",
		Scheduler:    "discrete",
		zImageOK:     true,
		flux2KleinOK: true,
	},
	{
		Name:         "Karras",
		Description:  "Karras noise schedule. Better quality at same step count. Not for ZImage.",
		Scheduler:    "karras",
		zImageOK:     false,
		flux2KleinOK: true,
	},
	{
		Name:         "Exponential",
		Description:  "Exponential schedule. Fast convergence. Not for ZImage.",
		Scheduler:    "exponential",
		zImageOK:     false,
		flux2KleinOK: true,
	},
	{
		Name:         "AYS",
		Description:  "Align Your Steps. Best for SDXL/Flux1. Crashes Flux2Klein — hidden for that model.",
		Scheduler:    "ays",
		zImageOK:     false,
		flux2KleinOK: false,
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
		return m.inputs[fiFluxMode].Value() == "urn:air:flux1:checkpoint:civitai:618692@1088507"
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
	val := m.inputs[fiModel].Value()
	for _, preset := range modelPresets {
		if preset.AIR == val {
			return preset.Ecosystem
		}
	}
	valLower := strings.ToLower(val)
	if strings.Contains(valLower, "flux1") {
		return "flux1"
	}
	if strings.Contains(valLower, "flux2") || strings.Contains(valLower, "klein") {
		return "flux2"
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

// maxCFGScale returns the maximum CFG scale allowed for the given ecosystem.
func maxCFGScale(eco string) float64 {
	if eco == "flux1" || eco == "flux2" {
		return 7.0
	}
	return 30.0
}
// Index aligns with the fi* constants.
var fieldHelpText = []string{
	/* fiPrompt */ "Describe the image you want to generate. Be specific and descriptive. E.g. 'a photo of an astronaut on Mars'. (Required)",
	/* fiNegativePrompt */ "Describe things you want to avoid in the generated image. E.g. 'blurry, low quality, distorted'. (Optional)",
	/* fiModel */ "Civitai Model AIR. Represents the base generator model. Use Right Arrow key to select from popular presets.",
	/* fiFluxMode */ "Flux model variant. Only applies when using a Flux1 model. Use Right Arrow to select from presets.",
	/* fiSampler */ "Diffusion sampler algorithm. Controls noise schedule and convergence. Use Right Arrow key to select from presets.",
	/* fiScheduler */ "Noise schedule for the sampler. 'simple' works everywhere. Advanced: discrete, karras, exponential, ays. Use Right Arrow to select.",
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
func NewModel(client *civit.Client) Model {
	inputWidth := 60

	inputs := make([]textinput.Model, numFormFields)
	inputs[fiPrompt] = newTextInput("a majestic cat wearing a top hat", "", inputWidth)
	inputs[fiNegativePrompt] = newTextInput("optional — what to avoid", "", inputWidth)
	inputs[fiModel] = newTextInput("air:flux1:checkpoint:civitai:618692@691639", "air:flux1:checkpoint:civitai:618692@691639", inputWidth)
	inputs[fiFluxMode] = newTextInput("Standard", "urn:air:flux1:checkpoint:civitai:618692@691639", inputWidth)
	inputs[fiSampler] = newTextInput("Euler a", "Euler a", inputWidth)
	inputs[fiScheduler] = newTextInput("simple", "simple", inputWidth)
	inputs[fiAspectRatio] = newTextInput("1:1", "1:1", 14)
	inputs[fiWidth] = newTextInput("1024", "1024", 10)
	inputs[fiHeight] = newTextInput("1024", "1024", 10)
	inputs[fiSteps] = newTextInput("20", "20", 6)
	inputs[fiCFGScale] = newTextInput("7.0", "7.0", 8)
	inputs[fiQuantity] = newTextInput("4", "4", 6)
	inputs[fiOutputFormat] = newTextInput("jpeg", "jpeg", inputWidth)
	inputs[fiSeed] = newTextInput("random", "", 16)
	inputs[fiDraft] = newTextInput("false", "false", inputWidth)
	inputs[fiDenoise] = newTextInput("0.4", "", 8)
	inputs[fiClipSkip] = newTextInput("2", "2", 6)
	inputs[fiUpscaleWidth] = newTextInput("disabled", "", 10)
	inputs[fiUpscaleHeight] = newTextInput("disabled", "", 10)
	inputs[fiExperimental] = newTextInput("false", "false", inputWidth)
	inputs[fiFluxUltraRaw] = newTextInput("false", "false", inputWidth)

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
		maxVal := 30.0
		modelVal := strings.ToLower(inputs[fiModel].Value())
		if strings.Contains(modelVal, "flux") {
			maxVal = 7.0
		}
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

	return Model{
		client:      client,
		phase:       PhaseConfig,
		inputs:      inputs,
		activeInput: 0,
	}
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

// refocusConfig snaps the phase back to config and re-focuses the active input.
// Use this on every return-to-config path (errors, confirm cancel) so the
// user's keystrokes land somewhere instead of vanishing into a blurred field.
func (m *Model) refocusConfig() {
	m.phase = PhaseConfig
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
		// Only return tick command if we're in a phase that needs the spinner.
		if m.phase == PhasePricing || m.phase == PhaseSubmitting ||
			m.phase == PhasePolling || m.phase == PhaseDownloading {
			return m, tickCmd()
		}

	case priceResultMsg:
		if msg.err != nil {
			m.errMsg = msg.err.Error()
			m.refocusConfig()
		} else {
			m.cost = msg.cost
			m.phase = PhaseConfirm
		}
		return m, nil

	case submitResultMsg:
		if msg.err != nil {
			m.errMsg = msg.err.Error()
			m.refocusConfig()
		} else {
			m.jobID = msg.jobID
			m.pollTick = 0
			m.downloadPaths = nil
			m.downloadErrors = nil
			m.phase = PhasePolling
			return m, pollCmd(m.client, msg.jobID)
		}

	case pollResultMsg:
		if msg.err != nil {
			m.errMsg = msg.err.Error()
			m.refocusConfig()
			return m, nil
		}
		m.pollResp = msg.resp
		m.pollTick++

		switch msg.resp.Status {
		case "succeeded":
			m.downloadPaths = nil
			m.downloadErrors = nil
			m.phase = PhaseDownloading
			return m, downloadCmd(m.client, msg.resp)
		case "failed", "cancelled":
			m.errMsg = fmt.Sprintf("job %s", msg.resp.Status)
			m.refocusConfig()
			return m, nil
		default:
			return m, pollCmd(m.client, m.jobID)
		}

	case downloadResultMsg:
		if msg.err != nil {
			m.downloadErrors = append(m.downloadErrors, msg.err.Error())
		} else {
			m.downloadPaths = append(m.downloadPaths, msg.path)
		}
		expected := 0
		if m.pollResp != nil && len(m.pollResp.Steps) > 0 {
			expected = len(m.pollResp.Steps[0].Output.Images)
		}
		done := len(m.downloadPaths) + len(m.downloadErrors)
		if done >= expected && expected > 0 {
			m.phase = PhaseDone
		}
		return m, nil
	}

	// Route Blink and other internal messages to the active textinput
	// during the config phase so cursor blinking works.
	if m.phase == PhaseConfig && !m.inPresetsPane {
		m.inputs[m.activeInput], cmd = m.inputs[m.activeInput].Update(msg)
		return m, cmd
	}

	return m, cmd
}

// View renders the current UI based on the active phase.
func (m Model) View() string {
	var b strings.Builder

	// Header bar.
	b.WriteString(headerStyle.Render(" civitui "))
	b.WriteString("  ")
	b.WriteString(phaseStyle.Render(fmt.Sprintf(" [%s] ", m.phase)))
	b.WriteString("\n\n")

	switch m.phase {
	case PhaseConfig:
		m.viewConfig(&b)
	case PhasePricing:
		m.viewPricing(&b)
	case PhaseConfirm:
		m.viewConfirm(&b)
	case PhaseSubmitting:
		m.viewSubmitting(&b)
	case PhasePolling:
		m.viewPolling(&b)
	case PhaseDownloading:
		m.viewDownloading(&b)
	case PhaseDone:
		m.viewDone(&b)
	}

	// Error bar.
	if m.errMsg != "" {
		b.WriteString("\n")
		b.WriteString(errorStyle.Render(" ✗ " + m.errMsg))
	}

	// Footer.
	b.WriteString(fmt.Sprintf("\n\n%s", footerStyle.Render("ctrl+c quit  •  tab/↑↓ navigate  •  enter confirm")))

	return b.String()
}

// ── View Helpers ─────────────────────────────────────────────────────────────

func (m *Model) viewConfig(b *strings.Builder) {
	b.WriteString("Configure your generation:\n\n")

	// Left-column width: label + cursor + input, padded for alignment.
	const leftColWidth = 80

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
			for _, descLine := range wrapText(preset.Description, 38) {
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
			for _, helpLine := range wrapText(fieldHelpText[m.activeInput], 40) {
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

func (m *Model) viewPricing(b *strings.Builder) {
	b.WriteString(loadingStyle.Render("  " + spinnerFrames[m.spinnerFrame] + " Calculating price..."))
}

func (m *Model) viewConfirm(b *strings.Builder) {
	b.WriteString(fmt.Sprintf("  This generation will cost %s Buzz.\n\n", costStyle.Render(strconv.Itoa(m.cost))))
	b.WriteString("  Press ")
	b.WriteString(keyStyle.Render("y"))
	b.WriteString(" to confirm, ")
	b.WriteString(keyStyle.Render("n"))
	b.WriteString(" to cancel.")
}

func (m *Model) viewSubmitting(b *strings.Builder) {
	b.WriteString(loadingStyle.Render("  " + spinnerFrames[m.spinnerFrame] + " Submitting job..."))
}

func (m *Model) viewPolling(b *strings.Builder) {
	b.WriteString(loadingStyle.Render("  " + spinnerFrames[m.spinnerFrame] + " Waiting for results"))
	if m.jobID != "" {
		b.WriteString(dimStyle.Render(fmt.Sprintf("  [%s]", m.jobID[:min(8, len(m.jobID))])))
	}
	b.WriteString(fmt.Sprintf("  poll #%d", m.pollTick))

	if m.pollResp != nil {
		b.WriteString(fmt.Sprintf("  status: %s", statusStyle.Render(m.pollResp.Status)))
	}
}

func (m *Model) viewDownloading(b *strings.Builder) {
	b.WriteString(loadingStyle.Render("  " + spinnerFrames[m.spinnerFrame] + " Downloading images..."))
	if len(m.downloadPaths) > 0 {
		b.WriteString(fmt.Sprintf("  %d/%d", len(m.downloadPaths), len(m.downloadPaths)+len(m.downloadErrors)))
	}
}

func (m *Model) viewDone(b *strings.Builder) {
	b.WriteString(successStyle.Render("  ✓ Generation complete!"))
	b.WriteString("\n\n")

	// Canvas / Viewport placeholder showing downloaded images.
	b.WriteString(canvasStyle.Render("┌─ Canvas / Viewport ───────────────────────────────┐"))
	b.WriteString("\n")

	if len(m.downloadPaths) > 0 {
		for i, p := range m.downloadPaths {
			b.WriteString(canvasStyle.Render(fmt.Sprintf("│  [%d] %s", i+1, p)))
			b.WriteString("\n")
		}
		for i := len(m.downloadPaths); i < 6; i++ {
			b.WriteString(canvasStyle.Render("│"))
			b.WriteString("\n")
		}
	} else {
		for i := 0; i < 3; i++ {
			b.WriteString(canvasStyle.Render("│  (image viewport — results appear here)"))
			b.WriteString("\n")
		}
	}
	b.WriteString(canvasStyle.Render("└──────────────────────────────────────────────────┘"))

	if len(m.downloadErrors) > 0 {
		b.WriteString("\n\n")
		b.WriteString(errorStyle.Render(fmt.Sprintf("  %d download(s) failed", len(m.downloadErrors))))
	}
}

// ── Key Handling ─────────────────────────────────────────────────────────────

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	switch key {
	case "ctrl+c":
		return m, tea.Quit
	case "esc":
		switch m.phase {
		case PhaseConfig:
			if m.inPresetsPane {
				m.inPresetsPane = false
				return m, nil
			}
			return m, tea.Quit
		case PhaseConfirm, PhaseDone:
			m.refocusConfig()
			return m, nil
		default:
			return m, tea.Quit
		}
	}

	switch m.phase {
	case PhaseConfig:
		return m.handleConfigKey(msg)
	case PhaseConfirm:
		return m.handleConfirmKey(key)
	case PhaseDone:
		if key == "q" || key == "enter" {
			m.refocusConfig()
			return m, nil
		}
	}

	return m, nil
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
		for {
			m.activeInput = (m.activeInput + 1) % numFormFields
			if m.isFieldVisible(m.activeInput) {
				break
			}
		}
		m.inputs[m.activeInput].Focus()
		m.justFocused = isReplaceOnFocus(m.activeInput)
		return m, nil

	case "shift+tab", "up":
		// Blur current, focus previous visible field.
		m.inputs[m.activeInput].Blur()
		for {
			m.activeInput = (m.activeInput - 1 + numFormFields) % numFormFields
			if m.isFieldVisible(m.activeInput) {
				break
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
		m.phase = PhasePricing
		return m, priceCmd(m.client, m.toRequest())

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

// filteredSchedulerPresets returns the scheduler presets that are valid
// for the currently selected model. ZImage only supports simple+discrete;
// Flux2Klein supports all except ays.
func (m *Model) filteredSchedulerPresets() []SchedulerPreset {
	modelVal := strings.ToLower(m.inputs[fiModel].Value())
	isZImage := strings.Contains(modelVal, "zimage")
	isFlux2Klein := strings.Contains(modelVal, "flux2") ||
		strings.Contains(modelVal, "flux-klein") ||
		strings.Contains(modelVal, "klein")
	var visible []SchedulerPreset
	for _, p := range schedulerPresets {
		if isZImage && !p.zImageOK {
			continue
		}
		if isFlux2Klein && !p.flux2KleinOK {
			continue
		}
		visible = append(visible, p)
	}
	return visible
}

func (m Model) handleConfirmKey(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "y", "Y":
		m.phase = PhaseSubmitting
		return m, submitCmd(m.client, m.toRequest())
	case "n", "N":
		m.refocusConfig()
		return m, nil
	}
	return m, nil
}

// ── Async Commands ───────────────────────────────────────────────────────────

// priceResultMsg is sent when CalculatePrice completes.
type priceResultMsg struct {
	cost int
	err  error
}

func priceCmd(client *civit.Client, req civit.GenerationRequest) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		cost, err := client.CalculatePrice(ctx, req)
		return priceResultMsg{cost: cost, err: err}
	}
}

// submitResultMsg is sent when SubmitJob completes.
type submitResultMsg struct {
	jobID string
	err   error
}

func submitCmd(client *civit.Client, req civit.GenerationRequest) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		jobID, err := client.SubmitJob(ctx, req)
		return submitResultMsg{jobID: jobID, err: err}
	}
}

// pollResultMsg is sent each time PollJobStatus completes.
type pollResultMsg struct {
	resp *civit.WorkflowResponse
	err  error
}

func pollCmd(client *civit.Client, jobID string) tea.Cmd {
	return func() tea.Msg {
		time.Sleep(2 * time.Second)
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		resp, err := client.PollJobStatus(ctx, jobID)
		return pollResultMsg{resp: resp, err: err}
	}
}

// downloadResultMsg is sent each time an image download completes.
type downloadResultMsg struct {
	path string
	err  error
}

func downloadCmd(client *civit.Client, resp *civit.WorkflowResponse) tea.Cmd {
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
			return downloadResultMsg{path: dest, err: err}
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
)
