# Specification: fluxMode, outputFormat, and scheduler

Three tier-1 parameters from `specs/api_parameters_audit.md`, implemented together
since they all follow the same presets-dropdown pattern (same shape as Model, Sampler,
and AspectRatio).

---

## Field Re-indexing

Adding 3 fields bumps `numFormFields` from 11 to 14. All existing `fi*` constants
shift — the new order groups related fields:

```go
const numFormFields = 14

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
)
```

**`isPresetsField`** must return true for the three new indices:

```go
func isPresetsField(idx int) bool {
    return idx == fiModel || idx == fiFluxMode || idx == fiSampler ||
        idx == fiScheduler || idx == fiAspectRatio || idx == fiOutputFormat
}
```

**`isReplaceOnFocus`** shifts to cover `fiWidth` through `fiSeed` (indices 7–13):

```go
func isReplaceOnFocus(idx int) bool {
    return idx >= fiWidth && idx <= fiSeed
}
```

---

## 1. fluxMode

### 1a. Civitai API — `GenerationRequest` field

In `pkg/civit/civit.go`, add to the struct (between Model and Sampler, matching
field-index order):

```go
type GenerationRequest struct {
    Prompt         string  `json:"prompt"`
    NegativePrompt string  `json:"negativePrompt,omitempty"`
    Model          string  `json:"model"`
    FluxMode       string  `json:"fluxMode,omitempty"`      // Add
    Sampler        string  `json:"sampler,omitempty"`
    Scheduler      string  `json:"scheduler,omitempty"`     // Add (see §3)
    AspectRatio    string  `json:"aspectRatio,omitempty"`
    Width          int     `json:"width"`
    Height         int     `json:"height"`
    Steps          int     `json:"steps"`
    CFGScale       float64 `json:"cfgScale"`
    Quantity       int     `json:"quantity"`
    OutputFormat   string  `json:"outputFormat,omitempty"`  // Add (see §2)
    Seed           *int64  `json:"seed,omitempty"`
}
```

### 1b. TUI — field labels and help

Insert at index 3 in both slices:

```go
var fieldLabels = []string{
    "Prompt",
    "Negative Prompt",
    "Model",
    "Flux Mode",          // ← new
    "Sampler",
    "Scheduler",          // ← new (§3)
    "Aspect Ratio",
    "Width",
    "Height",
    "Steps",
    "CFG Scale",
    "Quantity",
    "Output Format",      // ← new (§2)
    "Seed",
}

var fieldHelpText = []string{
    /* fiPrompt */         "Describe the image …",
    /* fiNegativePrompt */ "Describe things you want to avoid …",
    /* fiModel */          "Civitai Model AIR. …",
    /* fiFluxMode */       "Flux model variant. Only applies when using a Flux1 model. Use Right Arrow to select from presets.",  // new
    /* fiSampler */        "Diffusion sampler algorithm. …",
    /* fiScheduler */      "Noise schedule for the sampler. 'simple' works everywhere. Advanced: discrete, karras, exponential, ays. Use Right Arrow to select.",  // new
    /* fiAspectRatio */    "Image aspect ratio. …",
    /* fiWidth */          "Width …",
    /* fiHeight */         "Height …",
    /* fiSteps */          "Number of denoising steps. …",
    /* fiCFGScale */       "Classifier Free Guidance scale. …",
    /* fiQuantity */       "Number of images …",
    /* fiOutputFormat */   "Output image format. JPEG is smaller; PNG is lossless. Use Right Arrow to select.",  // new
    /* fiSeed */           "Random seed …",
}
```

### 1c. FluxMode presets

Five Flux variants — friendly names with full AIR URNs. Only applied when the
base model is Flux1.

```go
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
```

### 1d. Input initialization

In `NewModel`, after `inputs[fiModel]`:

```go
inputs[fiFluxMode] = newTextInput("Standard", "urn:air:flux1:checkpoint:civitai:618692@691639", inputWidth)
```

Default is Standard (the most commonly used variant).

### 1e. toRequest

In `toRequest()`, add between Model and Sampler:

```go
return civit.GenerationRequest{
    Prompt:         m.inputs[fiPrompt].Value(),
    NegativePrompt: m.inputs[fiNegativePrompt].Value(),
    Model:          m.inputs[fiModel].Value(),
    FluxMode:       m.inputs[fiFluxMode].Value(),       // new
    Sampler:        m.inputs[fiSampler].Value(),
    Scheduler:      m.inputs[fiScheduler].Value(),      // new (§3)
    AspectRatio:    m.inputs[fiAspectRatio].Value(),
    Width:          width,
    Height:         height,
    Steps:          steps,
    CFGScale:       cfgScale,
    Quantity:       quantity,
    OutputFormat:   m.inputs[fiOutputFormat].Value(),   // new (§2)
    Seed:           seed,
}
```

### 1f. Right-pane rendering — viewConfig

In `viewConfig`, the `switch m.activeInput` block needs a new case:

```go
case fiFluxMode:
    rightLines = append(rightLines, dimStyle.Render("── Flux Mode ──"))
    for _, preset := range fluxModePresets {
        presetsList = append(presetsList, struct {
            Name        string
            Description string
        }{preset.Name, preset.Description})
    }
```

### 1g. Key handling — handleConfigKey

In `handleConfigKey`, the presets-length switch needs a `fiFluxMode` case:

```go
case fiFluxMode:
    presetsLen = len(fluxModePresets)
```

And in the Enter handler:

```go
case fiFluxMode:
    m.inputs[fiFluxMode].SetValue(fluxModePresets[m.activePreset].URN)
```

When `isPresetsField(m.activeInput)` is true for fiFluxMode, the existing
typing-block and right-arrow-enter logic already applies.

---

## 2. outputFormat

### 2a. Civitai API — `GenerationRequest` field

Already included in the struct in §1a. JSON tag: `"outputFormat,omitempty"`.

### 2b. TUI — labels and help

Already included in the slices in §1b. Placed at index 12, second-to-last
(before Seed).

### 2c. OutputFormat presets

Two options — simple enum. Follows the same preset pattern as the others.

```go
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
```

### 2d. Input initialization

In `NewModel`, after `inputs[fiQuantity]`:

```go
inputs[fiOutputFormat] = newTextInput("jpeg", "jpeg", inputWidth)
```

Default is `"jpeg"` — matches CivitAI web default.

### 2e. toRequest

Already included in §1e.

### 2f. Right-pane rendering — viewConfig

```go
case fiOutputFormat:
    rightLines = append(rightLines, dimStyle.Render("── Output Format ──"))
    for _, preset := range outputFormatPresets {
        presetsList = append(presetsList, struct {
            Name        string
            Description string
        }{preset.Name, preset.Description})
    }
```

### 2g. Key handling — handleConfigKey

Preset length:

```go
case fiOutputFormat:
    presetsLen = len(outputFormatPresets)
```

Enter:

```go
case fiOutputFormat:
    m.inputs[fiOutputFormat].SetValue(outputFormatPresets[m.activePreset].Format)
```

---

## 3. scheduler

### 3a. Civitai API — `GenerationRequest` field

Already included in the struct in §1a. JSON tag: `"scheduler,omitempty"`.

### 3b. TUI — labels and help

Already included in the slices in §1b. Placed at index 5, right after Sampler.

### 3c. Scheduler presets

Five options. The API defaults to `"simple"` if an unrecognized value is sent.

```go
type SchedulerPreset struct {
    Name        string
    Description string
    Scheduler   string
    // zImageOK is true when this scheduler works with ZImage models.
    // ZImage only supports "simple" and "discrete".
    zImageOK bool
    // flux2KleinOK is true when this scheduler works with Flux2Klein.
    // Flux2Klein supports all except "ays" (crashes 100%).
    flux2KleinOK bool
}

var schedulerPresets = []SchedulerPreset{
    {
        Name:          "Simple",
        Description:   "Standard uniform schedule. Works everywhere. Default.",
        Scheduler:     "simple",
        zImageOK:      true,
        flux2KleinOK:  true,
    },
    {
        Name:          "Discrete",
        Description:   "Discrete timesteps. Compatible with ZImage and Flux2Klein.",
        Scheduler:     "discrete",
        zImageOK:      true,
        flux2KleinOK:  true,
    },
    {
        Name:          "Karras",
        Description:   "Karras noise schedule. Better quality at same step count. Not for ZImage.",
        Scheduler:     "karras",
        zImageOK:      false,
        flux2KleinOK:  true,
    },
    {
        Name:          "Exponential",
        Description:   "Exponential schedule. Fast convergence. Not for ZImage.",
        Scheduler:     "exponential",
        zImageOK:      false,
        flux2KleinOK:  true,
    },
    {
        Name:          "AYS",
        Description:   "Align Your Steps. Best for SDXL/Flux1. Crashes Flux2Klein — hidden for that model.",
        Scheduler:     "ays",
        zImageOK:      false,
        flux2KleinOK:  false,
    },
}
```

### 3d. Input initialization

In `NewModel`, after `inputs[fiSampler]`:

```go
inputs[fiScheduler] = newTextInput("simple", "simple", inputWidth)
```

Default is `"simple"` — universally compatible.

### 3e. toRequest

Already included in §1e.

### 3f. Right-pane rendering — viewConfig (model-aware)

The scheduler presets are model-aware: ZImage models only support `simple` and
`discrete`; Flux2Klein supports all except `ays`. Filter the presets list based
on the current model value.

```go
case fiScheduler:
    rightLines = append(rightLines, dimStyle.Render("── Scheduler ──"))
    modelVal := m.inputs[fiModel].Value()
    isZImage := strings.Contains(strings.ToLower(modelVal), "zimage")
    isFlux2Klein := strings.Contains(strings.ToLower(modelVal), "flux2") ||
        strings.Contains(strings.ToLower(modelVal), "flux-klein") ||
        strings.Contains(strings.ToLower(modelVal), "klein")
    for _, preset := range schedulerPresets {
        if isZImage && !preset.zImageOK {
            continue
        }
        if isFlux2Klein && !preset.flux2KleinOK {
            continue
        }
        presetsList = append(presetsList, struct {
            Name        string
            Description string
        }{preset.Name, preset.Description})
    }
```

When neither ZImage nor Flux2Klein is detected, show all 5 schedulers
(safe for SD/SDXL/Pony/Flux1 — the API falls back to `"simple"` for
unrecognized values).

### 3g. Key handling — handleConfigKey

Preset length is dynamic (model-aware filter), so compute it the same way as
viewConfig:

```go
case fiScheduler:
    // Count visible presets based on model.
    modelVal := m.inputs[fiModel].Value()
    isZImage := strings.Contains(strings.ToLower(modelVal), "zimage")
    isFlux2Klein := strings.Contains(strings.ToLower(modelVal), "flux2") ||
        strings.Contains(strings.ToLower(modelVal), "flux-klein") ||
        strings.Contains(strings.ToLower(modelVal), "klein")
    count := 0
    for _, p := range schedulerPresets {
        if isZImage && !p.zImageOK {
            continue
        }
        if isFlux2Klein && !p.flux2KleinOK {
            continue
        }
        count++
    }
    presetsLen = count
```

Enter handler — map activePreset to the correct scheduler value, accounting
for the filter:

```go
case fiScheduler:
    // Build the visible-scheduler slice in the same order as viewConfig.
    modelVal := m.inputs[fiModel].Value()
    isZImage := strings.Contains(strings.ToLower(modelVal), "zimage")
    isFlux2Klein := strings.Contains(strings.ToLower(modelVal), "flux2") ||
        strings.Contains(strings.ToLower(modelVal), "flux-klein") ||
        strings.Contains(strings.ToLower(modelVal), "klein")
    visible := make([]SchedulerPreset, 0, len(schedulerPresets))
    for _, p := range schedulerPresets {
        if isZImage && !p.zImageOK {
            continue
        }
        if isFlux2Klein && !p.flux2KleinOK {
            continue
        }
        visible = append(visible, p)
    }
    if m.activePreset < len(visible) {
        m.inputs[fiScheduler].SetValue(visible[m.activePreset].Scheduler)
    }
```

> **Pitfall**: If the user changes the model after selecting a scheduler that
> isn't compatible, the field still holds the old value. The API will fall back
> to `"simple"` for unrecognized schedulers, so this is safe. A future
> refinement could auto-reset the scheduler when the model changes to an
> incompatible one.

---

## 4. Summary of all changes

| File | Change |
|------|--------|
| `pkg/civit/civit.go` | Add `FluxMode`, `Scheduler`, `OutputFormat` to `GenerationRequest` |
| `internal/ui/ui.go` | Bump `numFormFields` 11→14; re-index all `fi*` constants; add 3 labels + help; add 3 preset tables; add 3 input inits; add 3 toRequest lines; add 3 viewConfig cases; add 3 handleConfigKey cases |
| `internal/ui/ui.go` | Update `isPresetsField` to cover new indices; update `isReplaceOnFocus` range |

### Verification checklist

- [ ] `numFormFields` is 14
- [ ] `fi*` constants span 0–13 with no gaps and no duplicate iota values
- [ ] `fieldLabels` has 14 entries matching index order
- [ ] `fieldHelpText` has 14 entries matching index order
- [ ] `isPresetsField` returns true for indices 2,3,4,5,6,12
- [ ] `isReplaceOnFocus` returns true for indices 7–13
- [ ] All preset tables (`fluxModePresets`, `outputFormatPresets`, `schedulerPresets`) have Name + Description + value field
- [ ] `NewModel` initializes all 14 inputs with correct defaults
- [ ] `toRequest` populates `FluxMode`, `Scheduler`, `OutputFormat`
- [ ] `viewConfig` switch has cases for `fiFluxMode`, `fiOutputFormat`, `fiScheduler`
- [ ] `handleConfigKey` presets-length switch has cases for all 3
- [ ] `handleConfigKey` enter handler has cases for all 3
- [ ] Scheduler presets filter correctly for ZImage (only simple+discrete) and Flux2Klein (all except ays)
- [ ] Compiles without unused-import errors (the new preset types need no new imports; model-aware filter uses `strings` which is already imported)
