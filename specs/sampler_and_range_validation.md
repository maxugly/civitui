# Specification: Sampler Addition and Numeric Range Validation

This specification details the implementation plan for:
1. **Sampler Field and Presets**: Adds a Sampler parameter to the generation request and the TUI config form, including an interactive presets selection panel.
2. **Numeric Range Validation**: Limits user input for Width, Height, Steps, CFG Scale, and Quantity to safe, valid bounds.

---

## 1. Civitai API Sampler Parameter

### Goal
Pass the `sampler` parameter to the Civitai `textToImage` generator workflow payload.

### Code Specification
In [pkg/civit/civit.go](file:///home/m/snc/cod/civitui/pkg/civit/civit.go):
Add `Sampler` (JSON tag `sampler,omitempty`) to the `GenerationRequest` struct:
```go
type GenerationRequest struct {
	Prompt         string  `json:"prompt"`
	NegativePrompt string  `json:"negativePrompt,omitempty"`
	Model          string  `json:"model"`
	Sampler        string  `json:"sampler,omitempty"` // Add this field
	Width          int     `json:"width"`
	Height         int     `json:"height"`
	Steps          int     `json:"steps"`
	CFGScale       float64 `json:"cfgScale"`
	Quantity       int     `json:"quantity"`
	Seed           *int64  `json:"seed,omitempty"`
}
```

---

## 2. TUI Sampler Field and Presets

### Goal
Include the `Sampler` field as the 4th field in the CONFIG form, and show interactive presets in the right pane when active.

### Constants and Labels
In [internal/ui/ui.go](file:///home/m/snc/cod/civitui/internal/ui/ui.go):
* Increase `numFormFields` to `10`.
* Update field index constants:
```go
const (
	fiPrompt         = iota // 0
	fiNegativePrompt        // 1
	fiModel                 // 2
	fiSampler               // 3 - New field
	fiWidth                 // 4
	fiHeight                // 5
	fiSteps                 // 6
	fiCFGScale              // 7
	fiQuantity              // 8
	fiSeed                  // 9
)
```
* Insert `"Sampler"` into `fieldLabels` at index 3.
* Insert sampler help text into `fieldHelpText` at index 3.

### Sampler Presets Definition
```go
type SamplerPreset struct {
	Name        string
	Description string
	Sampler     string
}

var samplerPresets = []SamplerPreset{
	{
		Name:        "Euler a",
		Description: "Euler Ancestral. Fast, creative, slightly unstable at high steps.",
		Sampler:     "Euler a",
	},
	{
		Name:        "DPM++ 2M Karras",
		Description: "Recommended for SDXL. High quality, excellent convergence.",
		Sampler:     "DPM++ 2M Karras",
	},
	{
		Name:        "DPM++ SDE Karras",
		Description: "Highly realistic, great detail, but takes twice as long.",
		Sampler:     "DPM++ SDE Karras",
	},
	{
		Name:        "Euler",
		Description: "Classic simple ODE solver. Fast and consistent.",
		Sampler:     "Euler",
	},
	{
		Name:        "Heun",
		Description: "Very high accuracy, but requires more compute per step.",
		Sampler:     "Heun",
	},
}
```

### Form Input Initialization and Request Parsing
In `NewModel`:
Initialize `inputs[fiSampler]` with default value `"Euler a"`.
In `toRequest()`:
Populate the request's `Sampler` field from `m.inputs[fiSampler].Value()`.

### Key and View Handling
* Reuse `m.inPresetsPane` and `m.activePreset` for both `fiModel` and `fiSampler`.
* In `handleConfigKey`: update control intercepts to toggle and navigate through `samplerPresets` when `m.activeInput == fiSampler`.
* In `viewConfig`: update right pane rendering to output `samplerPresets` when `m.activeInput == fiSampler`.

---

## 3. Numeric Range Validation

### Goal
Prevent submitting generation requests with out-of-bound parameters.

### Range Boundaries
Verify and enforce the following ranges in `validateNumericFields()` when the user presses Enter:
* **Width**: `64` to `4096` (inclusive). Raised from 2048 to 4K per operator request.
* **Height**: `64` to `4096` (inclusive). Raised from 2048 to 4K per operator request.
* **Steps**: `1` to `100` (inclusive). Reduced from 150 to match CivitAI API Zod schema (`max(100)`).
* **CFG Scale**: `1.0` to `7.0` (inclusive). Reduced from 30.0 per operator request. Also enforced at keystroke level via input masking — the Validate callback blocks any character that would push the value past 7.0 before the character lands in the field. Note: CivitAI API allows up to 30; the 7.0 cap is an intentional operator constraint, not an API limit.
* **Quantity**: `1` to `20` (inclusive). CivitAI API Zod schema: `max(20)`. Default value set to 4 per operator request.
* **Seed**: `1` to `4294967295` (inclusive, uint32 max). CivitAI API: `maxValues.seed = 4294967295`. Empty resolves to random.

### Code Specification
Update `validateNumericFields()` in [internal/ui/ui.go](file:///home/m/snc/cod/civitui/internal/ui/ui.go):
```go
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
		if val < 1.0 || val > 7.0 {
			return fmt.Errorf("cfg scale must be between 1.0 and 7.0")
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
	return nil
}
```
