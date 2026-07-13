# Specification: Input Data Validation and Model Presets

This specification details the implementation plans for two new features requested by the operator:
1. **Character Validation on Numeric Inputs**: Prevents typing non-numeric characters (letters) into numeric fields.
2. **Context-Sensitive Help and Model Presets**: Adds a split-pane layout showing context help or interactive model presets.

---

## 1. Character Validation on Numeric Inputs

### Goal
Prevent the user from entering invalid characters (such as letters) into numeric settings: Width, Height, Steps, CFG Scale, Quantity, and Seed.

### Implementation
**Important:** `textinput.Validate` sets `Err` but does **not** reject keystrokes — the rune always lands in the value regardless of whether validation passes or fails. To actually block letters, we use a **post-update rollback** pattern in `handleConfigKey`:

1. Capture `oldValue` and `oldPos` (cursor position) before calling `textinput.Update()`.
2. Call Update.
3. If `input.Err != nil`, restore `oldValue`, `oldPos`, and clear `Err`.

Validate callbacks are still wired on the numeric inputs so that the rollback check has a signal to act on — but the blocking logic lives in our key path, not in bubbles.

Using Charm's `textinput.Model` built-in `Validate` property as the detection signal:
* **Integer Validation** (Width, Height, Steps, Quantity, Seed): Allow only digits (`0-9`) or empty input (which resolves to default).
* **Float Validation** (CFG Scale): Allow digits (`0-9`), a single decimal point (`.`), or empty input.

### Code Specification
At the file level of [internal/ui/ui.go](file:///home/m/snc/cod/civitui/internal/ui/ui.go):
```go
var (
	intRegex   = regexp.MustCompile(`^[0-9]*$`)
	floatRegex = regexp.MustCompile(`^[0-9]*\.?[0-9]*$`)
)
```

In `NewModel`:
```go
	inputs[fiWidth].Validate = func(s string) error {
		if !intRegex.MatchString(s) {
			return fmt.Errorf("must be an integer")
		}
		return nil
	}
	inputs[fiHeight].Validate = func(s string) error {
		if !intRegex.MatchString(s) {
			return fmt.Errorf("must be an integer")
		}
		return nil
	}
	inputs[fiSteps].Validate = func(s string) error {
		if !intRegex.MatchString(s) {
			return fmt.Errorf("must be an integer")
		}
		return nil
	}
	inputs[fiCFGScale].Validate = func(s string) error {
		if !floatRegex.MatchString(s) {
			return fmt.Errorf("must be a float")
		}
		return nil
	}
	inputs[fiQuantity].Validate = func(s string) error {
		if !intRegex.MatchString(s) {
			return fmt.Errorf("must be an integer")
		}
		return nil
	}
	inputs[fiSeed].Validate = func(s string) error {
		if !intRegex.MatchString(s) {
			return fmt.Errorf("must be an integer")
		}
		return nil
	}
```

---

## 2. Interactive Presets and Context-Sensitive Help Pane

### Goal
Provide a split-pane interface in `PhaseConfig`:
* **Left Panel**: 9 input fields (Prompt through Seed).
* **Right Panel**: Context-sensitive help or interactive model presets.
  * If the active field is **Model**: show a selectable list of 4 model presets. Pressing `→` (Right Arrow) moves focus to the presets. Pressing `Enter` applies the selected model to the input. Pressing `←` or `esc` returns to the input form.
  * For other fields: show helpful descriptions (e.g. recommended values, limits).

### Model Struct Updates
Add these fields to `Model` in [internal/ui/ui.go](file:///home/m/snc/cod/civitui/internal/ui/ui.go):
```go
	inPresetsPane bool // true if navigating right-hand model presets
	activePreset  int  // currently highlighted index in right-hand preset list
```

### Model Presets Definition
```go
type ModelPreset struct {
	Name        string
	Description string
	AIR         string
}

var modelPresets = []ModelPreset{
	{
		Name:        "Flux.1 Dev (Standard)",
		Description: "Recommended. Balanced and high quality for general prompts.",
		AIR:         "air:flux1:checkpoint:civitai:618692@691639",
	},
	{
		Name:        "Pony Diffusion V6 XL",
		Description: "Extremely popular SDXL checkpoint for expressive art.",
		AIR:         "air:pony-diffusion-v6:checkpoint:civitai:257204@290640",
	},
	{
		Name:        "SDXL 1.0 (Base)",
		Description: "Stability AI official base model. Fast and reliable.",
		AIR:         "air:sdxl:checkpoint:civitai:101055@128078",
	},
	{
		Name:        "Stable Diffusion 1.5",
		Description: "Classic model. Extremely fast and lightweight.",
		AIR:         "air:sd15:checkpoint:civitai:1102@11124",
	},
}
```

### Context Help Definition
```go
var fieldHelpText = []string{
	/* fiPrompt */         "Describe the image you want to generate. Be specific and descriptive. E.g. 'a photo of an astronaut on Mars'. (Required)",
	/* fiNegativePrompt */ "Describe things you want to avoid in the generated image. E.g. 'blurry, low quality, distorted'. (Optional)",
	/* fiModel */          "Civitai Model AIR. Represents the base generator model. Use Right Arrow key to select from popular presets.",
	/* fiWidth */          "Width of the generated image in pixels. Recommended standard is 1024. Only digits allowed.",
	/* fiHeight */         "Height of the generated image in pixels. Recommended standard is 1024. Only digits allowed.",
	/* fiSteps */          "Number of denoising steps. Recommended: 20-30. More steps take longer but add detail.",
	/* fiCFGScale */       "Classifier Free Guidance. Recommended: 3.5-7.0. Higher values adhere closer to prompt.",
	/* fiQuantity */       "Number of images to generate (1-10). Each image is processed concurrently in the workflow.",
	/* fiSeed */           "Random seed for generation consistency. Leave empty/blank for a random seed.",
}
```

### Key Handling Transitions
In `handleKey`:
```go
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
```

In `handleConfigKey`:
```go
	// Intercept right-pane preset controls
	if m.activeInput == fiModel && m.inPresetsPane {
		switch key {
		case "up", "k":
			m.activePreset = (m.activePreset - 1 + len(modelPresets)) % len(modelPresets)
			return m, nil
		case "down", "j":
			m.activePreset = (m.activePreset + 1) % len(modelPresets)
			return m, nil
		case "left", "esc", "tab":
			m.inPresetsPane = false
			return m, nil
		case "enter":
			m.inputs[fiModel].SetValue(modelPresets[m.activePreset].AIR)
			m.inPresetsPane = false
			return m, nil
		default:
			return m, nil // block typing when in right panel
		}
	}

	// Trigger transition into right pane
	if m.activeInput == fiModel && !m.inPresetsPane && key == "right" {
		m.inPresetsPane = true
		return m, nil
	}
```

### View Formatting
Modify `viewConfig` to output side-by-side lines padded to 80 characters for the left column, using `stripANSI` and `wrapText` helpers:
```go
var ansiRegex = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

func stripANSI(str string) string {
	return ansiRegex.ReplaceAllString(str, "")
}

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
```
