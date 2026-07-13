# Specification: Upscale Dimensions (upscale)

This specification covers the implementation of parameters #8 (`upscaleWidth`) and #9 (`upscaleHeight`) from the API parameters audit ([specs/api_parameters_audit.md](file:///home/m/snc/cod/civitui/specs/api_parameters_audit.md)).

Upscaling dimensions tell the orchestrator to upscale the output image:
- Range: `320–3840` (paired integers).
- These are optional fields. Leaving them empty disables the upscaling pipeline.
- Visible for all standard models (SD 1.5, SDXL, Flux, ZImage).

---

## 1. Field Re-indexing

Adding all Tier 2 fields bumps `numFormFields` from `15` to `21`. `fiUpscaleWidth` and `fiUpscaleHeight` are located at indices `17` and `18` respectively:

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
`isPresetsField` remains false for the upscaling fields as they accept free-form numeric entries:
```go
func isPresetsField(idx int) bool {
	return idx == fiModel || idx == fiFluxMode || idx == fiSampler ||
		idx == fiScheduler || idx == fiAspectRatio || idx == fiOutputFormat ||
		idx == fiDraft || idx == fiExperimental || idx == fiFluxUltraRaw
}
```

### 1b. Replace-on-Focus Verification
`isReplaceOnFocus` must return true for upscaling fields:
```go
func isReplaceOnFocus(idx int) bool {
	return (idx >= fiWidth && idx <= fiSeed) || idx == fiDenoise || 
		idx == fiClipSkip || idx == fiUpscaleWidth || idx == fiUpscaleHeight
}
```

---

## 2. Core Engine Changes (`pkg/civit/civit.go`)

### 2a. GenerationRequest Field Mapping
Add `UpscaleWidth` and `UpscaleHeight` as optional integer pointers to `GenerationRequest` in `pkg/civit/civit.go`:

```go
type GenerationRequest struct {
	// ... existing fields ...
	Draft          bool         `json:"draft"`
	SourceImage    *SourceImage `json:"sourceImage,omitempty"`
	Denoise        *float64     `json:"denoise,omitempty"`
	ClipSkip       *int         `json:"clipSkip,omitempty"`
	UpscaleWidth   *int         `json:"upscaleWidth,omitempty"`  // Add this field
	UpscaleHeight  *int         `json:"upscaleHeight,omitempty"` // Add this field
}
```

---

## 3. UI Layer Changes (`internal/ui/ui.go`)

### 3a. Visibility Helper
Upscale fields are visible for all standard models, but we specify a visibility handler that will control it dynamically if needed:

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
		return true // Always visible for standard workflows
	}
	return true
}
```

### 3b. Field Label and Help Text
Add the field labels to `fieldLabels`:
```go
// In fieldLabels slice:
/* fiUpscaleWidth */ "Upscale Width",
/* fiUpscaleHeight */ "Upscale Height",
```

Add the help text to `fieldHelpText`:
```go
// In fieldHelpText slice:
/* fiUpscaleWidth */ "Target width for output upscaling. Range: 320–3840. Leave empty/disabled to bypass upscaling.",
/* fiUpscaleHeight */ "Target height for output upscaling. Range: 320–3840. Leave empty/disabled to bypass upscaling.",
```

### 3c. Inputs Initialization and Validation (`NewModel`)
Initialize the field inputs and integer validators inside `NewModel`. Use `"disabled"` as placeholders to indicate they are optional:
```go
inputs[fiUpscaleWidth] = newTextInput("disabled", "", 10)
inputs[fiUpscaleHeight] = newTextInput("disabled", "", 10)

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
	if val < 320 || val > 3840 {
		return fmt.Errorf("upscale width must be between 320 and 3840")
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
	if val < 320 || val > 3840 {
		return fmt.Errorf("upscale height must be between 320 and 3840")
	}
	return nil
}
```

### 3d. Request Mapping (`toRequest`)
Map `inputs[fiUpscaleWidth]` and `inputs[fiUpscaleHeight]` to integer pointers in `toRequest`:
```go
	var upscaleW, upscaleH *int
	if m.isFieldVisible(fiUpscaleWidth) {
		if val, err := strconv.Atoi(m.inputs[fiUpscaleWidth].Value()); err == nil {
			upscaleW = &val
		}
	}
	if m.isFieldVisible(fiUpscaleHeight) {
		if val, err := strconv.Atoi(m.inputs[fiUpscaleHeight].Value()); err == nil {
			upscaleH = &val
		}
	}
```
And pass them inside the returned `civit.GenerationRequest`:
```go
	return civit.GenerationRequest{
		// ... existing fields ...
		SourceImage:   m.sourceImage,
		Denoise:       denoise,
		ClipSkip:      clipSkip,
		UpscaleWidth:  upscaleW,
		UpscaleHeight: upscaleH,
	}
```

### 3e. Form Validation (`validateNumericFields`)
Add validation for upscale fields before submitting:
```go
	if m.isFieldVisible(fiUpscaleWidth) {
		if v := m.inputs[fiUpscaleWidth].Value(); v != "" {
			val, err := strconv.Atoi(v)
			if err != nil {
				return fmt.Errorf("upscale width: invalid integer %q", v)
			}
			if val < 320 || val > 3840 {
				return fmt.Errorf("upscale width must be between 320 and 3840")
			}
		}
	}
	if m.isFieldVisible(fiUpscaleHeight) {
		if v := m.inputs[fiUpscaleHeight].Value(); v != "" {
			val, err := strconv.Atoi(v)
			if err != nil {
				return fmt.Errorf("upscale height: invalid integer %q", v)
			}
			if val < 320 || val > 3840 {
				return fmt.Errorf("upscale height must be between 320 and 3840")
			}
		}
	}
```

---

## 4. Quality Gates & Testing

1. Run `go test ./pkg/civit/...` to verify serialization and logic.
2. Run `go build ./...` and `go vet ./...` to ensure clean compilation.
