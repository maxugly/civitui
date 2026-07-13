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

> [!WARNING]
> **DEPRECATED PRESETS (simple, discrete, karras, exponential, ays)**:
> The initial noise schedule presets (`simple`, `discrete`, `karras`, `exponential`, `ays`) are completely deprecated. The live CivitAI API rejects these values with:
> `The JSON value could not be converted to Civitai.Orchestration.Grains.Jobs.Scheduler`
> **Rationale**: The CivitAI Orchestrator API's `scheduler` parameter uses **sampler algorithm names** (e.g. `EulerA`, `DPM2`, `Heun`, etc.), rather than noise schedule names.

The actual valid Scheduler values accepted by the live API (verified via integration testing) are:

- `EulerA`
- `Euler`
- `Heun`
- `DPM2`
- `DPM2A`
- `DPM2SA`
- `DPM2M`
- `DPMSDE`
- `DPMFast`
- `DPMAdaptive`
- `LMSKarras`
- `DPM2Karras`
- `DPM2AKarras`
- `DPM2SAKarras`
- `DPM2MKarras`
- `DPMSDEKarras`
- `DDIM`
- `PLMS`
- `UniPC`
- `LCM`
- `DDPM`
- `DEIS`
- `LMS`

For the TUI, we propose a structured list of recommended presets covering standard, Karras, and fast solvers:

```go
type SchedulerPreset struct {
    Name        string
    Description string
    Scheduler   string
}

var schedulerPresets = []SchedulerPreset{
    {
        Name:        "Euler Ancestral",
        Description: "Euler Ancestral (EulerA). Fast, creative, standard default.",
        Scheduler:   "EulerA",
    },
    {
        Name:        "Euler",
        Description: "Euler. Simple, fast, and reliable.",
        Scheduler:   "Euler",
    },
    {
        Name:        "Heun",
        Description: "Heun. High accuracy, double compute per step.",
        Scheduler:   "Heun",
    },
    {
        Name:        "DPM++ 2M",
        Description: "DPM++ 2M (DPM2M). Fast with good convergence.",
        Scheduler:   "DPM2M",
    },
    {
        Name:        "DPM++ 2M Karras",
        Description: "DPM++ 2M Karras (DPM2MKarras). Highly recommended for SDXL.",
        Scheduler:   "DPM2MKarras",
    },
    {
        Name:        "LCM",
        Description: "Latent Consistency Model (LCM). Fast generation with few steps.",
        Scheduler:   "LCM",
    },
    {
        Name:        "UniPC",
        Description: "UniPC. Unified Predictor-Corrector method, fast convergence.",
        Scheduler:   "UniPC",
    },
}
```

### 3d. Input initialization

In `NewModel`, after `inputs[fiSampler]`:

```go
inputs[fiScheduler] = newTextInput("EulerA", "EulerA", inputWidth)
```

Default is `"EulerA"` — the standard default scheduler when omitted.

### 3e. toRequest

Already included in §1e.

### 3f. Right-pane rendering — viewConfig

The scheduler presets display the updated list in the right pane:

```go
case fiScheduler:
    rightLines = append(rightLines, dimStyle.Render("── Scheduler ──"))
    for _, preset := range schedulerPresets {
        presetsList = append(presetsList, struct {
            Name        string
            Description string
        }{preset.Name, preset.Description})
    }
```

### 3g. Key handling — handleConfigKey

Preset length:

```go
case fiScheduler:
    presetsLen = len(schedulerPresets)
```

Enter handler:

```go
case fiScheduler:
    m.inputs[fiScheduler].SetValue(schedulerPresets[m.activePreset].Scheduler)
```

> **Note**: Since the new valid values are independent of the model's ecosystem (unlike the deprecated noise schedules), the model-aware filtering in `handleConfigKey` and `viewConfig` can be removed or simplified. All 23 samplers are technically validated by the API, so a static preset list is clean and sufficient.

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
