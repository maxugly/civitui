# Specification: Denoising Strength (denoise)

This specification covers the implementation of parameter #7 (`denoise`) from the API parameters audit ([specs/api_parameters_audit.md](file:///home/m/snc/cod/civitui/specs/api_parameters_audit.md)).

Denoising strength controls how much a source image is altered during image-to-image (img2img) workflows:
- `0.0` means "don't change anything"
- `1.0` means "completely regenerate"
- Only relevant when a source image is attached (`img2img` mode). It must be hidden if no source image is attached.

---

## 1. Field Re-indexing

Adding all Tier 2 fields bumps `numFormFields` from `15` to `21`. `fiDenoise` is located at index `15`:

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
`isPresetsField` remains false for `fiDenoise` since it accepts free-form decimal values:
```go
func isPresetsField(idx int) bool {
	return idx == fiModel || idx == fiFluxMode || idx == fiSampler ||
		idx == fiScheduler || idx == fiAspectRatio || idx == fiOutputFormat ||
		idx == fiDraft || idx == fiExperimental || idx == fiFluxUltraRaw
}
```

### 1b. Replace-on-Focus Verification
`isReplaceOnFocus` must return true for `fiDenoise` to clear default values on focus:
```go
func isReplaceOnFocus(idx int) bool {
	return (idx >= fiWidth && idx <= fiSeed) || idx == fiDenoise || 
		idx == fiClipSkip || idx == fiUpscaleWidth || idx == fiUpscaleHeight
}
```

---

## 2. Core Engine Changes (`pkg/civit/civit.go`)

### 2a. Struct Definitions
We define the `SourceImage` struct and add `SourceImage` and `Denoise` to `GenerationRequest` in `pkg/civit/civit.go`:

```go
type SourceImage struct {
	URL           string `json:"url"`
	Width         int    `json:"width"`
	Height        int    `json:"height"`
	UpscaleWidth  *int   `json:"upscaleWidth,omitempty"`
	UpscaleHeight *int   `json:"upscaleHeight,omitempty"`
}

type GenerationRequest struct {
	// ... existing fields ...
	Draft          bool         `json:"draft"`
	SourceImage    *SourceImage `json:"sourceImage,omitempty"`
	Denoise        *float64     `json:"denoise,omitempty"`
}
```

---

## 3. UI Layer Changes (`internal/ui/ui.go`)

### 3a. Model State Extension
Add `sourceImage` field tracking to `Model` in `internal/ui/ui.go`:

```go
type Model struct {
	// ... existing fields ...
	sourceImage *civit.SourceImage // nil if no source image attached
}
```

### 3b. Visibility Helper
Define an `isFieldVisible` helper method to control dynamic field display and navigation:

```go
func (m *Model) isFieldVisible(idx int) bool {
	switch idx {
	case fiDenoise:
		return m.sourceImage != nil
	}
	return true
}
```

Update `viewConfig` rendering loop to skip invisible fields:
```diff
 	for i, input := range m.inputs {
+		if !m.isFieldVisible(i) {
+			continue
+		}
 		// Label.
 		label := fieldLabelStyle.Render(fmt.Sprintf("  %-16s", fieldLabels[i]+":"))
```

Update form navigation (`tab`/`down` and `shift+tab`/`up` keys in `handleConfigKey`) to skip invisible fields:
```diff
 	case "tab", "down":
 		m.inputs[m.activeInput].Blur()
-		m.activeInput = (m.activeInput + 1) % numFormFields
+		for {
+			m.activeInput = (m.activeInput + 1) % numFormFields
+			if m.isFieldVisible(m.activeInput) {
+				break
+			}
+		}
 		m.inputs[m.activeInput].Focus()
```

### 3c. Field Label and Help Text
Add the field label to `fieldLabels`:
```go
// In fieldLabels slice:
/* fiDenoise */ "Denoise",
```

Add the help text to `fieldHelpText`:
```go
// In fieldHelpText slice:
/* fiDenoise */ "Denoising strength for image-to-image (0.0 = no change, 1.0 = complete refresh). Only visible when a source image is attached.",
```

### 3d. Inputs Initialization and Validation (`NewModel`)
Initialize the field's text input and float validator inside `NewModel`:
```go
inputs[fiDenoise] = newTextInput("0.4", "0.4", 8)

inputs[fiDenoise].Validate = func(s string) error {
	if s == "" {
		return nil
	}
	if !floatRegex.MatchString(s) {
		return fmt.Errorf("must be a decimal number")
	}
	val, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return nil // intermediate typing state
	}
	if val < 0.0 || val > 1.0 {
		return fmt.Errorf("denoise must be between 0.0 and 1.0")
	}
	return nil
}
```

### 3e. Request Mapping (`toRequest`)
Map `inputs[fiDenoise]` to `Denoise` float pointer in `toRequest`:
```go
	var denoise *float64
	if m.isFieldVisible(fiDenoise) {
		if val, err := strconv.ParseFloat(m.inputs[fiDenoise].Value(), 64); err == nil {
			denoise = &val
		}
	}
```
And pass it inside the returned `civit.GenerationRequest`:
```go
	return civit.GenerationRequest{
		// ... existing fields ...
		SourceImage:  m.sourceImage,
		Denoise:      denoise,
	}
```

### 3f. Form Validation (`validateNumericFields`)
Add validation for the denoise field before submitting:
```go
	if m.isFieldVisible(fiDenoise) {
		if v := m.inputs[fiDenoise].Value(); v != "" {
			val, err := strconv.ParseFloat(v, 64)
			if err != nil {
				return fmt.Errorf("denoise: invalid decimal %q", v)
			}
			if val < 0.0 || val > 1.0 {
				return fmt.Errorf("denoise must be between 0.0 and 1.0")
			}
		}
	}
```

---

## 4. Quality Gates & Testing

1. Run `go test ./pkg/civit/...` to verify serialization and logic.
2. Run `go build ./...` and `go vet ./...` to ensure clean compilation.
