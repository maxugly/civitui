# Specification: Experimental Mode (experimental)

This specification covers the implementation of parameter #13 (`experimental`) from the API parameters audit ([specs/api_parameters_audit.md](file:///home/m/snc/cod/civitui/specs/api_parameters_audit.md)).

Experimental Mode enables bleeding-edge features for standard SDCPP-supported engines:
- Toggles the `enhancedCompatibility` pipeline on compatible base models.
- Implemented as a boolean toggle (presets: `true`/`false`).
- Placed in the "Advanced Settings" section of the configuration interface.

---

## 1. Field Re-indexing

Adding all Tier 2 fields bumps `numFormFields` from `15` to `21`. `fiExperimental` is located at index `19`:

```go
const numFormFields = 21

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
	fiDenoise               // 15 — float (0.0–1.0, hidden when no source image)
	fiClipSkip              // 16 — int (1–3, SD/SDXL only, default 2)
	fiUpscaleWidth          // 17 — int (320–3840, upscaling workflows)
	fiUpscaleHeight         // 18 — int (320–3840, upscaling workflows)
	fiExperimental          // 19 — boolean (presets: true/false, advanced)
	fiFluxUltraRaw          // 20 — boolean (presets: true/false, Flux Ultra only)
)
```

### 1a. Preset Fields Verification
`isPresetsField` must be updated to return true for `fiExperimental`:
```go
func isPresetsField(idx int) bool {
	return idx == fiModel || idx == fiFluxMode || idx == fiSampler ||
		idx == fiScheduler || idx == fiAspectRatio || idx == fiOutputFormat ||
		idx == fiDraft || idx == fiExperimental || idx == fiFluxUltraRaw
}
```

### 1b. Replace-on-Focus Verification
`isReplaceOnFocus` remains false for `fiExperimental` as it is a presets field:
```go
func isReplaceOnFocus(idx int) bool {
	return (idx >= fiWidth && idx <= fiSeed) || idx == fiDenoise || 
		idx == fiClipSkip || idx == fiUpscaleWidth || idx == fiUpscaleHeight
}
```

---

## 2. Core Engine Changes (`pkg/civit/civit.go`)

### 2a. GenerationRequest Field Mapping
Add `Experimental` as an optional boolean pointer with `omitempty` tag to `GenerationRequest` in `pkg/civit/civit.go`:

```go
type GenerationRequest struct {
	// ... existing fields ...
	Draft          bool         `json:"draft"`
	SourceImage    *SourceImage `json:"sourceImage,omitempty"`
	Denoise        *float64     `json:"denoise,omitempty"`
	ClipSkip       *int         `json:"clipSkip,omitempty"`
	UpscaleWidth   *int         `json:"upscaleWidth,omitempty"`
	UpscaleHeight  *int         `json:"upscaleHeight,omitempty"`
	Experimental   *bool        `json:"experimental,omitempty"` // Add this field
}
```

---

## 3. UI Layer Changes (`internal/ui/ui.go`)

### 3a. Presets Definition
Define the `experimentalPresets` mapping structure and preset list to control the toggle:

```go
type ExperimentalPreset struct {
	Name        string
	Description string
	Value       bool
}

var experimentalPresets = []ExperimentalPreset{
	{
		Name:        "false",
		Description: "Standard generation. Experimental features are disabled. Default.",
		Value:       false,
	},
	{
		Name:        "true",
		Description: "Enable SDCPP experimental features (enhanced compatibility mode). Use with care.",
		Value:       true,
	},
}
```

### 3b. Field Label and Help Text
Add the field label to `fieldLabels`:
```go
// In fieldLabels slice:
/* fiExperimental */ "Experimental Mode",
```

Add the help text to `fieldHelpText`:
```go
// In fieldHelpText slice:
/* fiExperimental */ "Enable experimental SDCPP features and compatibility overrides. Use Right Arrow to select.",
```

### 3c. Inputs Initialization (`NewModel`)
Initialize the field's text input inside `NewModel`:
```go
inputs[fiExperimental] = newTextInput("false", "false", inputWidth)
```

### 3d. Dynamic advanced section UI separator
In `viewConfig`, render a visual separator line before the first advanced field (e.g. `fiScheduler` or `fiExperimental`) in the left-hand column:

```go
// In viewConfig Rendering Loop:
	for i, input := range m.inputs {
		if !m.isFieldVisible(i) {
			continue
		}
		if i == fiScheduler {
			leftLines = append(leftLines, "")
			leftLines = append(leftLines, dimStyle.Render("  ── Advanced Settings ──"))
		}
        
		// Label and input rendering ...
	}
```

### 3e. Preset Pane Rendering (`viewConfig`)
In the presets rendering switch block inside `viewConfig`, handle rendering the experimental presets pane:
```go
		case fiExperimental:
			rightLines = append(rightLines, dimStyle.Render("── Experimental Mode ──"))
			for _, preset := range experimentalPresets {
				presetsList = append(presetsList, struct {
					Name        string
					Description string
				}{preset.Name, preset.Description})
			}
```

### 3f. Key Handling (`handleConfigKey`)
In the config key handler, support navigation length and selecting values for experimental presets:
```go
		case fiExperimental:
			presetsLen = len(experimentalPresets)
```
And in the `case "enter":` selection handler:
```go
			case fiExperimental:
				m.inputs[fiExperimental].SetValue(experimentalPresets[m.activePreset].Name)
```

### 3g. Request Mapping (`toRequest`)
Map `inputs[fiExperimental]` to `Experimental` boolean pointer in `toRequest`:
```go
	var experimental *bool
	if m.isFieldVisible(fiExperimental) {
		val := m.inputs[fiExperimental].Value() == "true"
		experimental = &val
	}
```
And pass it inside the returned `civit.GenerationRequest`:
```go
	return civit.GenerationRequest{
		// ... existing fields ...
		SourceImage:   m.sourceImage,
		Denoise:       denoise,
		ClipSkip:      clipSkip,
		UpscaleWidth:  upscaleW,
		UpscaleHeight: upscaleH,
		Experimental:  experimental,
	}
```

---

## 4. Quality Gates & Testing

1. Run `go test ./pkg/civit/...` to verify serialization and logic.
2. Run `go build ./...` and `go vet ./...` to ensure clean compilation.
