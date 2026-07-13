# Specification: Dynamic UI Field Visibility

This specification covers the implementation details for dynamic form field visibility in the civitui Terminal User Interface (TUI). The main goal is to hide form fields that do not apply to the selected model, ensuring that the interface remains clean and only shows inputs that have a functional effect on the model's generation.

Following the API parameters audit ([specs/api_parameters_audit.md](file:///home/m/snc/cod/civitui/specs/api_parameters_audit.md)), field applicability varies by model family (ecosystem):
- **Flux1** models use `fluxMode` for variation and do not expose user-visible samplers (managed internally by `fluxMode`).
- **Flux2Klein** models do not use `fluxMode` but instead expose a restricted set of samplers (euler, heun, dpm++2s_a, dpm++2m, dpm++2mv2, ipndm, ipndm_v, lcm).
- **ZImage** models support only the `euler` and `heun` samplers, and do not use `fluxMode`.
- **SD/SDXL/Pony/Illustrious** models support standard samplers and do not use `fluxMode`.
- **CFG Scale** limits are ecosystem-specific (up to 7.0 for Flux models, and up to 30.0 for others).

---

## 1. Model Preset Ecosystem Field

To support robust filtering, we will add an `Ecosystem` field to the [ModelPreset](file:///home/m/snc/cod/civitui/internal/ui/ui.go#L206-L210) struct in [internal/ui/ui.go](file:///home/m/snc/cod/civitui/internal/ui/ui.go). This field will explicitly identify the model family.

### 1a. Update `ModelPreset` Struct
```go
type ModelPreset struct {
	Name        string
	Description string
	AIR         string
	Ecosystem   string // "flux1", "flux2", "sdxl", "sd1", "zimage"
}
```

### 1b. Update `modelPresets` Definitions
We will update the predefined presets in [internal/ui/ui.go](file:///home/m/snc/cod/civitui/internal/ui/ui.go#L212-L233) with their respective ecosystems:
```go
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
```

---

## 2. Dynamic Ecosystem Detection Helper

To support both preset models and manually typed custom AIRs in `fiModel`, we will implement a `currentEcosystem` method on `Model` in [internal/ui/ui.go](file:///home/m/snc/cod/civitui/internal/ui/ui.go).

```go
// currentEcosystem returns the ecosystem of the currently selected model.
// Keys off the modelPreset's Ecosystem if matched, or tries to extract from the AIR string.
func (m *Model) currentEcosystem() string {
	val := m.inputs[fiModel].Value()
	// Check predefined presets first
	for _, preset := range modelPresets {
		if preset.AIR == val {
			return preset.Ecosystem
		}
	}
	// Fallback parsing for custom typed AIR values
	valLower := strings.ToLower(val)
	if strings.Contains(valLower, "flux1") {
		return "flux1"
	}
	if strings.Contains(valLower, "flux2") || strings.Contains(valLower, "flux-klein") || strings.Contains(valLower, "klein") {
		return "flux2"
	}
	if strings.Contains(valLower, "zimage") {
		return "zimage"
	}
	if strings.Contains(valLower, "sdxl") || strings.Contains(valLower, "pony") || strings.Contains(valLower, "illustrious") {
		return "sdxl"
	}
	if strings.Contains(valLower, "sd15") || strings.Contains(valLower, "sd1") || strings.Contains(valLower, "stable-diffusion-1") {
		return "sd1"
	}
	// Default fallback
	return "sdxl"
}
```

---

## 3. Dynamic Field Visibility Updates (`isFieldVisible`)

We will update the [isFieldVisible](file:///home/m/snc/cod/civitui/internal/ui/ui.go#L459-L471) method in [internal/ui/ui.go](file:///home/m/snc/cod/civitui/internal/ui/ui.go) to control form rendering and navigation dynamically based on the current ecosystem.

```go
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
		// Only show fluxMode when base model is Flux1 (Draft/Dev/Pro/Ultra)
		return m.currentEcosystem() == "flux1"
	case fiSampler:
		// Hide the sampler field entirely if there are 0 or 1 options
		return len(m.filteredSamplerPresets()) > 1
	default:
		return true
	}
}
```

---

## 4. Sampler Presets and Ecosystem Filtering

Different ecosystems support different sets of samplers. We will update [SamplerPreset](file:///home/m/snc/cod/civitui/internal/ui/ui.go#L238-L242) to store compatibility information and the specific API values needed for each ecosystem, then implement `filteredSamplerPresets`.

### 4a. Update `SamplerPreset` Struct
```go
type SamplerPreset struct {
	Name        string
	Description string
	ValSD       string // Value for SD1/SDXL/Pony/Illustrious (case-sensitive)
	ValZImage   string // Value for ZImage (case-sensitive)
	ValFlux2    string // Value for Flux2Klein (case-sensitive)
}
```

### 4b. Update `samplerPresets` Array
```go
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
```

### 4c. Implement `filteredSamplerPresets` Method
```go
// filteredSamplerPresets returns the sampler presets valid for the current model.
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
```

### 4d. Update TUI Presets Selection and Rendering
- **Presets count (`handleConfigKey`)**:
  ```diff
  -		case fiSampler:
  -			presetsLen = len(samplerPresets)
  +		case fiSampler:
  +			presetsLen = len(m.filteredSamplerPresets())
  ```
- **Presets submission (`handleConfigKey`)**:
  ```diff
  -			case fiSampler:
  -				m.inputs[fiSampler].SetValue(samplerPresets[m.activePreset].Sampler)
  +			case fiSampler:
  +				visible := m.filteredSamplerPresets()
  +				if m.activePreset < len(visible) {
  +					eco := m.currentEcosystem()
  +					var val string
  +					switch eco {
  +					case "flux2":
  +						val = visible[m.activePreset].ValFlux2
  +					case "zimage":
  +						val = visible[m.activePreset].ValZImage
  +					default:
  +						val = visible[m.activePreset].ValSD
  +					}
  +					m.inputs[fiSampler].SetValue(val)
  +				}
  ```
- **Presets list rendering (`viewConfig`)**:
  ```diff
  -		case fiSampler:
  -			rightLines = append(rightLines, dimStyle.Render("── Sampler Presets ──"))
  -			for _, preset := range samplerPresets {
  -				presetsList = append(presetsList, struct {
  -					Name        string
  -					Description string
  -				}{preset.Name, preset.Description})
  -			}
  +		case fiSampler:
  +			rightLines = append(rightLines, dimStyle.Render("── Sampler Presets ──"))
  +			for _, preset := range m.filteredSamplerPresets() {
  +				presetsList = append(presetsList, struct {
  +					Name        string
  +					Description string
  +				}{preset.Name, preset.Description})
  +			}
  ```

---

## 5. Model-Dependent CFG Scale Validation

Currently, the CFG scale is capped at `7.0` for all models. Under the new specification, the maximum CFG scale will be model-dependent: `7.0` for Flux (Flux1 and Flux2) models, and `30.0` for SD/SDXL/Pony models.

### 5a. Helper Function for Max CFG Scale
```go
// maxCFGScale returns the maximum CFG scale allowed for the given ecosystem.
func maxCFGScale(eco string) float64 {
	if eco == "flux1" || eco == "flux2" {
		return 7.0
	}
	return 30.0
}
```

### 5b. Update Real-Time Validate Closure in `NewModel`
The Validate callback of `inputs[fiCFGScale]` will be updated to fetch the current model input value dynamically and enforce the appropriate maximum value:
```go
	inputs[fiCFGScale].Validate = func(s string) error {
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

		// Determine the max scale from current model field value
		modelVal := inputs[fiModel].Value()
		eco := "sdxl"
		for _, preset := range modelPresets {
			if preset.AIR == modelVal {
				eco = preset.Ecosystem
				break
			}
		}
		if eco == "sdxl" {
			valLower := strings.ToLower(modelVal)
			if strings.Contains(valLower, "flux1") {
				eco = "flux1"
			} else if strings.Contains(valLower, "flux2") || strings.Contains(valLower, "flux-klein") || strings.Contains(valLower, "klein") {
				eco = "flux2"
			} else if strings.Contains(valLower, "zimage") {
				eco = "zimage"
			} else if strings.Contains(valLower, "sd15") || strings.Contains(valLower, "sd1") || strings.Contains(valLower, "stable-diffusion-1") {
				eco = "sd1"
			}
		}

		maxVal := maxCFGScale(eco)
		if val > maxVal {
			return fmt.Errorf("cfg scale max is %.1f", maxVal)
		}
		return nil
	}
```

### 5c. Update Submit-Time Validation in `validateNumericFields`
We will update [validateNumericFields](file:///home/m/snc/cod/civitui/internal/ui/ui.go#L816-L881) in [internal/ui/ui.go](file:///home/m/snc/cod/civitui/internal/ui/ui.go) to validate against the model-dependent maximum value dynamically:
```go
	if v := m.inputs[fiCFGScale].Value(); v != "" {
		val, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return fmt.Errorf("cfg scale: invalid float %q", v)
		}
		eco := m.currentEcosystem()
		maxVal := maxCFGScale(eco)
		if val < 1.0 || val > maxVal {
			return fmt.Errorf("cfg scale must be between 1.0 and %.1f", maxVal)
		}
	}
```

---

## 6. General UI Layer Updates

### 6a. Update `fieldHelpText`
Update help text for `fiCFGScale` in `fieldHelpText` to document the dynamic limit:
```go
	/* fiCFGScale */ "Classifier Free Guidance scale. Range: 1.0–7.0 (Flux) / 1.0–30.0 (others). Higher values follow the prompt more strictly.",
```

### 6b. Request Generation Mapping (`toRequest`)
Ensure that parameters not applicable to the current ecosystem are cleaned from the final generation request payload inside [toRequest](file:///home/m/snc/cod/civitui/internal/ui/ui.go#L736-L762):
```go
	return civit.GenerationRequest{
		// ...
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
		// ...
	}
```

---

## 7. Quality Gates & Testing

To verify correctness of the changes:
1. **Compilation**: Run:
   ```bash
   go build ./...
   ```
2. **Linter**: Run:
   ```bash
   go vet ./...
   ```
3. **Unit Tests**: Run:
   ```bash
   go test ./... -count=1
   ```
4. **Manual Validation**:
   - **Flux.1 Dev Preset**: Verify `Flux Mode` is visible and `Sampler` is hidden. Verify trying to type `8.0` in `CFG Scale` is blocked by real-time validation.
   - **SDXL/Pony Preset**: Verify `Flux Mode` is hidden and `Sampler` is visible with all 5 standard samplers. Verify `CFG Scale` allows typing up to `30.0`.
   - **Custom Flux2 Model**: Type a custom Flux2 model AIR. Verify `Flux Mode` is hidden and `Sampler` is visible with only Flux2-specific samplers.

---

## 8. Future Considerations: Aspect Ratio

Flux Ultra supports additional aspect ratios (`21:9` and `9:21`). Currently, aspect ratio presets are identical across all models because Ultra ratios are handled by `fluxMode` itself. If the orchestrator requires passing custom width/height values directly for Ultra ratios, `aspectRatioPresets` should be made dynamic in a future ticket.
