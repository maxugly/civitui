# Specification: Draft Toggle

This specification covers the implementation of parameter #4 (`draft`) from the API parameters audit ([specs/api_parameters_audit.md](file:///home/m/snc/cod/civitui/specs/api_parameters_audit.md)).

Draft mode is a "speed over quality" toggle. When enabled (`true`), the orchestrator:
- Injects draft LoRA resources (SD1: id `424706`, SDXL: id `391999`)
- Radically reduces denoising steps to `6-8`
- Sets CFG scale to `1`

---

## 1. Field Re-indexing

Adding the `draft` toggle bumps `numFormFields` from `14` to `15`. We will append it as the last field at index `14`:

```go
const numFormFields = 15

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
)
```

### 1a. Preset Fields Verification
`isPresetsField` must be updated to return true for `fiDraft`:
```go
func isPresetsField(idx int) bool {
	return idx == fiModel || idx == fiFluxMode || idx == fiSampler ||
		idx == fiScheduler || idx == fiAspectRatio || idx == fiOutputFormat ||
		idx == fiDraft
}
```

### 1b. Replace-on-Focus Verification
`isReplaceOnFocus` remains unchanged as it only concerns numeric fields (indices 7–13):
```go
func isReplaceOnFocus(idx int) bool {
	return idx >= fiWidth && idx <= fiSeed
}
```

---

## 2. Core Engine Changes (`pkg/civit/civit.go`)

Add `Draft` parameter to `GenerationRequest` struct.

```go
type GenerationRequest struct {
	// ... existing fields ...
	OutputFormat   string  `json:"outputFormat,omitempty"`
	Seed           *int64  `json:"seed,omitempty"`
	Draft          bool    `json:"draft"` // Add this field
}
```

---

## 3. UI Layer Changes (`internal/ui/ui.go`)

### 3a. Preset Definitions
Define the `draftPresets` mapping structure and preset list to control the toggle:

```go
// DraftPreset selects the draft mode value.
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
```

### 3b. Field Label and Help Text
Add the field label at the end of the `fieldLabels` slice:
```go
var fieldLabels = []string{
	// ... existing labels ...
	"Draft Mode (fast preview)",
}
```

Add the help text at the end of the `fieldHelpText` slice:
```go
var fieldHelpText = []string{
	// ... existing help strings ...
	/* fiDraft */ "Enable Draft Mode for fast previews (speed over quality). Injects draft LoRAs, reduces steps, sets CFG to 1. Use Right Arrow.",
}
```

### 3c. Inputs Initialization (`NewModel`)
Initialize the field's text input inside `NewModel`:
```go
inputs[fiDraft] = newTextInput("false", "false", inputWidth)
```

### 3d. Preset Pane Rendering (`viewConfig`)
In the presets rendering switch block, handle rendering the draft presets pane:
```go
		case fiDraft:
			rightLines = append(rightLines, dimStyle.Render("── Draft Mode ──"))
			for _, preset := range draftPresets {
				presetsList = append(presetsList, struct {
					Name        string
					Description string
				}{preset.Name, preset.Description})
			}
```

### 3e. Key Handling (`handleConfigKey`)
In the config key handler, support navigation length and selecting values for draft presets:
```go
		switch m.activeInput {
		// ... existing cases ...
		case fiDraft:
			presetsLen = len(draftPresets)
		}
```
And in the `case "enter":` selection handler:
```go
			case fiDraft:
				m.inputs[fiDraft].SetValue(draftPresets[m.activePreset].Name)
```

### 3f. Request Mapping (`toRequest`)
Map the text value of `inputs[fiDraft]` to the boolean field in the `civit.GenerationRequest`:
```go
	return civit.GenerationRequest{
		// ... existing mapping ...
		Seed:         seed,
		Draft:        m.inputs[fiDraft].Value() == "true",
	}
```

---

## 4. Quality Gates & Testing

To verify the implementation:
1. Run `go test ./pkg/civit/...` to verify serialization and logic.
2. Run `go build ./...` and `go vet ./...` to ensure clean compilation and verify there are no warnings or errors.
