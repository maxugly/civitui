# Specification: CLIP Skip (clip-skip)

This specification covers the implementation of parameter #2 (`clipSkip`) from the API parameters audit ([specs/api_parameters_audit.md](file:///home/m/snc/cod/civitui/specs/api_parameters_audit.md)).

CLIP Skip controls the layering of text embeddings from the CLIP text encoder:
- Range: `1-3` (integer). Default is `2`.
- Supported models: SD 1.5 and SDXL-based checkpoints only.
- Hidden for Flux and ZImage models where it has no architectural impact.

---

## 1. Field Re-indexing

Adding all Tier 2 fields bumps `numFormFields` from `15` to `21`. `fiClipSkip` is located at index `16`:

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
`isPresetsField` remains false for `fiClipSkip` since it is edited as a numeric field:
```go
func isPresetsField(idx int) bool {
	return idx == fiModel || idx == fiFluxMode || idx == fiSampler ||
		idx == fiScheduler || idx == fiAspectRatio || idx == fiOutputFormat ||
		idx == fiDraft || idx == fiExperimental || idx == fiFluxUltraRaw
}
```

### 1b. Replace-on-Focus Verification
`isReplaceOnFocus` must return true for `fiClipSkip` to clear default values on focus:
```go
func isReplaceOnFocus(idx int) bool {
	return (idx >= fiWidth && idx <= fiSeed) || idx == fiDenoise || 
		idx == fiClipSkip || idx == fiUpscaleWidth || idx == fiUpscaleHeight
}
```

---

## 2. Core Engine Changes (`pkg/civit/civit.go`)

### 2a. GenerationRequest Field Mapping
Add `ClipSkip` as an optional integer pointer with `omitempty` tag to `GenerationRequest` in `pkg/civit/civit.go`:

```go
type GenerationRequest struct {
	// ... existing fields ...
	Draft          bool         `json:"draft"`
	SourceImage    *SourceImage `json:"sourceImage,omitempty"`
	Denoise        *float64     `json:"denoise,omitempty"`
	ClipSkip       *int         `json:"clipSkip,omitempty"` // Add this field
}
```

---

## 3. UI Layer Changes (`internal/ui/ui.go`)

### 3a. Visibility Helper
Extend the `isFieldVisible` helper method in `internal/ui/ui.go` to hide the field when Flux or ZImage models are active:

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
	}
	return true
}
```

### 3b. Field Label and Help Text
Add the field label to `fieldLabels`:
```go
// In fieldLabels slice:
/* fiClipSkip */ "CLIP Skip",
```

Add the help text to `fieldHelpText`:
```go
// In fieldHelpText slice:
/* fiClipSkip */ "Layers of the CLIP text encoder to skip. Range: 1–3. Default is 2. Only relevant for SD 1.5 and SDXL models (hidden for Flux/ZImage).",
```

### 3c. Inputs Initialization and Validation (`NewModel`)
Initialize the field's text input and integer validator inside `NewModel`:
```go
inputs[fiClipSkip] = newTextInput("2", "2", 6)

inputs[fiClipSkip].Validate = func(s string) error {
	if s == "" {
		return nil
	}
	if !intRegex.MatchString(s) {
		return fmt.Errorf("must be an integer")
	}
	val, err := strconv.Atoi(s)
	if err != nil {
		return nil // intermediate typing state
	}
	if val < 1 || val > 3 {
		return fmt.Errorf("clip skip must be between 1 and 3")
	}
	return nil
}
```

### 3d. Request Mapping (`toRequest`)
Map `inputs[fiClipSkip]` to `ClipSkip` integer pointer in `toRequest`:
```go
	var clipSkip *int
	if m.isFieldVisible(fiClipSkip) {
		if val, err := strconv.Atoi(m.inputs[fiClipSkip].Value()); err == nil {
			clipSkip = &val
		}
	}
```
And pass it inside the returned `civit.GenerationRequest`:
```go
	return civit.GenerationRequest{
		// ... existing fields ...
		SourceImage:  m.sourceImage,
		Denoise:      denoise,
		ClipSkip:     clipSkip,
	}
```

### 3e. Form Validation (`validateNumericFields`)
Add validation for the clip skip field before submitting:
```go
	if m.isFieldVisible(fiClipSkip) {
		if v := m.inputs[fiClipSkip].Value(); v != "" {
			val, err := strconv.Atoi(v)
			if err != nil {
				return fmt.Errorf("clip skip: invalid integer %q", v)
			}
			if val < 1 || val > 3 {
				return fmt.Errorf("clip skip must be between 1 and 3")
			}
		}
	}
```

---

## 4. Quality Gates & Testing

1. Run `go test ./pkg/civit/...` to verify serialization and logic.
2. Run `go build ./...` and `go vet ./...` to ensure clean compilation.
