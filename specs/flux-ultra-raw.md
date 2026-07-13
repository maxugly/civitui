# Specification: Flux Ultra Raw (flux-ultra-raw)

This specification covers the implementation of parameter #12 (`fluxUltraRaw`) from the API parameters audit ([specs/api_parameters_audit.md](file:///home/m/snc/cod/civitui/specs/api_parameters_audit.md)).

Flux Ultra Raw toggles raw mode for Flux Ultra generations:
- Helps bypass heavy default aesthetic biases to produce more photorealistic/natural results.
- Implemented as a boolean toggle (presets: `true`/`false`).
- Only visible and applicable when the base model is Flux and `fluxMode` is set to `"Ultra"` (`urn:air:flux1:checkpoint:civitai:618692@1088507`).

---

## 1. Field Re-indexing

Adding all Tier 2 fields bumps `numFormFields` from `15` to `21`. `fiFluxUltraRaw` is located at index `20`:

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
`isPresetsField` must be updated to return true for `fiFluxUltraRaw`:
```go
func isPresetsField(idx int) bool {
	return idx == fiModel || idx == fiFluxMode || idx == fiSampler ||
		idx == fiScheduler || idx == fiAspectRatio || idx == fiOutputFormat ||
		idx == fiDraft || idx == fiExperimental || idx == fiFluxUltraRaw
}
```

### 1b. Replace-on-Focus Verification
`isReplaceOnFocus` remains false for `fiFluxUltraRaw` as it is a presets field:
```go
func isReplaceOnFocus(idx int) bool {
	return (idx >= fiWidth && idx <= fiSeed) || idx == fiDenoise || 
		idx == fiClipSkip || idx == fiUpscaleWidth || idx == fiUpscaleHeight
}
```

---

## 2. Core Engine Changes (`pkg/civit/civit.go`)

### 2a. GenerationRequest Field Mapping
Add `FluxUltraRaw` as an optional boolean pointer with `omitempty` tag to `GenerationRequest` in `pkg/civit/civit.go`:

```go
type GenerationRequest struct {
	// ... existing fields ...
	Draft          bool         `json:"draft"`
	SourceImage    *SourceImage `json:"sourceImage,omitempty"`
	Denoise        *float64     `json:"denoise,omitempty"`
	ClipSkip       *int         `json:"clipSkip,omitempty"`
	UpscaleWidth   *int         `json:"upscaleWidth,omitempty"`
	UpscaleHeight  *int         `json:"upscaleHeight,omitempty"`
	Experimental   *bool        `json:"experimental,omitempty"`
	FluxUltraRaw   *bool        `json:"fluxUltraRaw,omitempty"` // Add this field
}
```

---

## 3. UI Layer Changes (`internal/ui/ui.go`)

### 3a. Presets Definition
Define the `fluxUltraRawPresets` mapping structure and preset list to control the toggle:

```go
type FluxUltraRawPreset struct {
	Name        string
	Description string
	Value       bool
}

var fluxUltraRawPresets = []FluxUltraRawPreset{
	{
		Name:        "false",
		Description: "Ultra aesthetic. Standard stylized output. Default.",
		Value:       false,
	},
	{
		Name:        "true",
		Description: "Raw Ultra mode. Bypasses stylized aesthetics for more realistic textures.",
		Value:       true,
	},
}
```

### 3b. Visibility Helper
Extend the `isFieldVisible` helper method in `internal/ui/ui.go` to hide the field when Flux Mode is not set to Ultra:

```go
func (m *Model) isFieldVisible(idx int) bool {
	switch idx {
	case fiDenoise:
		return m.sourceImage != nil
	case fiClipSkip:
		modelVal := strings.ToLower(m.inputs[fiModel].Value())
		isFlux := strings.Contains(modelVal, "flux")
		isZImage := strings.Contains(modelVal, "zimage")
		return !isFlux && !isZImage
	case fiUpscaleWidth, fiUpscaleHeight:
		return true
	case fiFluxUltraRaw:
		// Only show when the flux mode is specifically set to the Ultra URN.
		return m.inputs[fiFluxMode].Value() == "urn:air:flux1:checkpoint:civitai:618692@1088507"
	}
	return true
}
```

### 3c. Field Label and Help Text
Add the field label to `fieldLabels`:
```go
// In fieldLabels slice:
/* fiFluxUltraRaw */ "Flux Ultra Raw",
```

Add the help text to `fieldHelpText`:
```go
// In fieldHelpText slice:
/* fiFluxUltraRaw */ "Enable Raw mode for Flux Ultra to produce less-stylized images. Only visible when Flux Mode is set to Ultra. Use Right Arrow.",
```

### 3d. Inputs Initialization (`NewModel`)
Initialize the field's text input inside `NewModel`:
```go
inputs[fiFluxUltraRaw] = newTextInput("false", "false", inputWidth)
```

### 3e. Preset Pane Rendering (`viewConfig`)
In the presets rendering switch block inside `viewConfig`, handle rendering the experimental presets pane:
```go
		case fiFluxUltraRaw:
			rightLines = append(rightLines, dimStyle.Render("── Flux Ultra Raw ──"))
			for _, preset := range fluxUltraRawPresets {
				presetsList = append(presetsList, struct {
					Name        string
					Description string
				}{preset.Name, preset.Description})
			}
```

### 3f. Key Handling (`handleConfigKey`)
In the config key handler, support navigation length and selecting values for Flux Ultra Raw presets:
```go
		case fiFluxUltraRaw:
			presetsLen = len(fluxUltraRawPresets)
```
And in the `case "enter":` selection handler:
```go
			case fiFluxUltraRaw:
				m.inputs[fiFluxUltraRaw].SetValue(fluxUltraRawPresets[m.activePreset].Name)
```

### 3g. Request Mapping (`toRequest`)
Map `inputs[fiFluxUltraRaw]` to `FluxUltraRaw` boolean pointer in `toRequest`:
```go
	var fluxUltraRaw *bool
	if m.isFieldVisible(fiFluxUltraRaw) {
		val := m.inputs[fiFluxUltraRaw].Value() == "true"
		fluxUltraRaw = &val
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
		FluxUltraRaw:  fluxUltraRaw,
	}
```

---

## 4. Quality Gates & Testing

1. Run `go test ./pkg/civit/...` to verify serialization and logic.
2. Run `go build ./...` and `go vet ./...` to ensure clean compilation.
